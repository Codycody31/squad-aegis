package plugin_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// WasmHostImportModule is the WebAssembly import module for Aegis host functions (docs/wasm-guest-abi.md).
const WasmHostImportModule = "aegis_host_v1"

const maxWasmHostCallRequestJSONBytes = 256 * 1024
const maxWasmHostCallResponseJSONBytes = 512 * 1024

type wasmPlugin struct {
	mu sync.RWMutex

	def      PluginDefinition
	wasmPath string
	code     []byte

	rt       wazero.Runtime
	compiled wazero.CompiledModule
	instance api.Module

	apis   *PluginAPIs
	config map[string]interface{}
	status PluginStatus
}

func newWasmPlugin(def PluginDefinition, wasmPath string) *wasmPlugin {
	return &wasmPlugin{
		def:      def,
		wasmPath: wasmPath,
		status:   PluginStatusStopped,
	}
}

func (p *wasmPlugin) GetDefinition() PluginDefinition {
	return p.def
}

func (p *wasmPlugin) Initialize(config map[string]interface{}, apis *PluginAPIs) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.def.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	p.config = p.def.ConfigSchema.FillDefaults(config)
	p.apis = apis

	code, err := os.ReadFile(p.wasmPath)
	if err != nil {
		return fmt.Errorf("read wasm plugin: %w", err)
	}
	p.code = code

	ctx := context.Background()
	rcfg := wazero.NewRuntimeConfig().WithMemoryLimitPages(wasmMemoryLimitPages)
	p.rt = wazero.NewRuntimeWithConfig(ctx, rcfg)

	hostBuilder := p.rt.NewHostModuleBuilder(WasmHostImportModule).
		NewFunctionBuilder().WithFunc(p.hostLog).Export("log")
	if pluginDefinitionHasCapability(p.def, NativePluginCapabilityAPIConnector) {
		hostBuilder = hostBuilder.NewFunctionBuilder().WithFunc(p.hostConnectorInvoke).Export("connector_invoke")
	}
	hostBuilder = hostBuilder.NewFunctionBuilder().WithFunc(p.hostCall).Export("host_call")
	if _, err := hostBuilder.Instantiate(ctx); err != nil {
		p.closeLocked(ctx)
		return fmt.Errorf("wasm host imports: %w", err)
	}

	p.compiled, err = p.rt.CompileModule(ctx, code)
	if err != nil {
		p.closeLocked(ctx)
		return fmt.Errorf("compile wasm: %w", err)
	}

	p.instance, err = p.rt.InstantiateModule(ctx, p.compiled, wazero.NewModuleConfig())
	if err != nil {
		p.closeLocked(ctx)
		return fmt.Errorf("instantiate wasm: %w", err)
	}

	mem := p.instance.Memory()
	if mem == nil {
		p.closeLocked(ctx)
		return fmt.Errorf("wasm plugin must export memory")
	}

	cfgBytes, err := json.Marshal(p.config)
	if err != nil {
		p.closeLocked(ctx)
		return fmt.Errorf("marshal config: %w", err)
	}
	off, n, err := wasmWriteAppend(mem, cfgBytes)
	if err != nil {
		p.closeLocked(ctx)
		return err
	}

	initFn := p.instance.ExportedFunction("aegis_init")
	if initFn == nil {
		p.closeLocked(ctx)
		return fmt.Errorf("wasm plugin missing export aegis_init")
	}
	callCtx, cancel := context.WithTimeout(ctx, wasmHostCallTimeout)
	defer cancel()
	res, err := initFn.Call(callCtx, uint64(off), uint64(n))
	if err != nil {
		p.closeLocked(ctx)
		return fmt.Errorf("aegis_init: %w", err)
	}
	if len(res) > 0 && res[0] != 0 {
		p.closeLocked(ctx)
		return fmt.Errorf("aegis_init failed with code %d", res[0])
	}

	p.status = PluginStatusStopped
	return nil
}

func (p *wasmPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.instance == nil {
		return fmt.Errorf("wasm plugin not initialized")
	}
	startFn := p.instance.ExportedFunction("aegis_start")
	if startFn == nil {
		return nil
	}
	callCtx, cancel := context.WithTimeout(ctx, wasmHostCallTimeout)
	defer cancel()
	res, err := startFn.Call(callCtx)
	if err != nil {
		return fmt.Errorf("aegis_start: %w", err)
	}
	if len(res) > 0 && res[0] != 0 {
		return fmt.Errorf("aegis_start failed with code %d", res[0])
	}
	p.status = PluginStatusRunning
	return nil
}

