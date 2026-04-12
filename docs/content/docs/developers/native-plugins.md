---
title: Creating Native Plugins and Connectors
---

Squad Aegis supports two native extension types:

- A **plugin** runs against a specific game server. It can subscribe to events, react to chat/log/system activity, and call host APIs such as RCON, logging, rules, admin, database, Discord, and connectors.
- A **connector** is a reusable service that exposes a request/response interface. Plugins call connectors when they need shared logic or access to an external system.

Both are written in Go, built as standalone Linux binaries, and loaded by Aegis as subprocesses through [`hashicorp/go-plugin`](https://github.com/hashicorp/go-plugin). You ship a zip bundle with a `manifest.json` and one or more binaries. Aegis verifies the bundle, inspects the binary for its runtime definition, and then makes it available in the UI.

## Choose the Right Extension

| Build this | When you need | Typical example |
| --- | --- | --- |
| Native plugin | Per-server automation that reacts to Aegis events or calls host APIs | Moderate chat, kick/ban players, emit plugin events, store plugin data |
| Native connector | A reusable request/response service that other code can invoke | Talk to an external API, centralize integration logic, provide a stable service to multiple plugins |

In practice:

- If the feature is driven by game events, start with a plugin.
- If the feature is an integration boundary or shared service, start with a connector.
- If you need both, build a connector for the integration and a plugin that uses it through `ConnectorAPI`.

## How Native Extensions Work

The split is simple:

- `manifest.json` contains the bundle identity and compatibility data Aegis must validate before it runs anything.
- `GetDefinition()` inside your binary returns the runtime behavior Aegis can only learn by talking to the binary itself.

For plugins, that means:

- `manifest.json`: `plugin_id`, `name`, `description`, `version`, `author`, `license`, `official`, and `targets`
- `GetDefinition()`: `PluginID`, `ConfigSchema`, `Events`, `LongRunning`, `AllowMultipleInstances`, `RequiredConnectors`, `OptionalConnectors`

For connectors, that means:

- `manifest.json`: `connector_id`, `name`, `description`, `version`, `author`, `license`, `official`, optional `instance_key`, optional `legacy_ids`, and `targets`
- `GetDefinition()`: `ConnectorID` and `ConfigSchema`

Aegis cross-checks the manifest ID against the ID returned by the binary. If they do not match, the bundle is rejected.

## Start from the Checked-In Examples

The fastest way to get oriented is to read the example implementations in this repository:

- Plugin example: `examples/native-plugin-hello/main.go`
- Connector example: `examples/native-connector-hello/main.go`
- Plugin packager: `scripts/package-example-native-plugin.sh`
- Connector packager: `scripts/package-example-native-connector.sh`
- Signing helper: `scripts/sign_plugin_bundle/main.go`

If you are building outside this repository, create a normal Go module and depend only on the public SDK package you need:

```bash
mkdir my-aegis-extension
cd my-aegis-extension
go mod init example.com/my-aegis-extension
go get go.codycody31.dev/squad-aegis@latest
```

For production, pin that dependency to the specific tag or commit you want to support.

Use:

- `go.codycody31.dev/squad-aegis/pkg/pluginrpc` for plugins
- `go.codycody31.dev/squad-aegis/pkg/connectorrpc` for connectors

Do not import `internal/...` packages from an external extension.

## Build a Native Plugin

At minimum, a plugin needs:

- `main()` calling `pluginrpc.Serve(...)`
- `GetDefinition()` describing runtime behavior
- lifecycle methods such as `Initialize`, `Start`, `Stop`, `GetStatus`, `GetConfig`, `UpdateConfig`
- `HandleEvent(...)` if the plugin subscribes to events

The plugin interface is:

```go
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
```

A minimal event-driven plugin looks like this:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"

    pluginrpc "go.codycody31.dev/squad-aegis/pkg/pluginrpc"
)

type helloPlugin struct {
    config map[string]interface{}
    apis   *pluginrpc.HostAPIs
}

