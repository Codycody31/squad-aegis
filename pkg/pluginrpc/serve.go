package pluginrpc

import (
	goplugin "github.com/hashicorp/go-plugin"
)

// Serve is the entrypoint that a plugin binary's main() calls to hand
// control over to the Aegis plugin host. It blocks until the host requests
// termination, at which point it returns and main() should exit.
//
// Typical plugin main:
//
//	func main() {
//	    pluginrpc.Serve(&MyPlugin{})
//	}
//
// The impl argument must implement pluginrpc.Plugin. Any panic inside a
// plugin method is recovered by hashicorp/go-plugin's RPC layer and
// surfaced to the host as a normal error, so plugins cannot take down the
// host by crashing in a lifecycle method.
func Serve(impl Plugin) {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         PluginMap(impl),
	})
}
