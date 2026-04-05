package plugin_manager

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tetratelabs/wazero"

	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

func TestWasmConnectorExampleGuestLoads(t *testing.T) {
	t.Parallel()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	wasmPath := filepath.Join(filepath.Dir(file), "..", "..", "examples", "wasm-connector-hello", "plugin.wasm")
	code, err := os.ReadFile(wasmPath)
	if err != nil {
		t.Skipf("example wasm connector not built: %v", wasmPath)
	}

	ctx := context.Background()
	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx)

	compiled, err := r.CompileModule(ctx, code)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	defer compiled.Close(ctx)

	mod, err := r.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	defer mod.Close(ctx)

	for _, name := range []string{"aegis_init", "aegis_start", "aegis_stop", "aegis_invoke"} {
		if mod.ExportedFunction(name) == nil {
			t.Fatalf("missing export %s", name)
		}
	}
	if mod.Memory() == nil {
		t.Fatal("missing memory export")
	}
}

func TestWasmConnectorInvokePingPong(t *testing.T) {
	t.Parallel()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	wasmPath := filepath.Join(filepath.Dir(file), "..", "..", "examples", "wasm-connector-hello", "plugin.wasm")
	code, err := os.ReadFile(wasmPath)
	if err != nil {
		t.Skipf("example wasm connector not built: %v", wasmPath)
	}

	tmp := t.TempDir()
	wasmFile := filepath.Join(tmp, "plugin.wasm")
	if err := os.WriteFile(wasmFile, code, 0o644); err != nil {
		t.Fatal(err)
	}

	def := ConnectorDefinition{
		ID:           "com.squad-aegis.connectors.examples.wasm-hello",
		Name:         "test",
		Version:      "0.1.0",
		ConfigSchema: plug_config_schema.ConfigSchema{},
		Source:       PluginSourceWasm,
		InstallState:  PluginInstallStateReady,
		Distribution:  PluginDistributionSideload,
		MinHostAPIVersion: 1,
		TargetOS:      "wasm",
		TargetArch:    "wasm",
	}

	c := newWasmConnector(def, wasmFile)
	if err := c.Initialize(map[string]interface{}{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = c.Stop() }()

	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}

	resp, err := c.Invoke(context.Background(), &ConnectorInvokeRequest{
		V:    ConnectorWireProtocolV1,
		Data: map[string]interface{}{"action": "ping"},
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if resp == nil || !resp.OK {
		t.Fatalf("expected ok response, got %+v", resp)
	}
	if resp.Data == nil || resp.Data["message"] != "pong" {
		t.Fatalf("expected pong, got %+v", resp)
	}
}
