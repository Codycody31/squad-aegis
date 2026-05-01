package pluginrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	pluginrpcpb "go.codycody31.dev/squad-aegis/pkg/pluginrpc/proto"
)

// Plugin is the interface an out-of-process plugin author implements. It is
// intentionally close to plugin_manager.Plugin but takes the wire-safe
// HostAPIs instead of the host's in-process struct, and splits Start/Stop
// lifecycle so that the plugin need not hold a Go context across processes.
type Plugin interface {
	GetDefinition() PluginDefinition
	Initialize(config map[string]interface{}, apis *HostAPIs) error
	Start(ctx context.Context) error
	Stop() error
	HandleEvent(event *PluginEvent) error
	GetStatus() PluginStatus
	GetConfig() map[string]interface{}
	UpdateConfig(config map[string]interface{}) error
	GetCommands() []PluginCommand
	ExecuteCommand(commandID string, params map[string]interface{}) (*CommandResult, error)
	GetCommandExecutionStatus(executionID string) (*CommandExecutionStatus, error)
}

// -- gRPC server side (runs inside the plugin process) -----------------------

// pluginGRPCServer is the gRPC server exposed to the host. It wraps the
// plugin author's Plugin implementation and adapts each RPC method onto it.
type pluginGRPCServer struct {
	pluginrpcpb.UnimplementedPluginServer

	impl   Plugin
	broker *goplugin.GRPCBroker

	mu        sync.Mutex
	runCtx    context.Context
	runCancel context.CancelFunc
	hostAPIs  *HostAPIs
	hostConn  *grpc.ClientConn
}

// GetDefinition responds with the plugin definition.
func (s *pluginGRPCServer) GetDefinition(_ context.Context, _ *pluginrpcpb.Empty) (*pluginrpcpb.PluginDefinition, error) {
	return pluginDefinitionToProto(s.impl.GetDefinition())
}

