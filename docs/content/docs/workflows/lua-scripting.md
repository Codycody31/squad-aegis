---
title: "Lua Scripting"
---

Squad Aegis provides powerful Lua scripting capabilities within workflows, allowing you to implement complex logic, data processing, and server interactions using the Lua programming language.

## Overview

Lua scripts in workflows have access to:

- Complete workflow context (trigger events, variables, metadata)
- RCON command execution capabilities
- Persistent KV (Key-Value) storage
- Logging and debugging functions
- JSON processing utilities
- Safe data access functions

## API Structure

All workflow functions are organized under the `workflow` namespace for consistency and discoverability:

- `workflow.log.*` - Logging functions
- `workflow.variable.*` - Workflow variable operations
- `workflow.kv.*` - Persistent key-value storage
- `workflow.rcon.*` - RCON server commands
- `workflow.json.*` - JSON encoding/decoding
- `workflow.util.*` - Utility helper functions

> **Note:** Legacy global functions (`log()`, `kv_get()`, etc.) are still supported for backward compatibility but are deprecated. New scripts should use the namespaced API.

## Available Functions

### Logging Functions (workflow.log.*)

#### `workflow.log.info(message)`

Logs an informational message.

```lua
workflow.log.info("Player " .. player_name .. " triggered workflow")
```

#### `workflow.log.debug(message)`

Logs a debug message (only visible when debug logging is enabled).

```lua
workflow.log.debug("Processing trigger event: " .. workflow.json.encode(workflow.trigger_event))
```

#### `workflow.log.warn(message)`

Logs a warning message.

```lua
workflow.log.warn("High damage teamkill detected: " .. damage)
```

#### `workflow.log.error(message)`

Logs an error message.

```lua
workflow.log.error("Failed to process player data")
```

### Variable Functions (workflow.variable.*)

#### `workflow.variable.set(name, value)`

Sets a workflow variable that persists across workflow steps within the same execution.

```lua
workflow.variable.set("player_warning_count", 3)
workflow.variable.set("last_event_time", os.time())
```

#### `workflow.variable.get(name, default)`

Gets a workflow variable value. Returns `default` if the variable doesn't exist, or `nil` if no default provided.

```lua
local count = workflow.variable.get("player_warning_count", 0)
local last_time = workflow.variable.get("last_event_time")
```

### Persistent KV Store Functions (workflow.kv.*)

The KV Store provides persistent storage that survives across workflow executions and server restarts. See the [KV Store documentation](/docs/workflows/kv-store) for detailed information.

#### `workflow.kv.get(key, default)`

Retrieves a value from the persistent KV store.

```lua
local count = workflow.kv.get("player_warnings", 0)
local config = workflow.kv.get("server_config", {})
```

#### `workflow.kv.set(key, value)`

Sets a value in the persistent KV store (creates or updates).

**Returns:** `success, error`

```lua
local success, err = workflow.kv.set("player_count", 42)
if not success then
    workflow.log.error("Failed to save: " .. err)
end
```

#### `workflow.kv.delete(key)`

Deletes a key from the persistent KV store.

**Returns:** `success, error`

```lua
local success, err = workflow.kv.delete("old_data")
```

#### `workflow.kv.exists(key)`

Checks if a key exists in the persistent KV store.

**Returns:** `boolean`

```lua
if workflow.kv.exists("player_warnings") then
    workflow.log.info("Player has warnings on record")
end
```

#### `workflow.kv.increment(key, delta)`

Atomically increments a numeric value. If the key doesn't exist, starts from 0.

**Parameters:**
- `key` (string): The key to increment
- `delta` (number, optional): Amount to increment by (default: 1)

**Returns:** `new_value, error`

```lua
-- Increment by 1
local count, err = workflow.kv.increment("player_joins")

-- Increment by custom amount
local score, err = workflow.kv.increment("player_score", 10)

-- Decrement
local lives, err = workflow.kv.increment("lives", -1)
```

#### `workflow.kv.keys()`

Returns all keys in the persistent KV store.

**Returns:** Array of key names

```lua
local keys = workflow.kv.keys()
for i, key in ipairs(keys) do
    workflow.log.info("Found key: " .. key)
end
```

#### `workflow.kv.get_all()`

Returns all key-value pairs from the persistent KV store.

**Returns:** Table with all key-value pairs

```lua
local all_data = workflow.kv.get_all()
for key, value in pairs(all_data) do
    workflow.log.info("Key: " .. key)
end
```

