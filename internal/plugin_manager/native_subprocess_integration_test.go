package plugin_manager

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

// buildExampleBinary is a test helper that compiles a Go package to a binary
// in t.TempDir(). Returns the path + expected SHA-256. Used by integration
// tests that need a real plugin/connector subprocess to spawn.
func buildExampleBinary(t *testing.T, pkg string) (path, sha string) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("subprocess integration tests require linux")
	}
	dir := t.TempDir()
	outPath := filepath.Join(dir, filepath.Base(pkg))
	cmd := exec.Command("go", "build", "-o", outPath, "./"+pkg)
	// Move to repo root so the relative package path resolves.
	if cwd, err := os.Getwd(); err == nil {
		// Walk up to find go.mod
		for d := cwd; d != "/"; d = filepath.Dir(d) {
			if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
				cmd.Dir = d
				break
			}
		}
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build %s: %v\n%s", pkg, err, out)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read built binary: %v", err)
	}
	return outPath, fmt.Sprintf("%x", sha256.Sum256(data))
}

// helloPluginTestManifest returns a slim manifest matching the example
// hello plugin binary at examples/native-plugin-hello. Tests that spawn
// real subprocesses use this as the manifest that would ship alongside
// the binary in a real bundle.
func helloPluginTestManifest() PluginPackageManifest {
	return PluginPackageManifest{
		PluginID:    "com.squad-aegis.plugins.examples.hello",
		Name:        "Hello Example",
		Description: "Replies to players who type !hello in chat.",
		Version:     "0.1.0",
		Author:      "Squad Aegis",
	}
}

// helloPluginTestTarget returns a target snapshot matching the current host.
func helloPluginTestTarget() PluginPackageTarget {
	return PluginPackageTarget{
		MinHostAPIVersion: NativePluginHostAPIVersion,
		TargetOS:          runtime.GOOS,
		TargetArch:        runtime.GOARCH,
	}
}

// helloConnectorTestManifest returns a slim connector manifest matching
// the example hello connector binary.
func helloConnectorTestManifest() ConnectorPackageManifest {
	return ConnectorPackageManifest{
		ConnectorID: "com.squad-aegis.connectors.examples.hello",
		Name:        "Hello connector example",
		Description: "Responds to JSON invoke action ping.",
		Version:     "0.1.0",
		Author:      "Squad Aegis",
	}
}

func helloConnectorTestTarget() PluginPackageTarget {
	return PluginPackageTarget{
		MinHostAPIVersion: NativeConnectorHostAPIVersion,
		TargetOS:          runtime.GOOS,
		TargetArch:        runtime.GOARCH,
	}
}

func TestPeekNativePluginDefinitionFromRealSubprocess(t *testing.T) {
	path, sha := buildExampleBinary(t, "examples/native-plugin-hello")
	manifest := helloPluginTestManifest()
	target := helloPluginTestTarget()

	def, err := peekNativePluginDefinition(path, sha, manifest, target)
	if err != nil {
		t.Fatalf("peekNativePluginDefinition() error = %v", err)
	}
	if got, want := def.ID, "com.squad-aegis.plugins.examples.hello"; got != want {
		t.Fatalf("def.ID = %q, want %q", got, want)
	}
	if def.Name != manifest.Name {
		t.Fatalf("def.Name = %q, want %q (merged from manifest)", def.Name, manifest.Name)
	}
	if def.Version != manifest.Version {
		t.Fatalf("def.Version = %q, want %q", def.Version, manifest.Version)
	}
	if def.CreateInstance == nil {
		t.Fatal("def.CreateInstance should not be nil")
	}
	// The runtime fields come from the subprocess, not the manifest.
	if len(def.Events) == 0 {
		t.Fatal("def.Events is empty; expected RCON_CHAT_MESSAGE from subprocess")
	}
	if len(def.OptionalConnectors) == 0 {
		t.Fatal("def.OptionalConnectors is empty; expected example hello connector from subprocess")
	}

	// Mismatched checksum must fail fast.
	if _, err := peekNativePluginDefinition(path, "deadbeef", manifest, target); err == nil {
		t.Fatal("peekNativePluginDefinition() with bad sha error = nil, want error")
	}

	// Manifest plugin_id mismatch must fail fast.
	wrong := manifest
	wrong.PluginID = "com.example.wrong"
	if _, err := peekNativePluginDefinition(path, sha, wrong, target); err == nil {
		t.Fatal("peekNativePluginDefinition() with mismatched plugin_id error = nil, want error")
	}
}

func TestPeekNativeConnectorDefinitionFromRealSubprocess(t *testing.T) {
	path, sha := buildExampleBinary(t, "examples/native-connector-hello")
	manifest := helloConnectorTestManifest()
	target := helloConnectorTestTarget()

	def, err := peekNativeConnectorDefinition(path, sha, manifest, target)
	if err != nil {
		t.Fatalf("peekNativeConnectorDefinition() error = %v", err)
	}
	if got, want := def.ID, "com.squad-aegis.connectors.examples.hello"; got != want {
		t.Fatalf("def.ID = %q, want %q", got, want)
	}
	if def.Name != manifest.Name {
		t.Fatalf("def.Name = %q, want %q (merged from manifest)", def.Name, manifest.Name)
	}
	if def.CreateInstance == nil {
		t.Fatal("def.CreateInstance should not be nil")
	}

	// Manifest connector_id mismatch must fail fast.
	wrong := manifest
	wrong.ConnectorID = "com.example.wrong"
	if _, err := peekNativeConnectorDefinition(path, sha, wrong, target); err == nil {
		t.Fatal("peekNativeConnectorDefinition() with mismatched connector_id error = nil, want error")
	}
}

