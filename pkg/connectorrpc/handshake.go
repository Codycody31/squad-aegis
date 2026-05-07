package connectorrpc

import (
	"context"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	connectorrpcpb "go.codycody31.dev/squad-aegis/pkg/connectorrpc/proto"
)

// Handshake is the go-plugin handshake used by connector subprocesses.
var Handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  WireProtocolVersion,
	MagicCookieKey:   "AEGIS_NATIVE_CONNECTOR",
	MagicCookieValue: "squad-aegis-connector-v1",
}

// PluginName is the go-plugin key under which the connector is registered.
const PluginName = "aegis_connector"

// connectorAdapter implements go-plugin's GRPCPlugin interface. With
// AutoMTLS enabled on the host's ClientConfig the only protocol supported
// is gRPC, so net/rpc registration is intentionally absent.
type connectorAdapter struct {
	goplugin.NetRPCUnsupportedPlugin

	impl Connector
}

// GRPCServer runs inside the connector process and registers the Connector
// gRPC service.
func (a *connectorAdapter) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	connectorrpcpb.RegisterConnectorServer(s, &connectorGRPCServer{
		impl:   a.impl,
		broker: broker,
	})
	return nil
}

// GRPCClient runs inside the host process and returns the host-side stub.
func (a *connectorAdapter) GRPCClient(_ context.Context, broker *goplugin.GRPCBroker, conn *grpc.ClientConn) (interface{}, error) {
	return &ConnectorGRPCClient{
		client: connectorrpcpb.NewConnectorClient(conn),
		broker: broker,
		conn:   conn,
	}, nil
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
		GRPCServer:      goplugin.DefaultGRPCServer,
	})
}