#### `workflow.kv.clear()`

Clears all key-value pairs from the persistent KV store.

**Returns:** `success, error`

```lua
local success, err = workflow.kv.clear()
```

#### `workflow.kv.count()`

Returns the number of key-value pairs in the persistent KV store.

**Returns:** Number of items

```lua
local count = workflow.kv.count()
workflow.log.info("KV store contains " .. count .. " items")
```

### JSON Functions (workflow.json.*)

#### `workflow.json.encode(value)`

Converts a Lua table to a JSON string.

```lua
local data = { player = "John", score = 100 }
local json_string = workflow.json.encode(data)
-- Result: '{"player":"John","score":100}'
```

#### `workflow.json.decode(string)`

Parses a JSON string into a Lua table.

```lua
local json_string = '{"player":"John","score":100}'
local data = workflow.json.decode(json_string)
-- Access: data.player, data.score
```

### Utility Functions (workflow.util.*)

#### `workflow.util.safe_get(table, key, default)`

Safely gets a value from a table with a fallback default.

```lua
local player_name = workflow.util.safe_get(workflow.trigger_event, "player_name", "Unknown")
local damage = workflow.util.safe_get(workflow.trigger_event, "damage", 0)
```

#### `workflow.util.to_string(value, default)`

Safely converts any value to a string with an optional default.

```lua
local player_str = workflow.util.to_string(workflow.trigger_event.player_name, "Unknown Player")
local damage_str = workflow.util.to_string(workflow.trigger_event.damage, "0")
```

### RCON Command Functions (workflow.rcon.*)

#### `workflow.rcon.execute(command)`

Executes a raw RCON command and returns the response.

**Returns:** `response, error`

- `response` (string): Command response, or `nil` if failed
- `error` (string): Error message, or `nil` if successful

```lua
local response, err = workflow.rcon.execute("ShowCurrentMap")
if response then
    workflow.log.info("Current map: " .. response)
else
    workflow.log.error("Failed to get map: " .. err)
end
```

#### `workflow.rcon.kick(player_id, reason)`

Kicks a player from the server.

**Parameters:**

- `player_id` (string): Player's Steam ID or name
- `reason` (string): Optional kick reason

**Returns:** `success, response`

- `success` (boolean): Whether the command succeeded
- `response` (string): Server response or error message

```lua
local success, response = workflow.rcon.kick(player_steam_id, "Violation of server rules")
if success then
    workflow.log.info("Player kicked successfully")
else
    workflow.log.error("Failed to kick player: " .. response)
end
```

#### `workflow.rcon.ban(player_id, duration, reason)`

Bans a player from the server.

**Parameters:**

- `player_id` (string): Player's Steam ID or name
- `duration` (number): Ban duration in days (0 = permanent)
- `reason` (string): Optional ban reason

**Returns:** `success, response`

```lua
local success, response = workflow.rcon.ban(player_steam_id, 7, "Cheating")
if success then
    workflow.log.info("Player banned for 7 days")
else
    workflow.log.error("Failed to ban player: " .. response)
end
```

#### `workflow.rcon.warn(player_id, message)`

Sends a warning message to a specific player.

**Parameters:**

- `player_id` (string): Player's Steam ID or name
- `message` (string): Warning message

**Returns:** `success, response`

```lua
local success, response = workflow.rcon.warn(player_steam_id, "Please follow server rules!")
if success then
    workflow.log.info("Warning sent to player")
else
    workflow.log.error("Failed to warn player: " .. response)
end
```

#### `workflow.rcon.broadcast(message)`

Sends a broadcast message visible to all players.

**Parameters:**

- `message` (string): Broadcast message

**Returns:** `success, response`

```lua
local success, response = workflow.rcon.broadcast("Server restart in 5 minutes!")
if success then
    workflow.log.info("Broadcast sent successfully")
else
    workflow.log.error("Failed to broadcast: " .. response)
end
```

## Workflow Data Access

### `workflow.trigger_event`

Contains all data from the event that triggered the workflow.

```lua
-- Chat message events
local player_name = workflow.trigger_event.player_name
local message = workflow.trigger_event.message
local chat_type = workflow.trigger_event.chat_type

-- Player death events
local victim = workflow.trigger_event.victim_name
local attacker = workflow.trigger_event.attacker_name
local teamkill = workflow.trigger_event.teamkill
```

### `workflow.metadata`

Contains workflow execution metadata.

