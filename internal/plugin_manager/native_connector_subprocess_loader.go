package plugin_manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/rs/zerolog/log"

	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
	"go.codycody31.dev/squad-aegis/pkg/connectorrpc"
)

// nativeConnectorSubprocessLauncher is the test-injectable factory that
// builds a hashicorp/go-plugin client around a connector binary.
var nativeConnectorSubprocessLauncher = launchNativeConnectorSubprocess

// connectorSubprocessHandle holds the live subprocess + the gRPC stub.
type connectorSubprocessHandle struct {
	client *goplugin.Client
	rpc    *connectorrpc.ConnectorGRPCClient
}

// Kill terminates the subprocess. Safe to call multiple times.
func (h *connectorSubprocessHandle) Kill() {
	if h == nil || h.client == nil {
		return
	}
	killProcessGroup(h.client)
	h.client.Kill()
}

// launchNativeConnectorSubprocess verifies the checksum and spawns the
// connector subprocess.
func launchNativeConnectorSubprocess(runtimePath, expectedSHA256 string) (*connectorSubprocessHandle, error) {
	verifiedFile, err := verifyRuntimeBinaryChecksum(runtimePath, expectedSHA256)
	if err != nil {
		return nil, err
	}
	defer verifiedFile.Close()

	cmd := commandFromVerifiedRuntimeFile(verifiedFile)
	if err := applySubprocessHardening(cmd); err != nil {
		return nil, fmt.Errorf("failed to harden connector subprocess: %w", err)
	}
	client := goplugin.NewClient(nativeConnectorClientConfig(cmd))

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to establish connector rpc: %w", err)
	}

	raw, err := rpcClient.Dispense(connectorrpc.PluginName)
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to dispense connector: %w", err)
	}

	stub, ok := raw.(*connectorrpc.ConnectorGRPCClient)
	if !ok {
		client.Kill()
		return nil, fmt.Errorf("connector rpc dispenser returned unexpected type %T", raw)
	}

	return &connectorSubprocessHandle{client: client, rpc: stub}, nil
}

func nativeConnectorClientConfig(cmd *exec.Cmd) *goplugin.ClientConfig {
	return &goplugin.ClientConfig{
		HandshakeConfig:  connectorrpc.Handshake,
		Plugins:          connectorrpc.PluginMap(nil),
		Cmd:              cmd,
		Managed:          false,
		SkipHostEnv:      true,
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		AutoMTLS:         true,
		Logger: hclog.New(&hclog.LoggerOptions{
			Name:   "aegis-native-connector",
			Level:  hclog.Warn,
			Output: io.Discard,
		}),
	}
}

// subprocessConnectorShim adapts an out-of-process connector onto the
// host-side Connector interface. Each shim owns one subprocess; the
// subprocess is spawned lazily during Initialize.
type subprocessConnectorShim struct {
	connectorID  string
	definition   ConnectorDefinition
	runtimePath  string
	expectedHash string

	mu          sync.Mutex
	handle      *connectorSubprocessHandle
	status      ConnectorStatus
	onExit      func(error)
	stopWatcher chan struct{}
	watcherDone chan struct{}
	intentional bool
}

// OnUnexpectedExit registers a callback invoked when the connector
// subprocess dies outside an intentional Stop() call.
func (s *subprocessConnectorShim) OnUnexpectedExit(fn func(error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onExit = fn
}

// GetDefinition returns the cached definition.
func (s *subprocessConnectorShim) GetDefinition() ConnectorDefinition {
	return s.definition
}

// Initialize spawns the subprocess (if not already running) and runs the
// connector's Initialize RPC.
func (s *subprocessConnectorShim) Initialize(config map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.handle != nil {
		return errors.New("connector subprocess already initialized")
	}

	handle, err := nativeConnectorSubprocessLauncher(s.runtimePath, s.expectedHash)
	if err != nil {
		return fmt.Errorf("failed to spawn connector subprocess: %w", err)
	}
	if err := handle.rpc.Initialize(connectorrpc.InitializeArgs{Config: config}); err != nil {
		handle.Kill()
		return err
	}
	s.handle = handle
	s.intentional = false
	s.stopWatcher = make(chan struct{})
	s.watcherDone = make(chan struct{})
	go s.watchHealth(handle, s.stopWatcher, s.watcherDone)
	return nil
}

// watchHealth polls goplugin.Client.Exited() and fires onExit if the
// connector subprocess dies unexpectedly.
func (s *subprocessConnectorShim) watchHealth(handle *connectorSubprocessHandle, stopCh, doneCh chan struct{}) {
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
				s.status = ConnectorStatusError
				s.mu.Unlock()
				if intentional {
					return
				}
				// Re-check under lock so a concurrent Stop() that flipped
				// intentional between our first read and now is observed
				// before we fire the exit callback. Without this, cb() can
				// block on pm.connectorMu while Stop() blocks on watcherDone.
				s.mu.Lock()
				intentional = s.intentional
				s.mu.Unlock()
				if intentional {
					return
				}
				if cb != nil {
					cb(fmt.Errorf("connector subprocess %s exited unexpectedly", s.connectorID))
				} else {
					log.Warn().Str("connector_id", s.connectorID).Msg("Connector subprocess exited unexpectedly (no exit callback registered)")
				}
				return
			}
		}
	}
}

