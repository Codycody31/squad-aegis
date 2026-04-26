package pluginrpc

import (
	"context"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pluginrpcpb "go.codycody31.dev/squad-aegis/pkg/pluginrpc/proto"
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

// pluginAdapter implements go-plugin's GRPCPlugin interface. The host
// configures hashicorp/go-plugin with AutoMTLS so each subprocess gets a
// freshly-issued client certificate; with AutoMTLS the only protocol
// supported is gRPC (net/rpc has no per-connection auth).
type pluginAdapter struct {
	goplugin.NetRPCUnsupportedPlugin

	impl Plugin
}

// GRPCServer is called by go-plugin inside the plugin process to register
// the Plugin gRPC service onto the supplied server.
func (p *pluginAdapter) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	pluginrpcpb.RegisterPluginServer(s, &pluginGRPCServer{
		impl:   p.impl,
		broker: broker,
	})
	return nil
}

// GRPCClient is called by go-plugin inside the host process. It returns the
// host-side stub the loader uses to drive the plugin.
func (p *pluginAdapter) GRPCClient(_ context.Context, broker *goplugin.GRPCBroker, conn *grpc.ClientConn) (interface{}, error) {
	return &PluginGRPCClient{
		client: pluginrpcpb.NewPluginClient(conn),
		broker: broker,
		conn:   conn,
	}, nil
}

// PluginMap is the go-plugin PluginSet used by both sides. The host passes
// this to goplugin.NewClient and the plugin passes it to goplugin.Serve. Only
// the impl field is populated on the plugin side; the host uses a nil impl.
func PluginMap(impl Plugin) map[string]goplugin.Plugin {
	return map[string]goplugin.Plugin{
		PluginName: &pluginAdapter{impl: impl},
	}
}