```lua
local workflow_name = workflow.metadata.workflow_name
local execution_id = workflow.metadata.execution_id
local server_id = workflow.metadata.server_id
local started_at = workflow.metadata.started_at
```

### `workflow.variables`

Contains all workflow variables (for the current execution only).

```lua
-- Direct access
local player_count = workflow.variables.player_count

-- Safe access with fallback using workflow.variable.get
local warning_count = workflow.variable.get("warning_count", 0)
```

### `workflow.step_results`

Contains results from previous workflow steps (by step ID).

```lua
-- Access results from a previous step
local analysis_result = workflow.step_results["analyze_player_data"]
if analysis_result then
    local risk_level = analysis_result.risk_level
end
```

### `result` Table

Use the `result` table to store output data for other steps to access.

```lua
-- Store results for subsequent steps
result.player_risk_score = 75
result.recommended_action = "warn"
result.analysis_complete = true
```

## Common Patterns

### Persistent Player Warning System

Track warnings per player across workflow executions using the KV store:

```lua
-- Get player ID from trigger event
local player_id = workflow.util.safe_get(workflow.trigger_event, "steam_id", "")
local player_name = workflow.util.safe_get(workflow.trigger_event, "player_name", "Unknown")

if player_id == "" then
    workflow.log.error("No valid player ID")
    return
end

-- Get current warning count from KV store
local warning_key = "warnings_" .. player_id
local warnings = workflow.kv.get(warning_key, 0)

-- Increment warnings
warnings = warnings + 1
local success, err = workflow.kv.set(warning_key, warnings)

if not success then
    workflow.log.error("Failed to save warning count: " .. err)
    return
end

workflow.log.info("Player " .. player_name .. " now has " .. warnings .. " warnings")

-- Take action based on warning count
if warnings >= 3 then
    workflow.log.warn("Player " .. player_name .. " has reached 3 warnings, kicking...")
    workflow.rcon.kick(player_id, "Too many warnings")
    
    -- Reset warnings after kick
    workflow.kv.delete(warning_key)
elseif warnings >= 2 then
    workflow.rcon.warn(player_id, "WARNING: You have " .. warnings .. " warnings. One more and you will be kicked!")
else
    workflow.rcon.warn(player_id, "Warning issued. You have " .. warnings .. " warning(s).")
end

-- Store last warning time
workflow.kv.set("last_warning_time_" .. player_id, os.time())
```

### Rate Limiting with KV Store

Prevent actions from happening too frequently:

```lua
local action_name = "admin_broadcast"
local cooldown_seconds = 300 -- 5 minutes
local cooldown_key = "cooldown_" .. action_name

-- Get last execution time
local last_time = workflow.kv.get(cooldown_key, 0)
local current_time = os.time()

if current_time - last_time < cooldown_seconds then
    local remaining = cooldown_seconds - (current_time - last_time)
    workflow.log.warn("Action on cooldown. " .. remaining .. " seconds remaining.")
    return
end

-- Execute action
workflow.rcon.broadcast("Scheduled server message")

-- Update last execution time
workflow.kv.set(cooldown_key, current_time)
workflow.log.info("Broadcast sent, cooldown activated")
```

### Chat Command Processing

```lua
-- Get chat message data
local message = workflow.util.safe_get(workflow.trigger_event, "message", "")
local player_name = workflow.util.safe_get(workflow.trigger_event, "player_name", "Unknown")
local steam_id = workflow.util.safe_get(workflow.trigger_event, "steam_id", "")

-- Parse command
local command = message:match("^!(%w+)")
if not command then
    return -- Not a command
end

-- Convert to lowercase for case-insensitive matching
command = command:lower()

-- Handle different commands
if command == "help" then
    local help_text = "Available commands: !help, !rules, !discord, !admin"
    workflow.rcon.warn(steam_id, help_text)

elseif command == "rules" then
    local rules = "Check rules by pressing enter and selecting server rules!"
    workflow.rcon.warn(steam_id, rules)

elseif command == "discord" then
    local discord_link = "Join our Discord: discord.gg/yourserver"
    workflow.rcon.warn(steam_id, discord_link)

else
    workflow.rcon.warn(steam_id, "Unknown command. Type !help for available commands.")
end

-- Track command usage in KV store
local usage_count = workflow.kv.increment("command_usage_" .. command)

-- Store results
result.command = command
result.player = player_name
result.usage_count = usage_count
```

### Player Statistics Tracker

Maintain comprehensive player statistics in the KV store:

