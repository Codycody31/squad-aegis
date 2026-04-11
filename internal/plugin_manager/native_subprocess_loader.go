package plugin_manager

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	h.client.Kill()
}

// launchNativePluginSubprocess verifies the runtime binary's checksum and
// spawns it via hashicorp/go-plugin. On success the returned handle's rpc
// client can be used to drive the plugin; on failure the subprocess (if any)
// is killed and the error is returned.
func launchNativePluginSubprocess(runtimePath, expectedSHA256 string) (*pluginSubprocessHandle, error) {
	if err := verifyRuntimeBinaryChecksum(runtimePath, expectedSHA256); err != nil {
		return nil, err
	}

	cmd := exec.Command(runtimePath)
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
// verifies its SHA-256 against the expected value before returning. The fd
// is closed on return; the subsequent exec relies on the path-based
// integrity of the runtime directory (which must be exclusively writable by
// the Aegis process user).
func verifyRuntimeBinaryChecksum(runtimePath, expectedSHA256 string) error {
	expected := strings.TrimSpace(expectedSHA256)
	if expected == "" {
		return nil
	}
	file, err := openNoFollow(runtimePath)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("failed to hash plugin runtime binary: %w", err)
	}
	actual := fmt.Sprintf("%x", hasher.Sum(nil))
	if !strings.EqualFold(expected, actual) {
		return fmt.Errorf("plugin runtime binary checksum mismatch: expected %s, got %s", expected, actual)
	}
	return nil
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

	mu           sync.Mutex
	handle       *pluginSubprocessHandle
	hostAPISvc   *hostAPIServer
	status       PluginStatus
	onExit       func(error)
	stopWatcher  chan struct{}
	watcherDone  chan struct{}
	intentional  bool // set when Stop() is called, so the watcher knows the exit was deliberate
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
// plugin's static definition, converts it to the host type, and then kills
// the subprocess. The returned definition's CreateInstance factory spawns a
// fresh subprocess per instance via the same launcher.
func peekNativePluginDefinition(runtimePath, expectedSHA256 string) (PluginDefinition, error) {
	handle, err := nativePluginSubprocessLauncher(runtimePath, expectedSHA256)
	if err != nil {
		return PluginDefinition{}, err
	}
	defer handle.Kill()

	wire, err := handle.rpc.GetDefinition()
	if err != nil {
		return PluginDefinition{}, fmt.Errorf("failed to fetch plugin definition: %w", err)
	}
	hostDef, err := wirePluginDefinitionToHost(wire)
	if err != nil {
		return PluginDefinition{}, err
	}

	captured := PluginDefinition{}
	captured = hostDef
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

	svc, brokerID, err := startHostAPIServer(handle.rpc, apis)
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

// wirePluginDefinitionToHost converts the wire-safe PluginDefinition into the
// host's runtime type, re-marshaling the config schema through JSON so the
// host's plug_config_schema package owns the validation surface.
func wirePluginDefinitionToHost(wire pluginrpc.PluginDefinition) (PluginDefinition, error) {
	hostDef := PluginDefinition{
		ID:                     wire.ID,
		Name:                   wire.Name,
		Description:            wire.Description,
		Version:                wire.Version,
		Author:                 wire.Author,
		Source:                 PluginSource(wire.Source),
		Official:               wire.Official,
		MinHostAPIVersion:      wire.MinHostAPIVersion,
		RequiredCapabilities:   append([]string(nil), wire.RequiredCapabilities...),
		AllowMultipleInstances: wire.AllowMultipleInstances,
		RequiredConnectors:     append([]string(nil), wire.RequiredConnectors...),
		OptionalConnectors:     append([]string(nil), wire.OptionalConnectors...),
		LongRunning:            wire.LongRunning,
	}
	if hostDef.Source == "" {
		hostDef.Source = PluginSourceNative
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
