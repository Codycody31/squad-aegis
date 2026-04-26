package plugin_manager

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/rs/zerolog/log"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/config"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
	"go.codycody31.dev/squad-aegis/pkg/pluginrpc"
)

// nativePluginSubprocessLauncher is the test-injectable factory that builds
// a hashicorp/go-plugin client around a plugin binary. Production code uses
// launchNativePluginSubprocess; tests override this to avoid spawning real
// processes.
var nativePluginSubprocessLauncher = launchNativePluginSubprocess

func commandFromVerifiedRuntimeFile(verifiedFile *os.File) *exec.Cmd {
	// Execute from the verified fd to eliminate the TOCTOU window between
	// checksum verification and exec. exec.Cmd only preserves stdio plus
	// ExtraFiles, so publish the verified binary as the first extra file.
	// In the child that descriptor is always fd 3.
	cmd := exec.Command("/proc/self/fd/3")
	cmd.ExtraFiles = []*os.File{verifiedFile}
	// Restrict the subprocess environment to a minimal allowlist so host
	// credentials (DB passwords, API keys, etc.) are never leaked.
	cmd.Env = []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=/nonexistent",
		"LANG=C.UTF-8",
	}
	return cmd
}

// pluginSubprocessHandle holds the live subprocess + the RPC client stub.
// A handle must be released via Kill() when the plugin is unloaded.
type pluginSubprocessHandle struct {
	client *goplugin.Client
	rpc    *pluginrpc.PluginRPCClient
}

// Kill terminates the subprocess. Safe to call multiple times.
func (h *pluginSubprocessHandle) Kill() {
	if h == nil || h.client == nil {
		return
	}
	killProcessGroup(h.client)
	h.client.Kill()
}

// launchNativePluginSubprocess verifies the runtime binary's checksum and
// spawns it via hashicorp/go-plugin. On success the returned handle's rpc
// client can be used to drive the plugin; on failure the subprocess (if any)
// is killed and the error is returned.
func launchNativePluginSubprocess(runtimePath, expectedSHA256 string) (*pluginSubprocessHandle, error) {
	verifiedFile, err := verifyRuntimeBinaryChecksum(runtimePath, expectedSHA256)
	if err != nil {
		return nil, err
	}
	defer verifiedFile.Close()

	cmd := commandFromVerifiedRuntimeFile(verifiedFile)
	if err := applySubprocessHardening(cmd); err != nil {
		return nil, fmt.Errorf("failed to harden plugin subprocess: %w", err)
	}
	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig: pluginrpc.Handshake,
		Plugins:         pluginrpc.PluginMap(nil),
		Cmd:             cmd,
		Managed:         false,
		Logger: hclog.New(&hclog.LoggerOptions{
			Name:   "aegis-native-plugin",
			Level:  hclog.Warn,
			Output: io.Discard,
		}),
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to establish plugin rpc: %w", err)
	}

	raw, err := rpcClient.Dispense(pluginrpc.PluginName)
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to dispense plugin: %w", err)
	}

	stub, ok := raw.(*pluginrpc.PluginRPCClient)
	if !ok {
		client.Kill()
		return nil, fmt.Errorf("plugin rpc dispenser returned unexpected type %T", raw)
	}

	return &pluginSubprocessHandle{client: client, rpc: stub}, nil
}

// verifyRuntimeBinaryChecksum opens the runtime file with O_NOFOLLOW and
// verifies its SHA-256 against the expected value. On success, the verified
// file is returned with the fd still open so the caller can exec directly
// from /proc/self/fd/<fd>, eliminating the TOCTOU window between checksum
// verification and exec. The caller is responsible for closing the file.
func verifyRuntimeBinaryChecksum(runtimePath, expectedSHA256 string) (*os.File, error) {
	expected := strings.TrimSpace(expectedSHA256)
	if expected == "" {
		return nil, fmt.Errorf("refusing to launch plugin subprocess: no expected checksum configured")
	}
	file, err := openNoFollow(runtimePath)
	if err != nil {
		return nil, err
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to hash plugin runtime binary: %w", err)
	}
	actual := fmt.Sprintf("%x", hasher.Sum(nil))
	if !strings.EqualFold(expected, actual) {
		file.Close()
		return nil, fmt.Errorf("plugin runtime binary checksum mismatch: expected %s, got %s", expected, actual)
	}
	// Seek back to start so the fd is usable for exec.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to seek plugin binary: %w", err)
	}
	return file, nil
}