func (p *wasmPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ctx := context.Background()
	if p.instance != nil {
		if stop := p.instance.ExportedFunction("aegis_stop"); stop != nil {
			callCtx, cancel := context.WithTimeout(ctx, wasmHostCallTimeout)
			_, _ = stop.Call(callCtx)
			cancel()
		}
	}
	p.closeLocked(ctx)
	p.status = PluginStatusStopped
	return nil
}

func (p *wasmPlugin) closeLocked(ctx context.Context) {
	if p.instance != nil {
		_ = p.instance.Close(ctx)
		p.instance = nil
	}
	if p.compiled != nil {
		_ = p.compiled.Close(ctx)
		p.compiled = nil
	}
	if p.rt != nil {
		_ = p.rt.Close(ctx)
		p.rt = nil
	}
}

func (p *wasmPlugin) HandleEvent(event *PluginEvent) error {
	p.mu.RLock()
	inst := p.instance
	p.mu.RUnlock()
	if inst == nil {
		return nil
	}

	data, err := wasmPluginEventPayload(event)
	if err != nil {
		return err
	}

	mem := inst.Memory()
	if mem == nil {
		return fmt.Errorf("wasm memory unavailable")
	}

	typeStr := []byte(event.Type)
	tOff, tLen, err := wasmWriteAppend(mem, typeStr)
	if err != nil {
		return err
	}
	pOff, pLen, err := wasmWriteAppend(mem, data)
	if err != nil {
		return err
	}

	fn := inst.ExportedFunction("aegis_on_event")
	if fn == nil {
		return fmt.Errorf("wasm plugin missing aegis_on_event")
	}

	callCtx, cancel := context.WithTimeout(context.Background(), wasmHostCallTimeout)
	defer cancel()
	res, err := fn.Call(callCtx, uint64(tOff), uint64(tLen), uint64(pOff), uint64(pLen))
	if err != nil {
		return fmt.Errorf("aegis_on_event: %w", err)
	}
	if len(res) > 0 && res[0] != 0 {
		return fmt.Errorf("aegis_on_event failed with code %d", res[0])
	}
	return nil
}

func wasmPluginEventPayload(event *PluginEvent) ([]byte, error) {
	if event == nil {
		return []byte("{}"), nil
	}
	payload := map[string]interface{}{
		"id":        event.ID.String(),
		"server_id": event.ServerID.String(),
		"type":      event.Type,
		"source":    event.Source,
		"timestamp": event.Timestamp.UTC().Format(time.RFC3339Nano),
	}
	if event.Data != nil {
		raw, err := json.Marshal(event.Data)
		if err != nil {
			payload["data"] = map[string]string{"marshal_error": err.Error()}
		} else {
			var asJSON interface{}
			if err := json.Unmarshal(raw, &asJSON); err != nil {
				payload["data"] = string(raw)
			} else {
				payload["data"] = asJSON
			}
		}
	}
	if event.Raw != "" {
		payload["raw"] = event.Raw
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal wasm event payload: %w", err)
	}
	return b, nil
}

func (p *wasmPlugin) GetStatus() PluginStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

func (p *wasmPlugin) GetConfig() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.config == nil {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(p.config))
	for k, v := range p.config {
		out[k] = v
	}
	return out
}

