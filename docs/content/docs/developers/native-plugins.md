---
title: Creating Native Plugins
---

This guide covers the native plugin path for Squad Aegis: writing a plugin in Go, compiling it to a standalone binary, bundling it for upload, and enabling it in the UI.

## What a Native Plugin Is

Bundled plugins ship inside the main Aegis server binary. Native plugins are separate Go programs that run as **isolated subprocesses** spawned by the Aegis host via [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin). The host and plugin communicate over `net/rpc` on a private pipe; the plugin never shares memory with the Aegis server.

Security posture:

- A crash, panic, or malicious payload inside a plugin terminates only the subprocess. It cannot corrupt Aegis memory, steal secrets from unrelated packages, or monkey-patch the host runtime.
- The host still verifies a SHA-256 checksum and (unless `allow_unsafe_sideload` is on) a manifest signature before launching the subprocess, so an unauthorized bundle is rejected before any plugin code runs.
- All host APIs the plugin can call are proxied through a generic RPC dispatcher (`HostAPI`) on the host side — plugins cannot reach into the host database, filesystem, or configuration directly.
- Killing a plugin is a single `SIGTERM` away, so uploading an updated bundle can replace the running subprocess without restarting Aegis.

Current constraints:

- Native plugins must be compiled for Linux. The host refuses bundles whose `target_os` is not `linux`.
- The plugin binary Aegis selects at install time must match the server's `GOOS`/`GOARCH` and its target requirements must be satisfied by the host plugin runtime.
- External plugins must import `pkg/pluginrpc`, not `internal/...`. The `pluginrpc` package is deliberately self-contained and has no dependency on the host internals.
- The v1 SDK exposes typed wrappers for `LogAPI`, `RconAPI`, `ServerAPI`, `DatabaseAPI`, `RuleAPI`, `AdminAPI`, `EventAPI`, `DiscordAPI`, and `ConnectorAPI`.

## Plugin Shape

A native plugin is a standalone Go program whose `main` calls `pluginrpc.Serve(impl)` where `impl` implements the `pluginrpc.Plugin` interface:

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

The public SDK lives in `go.codycody31.dev/squad-aegis/pkg/pluginrpc` and exposes:

- The `Plugin` interface and `Serve(impl)` entrypoint
- Wire-safe lifecycle types: `PluginDefinition`, `PluginEvent`, `PluginCommand`, `CommandResult`, `PluginStatus`
- Config schema types: `ConfigSchema`, `ConfigField`, `FieldType*`
- Typed `HostAPIs` wrappers: `LogAPI`, `RconAPI`, `ServerAPI`, `DatabaseAPI`, `RuleAPI`, `AdminAPI`, `EventAPI`, `DiscordAPI`, `ConnectorAPI`

`PluginEvent.Data` arrives as `json.RawMessage`. Your plugin chooses its own struct to unmarshal into — the SDK does not force you to import the host event package.

## Manifest vs Runtime: Who Owns What

Every native plugin ships as a zip bundle with **two** sources of truth the host merges at load time:

| Field group | Lives in | Why |
|---|---|---|
| `plugin_id`, `name`, `description`, `version`, `author`, `license`, `official` | `manifest.json` | Immutable identity; operators need to see it before the binary is allowed to run. Part of the signed payload. |
| `min_host_api_version`, `required_capabilities`, `target_os`, `target_arch`, `library_path`, `sha256` | `manifest.json` (per target) | Distribution/compatibility gate; operators evaluate it at upload time before any code runs. |
| `config_schema`, `events`, `long_running`, `allow_multiple_instances`, `required_connectors`, `optional_connectors` | Plugin binary (returned from `GetDefinition()` over RPC) | Runtime behavior; only the binary knows it. |

The plugin author writes **identity/compatibility in `manifest.json`** (operator-facing, cryptographically signed) and **behavior in the `definition()` Go function** (runtime contract with the host). The host cross-checks `manifest.plugin_id == wire.PluginID` during load and rejects mismatches.

What this means for your plugin's `GetDefinition()`:

- Do **not** return `Name`, `Version`, `Author`, `Source`, `Official`, etc. — those fields no longer exist on `pluginrpc.PluginDefinition`.
- Do return `PluginID` (echoed back for the cross-check), `ConfigSchema`, `Events`, `LongRunning`, `AllowMultipleInstances`, `RequiredConnectors`, `OptionalConnectors`.

## Minimal Example

