---
title: "Lua Scripting"
---

Squad Aegis provides powerful Lua scripting capabilities within workflows, allowing you to implement complex logic, data processing, and server interactions using the Lua programming language.

## Overview

Lua scripts in workflows have access to:

- Complete workflow context (trigger events, variables, metadata)
- RCON command execution capabilities
- Logging and debugging functions
- JSON processing utilities
- Safe data access functions

## Available Functions

### Logging Functions

#### `log(message)`

Logs an informational message.

```lua
log("Player " .. player_name .. " triggered workflow")
```

#### `log_debug(message)`

Logs a debug message (only visible when debug logging is enabled).

```lua
log_debug("Processing trigger event: " .. json_encode(workflow.trigger_event))
```

#### `log_warn(message)`

Logs a warning message.

```lua
log_warn("High damage teamkill detected: " .. damage)
```

#### `log_error(message)`

Logs an error message.

```lua
log_error("Failed to process player data")
```

### Variable Functions

#### `set_variable(name, value)`

Sets a workflow variable that persists across workflow steps.

```lua
set_variable("player_warning_count", 3)
set_variable("last_event_time", os.time())
```

#### `get_variable(name)`

Gets a workflow variable value. Returns `nil` if the variable doesn't exist.

```lua
local count = get_variable("player_warning_count") or 0
local last_time = get_variable("last_event_time")
```

### Utility Functions

#### `json_encode(table)`

Converts a Lua table to a JSON string.

```lua
local data = { player = "John", score = 100 }
local json_string = json_encode(data)
-- Result: '{"player":"John","score":100}'
```

#### `json_decode(string)`

Parses a JSON string into a Lua table.

```lua
local json_string = '{"player":"John","score":100}'
local data = json_decode(json_string)
-- Access: data.player, data.score
```

#### `safe_get(table, key, default)`

Safely gets a value from a table with a fallback default.

```lua
local player_name = safe_get(workflow.trigger_event, "player_name", "Unknown")
local damage = safe_get(workflow.trigger_event, "damage", 0)
```

#### `to_string(value, default)`

Safely converts any value to a string with an optional default.

```lua
local player_str = to_string(workflow.trigger_event.player_name, "Unknown Player")
local damage_str = to_string(workflow.trigger_event.damage, "0")
```

### RCON Command Functions

#### `rcon_execute(command)`

Executes a raw RCON command and returns the response.

**Returns:** `response, error`

- `response` (string): Command response, or `nil` if failed
- `error` (string): Error message, or `nil` if successful

```lua
local response, err = rcon_execute("ShowCurrentMap")
if response then
    log("Current map: " .. response)
else
    log_error("Failed to get map: " .. err)
end
```

#### `rcon_kick(player_id, reason)`

Kicks a player from the server.

**Parameters:**

- `player_id` (string): Player's Steam ID or name
- `reason` (string): Optional kick reason

**Returns:** `success, response`

- `success` (boolean): Whether the command succeeded
- `response` (string): Server response or error message

```lua
local success, response = rcon_kick(player_steam_id, "Violation of server rules")
if success then
    log("Player kicked successfully")
else
    log_error("Failed to kick player: " .. response)
end
```

#### `rcon_ban(player_id, duration, reason)`

Bans a player from the server.

**Parameters:**

- `player_id` (string): Player's Steam ID or name
- `duration` (number): Ban duration in days (0 = permanent)
- `reason` (string): Optional ban reason

**Returns:** `success, response`

```lua
local success, response = rcon_ban(player_steam_id, 7, "Cheating")
if success then
    log("Player banned for 7 days")
else
    log_error("Failed to ban player: " .. response)
end
```

#### `rcon_warn(player_id, message)`

Sends a warning message to a specific player.

**Parameters:**

- `player_id` (string): Player's Steam ID or name
- `message` (string): Warning message

**Returns:** `success, response`

```lua
local success, response = rcon_warn(player_steam_id, "Please follow server rules!")
if success then
    log("Warning sent to player")
else
    log_error("Failed to warn player: " .. response)
end
```

#### `rcon_broadcast(message)`

Sends a broadcast message visible to all players.

**Parameters:**

- `message` (string): Broadcast message

**Returns:** `success, response`