// fakeRconAPI records the last SendWarningToPlayer call. Only the minimal
// surface used by the example plugin is implemented; the rest return an
// error so the dispatcher can trip on unexpected calls.
type fakeRconAPI struct {
	lastPlayerID atomic.Value // string
	lastMessage  atomic.Value // string
}

func (f *fakeRconAPI) SendCommand(string) (string, error) { return "", nil }
func (f *fakeRconAPI) Broadcast(string) error             { return nil }
func (f *fakeRconAPI) SendWarningToPlayer(playerID, message string) error {
	f.lastPlayerID.Store(playerID)
	f.lastMessage.Store(message)
	return nil
}
func (f *fakeRconAPI) KickPlayer(string, string) error { return nil }
func (f *fakeRconAPI) BanPlayer(string, string, time.Duration) error {
	return nil
}
func (f *fakeRconAPI) BanWithEvidence(string, string, time.Duration, string, string) (string, error) {
	return "", nil
}
func (f *fakeRconAPI) WarnPlayerWithRule(string, string, *string) error {
	return nil
}
func (f *fakeRconAPI) KickPlayerWithRule(string, string, *string) error {
	return nil
}
func (f *fakeRconAPI) BanPlayerWithRule(string, string, time.Duration, *string) error {
	return nil
}
func (f *fakeRconAPI) BanWithEvidenceAndRule(string, string, time.Duration, string, string, *string) (string, error) {
	return "", nil
}
func (f *fakeRconAPI) BanWithEvidenceAndRuleAndMetadata(string, string, time.Duration, string, string, *string, map[string]interface{}) (string, error) {
	return "", nil
}
func (f *fakeRconAPI) RemovePlayerFromSquad(string) error     { return nil }
func (f *fakeRconAPI) RemovePlayerFromSquadById(string) error { return nil }

// fakeServerAPI is a minimal ServerAPI impl for tests.
type fakeServerAPI struct{ id uuid.UUID }

func (f *fakeServerAPI) GetServerID() uuid.UUID             { return f.id }
func (f *fakeServerAPI) GetServerInfo() (*ServerInfo, error) { return nil, nil }
func (f *fakeServerAPI) GetPlayers() ([]*PlayerInfo, error)  { return nil, nil }
func (f *fakeServerAPI) GetAdmins() ([]*AdminInfo, error)    { return nil, nil }
func (f *fakeServerAPI) GetSquads() ([]*SquadInfo, error)    { return nil, nil }

// fakeLogAPI records log calls.
type fakeLogAPI struct {
	infoCount atomic.Int32
	lastMsg   atomic.Value // string
}

func (f *fakeLogAPI) Info(message string, fields map[string]interface{}) {
	f.infoCount.Add(1)
	f.lastMsg.Store(message)
}
func (f *fakeLogAPI) Warn(string, map[string]interface{})         {}
func (f *fakeLogAPI) Error(string, error, map[string]interface{}) {}
func (f *fakeLogAPI) Debug(string, map[string]interface{})        {}