// Initialize wires up the HostAPIs proxy via the broker ID and calls the
// plugin's Initialize method.
func (s *pluginGRPCServer) Initialize(_ context.Context, req *pluginrpcpb.InitializeRequest) (*pluginrpcpb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conn, err := s.broker.Dial(req.GetHostApiBrokerId())
	if err != nil {
		return nil, fmt.Errorf("failed to dial host api broker: %w", err)
	}
	s.hostConn = conn
	s.hostAPIs = newHostAPIsFromConn(conn)

	cfg, err := decodeJSONMap(req.GetConfigJson())
	if err != nil {
		_ = conn.Close()
		s.hostConn = nil
		s.hostAPIs = nil
		return nil, fmt.Errorf("decode config: %w", err)
	}

	if err := s.impl.Initialize(cfg, s.hostAPIs); err != nil {
		_ = conn.Close()
		s.hostConn = nil
		s.hostAPIs = nil
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

// Start runs the plugin's Start method inside a cancellable context.
func (s *pluginGRPCServer) Start(_ context.Context, _ *pluginrpcpb.Empty) (*pluginrpcpb.Empty, error) {
	s.mu.Lock()
	if s.runCtx != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("plugin already started")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.runCtx = ctx
	s.runCancel = cancel
	s.mu.Unlock()

	if err := s.impl.Start(ctx); err != nil {
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

// Stop cancels the run context and calls the plugin's Stop method.
func (s *pluginGRPCServer) Stop(_ context.Context, _ *pluginrpcpb.Empty) (*pluginrpcpb.Empty, error) {
	s.mu.Lock()
	cancel := s.runCancel
	s.runCancel = nil
	s.runCtx = nil
	conn := s.hostConn
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	err := s.impl.Stop()
	if conn != nil {
		_ = conn.Close()
	}
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

// HandleEvent forwards an event to the plugin.
func (s *pluginGRPCServer) HandleEvent(_ context.Context, ev *pluginrpcpb.PluginEvent) (*pluginrpcpb.Empty, error) {
	hostEvent, err := protoToPluginEvent(ev)
	if err != nil {
		return nil, err
	}
	if err := s.impl.HandleEvent(hostEvent); err != nil {
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

// GetStatus returns the plugin's current lifecycle status.
func (s *pluginGRPCServer) GetStatus(_ context.Context, _ *pluginrpcpb.Empty) (*pluginrpcpb.StatusResponse, error) {
	return &pluginrpcpb.StatusResponse{Status: string(s.impl.GetStatus())}, nil
}

// GetConfig returns the plugin's masked config map.
func (s *pluginGRPCServer) GetConfig(_ context.Context, _ *pluginrpcpb.Empty) (*pluginrpcpb.ConfigJSON, error) {
	cfg := s.impl.GetConfig()
	encoded, err := encodeJSONMap(cfg)
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.ConfigJSON{ConfigJson: encoded}, nil
}

// UpdateConfig applies a new config to the plugin.
func (s *pluginGRPCServer) UpdateConfig(_ context.Context, req *pluginrpcpb.ConfigJSON) (*pluginrpcpb.Empty, error) {
	cfg, err := decodeJSONMap(req.GetConfigJson())
	if err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if err := s.impl.UpdateConfig(cfg); err != nil {
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

// GetCommands returns the list of commands the plugin exposes.
func (s *pluginGRPCServer) GetCommands(_ context.Context, _ *pluginrpcpb.Empty) (*pluginrpcpb.CommandList, error) {
	commands := s.impl.GetCommands()
	out := &pluginrpcpb.CommandList{Commands: make([]*pluginrpcpb.PluginCommand, 0, len(commands))}
	for _, cmd := range commands {
		converted, err := pluginCommandToProto(cmd)
		if err != nil {
			return nil, err
		}
		out.Commands = append(out.Commands, converted)
	}
	return out, nil
}

// ExecuteCommand runs a command on the plugin.
func (s *pluginGRPCServer) ExecuteCommand(_ context.Context, req *pluginrpcpb.ExecuteCommandRequest) (*pluginrpcpb.CommandResult, error) {
	params, err := decodeJSONMap(req.GetParamsJson())
	if err != nil {
		return nil, fmt.Errorf("decode params: %w", err)
	}
	result, err := s.impl.ExecuteCommand(req.GetCommandId(), params)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return &pluginrpcpb.CommandResult{}, nil
	}
	return commandResultToProto(*result)
}

// GetCommandExecutionStatus returns the status of a previously-launched async command.
func (s *pluginGRPCServer) GetCommandExecutionStatus(_ context.Context, req *pluginrpcpb.ExecutionIDRequest) (*pluginrpcpb.CommandExecutionStatus, error) {
	status, err := s.impl.GetCommandExecutionStatus(req.GetExecutionId())
	if err != nil {
		return nil, err
	}
	if status == nil {
		return &pluginrpcpb.CommandExecutionStatus{}, nil
	}
	return commandExecutionStatusToProto(*status)
}

// -- gRPC client side (runs inside the host) ---------------------------------

// PluginGRPCClient is the host-side stub. It wraps a gRPC client and
// exposes typed methods the subprocess loader adapts onto
// plugin_manager.Plugin. Exported so host code in a different package can
// receive it from goplugin.Client().Dispense().
type PluginGRPCClient struct {
	client pluginrpcpb.PluginClient
	broker *goplugin.GRPCBroker
	conn   *grpc.ClientConn
}

// Broker returns the underlying go-plugin GRPCBroker so the host loader can
// start auxiliary services (e.g. HostAPI) on new broker IDs.
func (c *PluginGRPCClient) Broker() *goplugin.GRPCBroker {
	return c.broker
}

// StartHostAPIBroker allocates a new broker ID and serves the supplied
// gRPC server on it. The plugin subprocess dials this broker ID from its
// Initialize implementation. The returned stop function shuts down the
// host-side listener.
func (c *PluginGRPCClient) StartHostAPIBroker(register func(*grpc.Server)) (uint32, func(), error) {
	if c.broker == nil {
		return 0, nil, fmt.Errorf("plugin grpc client has no broker")
	}
	id := c.broker.NextId()

	stopCh := make(chan struct{})
	serverHolder := struct {
		mu sync.Mutex
		s  *grpc.Server
	}{}

	go c.broker.AcceptAndServe(id, func(opts []grpc.ServerOption) *grpc.Server {
		s := grpc.NewServer(opts...)
		register(s)
		serverHolder.mu.Lock()
		serverHolder.s = s
		closed := false
		select {
		case <-stopCh:
			closed = true
		default:
		}
		serverHolder.mu.Unlock()
		if closed {
			s.Stop()
			return s
		}
		return s
	})

	stop := func() {
		select {
		case <-stopCh:
			return
		default:
			close(stopCh)
		}
		serverHolder.mu.Lock()
		s := serverHolder.s
		serverHolder.mu.Unlock()
		if s != nil {
			s.Stop()
		}
	}
	return id, stop, nil
}

// ctxOrBackground returns ctx if non-nil, else context.Background().
// Used so internal callers that haven't been threaded a context yet do not
// panic on a nil context.
func ctxOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// GetDefinition fetches the plugin's static definition.
func (c *PluginGRPCClient) GetDefinition(ctx context.Context) (PluginDefinition, error) {
	resp, err := c.client.GetDefinition(ctxOrBackground(ctx), &pluginrpcpb.Empty{})
	if err != nil {
		return PluginDefinition{}, err
	}
	return protoToPluginDefinition(resp)
}

// Initialize hands off the config and a broker ID the plugin uses to dial
// the host API server. The provided ctx is propagated to the gRPC call so
// host shutdown / per-call cancellation aborts a hung Initialize.
func (c *PluginGRPCClient) Initialize(ctx context.Context, args InitializeArgs) error {
	cfg, err := encodeJSONMap(args.Config)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	_, err = c.client.Initialize(ctxOrBackground(ctx), &pluginrpcpb.InitializeRequest{
		ConfigJson:      cfg,
		HostApiBrokerId: args.HostAPIBrokerID,
		InstanceId:      args.InstanceID,
		ServerId:        args.ServerID,
		LogLevel:        args.LogLevel,
	})
	return err
}

// Start runs the plugin's Start. The provided ctx propagates to the RPC.
func (c *PluginGRPCClient) Start(ctx context.Context) error {
	_, err := c.client.Start(ctxOrBackground(ctx), &pluginrpcpb.Empty{})
	return err
}

// Stop asks the plugin to shut down. ctx propagates so host shutdown can
// bound the wait; the host-side hard-kill catches genuinely hung plugins.
func (c *PluginGRPCClient) Stop(ctx context.Context) error {
	_, err := c.client.Stop(ctxOrBackground(ctx), &pluginrpcpb.Empty{})
	return err
}

// HandleEvent delivers an event to the plugin. ctx propagates so a per-event
// cancellation can abort a wedged plugin handler.
func (c *PluginGRPCClient) HandleEvent(ctx context.Context, event PluginEvent) error {
	pb, err := pluginEventToProto(&event)
	if err != nil {
		return err
	}
	_, err = c.client.HandleEvent(ctxOrBackground(ctx), pb)
	return err
}

// GetStatus fetches the plugin's current lifecycle status.
func (c *PluginGRPCClient) GetStatus(ctx context.Context) (PluginStatus, error) {
	resp, err := c.client.GetStatus(ctxOrBackground(ctx), &pluginrpcpb.Empty{})
	if err != nil {
		return "", err
	}
	return PluginStatus(resp.GetStatus()), nil
}

// GetConfig fetches the plugin's masked config.
func (c *PluginGRPCClient) GetConfig(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.client.GetConfig(ctxOrBackground(ctx), &pluginrpcpb.Empty{})
	if err != nil {
		return nil, err
	}
	return decodeJSONMap(resp.GetConfigJson())
}

// UpdateConfig sets a new config on the plugin.
func (c *PluginGRPCClient) UpdateConfig(ctx context.Context, config map[string]interface{}) error {
	encoded, err := encodeJSONMap(config)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	_, err = c.client.UpdateConfig(ctxOrBackground(ctx), &pluginrpcpb.ConfigJSON{ConfigJson: encoded})
	return err
}

// GetCommands fetches the plugin's command list.
func (c *PluginGRPCClient) GetCommands(ctx context.Context) ([]PluginCommand, error) {
	resp, err := c.client.GetCommands(ctxOrBackground(ctx), &pluginrpcpb.Empty{})
	if err != nil {
		return nil, err
	}
	out := make([]PluginCommand, 0, len(resp.GetCommands()))
	for _, c := range resp.GetCommands() {
		converted, err := protoToPluginCommand(c)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}

// ExecuteCommand runs a command on the plugin.
func (c *PluginGRPCClient) ExecuteCommand(ctx context.Context, commandID string, params map[string]interface{}) (*CommandResult, error) {
	encoded, err := encodeJSONMap(params)
	if err != nil {
		return nil, fmt.Errorf("encode params: %w", err)
	}
	resp, err := c.client.ExecuteCommand(ctxOrBackground(ctx), &pluginrpcpb.ExecuteCommandRequest{
		CommandId:  commandID,
		ParamsJson: encoded,
	})
	if err != nil {
		return nil, err
	}
	result, err := protoToCommandResult(resp)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetCommandExecutionStatus fetches an async command's status.
func (c *PluginGRPCClient) GetCommandExecutionStatus(ctx context.Context, executionID string) (*CommandExecutionStatus, error) {
	resp, err := c.client.GetCommandExecutionStatus(ctxOrBackground(ctx), &pluginrpcpb.ExecutionIDRequest{
		ExecutionId: executionID,
	})
	if err != nil {
		return nil, err
	}
	status, err := protoToCommandExecutionStatus(resp)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

// -- shared encoding helpers -------------------------------------------------

// encodeJSONMap marshals a Go map into bytes for transport. A nil map encodes
// as an empty byte slice (not a JSON null) so the wire stays compact.
func encodeJSONMap(m map[string]interface{}) ([]byte, error) {
	if len(m) == 0 {
		return nil, nil
	}
	return json.Marshal(m)
}

// decodeJSONMap unmarshals bytes into a Go map. Empty bytes return an empty
// non-nil map so plugin authors can write `m["key"]` without nil-checking.
func decodeJSONMap(b []byte) (map[string]interface{}, error) {
	if len(b) == 0 {
		return map[string]interface{}{}, nil
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func encodeJSONValue(v interface{}) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func decodeJSONValue(b []byte) (interface{}, error) {
	if len(b) == 0 {
		return nil, nil
	}
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func encodeJSONList(values []interface{}) ([]byte, error) {
	if len(values) == 0 {
		return nil, nil
	}
	return json.Marshal(values)
}

func decodeJSONList(b []byte) ([]interface{}, error) {
	if len(b) == 0 {
		return nil, nil
	}
	out := []interface{}{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// -- proto <-> SDK conversions -----------------------------------------------

func configFieldsToProto(in []ConfigField) ([]*pluginrpcpb.ConfigField, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]*pluginrpcpb.ConfigField, 0, len(in))
	for _, f := range in {
		def, err := encodeJSONValue(f.Default)
		if err != nil {
			return nil, fmt.Errorf("encode default for field %q: %w", f.Name, err)
		}
		opts, err := encodeJSONList(f.Options)
		if err != nil {
			return nil, fmt.Errorf("encode options for field %q: %w", f.Name, err)
		}
		nested, err := configFieldsToProto(f.Nested)
		if err != nil {
			return nil, err
		}
		out = append(out, &pluginrpcpb.ConfigField{
			Name:        f.Name,
			Description: f.Description,
			Required:    f.Required,
			Type:        string(f.Type),
			DefaultJson: def,
			Sensitive:   f.Sensitive,
			OptionsJson: opts,
			Nested:      nested,
		})
	}
	return out, nil
}

// MaxConfigFieldDepth is the maximum permitted depth of nested ConfigField
// trees decoded from the wire. Bounded to prevent stack-blowing payloads and
// pathological recursive validation on the host.
const MaxConfigFieldDepth = 10

func protoToConfigFields(in []*pluginrpcpb.ConfigField) ([]ConfigField, error) {
	return protoToConfigFieldsAtDepth(in, MaxConfigFieldDepth)
}

// protoToConfigFieldsAtDepth performs the conversion bounded by remaining
// depth allowance. Recursion stops with a clear error before unmarshaling any
// child whose depth would exceed the limit. The depth check fires BEFORE
// recursion so a malicious schema cannot blow the host stack between the call
// and the post-condition validator.
func protoToConfigFieldsAtDepth(in []*pluginrpcpb.ConfigField, remainingDepth int) ([]ConfigField, error) {
	if len(in) == 0 {
		return nil, nil
	}
	if remainingDepth <= 0 {
		return nil, fmt.Errorf("config schema exceeds maximum nesting depth of %d", MaxConfigFieldDepth)
	}
	out := make([]ConfigField, 0, len(in))
	for _, f := range in {
		def, err := decodeJSONValue(f.GetDefaultJson())
		if err != nil {
			return nil, fmt.Errorf("decode default for field %q: %w", f.GetName(), err)
		}
		opts, err := decodeJSONList(f.GetOptionsJson())
		if err != nil {
			return nil, fmt.Errorf("decode options for field %q: %w", f.GetName(), err)
		}
		nested, err := protoToConfigFieldsAtDepth(f.GetNested(), remainingDepth-1)
		if err != nil {
			return nil, err
		}
		out = append(out, ConfigField{
			Name:        f.GetName(),
			Description: f.GetDescription(),
			Required:    f.GetRequired(),
			Type:        FieldType(f.GetType()),
			Default:     def,
			Sensitive:   f.GetSensitive(),
			Options:     opts,
			Nested:      nested,
		})
	}
	return out, nil
}

func configSchemaToProto(s ConfigSchema) (*pluginrpcpb.ConfigSchema, error) {
	fields, err := configFieldsToProto(s.Fields)
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.ConfigSchema{Fields: fields}, nil
}

func protoToConfigSchema(s *pluginrpcpb.ConfigSchema) (ConfigSchema, error) {
	if s == nil {
		return ConfigSchema{}, nil
	}
	fields, err := protoToConfigFields(s.GetFields())
	if err != nil {
		return ConfigSchema{}, err
	}
	return ConfigSchema{Fields: fields}, nil
}

func pluginDefinitionToProto(def PluginDefinition) (*pluginrpcpb.PluginDefinition, error) {
	schema, err := configSchemaToProto(def.ConfigSchema)
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.PluginDefinition{
		PluginId:               def.PluginID,
		AllowMultipleInstances: def.AllowMultipleInstances,
		LongRunning:            def.LongRunning,
		RequiredConnectors:     append([]string(nil), def.RequiredConnectors...),
		OptionalConnectors:     append([]string(nil), def.OptionalConnectors...),
		ConfigSchema:           schema,
		Events:                 append([]string(nil), def.Events...),
	}, nil
}

func protoToPluginDefinition(p *pluginrpcpb.PluginDefinition) (PluginDefinition, error) {
	if p == nil {
		return PluginDefinition{}, nil
	}
	schema, err := protoToConfigSchema(p.GetConfigSchema())
	if err != nil {
		return PluginDefinition{}, err
	}
	return PluginDefinition{
		PluginID:               p.GetPluginId(),
		AllowMultipleInstances: p.GetAllowMultipleInstances(),
		LongRunning:            p.GetLongRunning(),
		RequiredConnectors:     append([]string(nil), p.GetRequiredConnectors()...),
		OptionalConnectors:     append([]string(nil), p.GetOptionalConnectors()...),
		ConfigSchema:           schema,
		Events:                 append([]string(nil), p.GetEvents()...),
	}, nil
}

func pluginEventToProto(ev *PluginEvent) (*pluginrpcpb.PluginEvent, error) {
	if ev == nil {
		return &pluginrpcpb.PluginEvent{}, nil
	}
	return &pluginrpcpb.PluginEvent{
		Id:        ev.ID,
		ServerId:  ev.ServerID,
		Source:    string(ev.Source),
		Type:      ev.Type,
		DataJson:  []byte(ev.Data),
		Raw:       ev.Raw,
		Timestamp: timestamppb.New(ev.Timestamp),
	}, nil
}

func protoToPluginEvent(p *pluginrpcpb.PluginEvent) (*PluginEvent, error) {
	if p == nil {
		return nil, nil
	}
	out := &PluginEvent{
		ID:       p.GetId(),
		ServerID: p.GetServerId(),
		Source:   EventSource(p.GetSource()),
		Type:     p.GetType(),
		Raw:      p.GetRaw(),
	}
	if data := p.GetDataJson(); len(data) > 0 {
		out.Data = json.RawMessage(append([]byte(nil), data...))
	}
	if ts := p.GetTimestamp(); ts != nil {
		out.Timestamp = ts.AsTime()
	}
	return out, nil
}

func pluginCommandToProto(cmd PluginCommand) (*pluginrpcpb.PluginCommand, error) {
	params, err := configSchemaToProto(cmd.Parameters)
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.PluginCommand{
		Id:                  cmd.ID,
		Name:                cmd.Name,
		Description:         cmd.Description,
		Category:            cmd.Category,
		Parameters:          params,
		ExecutionType:       string(cmd.ExecutionType),
		RequiredPermissions: append([]string(nil), cmd.RequiredPermissions...),
		ConfirmMessage:      cmd.ConfirmMessage,
	}, nil
}

func protoToPluginCommand(p *pluginrpcpb.PluginCommand) (PluginCommand, error) {
	if p == nil {
		return PluginCommand{}, nil
	}
	params, err := protoToConfigSchema(p.GetParameters())
	if err != nil {
		return PluginCommand{}, err
	}
	return PluginCommand{
		ID:                  p.GetId(),
		Name:                p.GetName(),
		Description:         p.GetDescription(),
		Category:            p.GetCategory(),
		Parameters:          params,
		ExecutionType:       CommandExecutionType(p.GetExecutionType()),
		RequiredPermissions: append([]string(nil), p.GetRequiredPermissions()...),
		ConfirmMessage:      p.GetConfirmMessage(),
	}, nil
}

func commandResultToProto(r CommandResult) (*pluginrpcpb.CommandResult, error) {
	data, err := encodeJSONMap(r.Data)
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.CommandResult{
		Success:     r.Success,
		Message:     r.Message,
		DataJson:    data,
		ExecutionId: r.ExecutionID,
		Error:       r.Error,
	}, nil
}

func protoToCommandResult(p *pluginrpcpb.CommandResult) (CommandResult, error) {
	if p == nil {
		return CommandResult{}, nil
	}
	data, err := decodeJSONMap(p.GetDataJson())
	if err != nil {
		return CommandResult{}, err
	}
	return CommandResult{
		Success:     p.GetSuccess(),
		Message:     p.GetMessage(),
		Data:        data,
		ExecutionID: p.GetExecutionId(),
		Error:       p.GetError(),
	}, nil
}

func commandExecutionStatusToProto(s CommandExecutionStatus) (*pluginrpcpb.CommandExecutionStatus, error) {
	out := &pluginrpcpb.CommandExecutionStatus{
		ExecutionId: s.ExecutionID,
		CommandId:   s.CommandID,
		Status:      s.Status,
		Progress:    int32(s.Progress),
		Message:     s.Message,
		StartedAt:   timestamppb.New(s.StartedAt),
	}
	if s.Result != nil {
		r, err := commandResultToProto(*s.Result)
		if err != nil {
			return nil, err
		}
		out.Result = r
	}
	if s.CompletedAt != nil {
		out.CompletedAt = timestamppb.New(*s.CompletedAt)
	}
	return out, nil
}

func protoToCommandExecutionStatus(p *pluginrpcpb.CommandExecutionStatus) (CommandExecutionStatus, error) {
	if p == nil {
		return CommandExecutionStatus{}, nil
	}
	out := CommandExecutionStatus{
		ExecutionID: p.GetExecutionId(),
		CommandID:   p.GetCommandId(),
		Status:      p.GetStatus(),
		Progress:    int(p.GetProgress()),
		Message:     p.GetMessage(),
	}
	if ts := p.GetStartedAt(); ts != nil {
		out.StartedAt = ts.AsTime()
	}
	if ts := p.GetCompletedAt(); ts != nil {
		t := ts.AsTime()
		out.CompletedAt = &t
	}
	if p.GetResult() != nil {
		r, err := protoToCommandResult(p.GetResult())
		if err != nil {
			return CommandExecutionStatus{}, err
		}
		out.Result = &r
	}
	return out, nil
}