```lua
local success, response = rcon_broadcast("Server restart in 5 minutes!")
if success then
    log("Broadcast sent successfully")
else
    log_error("Failed to broadcast: " .. response)
end
```

### Persistent KV Store Functions

The KV (Key-Value) Store provides persistent storage that survives across workflow executions and server restarts. Each workflow has its own isolated KV store that cannot be accessed by other workflows or actions.

#### `kv_get(key, default_value)`

Retrieves a value from the persistent KV store.

**Parameters:**

- `key` (string): The key to retrieve
- `default_value` (optional): Value to return if key doesn't exist

**Returns:** The stored value, or `default_value` if key doesn't exist, or `nil` if no default provided

```lua
-- Get a counter, defaulting to 0 if it doesn't exist
local count = kv_get("player_warnings", 0)

-- Get a string value
local last_warned = kv_get("last_warned_player", "none")

-- Get a table/object
local player_stats = kv_get("player_stats", {})
```

#### `kv_set(key, value)`

Sets a value in the persistent KV store (creates or updates).

**Parameters:**

- `key` (string): The key to set (max 255 characters)
- `value` (any): The value to store (cannot be `nil`, must be JSON-serializable)

**Returns:** `success, error`

- `success` (boolean): `true` if successful
- `error` (string or nil): Error message if failed

```lua
-- Store a number
local success, err = kv_set("player_count", 42)

-- Store a string
kv_set("server_status", "active")

-- Store a table
kv_set("player_data", {
    name = "PlayerOne",
    warnings = 3,
    last_seen = os.time()
})

-- Always check for errors
if not success then
    log_error("Failed to save: " .. err)
end
```

#### `kv_delete(key)`

Deletes a key from the persistent KV store.

**Parameters:**

- `key` (string): The key to delete

**Returns:** `success, error`

```lua
local success, err = kv_delete("old_data")
if not success then
    log_error("Failed to delete key: " .. err)
end
```

#### `kv_exists(key)`

Checks if a key exists in the persistent KV store.

**Parameters:**

- `key` (string): The key to check

**Returns:** `boolean` - `true` if key exists, `false` otherwise

```lua
if kv_exists("player_warnings") then
    log("Player has warnings on record")
else
    log("Player has no warnings")
end
```

#### `kv_keys()`

Returns all keys in the persistent KV store.

**Returns:** Array of key names (strings)

```lua
local keys = kv_keys()
for i, key in ipairs(keys) do
    log("Found key: " .. key)
end
```

#### `kv_get_all()`

Returns all key-value pairs from the persistent KV store.

**Returns:** Table with all key-value pairs

```lua
local all_data = kv_get_all()
for key, value in pairs(all_data) do
    log("Key: " .. key .. ", Value: " .. tostring(value))
end
```

#### `kv_clear()`

Clears all key-value pairs from the persistent KV store.

**Returns:** `success, error`

```lua
local success, err = kv_clear()
if success then
    log("KV store cleared")
else
    log_error("Failed to clear KV store: " .. err)
end
```

#### `kv_count()`

Returns the number of key-value pairs in the persistent KV store.

**Returns:** Number of items in the store

```lua
local count = kv_count()
log("KV store contains " .. count .. " items")
```

#### `kv_increment(key, delta)`

Atomically increments a numeric value in the persistent KV store. If the key doesn't exist, it starts from 0.

**Parameters:**

- `key` (string): The key to increment
- `delta` (number, optional): Amount to increment by (default: 1)

**Returns:** `new_value, error`

- `new_value` (number): The new value after incrementing
- `error` (string or nil): Error message if failed

```lua
-- Increment by 1 (default)
local new_count, err = kv_increment("player_joins")

-- Increment by custom amount
local score, err = kv_increment("player_score", 10)

-- Decrement (negative delta)
local remaining, err = kv_increment("lives", -1)

if err then
    log_error("Failed to increment: " .. err)
else
    log("New value: " .. new_count)
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

Contains all workflow variables.

```lua
-- Direct access
local player_count = workflow.variables.player_count

-- Safe access with fallback
local warning_count = workflow.variables.warning_count or 0
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

Track warnings per player across workflow executions:

