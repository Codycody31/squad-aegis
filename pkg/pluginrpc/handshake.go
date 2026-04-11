package pluginrpc

import (
	"net/rpc"

	goplugin "github.com/hashicorp/go-plugin"
)

// Handshake is the go-plugin handshake the host and plugin both use to
// negotiate a common protocol. ProtocolVersion is bumped whenever the wire
// format changes in an incompatible way.
var Handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  WireProtocolVersion,
	MagicCookieKey:   "AEGIS_NATIVE_PLUGIN",
	MagicCookieValue: "squad-aegis-plugin-v1",
}

// PluginName is the key under which the plugin is registered with go-plugin.
// Both sides must use the same name.
const PluginName = "aegis_plugin"

// pluginAdapter implements go-plugin's Plugin interface, returning a net/rpc
// server on the plugin side and a client stub on the host side.
type pluginAdapter struct {
	impl Plugin
}

// Server is called by go-plugin inside the plugin process. It wraps the
// author's Plugin implementation in an RPC server object.
func (p *pluginAdapter) Server(broker *goplugin.MuxBroker) (interface{}, error) {
	return &pluginRPCServer{
		impl:   p.impl,
		broker: broker,
	}, nil
}

// Client is called by go-plugin inside the host process. It returns a stub
// the host loader can call into to drive the plugin.
func (p *pluginAdapter) Client(broker *goplugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &PluginRPCClient{client: c, broker: broker}, nil
}

// PluginMap is the go-plugin PluginSet used by both sides. The host passes
// this to goplugin.NewClient and the plugin passes it to goplugin.Serve. Only
// the impl field is populated on the plugin side; the host uses a nil impl.
func PluginMap(impl Plugin) map[string]goplugin.Plugin {
	return map[string]goplugin.Plugin{
		PluginName: &pluginAdapter{impl: impl},
	}
}