// Start runs the connector's Start RPC.
func (s *subprocessConnectorShim) Start(ctx context.Context) error {
	_ = ctx
	s.mu.Lock()
	handle := s.handle
	s.mu.Unlock()
	if handle == nil {
		return errors.New("connector subprocess is not initialized")
	}
	if err := handle.rpc.Start(); err != nil {
		s.mu.Lock()
		s.status = ConnectorStatusError
		s.mu.Unlock()
		return err
	}
	s.mu.Lock()
	s.status = ConnectorStatusRunning
	s.mu.Unlock()
	return nil
}

// Stop runs Stop RPC, kills the subprocess, and flips the status. Signals
// the health watcher to exit cleanly so a deliberate shutdown does not
// trigger the unexpected-exit callback.
func (s *subprocessConnectorShim) Stop() error {
	s.mu.Lock()
	handle := s.handle
	stopWatcher := s.stopWatcher
	watcherDone := s.watcherDone
	s.handle = nil
	s.stopWatcher = nil
	s.watcherDone = nil
	s.intentional = true
	s.status = ConnectorStatusStopped
	s.mu.Unlock()

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
	if handle != nil {
		handle.Kill()
	}
	if watcherDone != nil {
		<-watcherDone
	}
	return stopErr
}

// GetStatus returns the cached status.
func (s *subprocessConnectorShim) GetStatus() ConnectorStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// GetConfig fetches the current config from the subprocess.
func (s *subprocessConnectorShim) GetConfig() map[string]interface{} {
	s.mu.Lock()
	handle := s.handle
	s.mu.Unlock()
	if handle == nil {
		return map[string]interface{}{}
	}
	cfg, err := handle.rpc.GetConfig()
	if err != nil {
		log.Warn().Err(err).Str("connector_id", s.connectorID).Msg("Failed to fetch connector config via subprocess RPC")
		return map[string]interface{}{}
	}
	return cfg
}

// UpdateConfig pushes a new config to the subprocess.
func (s *subprocessConnectorShim) UpdateConfig(config map[string]interface{}) error {
	s.mu.Lock()
	handle := s.handle
	s.mu.Unlock()
	if handle == nil {
		return errors.New("connector subprocess is not initialized")
	}
	return handle.rpc.UpdateConfig(config)
}

// GetAPI is unsupported for subprocess connectors; plugins should use the
// JSON Invoke surface via ConnectorAPI.Call instead.
func (s *subprocessConnectorShim) GetAPI() interface{} {
	return nil
}

// Invoke forwards a JSON invocation to the subprocess, carrying the context
// deadline as a millisecond timeout on the wire.
func (s *subprocessConnectorShim) Invoke(ctx context.Context, req *ConnectorInvokeRequest) (*ConnectorInvokeResponse, error) {
	s.mu.Lock()
	handle := s.handle
	s.mu.Unlock()
	if handle == nil {
		return nil, errors.New("connector subprocess is not initialized")
	}

	wireReq := connectorrpc.ConnectorInvokeRequest{}
	if req != nil {
		wireReq.V = req.V
		wireReq.Data = req.Data
	}
	var timeoutMs int64
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, context.DeadlineExceeded
		}
		timeoutMs = remaining.Milliseconds()
	}
	resp, err := handle.rpc.Invoke(wireReq, timeoutMs)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}
	return &ConnectorInvokeResponse{
		V:     resp.V,
		OK:    resp.OK,
		Data:  resp.Data,
		Error: resp.Error,
	}, nil
}

// mergeWireConnectorIntoHost assembles a host ConnectorDefinition by
// combining identity/compatibility from the signed manifest with the
// runtime config schema from the subprocess. LegacyIDs and InstanceKey come
// from the manifest because they are identity-level migration/routing
// helpers, not runtime behavior.
func mergeWireConnectorIntoHost(wire connectorrpc.ConnectorDefinition, manifest ConnectorPackageManifest, target PluginPackageTarget) (ConnectorDefinition, error) {
	hostDef := ConnectorDefinition{
		// Identity comes from the manifest.
		ID:          manifest.ConnectorID,
		LegacyIDs:   append([]string(nil), manifest.LegacyIDs...),
		InstanceKey: manifest.InstanceKey,
		Source:      PluginSourceNative,
		Name:        manifest.Name,
		Description: manifest.Description,
		Version:     manifest.Version,
		Authors:     manifest.Authors,

		// Compatibility comes from the selected target.
		MinHostAPIVersion:    target.MinHostAPIVersion,
		RequiredCapabilities: cloneRequiredCapabilities(target.RequiredCapabilities),
		TargetOS:             target.TargetOS,
		TargetArch:           target.TargetArch,
	}

	schemaJSON, err := json.Marshal(wire.ConfigSchema)
	if err != nil {
		return ConnectorDefinition{}, fmt.Errorf("failed to marshal connector config schema: %w", err)
	}
	var schema plug_config_schema.ConfigSchema
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		return ConnectorDefinition{}, fmt.Errorf("failed to parse connector config schema: %w", err)
	}
	hostDef.ConfigSchema = schema
	return hostDef, nil
}