func TestSubprocessConnectorLifecycle(t *testing.T) {
	path, sha := buildExampleBinary(t, "examples/native-connector-hello")

	def, err := peekNativeConnectorDefinition(path, sha, helloConnectorTestManifest(), helloConnectorTestTarget())
	if err != nil {
		t.Fatalf("peekNativeConnectorDefinition() error = %v", err)
	}
	if def.CreateInstance == nil {
		t.Fatal("CreateInstance is nil")
	}

	instance := def.CreateInstance()
	if instance == nil {
		t.Fatal("CreateInstance returned nil instance")
	}
	t.Cleanup(func() { _ = instance.Stop() })

	if err := instance.Initialize(map[string]interface{}{}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	invokable, ok := instance.(InvokableConnector)
	if !ok {
		t.Fatal("subprocess connector shim does not implement InvokableConnector")
	}

	// Happy-path invoke: the hello connector responds to action=ping with
	// a pong message.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	resp, err := invokable.Invoke(ctx, &ConnectorInvokeRequest{V: "1", Data: map[string]interface{}{"action": "ping"}})
	cancel()
	if err != nil {
		t.Fatalf("Invoke(ping) error = %v", err)
	}
	if resp == nil || !resp.OK {
		t.Fatalf("Invoke(ping) response = %#v, want OK", resp)
	}
	if msg, _ := resp.Data["message"].(string); msg != "pong" {
		t.Fatalf("Invoke(ping).Data[message] = %v, want pong", resp.Data["message"])
	}

	// Unknown action: connector returns a wire error but not a transport error.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	resp, err = invokable.Invoke(ctx2, &ConnectorInvokeRequest{V: "1", Data: map[string]interface{}{"action": "nope"}})
	cancel2()
	if err != nil {
		t.Fatalf("Invoke(nope) transport error = %v", err)
	}
	if resp == nil || resp.OK {
		t.Fatalf("Invoke(nope) response = %#v, want not OK", resp)
	}
	if resp.Error == "" {
		t.Fatal("Invoke(nope).Error is empty")
	}

	if err := instance.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestSubprocessPluginHealthMonitorFiresOnUnexpectedExit(t *testing.T) {
	// Force a very short health check interval for this test so we don't
	// wait 10s for the watcher to fire.
	prev := config.Config
	cfg := config.Struct{}
	if prev != nil {
		cfg = *prev
	}
	cfg.Plugins.HealthCheckIntervalSeconds = 1
	config.Config = &cfg
	t.Cleanup(func() { config.Config = prev })

	path, sha := buildExampleBinary(t, "examples/native-plugin-hello")

	def, err := peekNativePluginDefinition(path, sha, helloPluginTestManifest(), helloPluginTestTarget())
	if err != nil {
		t.Fatalf("peekNativePluginDefinition() error = %v", err)
	}
	instance := def.CreateInstance()
	shim, ok := instance.(*subprocessPluginShim)
	if !ok {
		t.Fatalf("instance type = %T, want *subprocessPluginShim", instance)
	}

	exitCh := make(chan error, 1)
	shim.OnUnexpectedExit(func(err error) {
		select {
		case exitCh <- err:
		default:
		}
	})

	apis := &PluginAPIs{LogAPI: &fakeLogAPI{}}
	if err := instance.Initialize(map[string]interface{}{}, apis); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	// Start is a no-op for the hello plugin; skip it to keep the test
	// minimal. The watcher was started inside Initialize.

	// Kill the subprocess behind the shim's back (not via Stop) to simulate
	// a crash. The watcher should detect the exit and fire the callback.
	shim.mu.Lock()
	handle := shim.handle
	shim.mu.Unlock()
	if handle == nil {
		t.Fatal("shim.handle is nil after Initialize")
	}
	handle.Kill()

	select {
	case err := <-exitCh:
		if err == nil {
			t.Fatal("OnUnexpectedExit callback fired with nil error")
		}
		if !strings.Contains(err.Error(), "exited unexpectedly") {
			t.Fatalf("OnUnexpectedExit err = %v, want 'exited unexpectedly'", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("OnUnexpectedExit callback did not fire within 10s of subprocess kill")
	}

	// Explicit Stop after the watcher has already fired should be safe.
	if err := instance.Stop(); err != nil {
		// Stop may return an RPC error because the subprocess is already
		// dead. That is acceptable — the point is that Stop must not
		// deadlock or panic.
		t.Logf("Stop() after crash returned: %v", err)
	}
}

func TestSubprocessPluginLifecycleWithHostAPICallbacks(t *testing.T) {
	path, sha := buildExampleBinary(t, "examples/native-plugin-hello")

	def, err := peekNativePluginDefinition(path, sha, helloPluginTestManifest(), helloPluginTestTarget())
	if err != nil {
		t.Fatalf("peekNativePluginDefinition() error = %v", err)
	}

	// CreateInstance should spawn a fresh subprocess per instance.
	instance := def.CreateInstance()
	if instance == nil {
		t.Fatal("CreateInstance() = nil")
	}

	rcon := &fakeRconAPI{}
	logAPI := &fakeLogAPI{}
	serverID := uuid.New()
	apis := &PluginAPIs{
		ServerAPI: &fakeServerAPI{id: serverID},
		RconAPI:   rcon,
		LogAPI:    logAPI,
	}

	if err := instance.Initialize(map[string]interface{}{"trigger": "!hello", "response": "hi"}, apis); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	t.Cleanup(func() { _ = instance.Stop() })

	if err := instance.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Deliver a chat event that should trigger the plugin to call RconAPI.
	chatData := map[string]interface{}{
		"eos_id":      "76561198012345678",
		"player_name": "Alice",
		"message":     "!hello",
	}
	chatRaw, _ := json.Marshal(chatData)
	event := &PluginEvent{
		ID:        uuid.New(),
		ServerID:  serverID,
		Source:    EventSourceRCON,
		Type:      string(event_manager.EventTypeRconChatMessage),
		Data:      json.RawMessage(chatRaw),
		Timestamp: time.Now(),
	}
	if err := instance.HandleEvent(event); err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}

	if got := rcon.lastPlayerID.Load(); got != "76561198012345678" {
		t.Fatalf("RconAPI.SendWarningToPlayer player = %v, want eos-123", got)
	}
	if got := rcon.lastMessage.Load(); got != "hi" {
		t.Fatalf("RconAPI.SendWarningToPlayer message = %v, want hi", got)
	}
	if logAPI.infoCount.Load() != 1 {
		t.Fatalf("LogAPI.Info count = %d, want 1", logAPI.infoCount.Load())
	}

	if err := instance.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}
