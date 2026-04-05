package plugin_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// WASM connector guest ABI (sideloaded connector packages). See docs/wasm-guest-abi.md.
//   - Optional import: aegis_host_v1.log when manifest required_capabilities includes api.log.
//   - Exports: memory; aegis_init(config_ptr, config_len) -> i32; aegis_start() -> i32; aegis_stop() -> i32;
//     aegis_invoke(req_ptr, req_len, out_ptr, out_cap, out_written_ptr) -> i32.
//     The request is ConnectorInvokeRequest JSON. On success the guest writes ConnectorInvokeResponse JSON to out_ptr
//     and the UTF-8 byte length as uint32 little-endian at out_written_ptr.
//     Return codes: 0 = success (wasmHostOK), 1 = memory (wasmHostErrMemory), 2 = buffer (wasmHostErrBuffer).
const wasmConnectorInvokeOutCap = 512 * 1024

type wasmConnector struct {
	mu sync.RWMutex

	def      ConnectorDefinition
	wasmPath string
	code     []byte

	rt       wazero.Runtime
	compiled wazero.CompiledModule
	instance api.Module

	logAPI LogAPI
	config map[string]interface{}
	status ConnectorStatus
}

func connectorDefinitionHasCapability(def ConnectorDefinition, capability string) bool {
	return pluginDefinitionHasCapability(PluginDefinition{RequiredCapabilities: def.RequiredCapabilities}, capability)
}

type connectorWasmLog struct {
	connectorID string
}

func newConnectorWasmLog(connectorID string) *connectorWasmLog {
	return &connectorWasmLog{connectorID: strings.TrimSpace(connectorID)}
}

func (l *connectorWasmLog) Info(message string, fields map[string]interface{}) {
	log.Info().Str("wasm_connector", l.connectorID).Interface("fields", fields).Msg(message)
}

func (l *connectorWasmLog) Warn(message string, fields map[string]interface{}) {
	log.Warn().Str("wasm_connector", l.connectorID).Interface("fields", fields).Msg(message)
}

func (l *connectorWasmLog) Error(message string, err error, fields map[string]interface{}) {
	ev := log.Error().Str("wasm_connector", l.connectorID).Interface("fields", fields)
	if err != nil {
		ev = ev.Err(err)
	}
	ev.Msg(message)
}

func (l *connectorWasmLog) Debug(message string, fields map[string]interface{}) {
	log.Debug().Str("wasm_connector", l.connectorID).Interface("fields", fields).Msg(message)
}

func newWasmConnector(def ConnectorDefinition, wasmPath string) *wasmConnector {
	c := &wasmConnector{
		def:      def,
		wasmPath: wasmPath,
		status:   ConnectorStatusStopped,
	}
	if connectorDefinitionHasCapability(def, NativePluginCapabilityAPILog) {
		c.logAPI = newConnectorWasmLog(def.ID)
	}
	return c
}

func (c *wasmConnector) hostLog(ctx context.Context, m api.Module, level, ptr, length uint32) {
	_ = ctx
	mem := m.Memory()
	if mem == nil || c.logAPI == nil {
		return
	}
	b, ok := mem.Read(ptr, length)
	if !ok {
		return
	}
	msg := string(b)
	switch level {
	case 0:
		c.logAPI.Info(msg, nil)
	case 1:
		c.logAPI.Warn(msg, nil)
	case 2:
		c.logAPI.Error(msg, nil, nil)
	default:
		c.logAPI.Debug(msg, nil)
	}
}

func (pm *PluginManager) loadWasmConnectorPackage(pkg *InstalledConnectorPackage) error {
	manifest := pkg.Manifest
	author := strings.TrimSpace(manifest.Author)
	if author == "" {
		author = "WebAssembly package"
	}
	instanceKey := strings.TrimSpace(manifest.InstanceKey)

	defBase := ConnectorDefinition{
		ID:                   manifest.ConnectorID,
		LegacyIDs:            append([]string(nil), manifest.LegacyIDs...),
		InstanceKey:          instanceKey,
		Name:                 manifest.Name,
		Description:          manifest.Description,
		Version:              manifest.Version,
		Author:               author,
		ConfigSchema:         manifest.ConfigSchema,
		Source:               PluginSourceWasm,
		Official:             pkg.Official,
		InstallState:         pkg.InstallState,
		Distribution:         pkg.Distribution,
		MinHostAPIVersion:    pkg.MinHostAPIVersion,
		RequiredCapabilities: cloneRequiredCapabilities(pkg.RequiredCapabilities),
		TargetOS:             pkg.TargetOS,
		TargetArch:           pkg.TargetArch,
		RuntimePath:          pkg.RuntimePath,
		SignatureVerified:    pkg.SignatureVerified,
		Unsafe:               pkg.Unsafe,
		CreateInstance:       nil,
	}

	runtimePath := pkg.RuntimePath
	registration := defBase
	registration.CreateInstance = func() Connector {
		d := defBase
		return newWasmConnector(d, runtimePath)
	}

	if err := pm.connectorRegistry.RegisterConnector(registration); err != nil {
		return fmt.Errorf("failed to register wasm connector: %w", err)
	}

	pm.nativeMu.Lock()
	pm.loadedNativeConnectors[pkg.ConnectorID] = pkg.Version
	pm.nativeMu.Unlock()

	return nil
}

