# Squad Aegis WebAssembly guest ABI

This document defines the contract between **Aegis (host)** and **sideloaded WASM plugins and connectors**. It matches the same **`required_capabilities`** strings as native `.so` plugins (see `internal/plugin_manager/interfaces.go`); the host only binds imports that the package is allowed to use.

## Limits

| Limit | Value |
|-------|--------|
| Linear memory | 512 pages (32 MiB) default; see host runtime config |
| Host call timeout | 30s per import invocation |
| `host_call` request JSON | 256 KiB max |
| `host_call` response JSON | 512 KiB max |
| `connector_invoke` JSON | Same pattern as plugins; request data capped elsewhere in `ConnectorAPI` |

## Import module `aegis_host_v1`

All imports use the module name **`aegis_host_v1`**.

### `log(level i32, ptr i32, len i32)`

**Capability:** `api.log`

UTF-8 message at `ptr`/`len`. `level`: 0=info, 1=warn, 2=error, other=debug.

### `connector_invoke(id_ptr, id_len, req_ptr, req_len, out_ptr, out_cap, out_written_ptr) -> i32`

**Capability:** `api.connector`

Passes through to `ConnectorAPI.Call`. Return codes: `0` ok, `1` memory, `2` buffer.

### `host_call(cat_ptr, cat_len, method_ptr, method_len, req_ptr, req_len, out_ptr, out_cap, out_written_ptr) -> i32`

Unified JSON RPC-style surface for server/RCON/database/rules/admin/discord/event APIs.

Return codes:

| Code | Meaning |
|------|---------|
| 0 | Success; response JSON written (`{"ok":...}` envelope) |
| 1 | Memory read/write failure |
| 2 | Output buffer too small or request JSON over limit |
| 3 | Category not allowed (missing capability); host may not write `out` |
| 4 | Unknown category or JSON marshal failure |

- **cat**: UTF-8 category name (see table below).
- **method**: UTF-8 method name for that category.
- **req**: UTF-8 JSON object (may be empty; use zero `req_len` for `{}`).
- **out**: response JSON written at `out_ptr`; `out_written_ptr` receives little-endian `u32` byte length.

Response envelope (always JSON object):

```json
{"ok": true, "data": { ... }}
```

or

```json
{"ok": false, "error": "message"}
```

When `ok` is true, `data` holds the method result (object, array, or scalar depending on method).

#### Category → capability

| Category | Capability |
|----------|------------|
| `server` | `api.server` |
| `rcon` | `api.rcon` |
| `database` | `api.database` |
| `rule` | `api.rule` |
| `admin` | `api.admin` |
| `discord` | `api.discord` |
| `event` | `api.event` |

#### `server` methods

| Method | Request JSON | Data |
|--------|--------------|------|
| `GetServerID` | (empty) | `{"server_id":"<uuid>"}` |
| `GetServerInfo` | (empty) | `ServerInfo` |
| `GetPlayers` | (empty) | `[PlayerInfo,...]` |
| `GetAdmins` | (empty) | `[AdminInfo,...]` |
| `GetSquads` | (empty) | `[SquadInfo,...]` |

#### `database` methods

| Method | Request JSON | Data |
|--------|--------------|------|
| `GetPluginData` | `{"key":"..."}` | `{"value":"..."}` |
| `SetPluginData` | `{"key":"...","value":"..."}` | `null` |
| `DeletePluginData` | `{"key":"..."}` | `null` |

#### `rule` methods

| Method | Request JSON | Data |
|--------|--------------|------|
| `ListServerRules` | `{"parent_rule_id":"..."}` or `{}` (null parent) | `[RuleInfo,...]` |
| `ListServerRuleActions` | `{"rule_id":"..."}` | `[RuleActionInfo,...]` |

#### `rcon` methods

| Method | Request JSON fields |
|--------|---------------------|
| `SendCommand` | `command` |
| `Broadcast` | `message` |
| `SendWarningToPlayer` | `player_id`, `message` |
| `KickPlayer` | `player_id`, `reason` |
| `BanPlayer` | `player_id`, `reason`, `duration_ns` (int64 nanoseconds) |
| `BanWithEvidence` | `player_id`, `reason`, `duration_ns`, `event_id`, `event_type` |
| `WarnPlayerWithRule` | `player_id`, `message`, `rule_id` optional |
| `KickPlayerWithRule` | `player_id`, `reason`, `rule_id` optional |
| `BanPlayerWithRule` | `player_id`, `reason`, `duration_ns`, `rule_id` optional |
| `BanWithEvidenceAndRule` | `player_id`, `reason`, `duration_ns`, `event_id`, `event_type`, `rule_id` optional |
| `BanWithEvidenceAndRuleAndMetadata` | above + `metadata` object |
| `RemovePlayerFromSquad` | `player_id` |
| `RemovePlayerFromSquadById` | `player_id` |

Ban methods that return a ban UUID return `{"ban_id":"..."}` in `data`.

#### `admin` methods

| Method | Request JSON |
|--------|--------------|
| `AddTemporaryAdmin` | `player_id`, `role_name`, `notes`, `expires_at` (RFC3339 string or omit) |
| `RemoveTemporaryAdmin` | `player_id`, `notes` |
| `RemoveTemporaryAdminRole` | `player_id`, `role_name`, `notes` |
| `GetPlayerAdminStatus` | `player_id` → data `PlayerAdminStatus` |
| `ListTemporaryAdmins` | (empty) → `[TemporaryAdminInfo,...]` |

#### `discord` methods

| Method | Request JSON |
|--------|--------------|
| `SendMessage` | `channel_id`, `content` → `{"message_id":"..."}` |
| `SendEmbed` | `channel_id`, `embed` (`DiscordEmbed` object) → `{"message_id":"..."}` |

#### `event` methods

| Method | Request JSON |
|--------|--------------|
| `PublishEvent` | `event_type`, `data` (object), `raw` (string, optional) |

**Note:** `SubscribeToEvents` is not exposed over WASM; use manifest `events` and the guest export `aegis_on_event`.

## Guest exports (plugins)

- `memory` — linear memory
- `aegis_init(config_ptr, config_len) -> i32`
- `aegis_start() -> i32`
- `aegis_stop() -> i32`
- `aegis_on_event(type_ptr, type_len, payload_ptr, payload_len) -> i32`

## Guest exports (WASM connectors)

Same as plugins for lifecycle, plus:

- `aegis_invoke(req_ptr, req_len, out_ptr, out_cap, out_written_ptr) -> i32` — `ConnectorInvokeRequest` / `ConnectorInvokeResponse` JSON.

WASM connectors may import **`log`** only when `api.log` is declared on the connector package target.

## Manifest

- `kind`: `"wasm"`
- `wasm_abi_version`: `1` (stable import module name and core exports; new `host_call` methods may be added without bumping this if guest uses optional imports only)
- `required_capabilities`: any subset of host-documented capabilities in `NativePluginHostCapabilities()` the host implements for WASM

## Non-goals

- Raw TCP/UDP from guests (use native plugins or a future dedicated `api.http_fetch`-style capability with policy).