type rconChatMessage struct {
    EOSID   string `json:"eos_id,omitempty"`
    SteamID string `json:"steam_id,omitempty"`
    Message string `json:"message,omitempty"`
}

func main() {
    pluginrpc.Serve(&helloPlugin{})
}

func (p *helloPlugin) GetDefinition() pluginrpc.PluginDefinition {
    return pluginrpc.PluginDefinition{
        PluginID: "com.example.plugins.hello",
        ConfigSchema: pluginrpc.ConfigSchema{
            Fields: []pluginrpc.ConfigField{
                {
                    Name:        "trigger",
                    Description: "Chat command to listen for.",
                    Type:        pluginrpc.FieldTypeString,
                    Default:     "!hello",
                },
            },
        },
        Events: []string{"RCON_CHAT_MESSAGE"},
    }
}

func (p *helloPlugin) Initialize(config map[string]interface{}, apis *pluginrpc.HostAPIs) error {
    p.config = config
    p.apis = apis
    return nil
}

func (p *helloPlugin) Start(context.Context) error       { return nil }
func (p *helloPlugin) Stop() error                       { return nil }
func (p *helloPlugin) GetStatus() pluginrpc.PluginStatus { return pluginrpc.PluginStatusRunning }
func (p *helloPlugin) GetConfig() map[string]interface{} { return p.config }
func (p *helloPlugin) UpdateConfig(c map[string]interface{}) error {
    p.config = c
    return nil
}
func (p *helloPlugin) GetCommands() []pluginrpc.PluginCommand { return nil }
func (p *helloPlugin) ExecuteCommand(string, map[string]interface{}) (*pluginrpc.CommandResult, error) {
    return nil, fmt.Errorf("no commands")
}
func (p *helloPlugin) GetCommandExecutionStatus(string) (*pluginrpc.CommandExecutionStatus, error) {
    return nil, fmt.Errorf("no commands")
}

func (p *helloPlugin) HandleEvent(event *pluginrpc.PluginEvent) error {
    if event == nil || event.Type != "RCON_CHAT_MESSAGE" {
        return nil
    }

    var msg rconChatMessage
    if err := json.Unmarshal(event.Data, &msg); err != nil {
        return err
    }

    if strings.TrimSpace(msg.Message) != "!hello" {
        return nil
    }

    playerID := msg.EOSID
    if playerID == "" {
        playerID = msg.SteamID
    }
    return p.apis.RconAPI.SendWarningToPlayer(playerID, "Hello from Squad Aegis.")
}
```

Key points:

- `PluginEvent.Data` is `json.RawMessage`. Define the payload struct you need in your plugin and unmarshal it yourself.
- `GetDefinition()` should describe runtime behavior only. Do not duplicate manifest identity fields there.
- Event-driven plugins can often return `nil` from `Start`. Use `LongRunning: true` only when the plugin actually maintains background work.
- If your plugin depends on a connector, declare it in `RequiredConnectors` or `OptionalConnectors`.

## Build a Native Connector

Connectors are smaller than plugins. They expose a stable `Invoke` entrypoint and manage their own lifecycle.

The connector interface is:

```go
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
```

A minimal connector looks like this:

```go
package main

import (
    "context"

    connectorrpc "go.codycody31.dev/squad-aegis/pkg/connectorrpc"
)

type helloConnector struct {
    config map[string]interface{}
}

func main() {
    connectorrpc.Serve(&helloConnector{})
}

func (c *helloConnector) GetDefinition() connectorrpc.ConnectorDefinition {
    return connectorrpc.ConnectorDefinition{
        ConnectorID: "com.example.connectors.hello",
        ConfigSchema: connectorrpc.ConfigSchema{
            Fields: []connectorrpc.ConfigField{},
        },
    }
}

func (c *helloConnector) Initialize(config map[string]interface{}) error {
    c.config = config
    return nil
}

