package plugin_manager

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

func TestWasmExampleGuestLoads(t *testing.T) {
	t.Parallel()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	wasmPath := filepath.Join(filepath.Dir(file), "..", "..", "examples", "wasm-plugin-hello", "plugin.wasm")
	code, err := os.ReadFile(wasmPath)
	if err != nil {
		t.Skipf("example wasm not built: %v", wasmPath)
	}

	ctx := context.Background()
	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx)

	_, err = r.NewHostModuleBuilder(WasmHostImportModule).
		NewFunctionBuilder().WithFunc(func(context.Context, api.Module, uint32, uint32, uint32) {}).Export("log").
		Instantiate(ctx)
	if err != nil {
		t.Fatalf("host: %v", err)
	}

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

	for _, name := range []string{"aegis_init", "aegis_start", "aegis_stop", "aegis_on_event"} {
		if mod.ExportedFunction(name) == nil {
			t.Fatalf("missing export %s", name)
		}
	}
	if mod.Memory() == nil {
		t.Fatal("missing memory export")
	}
}