func (c *wasmConnector) GetDefinition() ConnectorDefinition {
	return c.def
}

func (c *wasmConnector) Initialize(config map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.def.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	c.config = c.def.ConfigSchema.FillDefaults(config)
	c.status = ConnectorStatusStarting

	code, err := os.ReadFile(c.wasmPath)
	if err != nil {
		c.status = ConnectorStatusError
		return fmt.Errorf("read wasm connector: %w", err)
	}
	c.code = code

	ctx := context.Background()
	rcfg := wazero.NewRuntimeConfig().WithMemoryLimitPages(wasmMemoryLimitPages)
	c.rt = wazero.NewRuntimeWithConfig(ctx, rcfg)

	if c.logAPI != nil {
		if _, err = c.rt.NewHostModuleBuilder(WasmHostImportModule).
			NewFunctionBuilder().WithFunc(c.hostLog).Export("log").
			Instantiate(ctx); err != nil {
			c.closeLocked(ctx)
			c.status = ConnectorStatusError
			return fmt.Errorf("wasm connector host imports: %w", err)
		}
	}

	c.compiled, err = c.rt.CompileModule(ctx, code)
	if err != nil {
		c.closeLocked(ctx)
		c.status = ConnectorStatusError
		return fmt.Errorf("compile wasm connector: %w", err)
	}

	c.instance, err = c.rt.InstantiateModule(ctx, c.compiled, wazero.NewModuleConfig())
	if err != nil {
		c.closeLocked(ctx)
		c.status = ConnectorStatusError
		return fmt.Errorf("instantiate wasm connector: %w", err)
	}

	mem := c.instance.Memory()
	if mem == nil {
		c.closeLocked(ctx)
		c.status = ConnectorStatusError
		return fmt.Errorf("wasm connector must export memory")
	}

	cfgBytes, err := json.Marshal(c.config)
	if err != nil {
		c.closeLocked(ctx)
		c.status = ConnectorStatusError
		return fmt.Errorf("marshal config: %w", err)
	}
	off, n, err := wasmWriteAppend(mem, cfgBytes)
	if err != nil {
		c.closeLocked(ctx)
		c.status = ConnectorStatusError
		return err
	}

	initFn := c.instance.ExportedFunction("aegis_init")
	if initFn == nil {
		c.closeLocked(ctx)
		c.status = ConnectorStatusError
		return fmt.Errorf("wasm connector missing export aegis_init")
	}
	callCtx, cancel := context.WithTimeout(ctx, wasmHostCallTimeout)
	defer cancel()
	res, err := initFn.Call(callCtx, uint64(off), uint64(n))
	if err != nil {
		c.closeLocked(ctx)
		c.status = ConnectorStatusError
		return fmt.Errorf("aegis_init: %w", err)
	}
	if len(res) > 0 && res[0] != 0 {
		c.closeLocked(ctx)
		c.status = ConnectorStatusError
		return fmt.Errorf("aegis_init failed with code %d", res[0])
	}

	c.status = ConnectorStatusStopped
	return nil
}

func (c *wasmConnector) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.instance == nil {
		return fmt.Errorf("wasm connector not initialized")
	}
	startFn := c.instance.ExportedFunction("aegis_start")
	if startFn == nil {
		c.status = ConnectorStatusRunning
		return nil
	}
	callCtx, cancel := context.WithTimeout(ctx, wasmHostCallTimeout)
	defer cancel()
	res, err := startFn.Call(callCtx)
	if err != nil {
		c.status = ConnectorStatusError
		return fmt.Errorf("aegis_start: %w", err)
	}
	if len(res) > 0 && res[0] != 0 {
		c.status = ConnectorStatusError
		return fmt.Errorf("aegis_start failed with code %d", res[0])
	}
	c.status = ConnectorStatusRunning
	return nil
}