func (c *helloConnector) Start(context.Context) error         { return nil }
func (c *helloConnector) Stop() error                         { return nil }
func (c *helloConnector) GetStatus() connectorrpc.ConnectorStatus { return connectorrpc.ConnectorStatusRunning }
func (c *helloConnector) GetConfig() map[string]interface{}   { return c.config }
func (c *helloConnector) UpdateConfig(config map[string]interface{}) error {
    c.config = config
    return nil
}

func (c *helloConnector) Invoke(ctx context.Context, req *connectorrpc.ConnectorInvokeRequest) (*connectorrpc.ConnectorInvokeResponse, error) {
    _ = ctx
    return &connectorrpc.ConnectorInvokeResponse{
        V:    "1",
        OK:   true,
        Data: map[string]interface{}{"message": "pong"},
    }, nil
}
```

Key points:

- A connector does not receive `HostAPIs`; it is its own boundary.
- `Invoke` is where the useful work happens.
- The request/response envelope currently uses version `"1"` in `V`.
- `instance_key` and `legacy_ids` belong in the connector manifest, not in `GetDefinition()`.

## Host APIs Available to Plugins

Plugins receive `*pluginrpc.HostAPIs` during `Initialize`. The current SDK exposes:

- `LogAPI`
- `RconAPI`
- `ServerAPI`
- `DatabaseAPI`
- `RuleAPI`
- `AdminAPI`
- `EventAPI`
- `DiscordAPI`
- `ConnectorAPI`

Two practical patterns matter:

- Use `RconAPI` and `LogAPI` first. Many moderation and automation plugins need nothing else.
- Use `ConnectorAPI.Call(...)` when a plugin needs to talk to a connector.

Example:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

resp, err := apis.ConnectorAPI.Call(ctx, "com.example.connectors.hello", &pluginrpc.ConnectorInvokeRequest{
    V:    "1",
    Data: map[string]interface{}{"action": "ping"},
})
```

## Build the Binary

Native plugins and connectors currently target Linux only.

Build manually:

```bash
GOOS=linux GOARCH=amd64 go build -o dist/my-plugin .
GOOS=linux GOARCH=amd64 go build -o dist/my-connector .
```

Or use the checked-in helper scripts from this repository:

```bash
./scripts/package-example-native-plugin.sh
./scripts/package-example-native-connector.sh
```

Both scripts can build multiple targets:

```bash
TARGETS=linux/amd64,linux/arm64 ./scripts/package-example-native-plugin.sh
TARGETS=linux/amd64,linux/arm64 ./scripts/package-example-native-connector.sh
```

## Bundle Layout

Every bundle is a zip archive containing:

- `manifest.json`
- one or more binaries under `bin/...`

Signed bundles also include:

- `manifest.sig`
- `manifest.pub`

Example layout:

```text
my-extension.zip
├── manifest.json
├── manifest.sig
├── manifest.pub
└── bin/
    ├── linux-amd64/my-extension
    └── linux-arm64/my-extension
```

The `library_path` in `manifest.json` points at the binary Aegis should execute for a matching target.

## Plugin Manifest Example

```json
{
  "plugin_id": "com.example.plugins.hello",
  "name": "Hello Plugin",
  "description": "Replies to a chat command.",
  "version": "0.1.0",
  "author": "Example Team",
  "license": "MIT",
  "official": false,
  "targets": [
    {
      "min_host_api_version": 1,
      "required_capabilities": ["api.rcon", "events.rcon"],
      "target_os": "linux",
      "target_arch": "amd64",
      "sha256": "REPLACE_WITH_BINARY_SHA256",
      "library_path": "bin/linux-amd64/hello-plugin"
    }
  ]
}
```

## Connector Manifest Example

```json
{
  "connector_id": "com.example.connectors.hello",
  "name": "Hello Connector",
  "description": "Responds to ping requests.",
  "version": "0.1.0",
  "author": "Example Team",
  "license": "MIT",
  "official": false,
  "instance_key": "",
  "legacy_ids": [],
  "targets": [
    {
      "min_host_api_version": 1,
      "required_capabilities": [],
      "target_os": "linux",
      "target_arch": "amd64",
      "sha256": "REPLACE_WITH_BINARY_SHA256",
      "library_path": "bin/linux-amd64/hello-connector"
    }
  ]
}
```

