package pluginrpc

import (
	"context"
	"fmt"
	"net/rpc"
	"sync"

	goplugin "github.com/hashicorp/go-plugin"
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

// -- RPC server side (runs inside the plugin process) -----------------------

// pluginRPCServer is the net/rpc server exposed to the host. It wraps the
// plugin author's Plugin implementation and adapts each RPC method onto it.
type pluginRPCServer struct {
	impl   Plugin
	broker *goplugin.MuxBroker

	mu         sync.Mutex
	runCtx     context.Context
	runCancel  context.CancelFunc
	hostAPIs   *HostAPIs
	hostClient *rpc.Client
}

// Empty is used as the reply type for methods that return nothing.
type Empty struct{}

// GetDefinition responds with the plugin definition. The host uses the
// returned ID/Version to cross-check the manifest.
func (s *pluginRPCServer) GetDefinition(_ Empty, reply *PluginDefinition) error {
	*reply = s.impl.GetDefinition()
	return nil
}

// Initialize wires up the HostAPIs proxy via the broker ID and calls the
// plugin's Initialize method. The hostAPIs client is stored so Stop can
// close it cleanly.
func (s *pluginRPCServer) Initialize(args InitializeArgs, _ *Empty) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// The host has opened an RPC listener and advertised its broker ID via
	// the InitializeArgs. Dial back into it so the plugin can make HostAPI
	// calls (LogAPI.Info, RconAPI.SendCommand, etc.).
	conn, err := s.broker.Dial(args.HostAPIBrokerID)
	if err != nil {
		return fmt.Errorf("failed to dial host api broker: %w", err)
	}
	s.hostClient = rpc.NewClient(conn)
	s.hostAPIs = newHostAPIsFromClient(s.hostClient)

	if err := s.impl.Initialize(args.Config, s.hostAPIs); err != nil {
		_ = s.hostClient.Close()
		s.hostClient = nil
		s.hostAPIs = nil
		return err
	}
	return nil
}