```lua
-- Get player ID from trigger event
local player_id = safe_get(workflow.trigger_event, "steam_id", "")
local player_name = safe_get(workflow.trigger_event, "player_name", "Unknown")

if player_id == "" then
    log_error("No valid player ID")
    return
end

-- Get current warning count from KV store
local warning_key = "warnings_" .. player_id
local warnings = kv_get(warning_key, 0)

-- Increment warnings
warnings = warnings + 1
local success, err = kv_set(warning_key, warnings)

if not success then
    log_error("Failed to save warning count: " .. err)
    return
end

log("Player " .. player_name .. " now has " .. warnings .. " warnings")

-- Take action based on warning count
if warnings >= 3 then
    log_warn("Player " .. player_name .. " has reached 3 warnings, kicking...")
    rcon_kick(player_id, "Too many warnings")
    
    -- Reset warnings after kick
    kv_delete(warning_key)
elseif warnings >= 2 then
    rcon_warn(player_id, "WARNING: You have " .. warnings .. " warnings. One more and you will be kicked!")
else
    rcon_warn(player_id, "Warning issued. You have " .. warnings .. " warning(s).")
end

-- Store last warning time
kv_set("last_warning_time_" .. player_id, os.time())
```

### Rate Limiting with KV Store

Prevent actions from happening too frequently:

```lua
local action_name = "admin_broadcast"
local cooldown_seconds = 300 -- 5 minutes
local cooldown_key = "cooldown_" .. action_name

-- Get last execution time
local last_time = kv_get(cooldown_key, 0)
local current_time = os.time()

if current_time - last_time < cooldown_seconds then
    local remaining = cooldown_seconds - (current_time - last_time)
    log_warn("Action on cooldown. " .. remaining .. " seconds remaining.")
    return
end

-- Execute action
rcon_broadcast("Scheduled server message")

-- Update last execution time
kv_set(cooldown_key, current_time)
log("Broadcast sent, cooldown activated")
```

### Player Statistics Tracker

Maintain comprehensive player statistics:

```lua
local player_id = safe_get(workflow.trigger_event, "steam_id", "")
local event_type = safe_get(workflow.trigger_event, "event_type", "")

if player_id == "" then
    return
end

-- Get or create player stats
local stats_key = "stats_" .. player_id
local stats = kv_get(stats_key, {
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
kv_set(stats_key, stats)

log_debug("Updated stats for player " .. player_id .. " - K/D: " .. string.format("%.2f", kd_ratio))

-- Store result for other steps
result.player_stats = stats
result.kd_ratio = kd_ratio
```

### Dynamic Configuration Storage

Store and retrieve configuration values:

```lua
-- Initialize default configuration if not exists
if not kv_exists("config") then
    kv_set("config", {
        max_warnings = 3,
        ban_duration_days = 7,
        auto_kick_enabled = true,
        welcome_message = "Welcome to the server!"
    })
    log("Initialized default configuration")
end

-- Get configuration
local config = kv_get("config")

-- Use configuration values
if config.auto_kick_enabled then
    local player_id = safe_get(workflow.trigger_event, "steam_id", "")
    local warnings = kv_get("warnings_" .. player_id, 0)
    
    if warnings >= config.max_warnings then
        rcon_kick(player_id, "Exceeded maximum warnings (" .. config.max_warnings .. ")")
    end
end
```

### Simple Counter with kv_increment

Use atomic increment for reliable counting:

```lua
-- Increment total player joins
local total_joins, err = kv_increment("total_player_joins")
if err then
    log_error("Failed to increment counter: " .. err)
else
    log("Total player joins: " .. total_joins)
end

-- Increment event-specific counter
local event_type = safe_get(workflow.trigger_event, "event_type", "unknown")
local event_count, err = kv_increment("event_count_" .. event_type)

log("Event " .. event_type .. " has occurred " .. event_count .. " times")
```

## Common Patterns (Workflow Variables)

### Safe Event Data Access

Always check for nil values when accessing event data:

```lua
-- Bad: Will fail if player_name is nil
local message = "Hello " .. workflow.trigger_event.player_name

-- Good: Safe access with fallback
local player_name = safe_get(workflow.trigger_event, "player_name", "Unknown")
local message = "Hello " .. player_name

-- Alternative: Using to_string
local message = "Hello " .. to_string(workflow.trigger_event.player_name, "Unknown")
```

### Chat Command Processing

