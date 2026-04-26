package connectorrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	connectorrpcpb "go.codycody31.dev/squad-aegis/pkg/connectorrpc/proto"
)

// Connector is the interface a subprocess connector author implements.
type Connector interface {
	GetDefinition() ConnectorDefinition
	Initialize(config map[string]interface{}) error
	Start(ctx context.Context) error
	Stop() error
	GetStatus() ConnectorStatus
	GetConfig() map[string]interface{}
	UpdateConfig(config map[string]interface{}) error
	Invoke(ctx context.Context, req *ConnectorInvokeRequest) (*ConnectorInvokeResponse, error)
}

// connectorGRPCServer wraps the author's Connector impl in a gRPC server.
type connectorGRPCServer struct {
	connectorrpcpb.UnimplementedConnectorServer

	impl   Connector
	broker *goplugin.GRPCBroker

	mu        sync.Mutex
	runCtx    context.Context
	runCancel context.CancelFunc
}

// GetDefinition returns the static connector definition.
func (s *connectorGRPCServer) GetDefinition(_ context.Context, _ *connectorrpcpb.Empty) (*connectorrpcpb.ConnectorDefinition, error) {
	return connectorDefinitionToProto(s.impl.GetDefinition())
}

// Initialize runs the connector's Initialize method.
func (s *connectorGRPCServer) Initialize(_ context.Context, req *connectorrpcpb.InitializeRequest) (*connectorrpcpb.Empty, error) {
	cfg, err := decodeJSONMap(req.GetConfigJson())
	if err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if err := s.impl.Initialize(cfg); err != nil {
		return nil, err
	}
	return &connectorrpcpb.Empty{}, nil
}

// Start runs Start inside a cancellable goroutine.
func (s *connectorGRPCServer) Start(_ context.Context, _ *connectorrpcpb.Empty) (*connectorrpcpb.Empty, error) {
	s.mu.Lock()
	if s.runCtx != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("connector already started")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.runCtx = ctx
	s.runCancel = cancel
	s.mu.Unlock()

	if err := s.impl.Start(ctx); err != nil {
		return nil, err
	}
	return &connectorrpcpb.Empty{}, nil
}

// Stop cancels the run context and calls Stop.
func (s *connectorGRPCServer) Stop(_ context.Context, _ *connectorrpcpb.Empty) (*connectorrpcpb.Empty, error) {
	s.mu.Lock()
	cancel := s.runCancel
	s.runCancel = nil
	s.runCtx = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if err := s.impl.Stop(); err != nil {
		return nil, err
	}
	return &connectorrpcpb.Empty{}, nil
}

// GetStatus returns the connector's current status.
func (s *connectorGRPCServer) GetStatus(_ context.Context, _ *connectorrpcpb.Empty) (*connectorrpcpb.StatusResponse, error) {
	return &connectorrpcpb.StatusResponse{Status: string(s.impl.GetStatus())}, nil
}

// GetConfig returns the connector's current config.
func (s *connectorGRPCServer) GetConfig(_ context.Context, _ *connectorrpcpb.Empty) (*connectorrpcpb.ConfigJSON, error) {
	encoded, err := encodeJSONMap(s.impl.GetConfig())
	if err != nil {
		return nil, err
	}
	return &connectorrpcpb.ConfigJSON{ConfigJson: encoded}, nil
}

// UpdateConfig applies new config to the connector.
func (s *connectorGRPCServer) UpdateConfig(_ context.Context, req *connectorrpcpb.ConfigJSON) (*connectorrpcpb.Empty, error) {
	cfg, err := decodeJSONMap(req.GetConfigJson())
	if err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if err := s.impl.UpdateConfig(cfg); err != nil {
		return nil, err
	}
	return &connectorrpcpb.Empty{}, nil
}

// Invoke runs an invocation against the connector, respecting a per-call
// timeout carried on the wire.
func (s *connectorGRPCServer) Invoke(_ context.Context, req *connectorrpcpb.InvokeRequest) (*connectorrpcpb.InvokeResponse, error) {
	ctx := context.Background()
	timeout := time.Duration(req.GetTimeoutMs()) * time.Millisecond
	if timeout <= 0 || timeout > WaitTimeout {
		timeout = WaitTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	data, err := decodeJSONMap(req.GetDataJson())
	if err != nil {
		return nil, fmt.Errorf("decode invoke data: %w", err)
	}
	resp, err := s.impl.Invoke(ctx, &ConnectorInvokeRequest{V: req.GetV(), Data: data})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return &connectorrpcpb.InvokeResponse{}, nil
	}
	encoded, err := encodeJSONMap(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("encode invoke response data: %w", err)
	}
	return &connectorrpcpb.InvokeResponse{
		V:        resp.V,
		Ok:       resp.OK,
		DataJson: encoded,
		Error:    resp.Error,
	}, nil
}

// -- host-side client --------------------------------------------------------

// ConnectorGRPCClient is the host-side stub.
type ConnectorGRPCClient struct {
	client connectorrpcpb.ConnectorClient
	broker *goplugin.GRPCBroker
	conn   *grpc.ClientConn
}

// ctxOrBackground returns ctx if non-nil, else context.Background().
func ctxOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// GetDefinition fetches the static connector definition.
func (c *ConnectorGRPCClient) GetDefinition(ctx context.Context) (ConnectorDefinition, error) {
	resp, err := c.client.GetDefinition(ctxOrBackground(ctx), &connectorrpcpb.Empty{})
	if err != nil {
		return ConnectorDefinition{}, err
	}
	return protoToConnectorDefinition(resp)
}

