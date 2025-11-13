# Workflow KV Store Documentation

The Workflow KV (Key-Value) Store provides persistent storage for workflow scripts. Each workflow has its own isolated KV store that can only be accessed through Lua scripts.

## Overview

- **Persistent**: Data persists across workflow executions and server restarts
- **Isolated**: Each workflow has its own namespace - workflows cannot access each other's data
- **Lua-only**: KV store is only accessible through Lua scripts, not directly through actions
- **Flexible**: Supports any JSON-serializable data types (strings, numbers, booleans, tables/objects, arrays)

## Use Cases

- Track player statistics across multiple events
- Implement counters (e.g., rule violations, warnings)
- Store configuration data
- Maintain state between workflow executions
- Build custom caching mechanisms
- Implement rate limiting or cooldowns

## Available Functions

### `kv_get(key, default_value)`

Retrieves a value from the KV store.

**Parameters:**
- `key` (string): The key to retrieve
- `default_value` (optional): Value to return if key doesn't exist

**Returns:** The stored value, or `default_value` if key doesn't exist, or `nil` if no default provided

**Example:**
```lua
-- Get a counter, defaulting to 0 if it doesn't exist
local count = kv_get("player_warnings", 0)

-- Get a string value
local last_warned = kv_get("last_warned_player", "none")

-- Get a table/object
local player_stats = kv_get("player_stats", {})
```

### `kv_set(key, value)`

Sets a value in the KV store (creates or updates).

**Parameters:**
- `key` (string): The key to set
- `value` (any): The value to store (cannot be `nil`)

**Returns:** 
- `success` (boolean): `true` if successful
- `error` (string or nil): Error message if failed

**Example:**
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

-- Store an array
kv_set("banned_ips", {"192.168.1.1", "10.0.0.1"})
```

### `kv_delete(key)`

Deletes a key from the KV store.

**Parameters:**
- `key` (string): The key to delete

**Returns:**
- `success` (boolean): `true` if successful
- `error` (string or nil): Error message if failed

**Example:**
```lua
local success, err = kv_delete("old_data")
if not success then
    log_error("Failed to delete key: " .. err)
end
```

### `kv_exists(key)`

Checks if a key exists in the KV store.

**Parameters:**
- `key` (string): The key to check

**Returns:** `true` if key exists, `false` otherwise

**Example:**
```lua
if kv_exists("player_warnings") then
    log("Player has warnings on record")
else
    log("Player has no warnings")
end
```

### `kv_keys()`

Returns all keys in the KV store.

**Returns:** Array of key names (strings)

**Example:**
```lua
local keys = kv_keys()
for i, key in ipairs(keys) do
    log("Found key: " .. key)
end
```

### `kv_get_all()`

Returns all key-value pairs from the KV store.

**Returns:** Table with all key-value pairs

**Example:**
```lua
local all_data = kv_get_all()
for key, value in pairs(all_data) do
    log("Key: " .. key .. ", Value: " .. tostring(value))
end
```

### `kv_clear()`

Clears all key-value pairs from the KV store.

**Returns:**
- `success` (boolean): `true` if successful
- `error` (string or nil): Error message if failed

**Example:**
```lua
local success, err = kv_clear()
if success then
    log("KV store cleared")
else
    log_error("Failed to clear KV store: " .. err)
end
```

### `kv_count()`

Returns the number of key-value pairs in the KV store.

**Returns:** Number of items in the store

**Example:**
```lua
local count = kv_count()
log("KV store contains " .. count .. " items")
```

### `kv_increment(key, delta)`

Atomically increments a numeric value in the KV store. If the key doesn't exist, it starts from 0.

**Parameters:**
- `key` (string): The key to increment
- `delta` (number, optional): Amount to increment by (default: 1)

**Returns:**
- `new_value` (number): The new value after incrementing
- `error` (string or nil): Error message if failed

**Example:**
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

## Complete Examples

### Example 1: Player Warning System

Track warnings per player and take action on threshold:

```lua
-- Get player ID from trigger event
local player_id = workflow.trigger_event.player_id
local player_name = workflow.trigger_event.player_name

-- Get current warning count
local warning_key = "warnings_" .. player_id
local warnings = kv_get(warning_key, 0)

-- Increment warnings
warnings = warnings + 1
kv_set(warning_key, warnings)

log("Player " .. player_name .. " now has " .. warnings .. " warnings")