```lua
-- Get chat message data
local message = safe_get(workflow.trigger_event, "message", "")
local player_name = safe_get(workflow.trigger_event, "player_name", "Unknown")
local steam_id = safe_get(workflow.trigger_event, "steam_id", "")

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
    rcon_warn(steam_id, help_text)

elseif command == "rules" then
    local rules = "Check rules by pressing enter and selecting server rules!"
    rcon_warn(steam_id, rules)

elseif command == "discord" then
    local discord_link = "Join our Discord: discord.gg/readytobreach"
    rcon_warn(steam_id, discord_link)

else
    rcon_warn(steam_id, "Unknown command. Type !help for available commands.")
end

-- Track command usage
local usage_key = "command_usage_" .. command
local usage_count = get_variable(usage_key) or 0
set_variable(usage_key, usage_count + 1)

-- Store results
result.command = command
result.player = player_name
result.usage_count = usage_count + 1
```

## Best Practices

### Error Handling

Always check return values from RCON and KV store functions:

```lua
-- RCON functions
local success, response = rcon_kick(player_id, reason)
if not success then
    log_error("Failed to kick player: " .. response)
    return
end

-- KV store write operations
local success, err = kv_set("player_data", data)
if not success then
    log_error("Failed to save data: " .. err)
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
   - Use `kv_get_all()` instead of multiple `kv_get()` calls when needed
   - Use `kv_increment()` instead of get-modify-set patterns for counters
   - Cache frequently accessed KV values in local variables during a single execution

### Data Validation

Always validate input data:

```lua
local steam_id = safe_get(workflow.trigger_event, "steam_id", "")
if steam_id == "" then
    log_error("No valid Steam ID provided")
    return
end

-- Validate Steam ID format (basic check)
if not steam_id:match("^%d+$") then
    log_error("Invalid Steam ID format: " .. steam_id)
    return
end
```

### Variable Naming

Use consistent and descriptive variable names:

```lua
-- Good
local player_warning_count = get_variable("warning_count_" .. steam_id) or 0
local teamkill_threshold = get_variable("server_teamkill_threshold") or 3

-- Avoid
local c = get_variable("c") or 0
local x = get_variable("threshold")
```

### Logging

Use appropriate log levels:

```lua
-- Debug information
log_debug("Processing player: " .. player_name)

-- General information
log("Player warned for teamkilling")

-- Important warnings
log_warn("High damage teamkill detected: " .. damage)

-- Errors that need attention
log_error("Failed to execute RCON command: " .. error_message)
```

### KV Store Best Practices

1. **Use descriptive keys** with prefixes for organization:
   ```lua
   -- Good
   kv_set("warnings_" .. player_id, count)
   kv_set("config_max_players", 64)
   
   -- Avoid
   kv_set("w1", count)
   kv_set("temp", 64)
   ```

2. **Always provide defaults** to `kv_get()`:
   ```lua
   -- Good - handles missing keys gracefully
   local count = kv_get("counter", 0)
   
   -- Risky - may return nil
   local count = kv_get("counter")
   if count then count = count + 1 end
   ```

3. **Store related data in tables**:
   ```lua
   -- Good - organized structure
   kv_set("player_data_" .. player_id, {
       warnings = 3,
       last_warning = os.time(),
       banned = false
   })
   
   -- Less optimal - multiple separate keys
   kv_set("player_warnings_" .. player_id, 3)
   kv_set("player_last_warning_" .. player_id, os.time())
   kv_set("player_banned_" .. player_id, false)
   ```

4. **Use atomic operations** for counters:
   ```lua
   -- Good - atomic, no race conditions
   kv_increment("page_views")
   
   -- Less optimal - potential race condition
   local views = kv_get("page_views", 0)
   kv_set("page_views", views + 1)
   ```

5. **Clean up old data** periodically:
   ```lua
   -- Remove temporary or expired data
   local keys = kv_keys()
   for _, key in ipairs(keys) do
       if string.match(key, "^temp_") then
           kv_delete(key)
       end
   end
   ```

6. **Remember isolation** - Each workflow has its own KV store:
   ```lua
   -- Data stored in one workflow cannot be accessed by another workflow
   -- Use workflow variables if you need to pass data between steps in the same execution
   -- Use KV store for data that needs to persist across executions
   ```