// subprocessPluginShim adapts an out-of-process plugin onto the host-side
// Plugin interface. Each shim owns one subprocess; the subprocess is spawned
// lazily during Initialize so that "peek" operations which only need the
// definition do not leave a process hanging around.
type subprocessPluginShim struct {
	pluginID     string
	definition   PluginDefinition
	runtimePath  string
	expectedHash string

	mu          sync.Mutex
	handle      *pluginSubprocessHandle
	hostAPISvc  *hostAPIServer
	status      PluginStatus
	onExit      func(error)
	stopWatcher chan struct{}
	watcherDone chan struct{}
	intentional bool // set when Stop() is called, so the watcher knows the exit was deliberate
}

// OnUnexpectedExit registers a callback invoked when the subprocess dies
// outside an intentional Stop() call. Used by the plugin manager to mark
// the owning PluginInstance as errored when its subprocess crashes.
func (s *subprocessPluginShim) OnUnexpectedExit(fn func(error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onExit = fn
}

// peekNativePluginDefinition starts a throwaway subprocess, fetches the
// plugin's runtime definition, merges it with the identity fields from the
// manifest, kills the peek subprocess, and returns a host PluginDefinition.
// The returned definition's CreateInstance factory spawns a fresh subprocess
// per instance via the same launcher.
//
// The manifest is the source of truth for identity (ID, name, version,
// author, license, official, min host API version, required capabilities).
// The runtime RPC is the source of truth for behavior (config schema,
// events, long running, allow multiple instances, required/optional
// connectors). Any mismatch between manifest.plugin_id and the PluginID
// echoed by the subprocess aborts the load.
func peekNativePluginDefinition(runtimePath, expectedSHA256 string, manifest PluginPackageManifest, target PluginPackageTarget) (PluginDefinition, error) {
	handle, err := nativePluginSubprocessLauncher(runtimePath, expectedSHA256)
	if err != nil {
		return PluginDefinition{}, err
	}
	defer handle.Kill()

	wire, err := handle.rpc.GetDefinition()
	if err != nil {
		return PluginDefinition{}, fmt.Errorf("failed to fetch plugin definition: %w", err)
	}

	// Guard against adversarial config schemas with unbounded nesting depth.
	const maxConfigFieldDepth = 10
	if err := validateConfigFieldDepth(wire.ConfigSchema.Fields, maxConfigFieldDepth); err != nil {
		return PluginDefinition{}, fmt.Errorf("plugin %q: %w", manifest.PluginID, err)
	}

	if wire.PluginID != manifest.PluginID {
		return PluginDefinition{}, fmt.Errorf("plugin subprocess reported id %q but manifest declares %q", wire.PluginID, manifest.PluginID)
	}

	hostDef, err := mergeWirePluginIntoHost(wire, manifest, target)
	if err != nil {
		return PluginDefinition{}, err
	}

	captured := hostDef
	captured.CreateInstance = func() Plugin {
		return &subprocessPluginShim{
			pluginID:     captured.ID,
			definition:   captured,
			runtimePath:  runtimePath,
			expectedHash: expectedSHA256,
			status:       PluginStatusStopped,
		}
	}
	return captured, nil
}

// GetDefinition returns the cached definition captured during peek.
func (s *subprocessPluginShim) GetDefinition() PluginDefinition {
	return s.definition
}

// Initialize spawns the subprocess (if not already running), wires up the
// HostAPI RPC server on a fresh broker ID, and calls the plugin's Initialize
// RPC. If any step fails the subprocess is killed and the error is returned.
func (s *subprocessPluginShim) Initialize(config map[string]interface{}, apis *PluginAPIs) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.handle != nil {
		return errors.New("plugin subprocess already initialized")
	}

	handle, err := nativePluginSubprocessLauncher(s.runtimePath, s.expectedHash)
	if err != nil {
		return fmt.Errorf("failed to spawn plugin subprocess: %w", err)
	}

	svc, brokerID, err := startHostAPIServer(handle.rpc, apis, s.pluginID)
	if err != nil {
		handle.Kill()
		return fmt.Errorf("failed to start host api server: %w", err)
	}

	serverID := ""
	if apis != nil && apis.ServerAPI != nil {
		serverID = apis.ServerAPI.GetServerID().String()
	}
	initErr := handle.rpc.Initialize(pluginrpc.InitializeArgs{
		Config:          config,
		HostAPIBrokerID: brokerID,
		ServerID:        serverID,
	})
	if initErr != nil {
		svc.Close()
		handle.Kill()
		return initErr
	}

	s.handle = handle
	s.hostAPISvc = svc
	s.intentional = false
	s.stopWatcher = make(chan struct{})
	s.watcherDone = make(chan struct{})
	go s.watchHealth(handle, s.stopWatcher, s.watcherDone)
	return nil
}