func (p *wasmPlugin) UpdateConfig(config map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.def.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	p.config = p.def.ConfigSchema.FillDefaults(config)
	if p.instance == nil || p.rt == nil {
		return nil
	}
	cfgBytes, err := json.Marshal(p.config)
	if err != nil {
		return err
	}
	mem := p.instance.Memory()
	if mem == nil {
		return fmt.Errorf("wasm memory unavailable")
	}
	off, n, err := wasmWriteAppend(mem, cfgBytes)
	if err != nil {
		return err
	}
	initFn := p.instance.ExportedFunction("aegis_init")
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

func (p *wasmPlugin) GetCommands() []PluginCommand {
	return nil
}

func (p *wasmPlugin) ExecuteCommand(string, map[string]interface{}) (*CommandResult, error) {
	return nil, fmt.Errorf("wasm plugins do not expose commands in v1")
}

func (p *wasmPlugin) GetCommandExecutionStatus(string) (*CommandExecutionStatus, error) {
	return nil, fmt.Errorf("wasm plugins do not expose commands in v1")
}

func (p *wasmPlugin) hostLog(ctx context.Context, m api.Module, level, ptr, length uint32) {
	_ = ctx
	mem := m.Memory()
	if mem == nil || p.apis == nil || p.apis.LogAPI == nil {
		return
	}
	b, ok := mem.Read(ptr, length)
	if !ok {
		return
	}
	msg := string(b)
	switch level {
	case 0:
		p.apis.LogAPI.Info(msg, nil)
	case 1:
		p.apis.LogAPI.Warn(msg, nil)
	case 2:
		p.apis.LogAPI.Error(msg, nil, nil)
	default:
		p.apis.LogAPI.Debug(msg, nil)
	}
}

func (p *wasmPlugin) hostConnectorInvoke(ctx context.Context, m api.Module,
	idPtr, idLen, reqPtr, reqLen, outPtr, outCap, outWrittenPtr uint32,
) uint32 {
	mem := m.Memory()
	if mem == nil || p.apis == nil {
		return wasmHostErrMemory
	}
	idBytes, ok := mem.Read(idPtr, idLen)
	if !ok {
		return wasmHostErrMemory
	}
	reqBytes, ok := mem.Read(reqPtr, reqLen)
	if !ok {
		return wasmHostErrMemory
	}
	if p.apis.ConnectorAPI == nil {
		return p.writeConnectorInvokeError(mem, outPtr, outCap, outWrittenPtr, "connector api not available")
	}

	var req ConnectorInvokeRequest
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		return p.writeConnectorInvokeError(mem, outPtr, outCap, outWrittenPtr, fmt.Sprintf("invalid invoke json: %v", err))
	}

	callCtx := ctx
	if _, has := callCtx.Deadline(); !has {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(callCtx, wasmHostCallTimeout)
		defer cancel()
	}

	resp, err := p.apis.ConnectorAPI.Call(callCtx, string(idBytes), &req)
	if err != nil {
		return p.writeConnectorInvokeError(mem, outPtr, outCap, outWrittenPtr, err.Error())
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		return p.writeConnectorInvokeError(mem, outPtr, outCap, outWrittenPtr, err.Error())
	}
	if uint32(len(raw)) > outCap {
		return wasmHostErrBuffer
	}
	if !mem.Write(outPtr, raw) {
		return wasmHostErrMemory
	}
	if !mem.WriteUint32Le(outWrittenPtr, uint32(len(raw))) {
		return wasmHostErrMemory
	}
	return wasmHostOK
}

func (p *wasmPlugin) writeConnectorInvokeError(mem api.Memory, outPtr, outCap, outWrittenPtr uint32, msg string) uint32 {
	resp := &ConnectorInvokeResponse{V: ConnectorWireProtocolV1, OK: false, Error: msg}
	raw, err := json.Marshal(resp)
	if err != nil || uint32(len(raw)) > outCap || !mem.Write(outPtr, raw) {
		return wasmHostErrBuffer
	}
	if !mem.WriteUint32Le(outWrittenPtr, uint32(len(raw))) {
		return wasmHostErrMemory
	}
	return wasmHostOK
}

const (
	wasmMemoryLimitPages        = 512 // 32 MiB (64KiB per page)
	wasmHostCallTimeout         = 30 * time.Second
	wasmHostOK           uint32 = 0
	wasmHostErrMemory    uint32 = 1
	wasmHostErrBuffer    uint32 = 2
	wasmHostErrDenied    uint32 = 3
	wasmHostErrInvalid   uint32 = 4
)

func wasmWriteAppend(mem api.Memory, data []byte) (offset uint32, length uint32, err error) {
	n := uint32(len(data))
	if n == 0 {
		return 0, 0, nil
	}
	base := mem.Size()
	pages := (n + 65535) / 65536
	if _, ok := mem.Grow(pages); !ok {
		return 0, 0, fmt.Errorf("wasm memory grow failed")
	}
	if !mem.Write(base, data) {
		return 0, 0, fmt.Errorf("wasm memory write failed")
	}
	return base, n, nil
}

func pluginDefinitionHasCapability(def PluginDefinition, capability string) bool {
	capability = strings.TrimSpace(capability)
	for _, c := range def.RequiredCapabilities {
		if strings.TrimSpace(c) == capability {
			return true
		}
	}
	return false
}
