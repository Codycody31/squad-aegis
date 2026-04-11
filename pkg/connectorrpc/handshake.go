package connectorrpc

import (
	"net/rpc"

	goplugin "github.com/hashicorp/go-plugin"
)

// Handshake is the go-plugin handshake used by connector subprocesses.
var Handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  WireProtocolVersion,
	MagicCookieKey:   "AEGIS_NATIVE_CONNECTOR",
	MagicCookieValue: "squad-aegis-connector-v1",
}

// PluginName is the go-plugin key under which the connector is registered.
const PluginName = "aegis_connector"

// connectorAdapter implements go-plugin's Plugin interface.
type connectorAdapter struct {
	impl Connector
}

// Server runs inside the connector process.
func (a *connectorAdapter) Server(broker *goplugin.MuxBroker) (interface{}, error) {
	return &connectorRPCServer{impl: a.impl, broker: broker}, nil
}

// Client runs inside the host process.
func (a *connectorAdapter) Client(broker *goplugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &ConnectorRPCClient{client: c, broker: broker}, nil
}

// PluginMap is the go-plugin PluginSet used by both sides.
func PluginMap(impl Connector) map[string]goplugin.Plugin {
	return map[string]goplugin.Plugin{
		PluginName: &connectorAdapter{impl: impl},
	}
}

// Serve is the connector main() entrypoint. It blocks until the host
// terminates the connector, then returns.
func Serve(impl Connector) {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         PluginMap(impl),
	})
}