// watchHealth polls goplugin.Client.Exited() at the configured interval and
// invokes the onExit callback if the subprocess dies unexpectedly. Stops
// when stopCh is closed (during intentional Stop) or when an exit is
// observed (whichever comes first).
func (s *subprocessPluginShim) watchHealth(handle *pluginSubprocessHandle, stopCh, doneCh chan struct{}) {
	defer close(doneCh)
	interval := healthCheckInterval()
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			if handle == nil || handle.client == nil {
				return
			}
			if handle.client.Exited() {
				s.mu.Lock()
				cb := s.onExit
				intentional := s.intentional
				s.status = PluginStatusError
				s.mu.Unlock()
				if intentional {
					return
				}
				// Re-check under lock so a concurrent Stop() that flipped
				// intentional between our first read and now is observed
				// before we fire the exit callback. Without this, cb() can
				// block on pm.mu while Stop() blocks on watcherDone.
				s.mu.Lock()
				intentional = s.intentional
				s.mu.Unlock()
				if intentional {
					return
				}
				if cb != nil {
					cb(fmt.Errorf("plugin subprocess %s exited unexpectedly", s.pluginID))
				} else {
					log.Warn().Str("plugin_id", s.pluginID).Msg("Plugin subprocess exited unexpectedly (no exit callback registered)")
				}
				return
			}
		}
	}
}

// healthCheckInterval reads the configured interval from config.Config and
// clamps sub-second values up to one second to prevent accidental tight
// loops.
func healthCheckInterval() time.Duration {
	seconds := 10
	if config.Config != nil {
		seconds = config.Config.Plugins.HealthCheckIntervalSeconds
	}
	if seconds <= 0 {
		return 0
	}
	if seconds < 1 {
		seconds = 1
	}
	return time.Duration(seconds) * time.Second
}

// Start calls the plugin's Start RPC. The local ctx is NOT forwarded to the
// subprocess; cancellation is expressed through Stop instead.
func (s *subprocessPluginShim) Start(ctx context.Context) error {
	_ = ctx
	s.mu.Lock()
	handle := s.handle
	s.mu.Unlock()
	if handle == nil {
		return errors.New("plugin subprocess is not initialized")
	}
	if err := handle.rpc.Start(); err != nil {
		s.mu.Lock()
		s.status = PluginStatusError
		s.mu.Unlock()
		return err
	}
	s.mu.Lock()
	s.status = PluginStatusRunning
	s.mu.Unlock()
	return nil
}

// Stop calls the plugin's Stop RPC, closes the HostAPI listener, and kills
// the subprocess. Safe to call multiple times. The health watcher is also
// signalled to exit so it does not fire onExit for a deliberate shutdown.
func (s *subprocessPluginShim) Stop() error {
	s.mu.Lock()
	handle := s.handle
	svc := s.hostAPISvc
	stopWatcher := s.stopWatcher
	watcherDone := s.watcherDone
	s.handle = nil
	s.hostAPISvc = nil
	s.stopWatcher = nil
	s.watcherDone = nil
	s.intentional = true
	s.status = PluginStatusStopped
	s.mu.Unlock()

	// Signal the watcher first so it observes the intentional flag.
	if stopWatcher != nil {
		select {
		case <-stopWatcher:
		default:
			close(stopWatcher)
		}
	}

	var stopErr error
	if handle != nil && handle.rpc != nil {
		stopErr = handle.rpc.Stop()
	}
	if svc != nil {
		svc.Close()
	}
	if handle != nil {
		handle.Kill()
	}
	// Wait for watcher goroutine to finish so we don't race with a late
	// callback after Stop returns.
	if watcherDone != nil {
		<-watcherDone
	}
	return stopErr
}