// Start runs the plugin's Start method inside a cancellable goroutine so the
// host can request a Stop via the RPC interface. The returned error is the
// plugin's Start error, if any.
func (s *pluginRPCServer) Start(_ Empty, _ *Empty) error {
	s.mu.Lock()
	if s.runCtx != nil {
		s.mu.Unlock()
		return fmt.Errorf("plugin already started")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.runCtx = ctx
	s.runCancel = cancel
	s.mu.Unlock()

	return s.impl.Start(ctx)
}

// Stop cancels the run context (releasing any goroutines the plugin spawned
// inside Start) and then calls the plugin's Stop method.
func (s *pluginRPCServer) Stop(_ Empty, _ *Empty) error {
	s.mu.Lock()
	cancel := s.runCancel
	s.runCancel = nil
	s.runCtx = nil
	client := s.hostClient
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	err := s.impl.Stop()
	if client != nil {
		_ = client.Close()
	}
	return err
}

// HandleEvent forwards an event to the plugin.
func (s *pluginRPCServer) HandleEvent(args HandleEventArgs, _ *Empty) error {
	event := args.Event
	return s.impl.HandleEvent(&event)
}

// GetStatus returns the plugin's current lifecycle status.
func (s *pluginRPCServer) GetStatus(_ Empty, reply *PluginStatus) error {
	*reply = s.impl.GetStatus()
	return nil
}

// GetConfig returns the plugin's masked config map.
func (s *pluginRPCServer) GetConfig(_ Empty, reply *map[string]interface{}) error {
	*reply = s.impl.GetConfig()
	return nil
}

// UpdateConfig applies a new config to the plugin.
func (s *pluginRPCServer) UpdateConfig(args map[string]interface{}, _ *Empty) error {
	return s.impl.UpdateConfig(args)
}

// GetCommands returns the list of commands the plugin exposes.
func (s *pluginRPCServer) GetCommands(_ Empty, reply *[]PluginCommand) error {
	*reply = s.impl.GetCommands()
	return nil
}

// ExecuteCommand runs a command on the plugin and returns the result.
func (s *pluginRPCServer) ExecuteCommand(args ExecuteCommandArgs, reply *CommandResult) error {
	result, err := s.impl.ExecuteCommand(args.CommandID, args.Params)
	if err != nil {
		return err
	}
	if result != nil {
		*reply = *result
	}
	return nil
}

// GetCommandExecutionStatus returns the status of a previously-launched
// asynchronous command.
func (s *pluginRPCServer) GetCommandExecutionStatus(executionID string, reply *CommandExecutionStatus) error {
	status, err := s.impl.GetCommandExecutionStatus(executionID)
	if err != nil {
		return err
	}
	if status != nil {
		*reply = *status
	}
	return nil
}

// -- RPC client side (runs inside the host) ---------------------------------

// PluginRPCClient is the host-side stub. It wraps a net/rpc client and
// exposes typed methods the subprocess loader adapts onto
// plugin_manager.Plugin. Exported so host code in a different package can
// receive it from goplugin.Client().Dispense().
type PluginRPCClient struct {
	client *rpc.Client
	broker *goplugin.MuxBroker
}

// Broker returns the underlying go-plugin MuxBroker so the host loader can
// start auxiliary RPC services (e.g. HostAPI) on new broker IDs. Returns nil
// if the client has no broker attached (e.g. in tests).
func (c *PluginRPCClient) Broker() *goplugin.MuxBroker {
	return c.broker
}

// StartHostAPIBroker allocates a new broker ID and begins accepting a single
// HostAPI connection from the plugin subprocess on that ID. Once the
// subprocess dials back (from its Initialize impl), the returned rpc.Server
// serves calls on that connection for the life of the plugin. The returned
// stop function closes the connection, ending the HostAPI service.
func (c *PluginRPCClient) StartHostAPIBroker(server *rpc.Server) (uint32, func(), error) {
	if c.broker == nil {
		return 0, nil, fmt.Errorf("plugin rpc client has no broker")
	}
	id := c.broker.NextId()

	done := make(chan struct{})
	var connRef struct {
		mu sync.Mutex
		c  closer
	}
	go func() {
		conn, err := c.broker.Accept(id)
		if err != nil {
			return
		}
		connRef.mu.Lock()
		connRef.c = conn
		closed := false
		select {
		case <-done:
			closed = true
		default:
		}
		connRef.mu.Unlock()
		if closed {
			_ = conn.Close()
			return
		}
		server.ServeConn(conn)
	}()

	stop := func() {
		select {
		case <-done:
			return
		default:
			close(done)
		}
		connRef.mu.Lock()
		if connRef.c != nil {
			_ = connRef.c.Close()
		}
		connRef.mu.Unlock()
	}
	return id, stop, nil
}

// closer is satisfied by net.Conn without importing the net package here.
type closer interface {
	Close() error
}

// GetDefinition fetches the plugin's static definition.
func (c *PluginRPCClient) GetDefinition() (PluginDefinition, error) {
	var def PluginDefinition
	if err := c.client.Call("Plugin.GetDefinition", Empty{}, &def); err != nil {
		return PluginDefinition{}, err
	}
	return def, nil
}

// Initialize hands off the config and a broker ID the plugin can use to
// dial the host API server.
func (c *PluginRPCClient) Initialize(args InitializeArgs) error {
	return c.client.Call("Plugin.Initialize", args, &Empty{})
}

// Start runs the plugin's Start. Blocks until the plugin returns from Start.
func (c *PluginRPCClient) Start() error {
	return c.client.Call("Plugin.Start", Empty{}, &Empty{})
}

// Stop asks the plugin to shut down.
func (c *PluginRPCClient) Stop() error {
	return c.client.Call("Plugin.Stop", Empty{}, &Empty{})
}

// HandleEvent delivers an event to the plugin.
func (c *PluginRPCClient) HandleEvent(event PluginEvent) error {
	return c.client.Call("Plugin.HandleEvent", HandleEventArgs{Event: event}, &Empty{})
}

// GetStatus fetches the plugin's current lifecycle status.
func (c *PluginRPCClient) GetStatus() (PluginStatus, error) {
	var status PluginStatus
	if err := c.client.Call("Plugin.GetStatus", Empty{}, &status); err != nil {
		return "", err
	}
	return status, nil
}

// GetConfig fetches the plugin's masked config.
func (c *PluginRPCClient) GetConfig() (map[string]interface{}, error) {
	var cfg map[string]interface{}
	if err := c.client.Call("Plugin.GetConfig", Empty{}, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// UpdateConfig sets a new config on the plugin.
func (c *PluginRPCClient) UpdateConfig(config map[string]interface{}) error {
	return c.client.Call("Plugin.UpdateConfig", config, &Empty{})
}

// GetCommands fetches the plugin's command list.
func (c *PluginRPCClient) GetCommands() ([]PluginCommand, error) {
	var commands []PluginCommand
	if err := c.client.Call("Plugin.GetCommands", Empty{}, &commands); err != nil {
		return nil, err
	}
	return commands, nil
}

// ExecuteCommand runs a command on the plugin.
func (c *PluginRPCClient) ExecuteCommand(commandID string, params map[string]interface{}) (*CommandResult, error) {
	var result CommandResult
	if err := c.client.Call("Plugin.ExecuteCommand", ExecuteCommandArgs{CommandID: commandID, Params: params}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetCommandExecutionStatus fetches an async command's current status.
func (c *PluginRPCClient) GetCommandExecutionStatus(executionID string) (*CommandExecutionStatus, error) {
	var status CommandExecutionStatus
	if err := c.client.Call("Plugin.GetCommandExecutionStatus", executionID, &status); err != nil {
		return nil, err
	}
	return &status, nil
}