func (c *wasmConnector) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx := context.Background()
	if c.instance != nil {
		if stop := c.instance.ExportedFunction("aegis_stop"); stop != nil {
			callCtx, cancel := context.WithTimeout(ctx, wasmHostCallTimeout)
			_, _ = stop.Call(callCtx)
			cancel()
		}
	}
	c.closeLocked(ctx)
	c.status = ConnectorStatusStopped
	return nil
}

func (c *wasmConnector) closeLocked(ctx context.Context) {
	if c.instance != nil {
		_ = c.instance.Close(ctx)
		c.instance = nil
	}
	if c.compiled != nil {
		_ = c.compiled.Close(ctx)
		c.compiled = nil
	}
	if c.rt != nil {
		_ = c.rt.Close(ctx)
		c.rt = nil
	}
}

func (c *wasmConnector) GetStatus() ConnectorStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *wasmConnector) GetConfig() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.config == nil {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(c.config))
	for k, v := range c.config {
		out[k] = v
	}
	return out
}

func (c *wasmConnector) UpdateConfig(config map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.def.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	c.config = c.def.ConfigSchema.FillDefaults(config)
	if c.instance == nil || c.rt == nil {
		return nil
	}
	cfgBytes, err := json.Marshal(c.config)
	if err != nil {
		return err
	}
	mem := c.instance.Memory()
	if mem == nil {
		return fmt.Errorf("wasm memory unavailable")
	}
	off, n, err := wasmWriteAppend(mem, cfgBytes)
	if err != nil {
		return err
	}
	initFn := c.instance.ExportedFunction("aegis_init")
	if initFn == nil {
		return nil
	}
	callCtx, cancel := context.WithTimeout(context.Background(), wasmHostCallTimeout)
	defer cancel()
	res, err := initFn.Call(callCtx, uint64(off), uint64(n))
	if err != nil {
		return fmt.Errorf("aegis_init on config update: %w", err)
	}
	if len(res) > 0 && res[0] != 0 {
		return fmt.Errorf("aegis_init failed with code %d", res[0])
	}
	return nil
}

func (c *wasmConnector) GetAPI() interface{} {
	return nil
}

func (c *wasmConnector) Invoke(ctx context.Context, req *ConnectorInvokeRequest) (*ConnectorInvokeResponse, error) {
	out := &ConnectorInvokeResponse{V: ConnectorWireProtocolV1, OK: false}
	if req == nil {
		out.Error = "request is nil"
		return out, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.instance == nil {
		out.Error = "wasm connector not initialized"
		return out, nil
	}
	fn := c.instance.ExportedFunction("aegis_invoke")
	if fn == nil {
		out.Error = "wasm connector missing export aegis_invoke"
		return out, nil
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		out.Error = err.Error()
		return out, nil
	}

	mem := c.instance.Memory()
	if mem == nil {
		out.Error = "wasm memory unavailable"
		return out, nil
	}

	offReq, nReq, err := wasmWriteAppend(mem, reqBytes)
	if err != nil {
		out.Error = err.Error()
		return out, nil
	}

	outPtr := uint32(mem.Size())
	pages := uint32((wasmConnectorInvokeOutCap + 4 + 65535) / 65536)
	if _, ok := mem.Grow(pages); !ok {
		out.Error = "wasm memory grow failed for invoke output"
		return out, nil
	}
	outCap := uint32(wasmConnectorInvokeOutCap)
	outWrittenPtr := outPtr + outCap

	callCtx, cancel := context.WithTimeout(ctx, wasmHostCallTimeout)
	defer cancel()
	res, err := fn.Call(callCtx, uint64(offReq), uint64(nReq), uint64(outPtr), uint64(outCap), uint64(outWrittenPtr))
	if err != nil {
		out.Error = err.Error()
		return out, nil
	}
	if len(res) > 0 && res[0] != uint64(wasmHostOK) {
		out.Error = fmt.Sprintf("aegis_invoke failed with code %d", res[0])
		return out, nil
	}

	written, ok := mem.ReadUint32Le(outWrittenPtr)
	if !ok {
		out.Error = "read output length failed"
		return out, nil
	}
	if written > outCap {
		out.Error = "invalid invoke output length"
		return out, nil
	}
	respBytes, ok := mem.Read(outPtr, written)
	if !ok {
		out.Error = "read invoke response failed"
		return out, nil
	}

	if err := json.Unmarshal(respBytes, &out); err != nil {
		out = &ConnectorInvokeResponse{V: ConnectorWireProtocolV1, OK: false, Error: fmt.Sprintf("invalid invoke response json: %v", err)}
		return out, nil
	}
	return out, nil
}
