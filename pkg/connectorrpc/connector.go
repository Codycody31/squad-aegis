package connectorrpc

import (
	"context"
	"fmt"
	"net/rpc"
	"sync"
	"time"

	goplugin "github.com/hashicorp/go-plugin"
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

// Empty is the zero-byte reply used for methods that return nothing.
type Empty struct{}

// connectorRPCServer wraps the author's Connector impl in a net/rpc server.
type connectorRPCServer struct {
	impl   Connector
	broker *goplugin.MuxBroker

	mu        sync.Mutex
	runCtx    context.Context
	runCancel context.CancelFunc
}

// GetDefinition returns the static connector definition.
func (s *connectorRPCServer) GetDefinition(_ Empty, reply *ConnectorDefinition) error {
	*reply = s.impl.GetDefinition()
	return nil
}

// Initialize runs the connector's Initialize method.
func (s *connectorRPCServer) Initialize(args InitializeArgs, _ *Empty) error {
	return s.impl.Initialize(args.Config)
}

// Start runs Start inside a cancellable goroutine; returns whatever the
// connector's Start returned.
func (s *connectorRPCServer) Start(_ Empty, _ *Empty) error {
	s.mu.Lock()
	if s.runCtx != nil {
		s.mu.Unlock()
		return fmt.Errorf("connector already started")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.runCtx = ctx
	s.runCancel = cancel
	s.mu.Unlock()

	return s.impl.Start(ctx)
}

// Stop cancels the run context and calls Stop.
func (s *connectorRPCServer) Stop(_ Empty, _ *Empty) error {
	s.mu.Lock()
	cancel := s.runCancel
	s.runCancel = nil
	s.runCtx = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return s.impl.Stop()
}

// GetStatus returns the connector's current status.
func (s *connectorRPCServer) GetStatus(_ Empty, reply *ConnectorStatus) error {
	*reply = s.impl.GetStatus()
	return nil
}

// GetConfig returns the connector's current config.
func (s *connectorRPCServer) GetConfig(_ Empty, reply *map[string]interface{}) error {
	*reply = s.impl.GetConfig()
	return nil
}

// UpdateConfig applies new config to the connector.
func (s *connectorRPCServer) UpdateConfig(args map[string]interface{}, _ *Empty) error {
	return s.impl.UpdateConfig(args)
}

// Invoke runs an invocation against the connector, respecting a per-call
// timeout carried on the wire.
func (s *connectorRPCServer) Invoke(args InvokeArgs, reply *ConnectorInvokeResponse) error {
	ctx := context.Background()
	timeout := time.Duration(args.TimeoutMs) * time.Millisecond
	if timeout <= 0 || timeout > WaitTimeout {
		timeout = WaitTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := s.impl.Invoke(ctx, &args.Request)
	if err != nil {
		return err
	}
	if resp != nil {
		*reply = *resp
	}
	return nil
}

// -- host-side client --------------------------------------------------------

// ConnectorRPCClient is the host-side stub.
type ConnectorRPCClient struct {
	client *rpc.Client
	broker *goplugin.MuxBroker
}

// GetDefinition fetches the static connector definition.
func (c *ConnectorRPCClient) GetDefinition() (ConnectorDefinition, error) {
	var def ConnectorDefinition
	if err := c.client.Call("Plugin.GetDefinition", Empty{}, &def); err != nil {
		return ConnectorDefinition{}, err
	}
	return def, nil
}

// Initialize runs the connector's Initialize.
func (c *ConnectorRPCClient) Initialize(args InitializeArgs) error {
	return c.client.Call("Plugin.Initialize", args, &Empty{})
}

// Start runs the connector's Start.
func (c *ConnectorRPCClient) Start() error {
	return c.client.Call("Plugin.Start", Empty{}, &Empty{})
}

// Stop runs the connector's Stop.
func (c *ConnectorRPCClient) Stop() error {
	return c.client.Call("Plugin.Stop", Empty{}, &Empty{})
}

// GetStatus fetches the current status.
func (c *ConnectorRPCClient) GetStatus() (ConnectorStatus, error) {
	var status ConnectorStatus
	if err := c.client.Call("Plugin.GetStatus", Empty{}, &status); err != nil {
		return "", err
	}
	return status, nil
}

// GetConfig fetches the current config.
func (c *ConnectorRPCClient) GetConfig() (map[string]interface{}, error) {
	var cfg map[string]interface{}
	if err := c.client.Call("Plugin.GetConfig", Empty{}, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// UpdateConfig sets a new config on the connector.
func (c *ConnectorRPCClient) UpdateConfig(config map[string]interface{}) error {
	return c.client.Call("Plugin.UpdateConfig", config, &Empty{})
}

// Invoke runs a connector invocation.
func (c *ConnectorRPCClient) Invoke(req ConnectorInvokeRequest, timeoutMs int64) (*ConnectorInvokeResponse, error) {
	var reply ConnectorInvokeResponse
	if err := c.client.Call("Plugin.Invoke", InvokeArgs{Request: req, TimeoutMs: timeoutMs}, &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}