// Initialize runs the connector's Initialize.
func (c *ConnectorGRPCClient) Initialize(ctx context.Context, args InitializeArgs) error {
	encoded, err := encodeJSONMap(args.Config)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	_, err = c.client.Initialize(ctxOrBackground(ctx), &connectorrpcpb.InitializeRequest{
		ConfigJson: encoded,
		InstanceId: args.InstanceID,
	})
	return err
}

// Start runs the connector's Start.
func (c *ConnectorGRPCClient) Start(ctx context.Context) error {
	_, err := c.client.Start(ctxOrBackground(ctx), &connectorrpcpb.Empty{})
	return err
}

// Stop runs the connector's Stop.
func (c *ConnectorGRPCClient) Stop(ctx context.Context) error {
	_, err := c.client.Stop(ctxOrBackground(ctx), &connectorrpcpb.Empty{})
	return err
}

// GetStatus fetches the current status.
func (c *ConnectorGRPCClient) GetStatus(ctx context.Context) (ConnectorStatus, error) {
	resp, err := c.client.GetStatus(ctxOrBackground(ctx), &connectorrpcpb.Empty{})
	if err != nil {
		return "", err
	}
	return ConnectorStatus(resp.GetStatus()), nil
}

// GetConfig fetches the current config.
func (c *ConnectorGRPCClient) GetConfig(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.client.GetConfig(ctxOrBackground(ctx), &connectorrpcpb.Empty{})
	if err != nil {
		return nil, err
	}
	return decodeJSONMap(resp.GetConfigJson())
}

// UpdateConfig sets a new config on the connector.
func (c *ConnectorGRPCClient) UpdateConfig(ctx context.Context, config map[string]interface{}) error {
	encoded, err := encodeJSONMap(config)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	_, err = c.client.UpdateConfig(ctxOrBackground(ctx), &connectorrpcpb.ConfigJSON{ConfigJson: encoded})
	return err
}

// Invoke runs a connector invocation. The provided ctx is forwarded so the
// caller's deadline propagates over gRPC, in addition to the explicit
// timeoutMs that the connector subprocess uses for its own bounding.
func (c *ConnectorGRPCClient) Invoke(ctx context.Context, req ConnectorInvokeRequest, timeoutMs int64) (*ConnectorInvokeResponse, error) {
	encoded, err := encodeJSONMap(req.Data)
	if err != nil {
		return nil, fmt.Errorf("encode invoke data: %w", err)
	}
	resp, err := c.client.Invoke(ctxOrBackground(ctx), &connectorrpcpb.InvokeRequest{
		V:         req.V,
		DataJson:  encoded,
		TimeoutMs: timeoutMs,
	})
	if err != nil {
		return nil, err
	}
	out := &ConnectorInvokeResponse{
		V:     resp.GetV(),
		OK:    resp.GetOk(),
		Error: resp.GetError(),
	}
	if data := resp.GetDataJson(); len(data) > 0 {
		decoded, err := decodeJSONMap(data)
		if err != nil {
			return nil, fmt.Errorf("decode invoke response data: %w", err)
		}
		out.Data = decoded
	}
	return out, nil
}

// -- shared encoding helpers -------------------------------------------------

func encodeJSONMap(m map[string]interface{}) ([]byte, error) {
	if len(m) == 0 {
		return nil, nil
	}
	return json.Marshal(m)
}

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

func configFieldsToProto(in []ConfigField) ([]*connectorrpcpb.ConfigField, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]*connectorrpcpb.ConfigField, 0, len(in))
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
		out = append(out, &connectorrpcpb.ConfigField{
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

// MaxConfigFieldDepth bounds nested config tree depth for connector wire input.
const MaxConfigFieldDepth = 10

func protoToConfigFields(in []*connectorrpcpb.ConfigField) ([]ConfigField, error) {
	return protoToConfigFieldsAtDepth(in, MaxConfigFieldDepth)
}

func protoToConfigFieldsAtDepth(in []*connectorrpcpb.ConfigField, remainingDepth int) ([]ConfigField, error) {
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

func configSchemaToProto(s ConfigSchema) (*connectorrpcpb.ConfigSchema, error) {
	fields, err := configFieldsToProto(s.Fields)
	if err != nil {
		return nil, err
	}
	return &connectorrpcpb.ConfigSchema{Fields: fields}, nil
}

func protoToConfigSchema(s *connectorrpcpb.ConfigSchema) (ConfigSchema, error) {
	if s == nil {
		return ConfigSchema{}, nil
	}
	fields, err := protoToConfigFields(s.GetFields())
	if err != nil {
		return ConfigSchema{}, err
	}
	return ConfigSchema{Fields: fields}, nil
}

func connectorDefinitionToProto(def ConnectorDefinition) (*connectorrpcpb.ConnectorDefinition, error) {
	schema, err := configSchemaToProto(def.ConfigSchema)
	if err != nil {
		return nil, err
	}
	return &connectorrpcpb.ConnectorDefinition{
		ConnectorId:  def.ConnectorID,
		ConfigSchema: schema,
	}, nil
}

func protoToConnectorDefinition(p *connectorrpcpb.ConnectorDefinition) (ConnectorDefinition, error) {
	if p == nil {
		return ConnectorDefinition{}, nil
	}
	schema, err := protoToConfigSchema(p.GetConfigSchema())
	if err != nil {
		return ConnectorDefinition{}, err
	}
	return ConnectorDefinition{
		ConnectorID:  p.GetConnectorId(),
		ConfigSchema: schema,
	}, nil
}