-- Take action based on warning count
if warnings >= 3 then
    log_warn("Player " .. player_name .. " has reached 3 warnings, kicking...")
    rcon_kick(player_id, "Too many warnings")
    
    -- Reset warnings after kick
    kv_delete(warning_key)
elseif warnings >= 2 then
    rcon_warn(player_id, "WARNING: You have " .. warnings .. " warnings. One more and you will be kicked!")
end
```

### Example 2: Rate Limiting

Prevent actions from happening too frequently:

```lua
local action_name = "admin_broadcast"
local cooldown_seconds = 300 -- 5 minutes
local cooldown_key = "last_" .. action_name

-- Get last execution time
local last_time = kv_get(cooldown_key, 0)
local current_time = os.time()

if current_time - last_time < cooldown_seconds then
    local remaining = cooldown_seconds - (current_time - last_time)
    log_warn("Action on cooldown. " .. remaining .. " seconds remaining.")
    result.skip = true
    return
end

-- Execute action
rcon_broadcast("Scheduled server message")

-- Update last execution time
kv_set(cooldown_key, current_time)
```

### Example 3: Player Statistics Tracker

Maintain comprehensive player statistics:

```lua
local player_id = workflow.trigger_event.player_id
local event_type = workflow.trigger_event.event_type

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
```

### Example 4: Dynamic Configuration

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
    local warnings = kv_get("warnings_" .. player_id, 0)
    if warnings >= config.max_warnings then
        rcon_kick(player_id, "Exceeded maximum warnings")
    end
end
```

### Example 5: Leaderboard System

Track top players:

```lua
local player_id = workflow.trigger_event.player_id
local player_name = workflow.trigger_event.player_name
local points = 10 -- Points earned from this event

-- Get current leaderboard
local leaderboard = kv_get("leaderboard", {})

-- Update player score
if not leaderboard[player_id] then
    leaderboard[player_id] = {
        name = player_name,
        score = 0
    }
end

leaderboard[player_id].score = leaderboard[player_id].score + points

-- Save updated leaderboard
kv_set("leaderboard", leaderboard)

log(player_name .. " earned " .. points .. " points. Total: " .. leaderboard[player_id].score)
```

## Best Practices

1. **Use Descriptive Keys**: Use clear, descriptive key names with prefixes
   - Good: `"warnings_player_123"`, `"config_max_players"`
   - Bad: `"w1"`, `"temp"`

2. **Handle Defaults**: Always provide default values to `kv_get()` to avoid nil errors
   ```lua
   local count = kv_get("counter", 0) -- Good
   local count = kv_get("counter")    -- May return nil
   ```

3. **Check for Errors**: Always check return values from write operations
   ```lua
   local success, err = kv_set("key", value)
   if not success then
       log_error("Failed to save: " .. err)
   end
   ```

4. **Use Structured Data**: Store related data in tables/objects
   ```lua
   -- Good: Organized structure
   kv_set("player_data", {
       warnings = 3,
       last_warning = os.time(),
       banned = false
   })
   
   -- Less optimal: Separate keys
   kv_set("player_warnings", 3)
   kv_set("player_last_warning", os.time())
   kv_set("player_banned", false)
   ```

5. **Clean Up Old Data**: Periodically remove outdated entries
   ```lua
   local keys = kv_keys()
   for _, key in ipairs(keys) do
       if string.match(key, "^temp_") then
           kv_delete(key)
       end
   end
   ```

6. **Use `kv_increment()` for Counters**: More reliable than get-modify-set
   ```lua
   -- Good: Atomic operation
   kv_increment("page_views")
   
   -- Less optimal: Race condition possible
   local views = kv_get("page_views", 0)
   kv_set("page_views", views + 1)
   ```

## Limitations

- Keys are limited to 255 characters
- Values must be JSON-serializable
- Cannot store `nil` values (use `kv_delete()` instead)
- Each workflow has a separate KV store namespace
- No cross-workflow data sharing (by design)

## Performance Considerations

- KV operations involve database access, so avoid excessive reads/writes in tight loops
- Use `kv_get_all()` if you need multiple values instead of multiple `kv_get()` calls
- Consider caching frequently accessed values in workflow variables during a single execution
- Use `kv_increment()` instead of get-modify-set patterns

## Security

- KV store is isolated per workflow - workflows cannot access each other's data
- Only accessible through Lua scripts, not through direct actions
- SQL injection is prevented through parameterized queries
- All values are validated and sanitized