// HandleEvent forwards an event to the subprocess via RPC.
func (s *subprocessPluginShim) HandleEvent(event *PluginEvent) error {
	if event == nil {
		return nil
	}
	s.mu.Lock()
	handle := s.handle
	s.mu.Unlock()
	if handle == nil {
		return errors.New("plugin subprocess is not initialized")
	}
	wireEvent, err := hostPluginEventToWire(event)
	if err != nil {
		return err
	}
	return handle.rpc.HandleEvent(wireEvent)
}

// GetStatus returns the cached status.
func (s *subprocessPluginShim) GetStatus() PluginStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// GetConfig fetches the plugin's current config over RPC.
func (s *subprocessPluginShim) GetConfig() map[string]interface{} {
	s.mu.Lock()
	handle := s.handle
	s.mu.Unlock()
	if handle == nil {
		return map[string]interface{}{}
	}
	cfg, err := handle.rpc.GetConfig()
	if err != nil {
		log.Warn().Err(err).Str("plugin_id", s.pluginID).Msg("Failed to fetch plugin config via subprocess RPC")
		return map[string]interface{}{}
	}
	return cfg
}

// UpdateConfig pushes a new config to the plugin.
func (s *subprocessPluginShim) UpdateConfig(config map[string]interface{}) error {
	s.mu.Lock()
	handle := s.handle
	s.mu.Unlock()
	if handle == nil {
		return errors.New("plugin subprocess is not initialized")
	}
	return handle.rpc.UpdateConfig(config)
}

// GetCommands fetches the plugin's command list.
func (s *subprocessPluginShim) GetCommands() []PluginCommand {
	s.mu.Lock()
	handle := s.handle
	s.mu.Unlock()
	if handle == nil {
		return nil
	}
	wire, err := handle.rpc.GetCommands()
	if err != nil {
		log.Warn().Err(err).Str("plugin_id", s.pluginID).Msg("Failed to fetch plugin commands via subprocess RPC")
		return nil
	}
	commands := make([]PluginCommand, 0, len(wire))
	for _, cmd := range wire {
		converted, err := wirePluginCommandToHost(cmd)
		if err != nil {
			log.Warn().Err(err).Str("plugin_id", s.pluginID).Str("command_id", cmd.ID).Msg("Skipping malformed plugin command")
			continue
		}
		commands = append(commands, converted)
	}
	return commands
}

// ExecuteCommand forwards a command invocation to the plugin.
func (s *subprocessPluginShim) ExecuteCommand(commandID string, params map[string]interface{}) (*CommandResult, error) {
	s.mu.Lock()
	handle := s.handle
	s.mu.Unlock()
	if handle == nil {
		return nil, errors.New("plugin subprocess is not initialized")
	}
	wire, err := handle.rpc.ExecuteCommand(commandID, params)
	if err != nil {
		return nil, err
	}
	if wire == nil {
		return nil, nil
	}
	return &CommandResult{
		Success:     wire.Success,
		Message:     wire.Message,
		Data:        wire.Data,
		ExecutionID: wire.ExecutionID,
		Error:       wire.Error,
	}, nil
}

// GetCommandExecutionStatus fetches the status of an async command.
func (s *subprocessPluginShim) GetCommandExecutionStatus(executionID string) (*CommandExecutionStatus, error) {
	s.mu.Lock()
	handle := s.handle
	s.mu.Unlock()
	if handle == nil {
		return nil, errors.New("plugin subprocess is not initialized")
	}
	wire, err := handle.rpc.GetCommandExecutionStatus(executionID)
	if err != nil {
		return nil, err
	}
	if wire == nil {
		return nil, nil
	}
	status := &CommandExecutionStatus{
		ExecutionID: wire.ExecutionID,
		CommandID:   wire.CommandID,
		Status:      wire.Status,
		Progress:    wire.Progress,
		Message:     wire.Message,
		StartedAt:   wire.StartedAt,
		CompletedAt: wire.CompletedAt,
	}
	if wire.Result != nil {
		status.Result = &CommandResult{
			Success:     wire.Result.Success,
			Message:     wire.Result.Message,
			Data:        wire.Result.Data,
			ExecutionID: wire.Result.ExecutionID,
			Error:       wire.Result.Error,
		}
	}
	return status, nil
}