Manifest rules that matter in practice:

- Use a stable reverse-DNS identifier such as `com.example.plugins.foo`.
- `sha256` must match the shipped binary bytes exactly.
- `library_path` points to an executable binary, not a shared object.
- `targets` is the supported format for native bundles.
- `min_host_api_version` should be `1` unless the runtime contract changes in a future Aegis release.
- For connectors, `instance_key` is optional. Use it only when multiple connector IDs should resolve to the same underlying connector instance.
- For connectors, `legacy_ids` is optional and only needed for migration compatibility.

## Choose Capabilities Deliberately

The current native host API version is `1`.

The current host capability strings are:

```text
entrypoint.get_aegis_plugin
api.rcon
api.server
api.database
api.rule
api.admin
api.discord
api.connector
api.event
api.log
events.rcon
events.log
events.system
events.connector
events.plugin
```

A few common mappings:

- A plugin that reads RCON chat and warns players should declare `api.rcon` and `events.rcon`.
- A plugin that calls connectors should declare `api.connector`.
- A plugin that only logs and handles system events should declare `api.log` and `events.system`.
- A connector may not need any required capabilities at all.

Declare only what you actually need. It keeps the contract clearer and avoids confusing install-time failures.

## Sign Bundles for Shared or Production Environments

For local development, unsigned bundles are usually enough if the host is configured with:

```yaml
plugins:
  allow_unsafe_sideload: true
```

For any shared or production environment, sign the bundle.

This repository includes helpers for both extension types:

```bash
./scripts/sign-plugin-bundle.sh
./scripts/sign-connector-bundle.sh
```

The signing tool expects a base64-encoded Ed25519 private key file. It writes:

- `manifest.sig`
- `manifest.pub`
- a signed zip archive

Important rules:

- `manifest.pub` must be present in `plugins.trusted_signing_keys` on the Aegis host.
- A valid signature from an unknown key is still rejected.
- Aegis re-verifies stored signatures at server start, so rotating trust keys can invalidate old bundles.

## Upload and Enable in Squad Aegis

For plugins:

1. Upload the bundle in `/sudo/plugins`.
2. Wait for the package to reach `ready`.
3. Open the target server's plugins page.
4. Add the plugin to that server and fill in its config.

For connectors:

1. Open `/connectors`.
2. Upload the connector bundle.
3. Create or update the connector instance and configure it.

If the UI reports `pending restart`, restart Aegis before continuing.

## Troubleshooting

- `plugin_id` or `connector_id` mismatch: the manifest ID does not match the ID returned by `GetDefinition()`.
- `plugin checksum mismatch` or `connector checksum mismatch`: the `sha256` in the manifest does not match the binary in the archive.
- No matching target for the host: the bundle does not contain the current Linux architecture, or `min_host_api_version` is too high.
- Unsupported capabilities: the bundle declares capabilities this Aegis build does not expose.
- Signed bundle rejected: the manifest may be valid, but the public key is not present in `plugins.trusted_signing_keys`.
- Plugin never sees events: the plugin did not list the event type in `GetDefinition().Events`, or the manifest capabilities do not include the matching `events.*` capability.
- Plugin blocks the server workflow: host API calls are synchronous RPC. Long-running work belongs in your own goroutines or background loop, not inline in a hot event path.
- Connector calls time out: keep `Invoke` small and deterministic. Do network work with clear timeouts and return structured errors.

## Recommended Workflow

If you are starting fresh, this is a good default:

1. Copy the closest example from `examples/`.
2. Replace the IDs, config schema, and business logic.
3. Build a single `linux/amd64` target first.
4. Package it and upload it unsigned to a local Aegis instance with `plugins.allow_unsafe_sideload: true`.
5. Once the flow works end-to-end, add additional targets and sign the bundle.

That path gets you to a working extension quickly without hiding how the actual runtime contract works.
