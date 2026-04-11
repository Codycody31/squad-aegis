---
title: Creating Native Plugins
---

This guide covers the current native plugin path for Squad Aegis: writing a plugin in Go, compiling it to a Linux `.so`, bundling it for upload, and enabling it in the UI.

## What a Native Plugin Is

Bundled plugins ship inside the main Aegis server binary. Native plugins are separate Go plugins compiled as shared objects and loaded at runtime.

Current constraints:

- Native plugins are Linux-only.
- The server only loads `.so` plugins through Go's `plugin` package.
- The plugin binary Aegis selects at install time must match the server's `GOOS` and `GOARCH`, and its target requirements must be satisfied by the host plugin runtime.
- Native plugins cannot be unloaded safely. Installing a new plugin can load immediately, but updating or removing a loaded plugin may require an Aegis restart.
- External plugins must import `pkg/pluginapi`, not `internal/...`.

## Plugin Shape

A native plugin exposes one required symbol:

```go
func GetAegisPlugin() pluginapi.PluginDefinition
```

That definition must include a `CreateInstance` function which returns a type implementing the Aegis plugin interface.

The public SDK lives in `go.codycody31.dev/squad-aegis/pkg/pluginapi` and exposes:

- plugin lifecycle types such as `Plugin`, `PluginDefinition`, `PluginAPIs`, and `PluginEvent`
- config schema types such as `ConfigSchema`, `ConfigField`, and `FieldType*`
- event constants and event payload structs such as `EventTypeRconChatMessage` and `RconChatMessageData`
- API interfaces such as `RconAPI`, `ServerAPI`, `DatabaseAPI`, `RuleAPI`, `AdminAPI`, `DiscordAPI`, and `LogAPI`

## Minimal Example

A complete example plugin now lives at `examples/native-plugin-hello/main.go`. It listens for `!hello` in chat and replies to the player with a warning message.

The important pieces are:

```go
package main

import (
  "context"
  "fmt"
  "strings"

  pluginapi "go.codycody31.dev/squad-aegis/pkg/pluginapi"
)

func GetAegisPlugin() pluginapi.PluginDefinition {
  return pluginapi.PluginDefinition{
    ID:          "com.squad-aegis.plugins.examples.hello",
    Name:        "Hello Example",
    Description: "Replies to players who type !hello in chat.",
    Version:     "0.1.0",
    Source:      pluginapi.PluginSourceNative,
    ConfigSchema: pluginapi.ConfigSchema{
      Fields: []pluginapi.ConfigField{
        {
          Name:     "trigger",
          Type:     pluginapi.FieldTypeString,
          Default:  "!hello",
        },
      },
    },
    Events: []pluginapi.EventType{
      pluginapi.EventTypeRconChatMessage,
    },
    CreateInstance: func() pluginapi.Plugin {
      return &helloPlugin{}
    },
  }
}

func (p *helloPlugin) HandleEvent(event *pluginapi.PluginEvent) error {
  data, ok := event.Data.(*pluginapi.RconChatMessageData)
  if !ok {
    return fmt.Errorf("unexpected event payload %T", event.Data)
  }

  if strings.EqualFold(strings.TrimSpace(data.Message), "!hello") {
    return p.apis.RconAPI.SendWarningToPlayer(
      data.PreferredPlayerID(),
      "Hello from a native plugin.",
    )
  }

  return nil
}

func (p *helloPlugin) Start(context.Context) error { return nil }
func (p *helloPlugin) Stop() error                 { return nil }
```

## Start an External Plugin Module

If you are building outside the Squad Aegis repository, start with a normal Go module and pin the Aegis dependency to the server line you are targeting:

```bash
mkdir my-aegis-plugin
cd my-aegis-plugin
go mod init example.com/my-aegis-plugin
go get go.codycody31.dev/squad-aegis@v1.2.3
```

Build against a recent Aegis module release that exposes the APIs your plugin needs. Native bundle compatibility is no longer tied directly to the app's major/minor version.

## Build the `.so`

Build on Linux with plugin mode enabled:

```bash
go build -buildmode=plugin -o hello.so ./examples/native-plugin-hello
```

For an external plugin repository, the same pattern applies:

```bash
go build -buildmode=plugin -o dist/my-plugin.so .
```