// mergeWirePluginIntoHost assembles a host PluginDefinition by merging
// identity/compatibility fields from the signed manifest with the runtime
// behavioral fields returned by the subprocess over RPC. It re-marshals the
// config schema through JSON so the host's plug_config_schema package owns
// the validation surface.
func mergeWirePluginIntoHost(wire pluginrpc.PluginDefinition, manifest PluginPackageManifest, target PluginPackageTarget) (PluginDefinition, error) {
	hostDef := PluginDefinition{
		// Identity comes from the manifest.
		ID:          manifest.PluginID,
		Name:        manifest.Name,
		Description: manifest.Description,
		Version:     manifest.Version,
		Authors:     manifest.Authors,
		Source:      PluginSourceNative,
		Official:    manifest.Official,

		// Compatibility comes from the selected target.
		MinHostAPIVersion:    target.MinHostAPIVersion,
		RequiredCapabilities: cloneRequiredCapabilities(target.RequiredCapabilities),
		TargetOS:             target.TargetOS,
		TargetArch:           target.TargetArch,

		// Behavior comes from the subprocess runtime definition.
		AllowMultipleInstances: wire.AllowMultipleInstances,
		RequiredConnectors:     append([]string(nil), wire.RequiredConnectors...),
		OptionalConnectors:     append([]string(nil), wire.OptionalConnectors...),
		LongRunning:            wire.LongRunning,
	}

	schemaJSON, err := json.Marshal(wire.ConfigSchema)
	if err != nil {
		return PluginDefinition{}, fmt.Errorf("failed to marshal plugin config schema: %w", err)
	}
	var schema plug_config_schema.ConfigSchema
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		return PluginDefinition{}, fmt.Errorf("failed to parse plugin config schema: %w", err)
	}
	hostDef.ConfigSchema = schema

	hostDef.Events = make([]event_manager.EventType, 0, len(wire.Events))
	for _, ev := range wire.Events {
		hostDef.Events = append(hostDef.Events, event_manager.EventType(ev))
	}
	return hostDef, nil
}

// hostPluginEventToWire converts a host PluginEvent into its wire shape.
// The Data field is JSON-encoded so the plugin can unmarshal into its own
// event type without importing the host event package.
func hostPluginEventToWire(event *PluginEvent) (pluginrpc.PluginEvent, error) {
	wire := pluginrpc.PluginEvent{
		ID:        event.ID.String(),
		ServerID:  event.ServerID.String(),
		Source:    pluginrpc.EventSource(event.Source),
		Type:      event.Type,
		Raw:       event.Raw,
		Timestamp: event.Timestamp,
	}
	if event.Data != nil {
		payload, err := json.Marshal(event.Data)
		if err != nil {
			return pluginrpc.PluginEvent{}, fmt.Errorf("failed to marshal plugin event data: %w", err)
		}
		wire.Data = payload
	}
	return wire, nil
}

// validateConfigFieldDepth walks the config schema tree and returns an error
// if any branch exceeds maxDepth levels of nesting. This prevents a malicious
// subprocess from sending a deeply-recursive schema that could blow the host
// stack during JSON re-marshal or validation.
func validateConfigFieldDepth(fields []pluginrpc.ConfigField, maxDepth int) error {
	if maxDepth <= 0 {
		return fmt.Errorf("config schema exceeds maximum nesting depth")
	}
	for _, f := range fields {
		if len(f.Nested) > 0 {
			if err := validateConfigFieldDepth(f.Nested, maxDepth-1); err != nil {
				return err
			}
		}
	}
	return nil
}

// wirePluginCommandToHost converts a wire PluginCommand into the host type.
func wirePluginCommandToHost(wire pluginrpc.PluginCommand) (PluginCommand, error) {
	paramsJSON, err := json.Marshal(wire.Parameters)
	if err != nil {
		return PluginCommand{}, fmt.Errorf("failed to marshal plugin command parameters: %w", err)
	}
	var params plug_config_schema.ConfigSchema
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return PluginCommand{}, fmt.Errorf("failed to parse plugin command parameters: %w", err)
	}
	exec := CommandExecutionSync
	if wire.ExecutionType == pluginrpc.CommandExecutionAsync {
		exec = CommandExecutionAsync
	}
	return PluginCommand{
		ID:                  wire.ID,
		Name:                wire.Name,
		Description:         wire.Description,
		Category:            wire.Category,
		Parameters:          params,
		ExecutionType:       exec,
		RequiredPermissions: append([]string(nil), wire.RequiredPermissions...),
		ConfirmMessage:      wire.ConfirmMessage,
	}, nil
}