A complete example lives at `examples/native-plugin-hello/main.go`. It listens for `!hello` in chat and replies to the player with a warning message. The relevant pieces:

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
    EOSID      string `json:"eos_id"`
    SteamID    string `json:"steam_id"`
    PlayerName string `json:"player_name"`
    Message    string `json:"message"`
}

func main() {
    pluginrpc.Serve(&helloPlugin{})
}

// definition() returns only the runtime/behavioral contract. Identity
// (name, version, author, license, official) lives in manifest.json
// alongside the compiled binary.
func (p *helloPlugin) GetDefinition() pluginrpc.PluginDefinition {
    return pluginrpc.PluginDefinition{
        PluginID: "com.squad-aegis.plugins.examples.hello",
        ConfigSchema: pluginrpc.ConfigSchema{
            Fields: []pluginrpc.ConfigField{
                {Name: "trigger", Type: pluginrpc.FieldTypeString, Default: "!hello"},
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

func (p *helloPlugin) HandleEvent(event *pluginrpc.PluginEvent) error {
    if event.Type != "RCON_CHAT_MESSAGE" {
        return nil
    }
    var data rconChatMessage
    if err := json.Unmarshal(event.Data, &data); err != nil {
        return fmt.Errorf("decode chat event: %w", err)
    }
    if !strings.EqualFold(strings.TrimSpace(data.Message), "!hello") {
        return nil
    }
    playerID := data.EOSID
    if playerID == "" {
        playerID = data.SteamID
    }
    return p.apis.RconAPI.SendWarningToPlayer(playerID, "Hello from a native plugin.")
}

func (p *helloPlugin) Start(context.Context) error            { return nil }
func (p *helloPlugin) Stop() error                            { return nil }
func (p *helloPlugin) GetStatus() pluginrpc.PluginStatus      { return pluginrpc.PluginStatusRunning }
func (p *helloPlugin) GetConfig() map[string]interface{}      { return p.config }
func (p *helloPlugin) UpdateConfig(c map[string]interface{}) error { p.config = c; return nil }
func (p *helloPlugin) GetCommands() []pluginrpc.PluginCommand { return nil }
func (p *helloPlugin) ExecuteCommand(string, map[string]interface{}) (*pluginrpc.CommandResult, error) {
    return nil, fmt.Errorf("no commands")
}
func (p *helloPlugin) GetCommandExecutionStatus(string) (*pluginrpc.CommandExecutionStatus, error) {
    return nil, fmt.Errorf("no commands")
}
```

## Start an External Plugin Module

If you are building outside the Squad Aegis repository, start with a normal Go module and pin the Aegis dependency to the server line you are targeting:

```bash
mkdir my-aegis-plugin
cd my-aegis-plugin
go mod init example.com/my-aegis-plugin
go get go.codycody31.dev/squad-aegis@v1.2.3
```

Only `pkg/pluginrpc` is imported — the rest of the Aegis module graph is irrelevant to your plugin.

## Build the Binary

Build on Linux with the standard `go build` command:

```bash
go build -o hello-plugin ./examples/native-plugin-hello
```

For an external plugin repository, the same pattern applies:

```bash
go build -o dist/my-plugin .
```

Practical advice:

- Build with the same Go toolchain family you use for the target Aegis deployment.
- Build one binary per target platform you want to ship in the bundle, for example `linux/amd64` and `linux/arm64`.
- Set `min_host_api_version` to the oldest host plugin runtime you intend to support.
- Declare `required_capabilities` for the host features your plugin actually depends on.

The checked-in example packager can build a multi-target bundle:

```bash
TARGETS=linux/amd64,linux/arm64 ./scripts/package-example-native-plugin.sh
```

## Bundle Format

Aegis installs native plugins from a zip bundle. Every bundle must contain:

- `manifest.json`
- the compiled binary (any filename; referenced from `manifest.targets[].library_path`)

Unsigned bundles stop there. Signed bundles add a signature pair:

- `manifest.sig`
- `manifest.pub`

Example layout:

```text
hello-plugin.zip
├── manifest.json
├── manifest.sig
├── manifest.pub
└── bin/
    ├── linux-amd64/hello-plugin
    └── linux-arm64/hello-plugin
```

Example `manifest.json` (identity + distribution metadata only):

```json
{
  "plugin_id": "com.squad-aegis.plugins.examples.hello",
  "name": "Hello Example",
  "description": "Replies to players who type !hello in chat.",
  "version": "0.1.0",
  "author": "Squad Aegis",
  "license": "MIT",
  "official": false,
  "targets": [
    {
      "min_host_api_version": 1,
      "required_capabilities": ["api.rcon", "events.rcon"],
      "target_os": "linux",
      "target_arch": "amd64",
      "sha256": "REPLACE_WITH_SHA256_OF_LINUX_AMD64_BINARY",
      "library_path": "bin/linux-amd64/hello-plugin"
    },
    {
      "min_host_api_version": 1,
      "required_capabilities": ["api.rcon", "events.rcon"],
      "target_os": "linux",
      "target_arch": "arm64",
      "sha256": "REPLACE_WITH_SHA256_OF_LINUX_ARM64_BINARY",
      "library_path": "bin/linux-arm64/hello-plugin"
    }
  ]
}
```

Notes:

- Use a stable plugin ID. Reverse-DNS style IDs such as `com.squad-aegis.plugins.examples.hello` are recommended.
- There is **no** `entry_symbol` field anymore. The subprocess launch uses hashicorp/go-plugin's handshake, not a Go plugin entry point.
- There is **no** `config_schema` in the manifest. The plugin binary is the source of truth for its config schema; it is fetched over RPC at load time and cross-checked against the operator-supplied config before `Initialize` is called.
- `targets` is the only supported format. Each target describes one shipped binary.
- Every target must include `min_host_api_version`, `target_os`, `target_arch`, `library_path`, and `sha256`.
- `library_path` points at an **executable binary**, not a `.so`. There is no required file extension.
- `required_capabilities` is optional but recommended. Use it whenever your plugin depends on specific host-exposed APIs or event families.
- `sha256` is **required** and must be the hex-encoded SHA-256 of the target's binary bytes. The manifest signature only covers the manifest itself, so this field is what binds the signed manifest to the on-disk binary — Aegis rejects any bundle where the hashes disagree.
- Aegis selects the most capable target that matches the current host OS, architecture, host API version, and required capabilities.
- The host **also** spawns the plugin binary once during install (the "peek" step) and compares the `PluginID` echoed from `GetDefinition()` against the manifest's `plugin_id`. A mismatch aborts the install.
- Signed bundles must include both `manifest.sig` and `manifest.pub` together.
- **The public key embedded in `manifest.pub` must appear in `plugins.trusted_signing_keys`** on the host that receives the upload. A cryptographically valid signature from an unknown key is treated as unverified — without an operator-configured trust anchor, anyone with upload access could ship their own keypair.
- Stored bundles are re-verified at each server start against the current trust store. Rotating a key in `plugins.trusted_signing_keys` will retroactively mark bundles signed with the old key as untrusted and quarantine the runtime file.
- Unsafe archive paths and symlinks are rejected during install.

### Connector manifest

Connector bundles use the same split with two additional identity fields. Example:

```json
{
  "connector_id": "com.squad-aegis.connectors.examples.hello",
  "name": "Hello Connector",
  "description": "Responds to JSON invoke action ping.",
  "version": "0.1.0",
  "author": "Squad Aegis",
  "license": "MIT",
  "official": false,
  "instance_key": "",
  "legacy_ids": [],
  "targets": [
    {
      "min_host_api_version": 1,
      "target_os": "linux",
      "target_arch": "amd64",
      "sha256": "REPLACE_WITH_SHA256",
      "library_path": "bin/linux-amd64/hello-connector"
    }
  ]
}
```

- `instance_key` is an optional routing hint — when non-empty, multiple connector IDs sharing the same key share a single connector instance. Leave empty for independent connectors.
- `legacy_ids` lists previous connector IDs so operators migrating from an older build can continue to reference the connector by its old ID.

The connector binary's `GetDefinition()` returns only `ConnectorID` (echoed) and `ConfigSchema`.

## How the Subprocess Lifecycle Works

1. **Upload**: An operator uploads the bundle through `/sudo/plugins`. The host verifies the bundle, writes the binary to `plugins.runtime_dir/<plugin_id>/<version>/<binary>` with owner-only execute permissions, and persists the package row in the database.
2. **Peek**: The host spawns a temporary subprocess, calls `GetDefinition()` over RPC, compares the returned plugin ID to the manifest, and immediately kills the subprocess. This lets the host register the plugin with its registry without leaving a long-running subprocess attached.
3. **Instance spawn**: When an admin enables the plugin on a server, `CreateInstance()` starts a **fresh subprocess** dedicated to that instance. Each server instance gets its own OS process.
4. **Initialize**: The host opens an internal broker channel, registers the `HostAPI` RPC dispatcher (backed by the in-process `*PluginAPIs`), passes the broker ID to the plugin via `InitializeArgs`, and invokes `Plugin.Initialize`. The plugin dials the broker ID to get a `*HostAPIs` wrapper it can use to call back into the host.
5. **Events**: The host forwards every event the plugin subscribed to via `HandleEvent`. `PluginEvent.Data` is a `json.RawMessage`, so your plugin unmarshals into whatever Go struct it wants.
6. **Teardown**: `Stop()` fires the plugin's Stop RPC, closes the HostAPI broker channel, and sends SIGTERM to the subprocess. Uninstalling the package additionally removes the on-disk binary and DB row.

If the subprocess crashes (exits non-zero, panics, or closes the RPC pipe), the host reports a transport error on the next RPC call. A subsequent admin action (Restart / Disable / Enable on the instance page) spawns a fresh subprocess from the same binary.

## Host Capability Contract

- The current host plugin runtime version is `1`.
- `min_host_api_version` must be less than or equal to the host's current plugin runtime version.
- The current host capabilities are:
  - `entrypoint.get_aegis_plugin`
  - `api.rcon`
  - `api.server`
  - `api.database`
  - `api.rule`
  - `api.admin`
  - `api.discord`
  - `api.event`
  - `api.log`
  - `events.rcon`
  - `events.log`
  - `events.system`
  - `events.connector`
  - `events.plugin`

## Signatures

Signed bundles carry both the signature and the public key:

- `manifest.sig` must validate against the bundled `manifest.pub` AND
- `manifest.pub` must appear in `plugins.trusted_signing_keys` on the host receiving the upload

`plugins.trusted_signing_keys` is a comma-separated list of base64-encoded Ed25519 public keys. A cryptographically valid signature from a key that is not in this allowlist is treated as unverified — otherwise anyone with upload access could ship their own keypair inside the bundle and defeat the "signed sideload" gate.

Because native plugins now run in a separate process with narrowly-typed RPC channels, the signing private key is no longer equivalent to host root. It is still the **only** thing standing between an untrusted upload and a spawned subprocess that has full HostAPI access, so treat it accordingly: hardware-backed (YubiKey / HSM) or offline-signed are recommended for production.

### Trust store rotation

Aegis stores the signature and public-key bytes alongside each installed package and **re-verifies them against `plugins.trusted_signing_keys` at every server start**. If a package no longer verifies and `plugins.allow_unsafe_sideload` is off, the runtime binary is removed from disk and the database row is marked as errored. The operator must re-sign and re-upload the bundle (or temporarily enable unsafe sideload) to recover.

## Install and Enable

1. Open the Aegis sudo area and go to `/sudo/plugins`.
2. Use the upload flow to sideload your zip bundle.
3. After the package shows as `ready`, go to the target server's plugins page.
4. Add the plugin to that server and configure its fields.

## Using the Available APIs

Your plugin instance receives `*pluginrpc.HostAPIs` in `Initialize`. The most useful APIs are:

- `RconAPI` for broadcasts, warnings, kicks, bans, and squad removal
- `ServerAPI` for current players, squads, admins, and server metadata (returned as `map[string]interface{}` on the wire)
- `DatabaseAPI` for plugin-scoped key/value storage
- `RuleAPI` for read-only access to the current server's rules and escalation actions
- `AdminAPI` for temporary admin management
- `DiscordAPI` for sending messages or embeds when the Discord connector is available
- `EventAPI` for plugin-generated events (note: `SubscribeToEvents` is NOT exposed to subprocesses — events are delivered to your plugin via the `HandleEvent` RPC call)
- `LogAPI` for structured logs that show up under plugin logs

Start with `RconAPI` and `LogAPI`. They are enough for many moderation and automation plugins.

## Common Failure Modes

- `failed to spawn plugin subprocess`: the runtime binary is missing, not executable, or the `GOOS`/`GOARCH` does not match
- `plugin does not support host OS ...`: none of the bundle targets match the current OS
- `plugin does not support host architecture ... on ...`: the bundle has the right OS but not the current architecture
- `plugin requires host API version >= ...`: the matching target requires a newer plugin runtime than the host exposes
- `plugin requires unsupported host capabilities: ...`: the matching target depends on host features this Aegis build does not expose
- `plugin checksum mismatch`: `manifest.json` does not match the shipped binary
- `plugin runtime binary checksum mismatch`: the on-disk binary no longer matches the checksum recorded at install time — the runtime directory was tampered with
- `plugin runtime path rejected`: the stored `runtime_path` resolves outside `plugins.runtime_dir` (DB tampering or a misconfigured `runtime_dir`)
- `unsigned sideloads are disabled`: enable unsafe sideloads in config, or sign the bundle with a key listed in `plugins.trusted_signing_keys`
- `plugin signature cannot be re-verified against trusted keys`: the stored signature no longer verifies — re-sign and re-upload, or temporarily enable `allow_unsafe_sideload`

## Operator Configuration

Host-side knobs that tune the subprocess runtime (all under `plugins.`):

| Key | Default | Purpose |
|---|---|---|
| `plugins.native_enabled` | `true` | Master switch for the native plugin runtime |
| `plugins.runtime_dir` | `plugins` / `/etc/squad-aegis/plugins` in-container | Where extracted plugin binaries live. Must be absolute in production |
| `plugins.connector_runtime_dir` | `connectors` / `/etc/squad-aegis/connectors` | Same for connectors |
| `plugins.trusted_signing_keys` | *(empty)* | Comma-separated base64 Ed25519 public keys. Manifests signed by any other key are treated as unverified |
| `plugins.allow_unsafe_sideload` | `false` | Allow unsigned sideloads (dev/test only). Do NOT enable in production |
| `plugins.max_upload_size` | `52428800` (50 MiB) | Per-bundle upload cap |
| `plugins.host_api_rate_per_sec` | `50` | Sustained HostAPI RPC rate per plugin instance. Non-positive disables rate limiting |
| `plugins.host_api_burst` | `100` | Burst size for the HostAPI token bucket |
| `plugins.health_check_interval_seconds` | `10` | How often the host polls each subprocess for unexpected exit. Zero or negative disables health monitoring |
| `plugins.subprocess_uid` | `0` (inherit) | Non-zero UID drops plugin subprocesses to a dedicated user account on exec |
| `plugins.subprocess_gid` | `0` (defaults to UID) | Primary GID for the dropped user |
| `plugins.subprocess_groups` | *(empty)* | Comma-separated supplementary group IDs |
| `plugins.subprocess_no_new_privs` | `true` | Advisory; enforce via systemd `NoNewPrivileges=yes` on the Aegis unit so the prctl is inherited |

### Subprocess isolation posture

When a native plugin subprocess is spawned, the host applies the following hardening:

1. **UID/GID drop** — if `plugins.subprocess_uid` is non-zero, the plugin runs under that UID/GID with the configured supplementary groups. The default (zero) inherits the Aegis process UID. For production, provision a dedicated `aegis-plugins` user and set these knobs.
2. **Own process group** — `Setpgid=true` so the host can signal the entire subprocess tree on kill.
3. **No new privileges** — Go's `exec.Cmd` does not expose a `PR_SET_NO_NEW_PRIVS` hook directly, so the config knob is advisory. In production, run the Aegis server itself under a systemd unit with `NoNewPrivileges=yes` — the prctl is inherited by the subprocess across exec, which gives the same guarantee.
4. **HostAPI rate limiting** — each loaded plugin instance has its own `golang.org/x/time/rate.Limiter` that throttles RPC callbacks into the host. A compromised plugin that floods the host with `RconAPI.SendCommand` or `LogAPI.Info` is rate-limited to the configured budget; excess calls return a wire error so the plugin can back off.
5. **Health monitoring** — a background goroutine polls `goplugin.Client.Exited()` at `plugins.health_check_interval_seconds`. If the subprocess dies outside an intentional `Stop()`, the owning `PluginInstance` is flipped to `error` state and the operator sees the failure on the server's plugins page.
6. **Panic recovery** — every lifecycle call (`Initialize`, `Start`, `Stop`, `HandleEvent`) is wrapped in a recover so a panicking plugin call marks the instance as errored instead of crashing the host.

## Current Gaps

The native plugin system is usable and isolated, but still intentionally narrow:

- there is no built-in plugin catalog yet; distribution is upload-only
- `EventAPI.SubscribeToEvents` is not exposed to subprocess plugins (use the event list in your PluginDefinition and the `HandleEvent` RPC instead)
- host API calls are synchronous RPC; long-running operations inside the plugin should be handled with your own goroutines, not by blocking the HostAPI channel
- `PR_SET_NO_NEW_PRIVS` cannot currently be applied per-subprocess from Go's `exec.Cmd` — set it on the Aegis systemd unit so it inherits
- seccomp profiles are not applied by the host; a subprocess with untrusted code should be sandboxed at the deployment layer (systemd `SystemCallFilter=`, Docker `--security-opt seccomp=...`, etc.)

If you are starting from scratch, use the checked-in example as the baseline and keep your plugin on the public `pkg/pluginrpc` surface.