```lua
local player_id = workflow.util.safe_get(workflow.trigger_event, "steam_id", "")
local event_type = workflow.util.safe_get(workflow.trigger_event, "event_type", "")

if player_id == "" then
    return
end

-- Get or create player stats
local stats_key = "stats_" .. player_id
local stats = workflow.kv.get(stats_key, {
    kills = 0,
    deaths = 0,
    joins = 0,
    playtime = 0,
    last_seen = 0
})

-- Update stats based on event type
if event_type == "player_connected" then
    stats.joins = stats.joins + 1
    stats.last_seen = os.time()
elseif event_type == "player_killed" then
    stats.kills = stats.kills + 1
elseif event_type == "player_died" then
    stats.deaths = stats.deaths + 1
end

-- Calculate K/D ratio
local kd_ratio = stats.deaths > 0 and (stats.kills / stats.deaths) or stats.kills

-- Save updated stats
workflow.kv.set(stats_key, stats)

workflow.log.debug("Updated stats for player " .. player_id .. " - K/D: " .. string.format("%.2f", kd_ratio))

-- Store result for other steps
result.player_stats = stats
result.kd_ratio = kd_ratio
```

## Best Practices

### Error Handling

Always check return values from RCON and KV store functions:

```lua
-- RCON functions
local success, response = workflow.rcon.kick(player_id, reason)
if not success then
    workflow.log.error("Failed to kick player: " .. response)
    return
end

-- KV store write operations
local success, err = workflow.kv.set("player_data", data)
if not success then
    workflow.log.error("Failed to save data: " .. err)
    return
end
```

### Performance Considerations

1. **Keep scripts short** - Long scripts can block workflow execution
2. **Use timeouts** - Set appropriate timeout values for your scripts
3. **Avoid infinite loops** - Always have exit conditions
4. **Cache expensive operations** - Store results in variables when possible
5. **Use KV store efficiently**:
   - Avoid excessive reads/writes in tight loops
   - Use `workflow.kv.get_all()` instead of multiple `workflow.kv.get()` calls when needed
   - Use `workflow.kv.increment()` instead of get-modify-set patterns for counters
   - Cache frequently accessed KV values in local variables during a single execution

### Data Validation

Always validate input data:

```lua
local steam_id = workflow.util.safe_get(workflow.trigger_event, "steam_id", "")
if steam_id == "" then
    workflow.log.error("No valid Steam ID provided")
    return
end

-- Validate Steam ID format (basic check)
if not steam_id:match("^%d+$") then
    workflow.log.error("Invalid Steam ID format: " .. steam_id)
    return
end
```

### Variable Naming

Use consistent and descriptive variable names:

```lua
-- Good
local player_warning_count = workflow.variable.get("warning_count_" .. steam_id, 0)
local teamkill_threshold = workflow.variable.get("server_teamkill_threshold", 3)

-- Avoid
local c = workflow.variable.get("c", 0)
local x = workflow.variable.get("threshold")
```

### Logging

Use appropriate log levels:

```lua
-- Debug information (verbose, only when debugging)
workflow.log.debug("Processing player: " .. player_name)

-- General information (notable events)
workflow.log.info("Player warned for teamkilling")

-- Important warnings (potential issues)
workflow.log.warn("High damage teamkill detected: " .. damage)

-- Errors that need attention
workflow.log.error("Failed to execute RCON command: " .. error_message)
```

## Backward Compatibility

For backward compatibility, the following global functions are still supported but deprecated:

- `log()` → Use `workflow.log.info()`
- `log_debug()` → Use `workflow.log.debug()`
- `log_warn()` → Use `workflow.log.warn()`
- `log_error()` → Use `workflow.log.error()`
- `set_variable()` → Use `workflow.variable.set()`
- `get_variable()` → Use `workflow.variable.get()`
- `safe_get()` → Use `workflow.util.safe_get()`
- `to_string()` → Use `workflow.util.to_string()`
- `json_encode()` → Use `workflow.json.encode()`
- `json_decode()` → Use `workflow.json.decode()`
- `rcon_execute()` → Use `workflow.rcon.execute()`
- `rcon_kick()` → Use `workflow.rcon.kick()`
- `rcon_ban()` → Use `workflow.rcon.ban()`
- `rcon_warn()` → Use `workflow.rcon.warn()`
- `rcon_broadcast()` → Use `workflow.rcon.broadcast()`

**Recommendation:** Update existing scripts to use the new namespaced API for better organization and consistency.