Practical advice:

- Build with the same Go toolchain family you use for the target Aegis deployment.
- Build one `.so` per target platform you want to ship in the bundle, for example `linux/amd64` and `linux/arm64`.
- Set `min_host_api_version` to the oldest host plugin runtime you intend to support.
- Declare `required_capabilities` for the host features your plugin actually depends on.

The checked-in example packager can build a multi-target bundle:

```bash
TARGETS=linux/amd64,linux/arm64 ./scripts/package-example-native-plugin.sh
```

The checked-in example defaults to:

- `min_host_api_version: 1`
- `required_capabilities: ["api.rcon", "events.rcon"]`

## Bundle Format

Aegis installs native plugins from a zip bundle. Every bundle must contain:

- `manifest.json`
- the compiled `.so`

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
    ├── linux-amd64/hello.so
    └── linux-arm64/hello.so
```

Example `manifest.json`:

```json
{
  "plugin_id": "com.squad-aegis.plugins.examples.hello",
  "name": "Hello Example",
  "description": "Replies to players who type !hello in chat.",
  "version": "0.1.0",
  "official": false,
  "entry_symbol": "GetAegisPlugin",
  "targets": [
    {
      "min_host_api_version": 1,
      "required_capabilities": ["api.rcon", "events.rcon"],
      "target_os": "linux",
      "target_arch": "amd64",
      "sha256": "REPLACE_WITH_SHA256_OF_LINUX_AMD64_SO",
      "library_path": "bin/linux-amd64/hello.so"
    },
    {
      "min_host_api_version": 1,
      "required_capabilities": ["api.rcon", "events.rcon"],
      "target_os": "linux",
      "target_arch": "arm64",
      "sha256": "REPLACE_WITH_SHA256_OF_LINUX_ARM64_SO",
      "library_path": "bin/linux-arm64/hello.so"
    }
  ]
}
```

Notes:

- Use a stable plugin ID. Reverse-DNS style IDs such as `com.squad-aegis.plugins.examples.hello` are recommended.
- `entry_symbol` must be `GetAegisPlugin`.
- `targets` is the only supported format. Each target describes one shipped binary.
- Every target must include `min_host_api_version`, `target_os`, `target_arch`, `library_path`, and `sha256`.
- `required_capabilities` is optional but recommended. Use it whenever your plugin depends on specific host-exposed APIs or event families.
- `sha256` is **required** and must be the hex-encoded SHA-256 of the target's `.so` bytes. The manifest signature only covers the manifest itself, so this field is what binds the signed manifest to the on-disk library — Aegis rejects any bundle where the hashes disagree.
- Aegis selects the most capable target that matches the current host OS, architecture, host API version, and required capabilities.
- A single bundle can include multiple binaries for future Squad Aegis platform support, even if the current host only loads one of them.
- Signed bundles must include both `manifest.sig` and `manifest.pub` together.
- `manifest.sig` is the base64 encoding of the 64 raw Ed25519 signature bytes.
- `manifest.pub` is the base64 encoding of the 32 raw Ed25519 public key bytes.
- **The public key embedded in `manifest.pub` must appear in `plugins.trusted_signing_keys`** on the host that receives the upload. A cryptographically valid signature from an unknown key is rejected — without an operator-configured trust anchor, anyone with upload access could ship their own keypair. Set `plugins.trusted_signing_keys` to a comma-separated list of base64-encoded Ed25519 public keys (one per signer you trust).
- Aegis canonicalizes `manifest.json` deterministically before signing and before verification, so key order and whitespace do not affect the signature.
- The signature payload is the canonicalized `manifest.json` bytes only. Because `sha256` is a required manifest field, the signature transitively binds the on-disk `.so` contents.
- Stored bundles are re-verified at each server start against the current trust store. Rotating a key in `plugins.trusted_signing_keys` will retroactively mark bundles signed with the old key as untrusted; re-upload them with a trusted signer to re-enable them.
- Unsafe archive paths and symlinks are rejected during install.

Current host runtime contract:

- `min_host_api_version` must be less than or equal to the host's current plugin runtime version.
- The current host plugin runtime version is `1`.
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

`plugins.trusted_signing_keys` is a comma-separated list of base64-encoded Ed25519 public keys. A cryptographically valid signature from a key that is not in this allowlist is rejected — otherwise anyone with upload access could ship their own keypair inside the bundle and defeat the "signed sideload" gate.

Uploaded native plugins and connectors are treated as sideloaded packages even when they are signed.

### Trust store rotation

Aegis stores the signature and public-key bytes alongside each installed package and **re-verifies them against `plugins.trusted_signing_keys` at every server start**. This means:

- Removing a key from `plugins.trusted_signing_keys` retroactively marks every package signed with that key as untrusted on the next restart.
- Rotating your signing key requires re-signing and re-uploading existing bundles (or keeping the old public key in the allowlist until migration is complete).

### Upgrade from earlier versions

If you are upgrading from a build that predates `plugins.trusted_signing_keys`, packages installed under the old scheme have no stored signature material. On the first start after upgrading, those packages will flip to `error` state with the message `"plugin signature cannot be re-verified against trusted keys"`.

To recover:

1. Generate an Ed25519 keypair (`scripts/sign-plugin-bundle.sh` shows the signing workflow).
2. Set `plugins.trusted_signing_keys` to the base64-encoded public key.
3. Re-sign each affected bundle and re-upload it through `/sudo/plugins` (or `/sudo/connectors`).

For development or single-operator trust models, `plugins.allow_unsafe_sideload = true` bypasses signing entirely. Do not enable this in production.

## Install and Enable

1. Open the Aegis sudo area and go to `/sudo/plugins`.
2. Use the upload flow to sideload your zip bundle.
3. After the package shows as `ready`, go to the target server's plugins page.
4. Add the plugin to that server and configure its fields.

If the package shows `pending_restart`, restart Aegis before enabling or validating the updated version.

## Using the Available APIs

Your plugin instance receives `*pluginapi.PluginAPIs` in `Initialize`. The most useful APIs are:

- `RconAPI` for broadcasts, warnings, kicks, bans, and squad removal
- `ServerAPI` for current players, squads, admins, and server metadata
- `DatabaseAPI` for plugin-scoped key/value storage
- `RuleAPI` for read-only access to the current server's rules and escalation actions
- `AdminAPI` for temporary admin management
- `DiscordAPI` for sending messages or embeds when the Discord connector is available
- `EventAPI` for plugin-generated events
- `LogAPI` for structured logs that show up under plugin logs

Start with `RconAPI` and `LogAPI`. They are enough for many moderation and automation plugins.

## Common Failure Modes

- `failed to resolve GetAegisPlugin`: your exported symbol name or signature is wrong
- `plugin does not support host OS ...`: none of the bundle targets match the current OS
- `plugin does not support host architecture ... on ...`: the bundle has the right OS but not the current architecture
- `plugin requires host API version >= ...`: the matching target requires a newer plugin runtime than the host exposes
- `plugin requires unsupported host capabilities: ...`: the matching target depends on host features this Aegis build does not expose
- `plugin checksum mismatch`: `manifest.json` does not match the shipped `.so`
- `plugin manifest target N is missing sha256`: every manifest target must include `sha256` of its library
- `manifest signed by untrusted public key`: the bundle's `manifest.pub` is not in `plugins.trusted_signing_keys`
- `unsigned sideloads are disabled`: enable unsafe sideloads in config, or sign the bundle with a key listed in `plugins.trusted_signing_keys`
- `plugin runtime path rejected`: the stored `runtime_path` resolves outside `plugins.runtime_dir` (DB tampering or a misconfigured `runtime_dir`)
- `plugin runtime library checksum mismatch`: the on-disk `.so` no longer matches the checksum recorded at install time
- `plugin signature cannot be re-verified against trusted keys`: the stored signature no longer verifies against the current `plugins.trusted_signing_keys` — re-sign and re-upload, or temporarily enable `allow_unsafe_sideload`

## Current Gaps

The native plugin system is usable, but still intentionally narrow:

- v1 only supports server plugins, not dynamically loaded connectors
- there is no built-in plugin catalog yet; distribution is upload-only
- hot reloading updated native plugins is not supported because Go plugins cannot be unloaded

If you are starting from scratch, use the checked-in example as the baseline and keep your plugin on the public `pkg/pluginapi` surface.
