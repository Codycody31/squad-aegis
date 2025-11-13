---
title: "Persistent KV Store"
---

The Persistent KV (Key-Value) Store provides workflows with a dedicated storage system that persists data across workflow executions and server restarts. Each workflow has its own isolated KV store accessible only through Lua scripts.

## Overview

The KV Store is designed for:

- **Persistent Storage**: Data survives workflow executions and server restarts
- **Workflow Isolation**: Each workflow has its own namespace - workflows cannot access each other's data
- **Lua-Only Access**: Only accessible through Lua scripts, not directly through workflow actions
- **Flexible Data Types**: Supports any JSON-serializable data (strings, numbers, booleans, tables, arrays)

## Key Features

### Persistence
Unlike workflow variables which only exist during a single execution, KV store data persists in the database and is available across all executions of the workflow.

### Isolation
Each workflow has its own isolated KV store. This ensures:
- No data conflicts between workflows
- Enhanced security (workflows can't access each other's data)
- Simplified data management (no namespace collisions)

### Performance
The KV store uses database indexes for efficient queries and supports atomic operations like `kv_increment()` to prevent race conditions.

## Use Cases

### Player Statistics & Tracking
Track player behavior, statistics, or violations across multiple sessions:

```lua
-- Track player warnings across all sessions
local player_id = workflow.trigger_event.steam_id
local warning_count = kv_increment("warnings_" .. player_id)

if warning_count >= 3 then
    rcon_kick(player_id, "Too many warnings")
    kv_delete("warnings_" .. player_id) -- Reset after kick
end
```

### Rate Limiting & Cooldowns
Prevent actions from executing too frequently:

```lua
local cooldown_key = "last_broadcast"
local last_time = kv_get(cooldown_key, 0)
local current_time = os.time()

if current_time - last_time < 300 then -- 5 minute cooldown
    log_warn("Broadcast on cooldown")
    return
end

rcon_broadcast("Scheduled server message")
kv_set(cooldown_key, current_time)
```

### Dynamic Configuration
Store and retrieve configuration without modifying workflow definitions:

```lua
-- Initialize configuration on first run
if not kv_exists("config") then
    kv_set("config", {
        max_warnings = 3,
        ban_duration = 7,
        auto_kick = true
    })
end

local config = kv_get("config")
-- Use config.max_warnings, config.ban_duration, etc.
```

### Counters & Metrics
Track events and maintain statistics:

```lua
-- Increment counters atomically
kv_increment("total_player_joins")
kv_increment("chat_messages_today")
kv_increment("admin_actions_" .. os.date("%Y-%m-%d"))

-- Get current counts
local total_joins = kv_get("total_player_joins", 0)
log("Total player joins: " .. total_joins)
```

### Leaderboards
Maintain player rankings and scores:

```lua
local player_id = workflow.trigger_event.steam_id
local player_name = workflow.trigger_event.player_name
local points = 10

-- Get leaderboard
local leaderboard = kv_get("leaderboard", {})

-- Update player score
if not leaderboard[player_id] then
    leaderboard[player_id] = { name = player_name, score = 0 }
end
leaderboard[player_id].score = leaderboard[player_id].score + points

-- Save back
kv_set("leaderboard", leaderboard)
```

## Available Functions

See the [Lua Scripting](/docs/workflows/lua-scripting#persistent-kv-store-functions) documentation for detailed function references.

### Core Operations
- `kv_get(key, default)` - Retrieve a value
- `kv_set(key, value)` - Store a value
- `kv_delete(key)` - Remove a key
- `kv_exists(key)` - Check if key exists

### Bulk Operations
- `kv_keys()` - List all keys
- `kv_get_all()` - Get all key-value pairs
- `kv_clear()` - Remove all data
- `kv_count()` - Count stored items

### Atomic Operations
- `kv_increment(key, delta)` - Atomically increment a number

## Complete Examples

### Player Warning System

This example tracks warnings per player and takes escalating actions:

```lua
-- Get player information
local player_id = safe_get(workflow.trigger_event, "steam_id", "")
local player_name = safe_get(workflow.trigger_event, "player_name", "Unknown")

if player_id == "" then
    log_error("No valid player ID")
    return
end

-- Increment warning count atomically
local warning_key = "warnings_" .. player_id
local warnings, err = kv_increment(warning_key)

if err then
    log_error("Failed to increment warnings: " .. err)
    return
end

log("Player " .. player_name .. " now has " .. warnings .. " warning(s)")

-- Store warning details
local warning_history = kv_get("warning_history_" .. player_id, {})
table.insert(warning_history, {
    time = os.time(),
    reason = safe_get(workflow.trigger_event, "reason", "Unknown"),
    admin = safe_get(workflow.trigger_event, "admin_name", "System")
})
kv_set("warning_history_" .. player_id, warning_history)

-- Take escalating action
if warnings >= 3 then
    log_warn("Player " .. player_name .. " has 3+ warnings, banning...")
    local success, response = rcon_ban(player_id, 7, "Exceeded warning limit")
    
    if success then
        -- Reset warnings after ban
        kv_delete(warning_key)
        kv_delete("warning_history_" .. player_id)
        log("Player banned and warnings cleared")
    else
        log_error("Failed to ban player: " .. response)
    end
    
elseif warnings >= 2 then
    rcon_warn(player_id, "FINAL WARNING: You have " .. warnings .. " warnings. Next warning results in a ban!")
    
else
    rcon_warn(player_id, "You have been warned. Total warnings: " .. warnings)
end

-- Store last warning timestamp
kv_set("last_warning_time_" .. player_id, os.time())
```

### Advanced Rate Limiting

Implement sliding window rate limiting:

```lua
local action = "admin_broadcast"
local max_actions = 3
local window_seconds = 600 -- 10 minutes

-- Get action history
local history_key = "rate_limit_" .. action
local history = kv_get(history_key, {})
local current_time = os.time()

-- Remove old entries outside the window
local new_history = {}
for _, timestamp in ipairs(history) do
    if current_time - timestamp < window_seconds then
        table.insert(new_history, timestamp)
    end
end

-- Check if rate limit exceeded
if #new_history >= max_actions then
    local oldest = new_history[1]
    local reset_in = window_seconds - (current_time - oldest)
    log_warn("Rate limit exceeded. Try again in " .. reset_in .. " seconds")
    return
end

-- Execute action
rcon_broadcast("Server announcement: Check our Discord!")

-- Add current timestamp to history
table.insert(new_history, current_time)
kv_set(history_key, new_history)

log("Broadcast sent (" .. #new_history .. "/" .. max_actions .. " in window)")
```

### Player Session Tracking

Track player session information:

```lua
local event_type = workflow.trigger_event.event_type
local player_id = workflow.trigger_event.steam_id
local player_name = workflow.trigger_event.player_name

local session_key = "session_" .. player_id

if event_type == "player_connected" then
    -- Start new session
    local session = {
        player_name = player_name,
        connect_time = os.time(),
        kills = 0,
        deaths = 0,
        teamkills = 0
    }
    kv_set(session_key, session)
    
    -- Increment lifetime join counter
    local total_joins = kv_increment("total_joins_" .. player_id)
    log(player_name .. " connected (lifetime joins: " .. total_joins .. ")")
    
elseif event_type == "player_disconnected" then
    -- End session and archive
    local session = kv_get(session_key)
    if session then
        local duration = os.time() - session.connect_time
        session.duration = duration
        
        -- Archive session
        local archive = kv_get("session_archive_" .. player_id, {})
        table.insert(archive, session)
        
        -- Keep only last 10 sessions
        if #archive > 10 then
            table.remove(archive, 1)
        end
        
        kv_set("session_archive_" .. player_id, archive)
        kv_delete(session_key)
        
        log(player_name .. " disconnected (session duration: " .. duration .. "s)")
    end
    
elseif event_type == "player_killed" then
    -- Update session stats
    local attacker_id = workflow.trigger_event.attacker_steam_id
    if attacker_id then
        local attacker_session = kv_get("session_" .. attacker_id)
        if attacker_session then
            attacker_session.kills = attacker_session.kills + 1
            if workflow.trigger_event.teamkill then
                attacker_session.teamkills = attacker_session.teamkills + 1
            end
            kv_set("session_" .. attacker_id, attacker_session)
        end
    end
    
    local victim_session = kv_get(session_key)
    if victim_session then
        victim_session.deaths = victim_session.deaths + 1
        kv_set(session_key, victim_session)
    end
end
```

### Daily Statistics Reset

Automatically reset statistics at midnight:

```lua
local today = os.date("%Y-%m-%d")
local last_reset = kv_get("last_stats_reset", "")

if last_reset ~= today then
    log("Resetting daily statistics for new day: " .. today)
    
    -- Archive yesterday's stats
    if last_reset ~= "" then
        local yesterday_stats = {
            date = last_reset,
            player_joins = kv_get("daily_player_joins", 0),
            chat_messages = kv_get("daily_chat_messages", 0),
            admin_actions = kv_get("daily_admin_actions", 0)
        }
        
        -- Store in history
        local history = kv_get("daily_stats_history", {})
        table.insert(history, yesterday_stats)
        
        -- Keep only last 30 days
        if #history > 30 then
            table.remove(history, 1)
        end
        
        kv_set("daily_stats_history", history)
    end
    
    -- Reset counters
    kv_set("daily_player_joins", 0)
    kv_set("daily_chat_messages", 0)
    kv_set("daily_admin_actions", 0)
    kv_set("last_stats_reset", today)
    
    log("Daily statistics reset complete")
end

-- Increment today's counter
kv_increment("daily_player_joins")
```

## Best Practices

### Key Naming Conventions

Use clear, consistent naming with prefixes:

```lua
-- Good examples
"warnings_76561198123456789"
"config_max_players"
"session_active_76561198123456789"
"stats_daily_2024-01-15"
"cooldown_broadcast"

-- Avoid
"w1"
"temp"
"data"
"x"
```

### Always Provide Defaults

Prevent nil-related errors by providing sensible defaults:

```lua
-- Good - handles missing keys gracefully
local count = kv_get("player_count", 0)
local config = kv_get("config", { enabled = true })

-- Risky - may cause errors if key doesn't exist
local count = kv_get("player_count")
count = count + 1 -- Error if count is nil
```

### Check Write Operation Results

Always verify write operations succeeded:

```lua
local success, err = kv_set("important_data", data)
if not success then
    log_error("Failed to save data: " .. err)
    -- Handle error appropriately
    return
end
```

### Use Atomic Operations for Counters

Prefer `kv_increment()` over get-modify-set patterns:

```lua
-- Good - atomic, no race conditions
kv_increment("page_views")

-- Avoid - potential race condition if multiple executions run simultaneously
local views = kv_get("page_views", 0)
kv_set("page_views", views + 1)
```

### Store Related Data Together

Group related data in tables rather than separate keys:

```lua
-- Good - organized and efficient
kv_set("player_" .. player_id, {
    name = player_name,
    warnings = 3,
    last_seen = os.time(),
    total_joins = 42
})

-- Less optimal - multiple database operations
kv_set("player_name_" .. player_id, player_name)
kv_set("player_warnings_" .. player_id, 3)
kv_set("player_last_seen_" .. player_id, os.time())
kv_set("player_total_joins_" .. player_id, 42)
```

### Periodic Cleanup

Remove outdated or temporary data:

```lua
-- Clean up old temporary data
local keys = kv_keys()
local current_time = os.time()

for _, key in ipairs(keys) do
    if string.match(key, "^temp_") then
        local data = kv_get(key)
        if data and data.expires_at and current_time > data.expires_at then
            kv_delete(key)
            log_debug("Cleaned up expired key: " .. key)
        end
    end
end
```

### Cache Frequently Accessed Values

For values accessed multiple times in a single execution, cache in local variables:

```lua
-- Good - single database read
local config = kv_get("config")
local max_warnings = config.max_warnings
local ban_duration = config.ban_duration
-- Use max_warnings and ban_duration throughout script

-- Less optimal - multiple database reads
if kv_get("config").max_warnings > 3 then
    rcon_ban(player_id, kv_get("config").ban_duration, "Too many warnings")
end
```

## Limitations

- **Key Length**: Maximum 255 characters
- **Value Types**: Must be JSON-serializable (no functions, userdata, etc.)
- **No Nil Values**: Cannot store `nil` - use `kv_delete()` to remove keys
- **Workflow Isolation**: Cannot share data between different workflows
- **Database Access**: Each operation involves database I/O

## Performance Considerations

### Minimize Database Operations

```lua
-- Good - batch operations
local all_data = kv_get_all()
for key, value in pairs(all_data) do
    -- Process all data
end

-- Less optimal - multiple queries
local keys = kv_keys()
for _, key in ipairs(keys) do
    local value = kv_get(key) -- Separate query for each key
end
```

### Avoid Excessive Writes in Loops

```lua
-- Good - accumulate changes, then write once
local stats = kv_get("player_stats", {})
for i = 1, 100 do
    stats.total = stats.total + 1
end
kv_set("player_stats", stats)

-- Avoid - writes to database 100 times
for i = 1, 100 do
    kv_increment("counter") -- 100 database writes
end
```

### Use Appropriate Data Structures

Choose the right data structure for your use case:

```lua
-- For frequently updated counters, use numbers
kv_set("player_joins", 42)

-- For collections, use tables
kv_set("banned_players", {
    ["76561198123456789"] = true,
    ["76561198987654321"] = true
})

-- For time-series data, use arrays
kv_set("daily_events", {
    { date = "2024-01-15", count = 100 },
    { date = "2024-01-16", count = 150 }
})
```

## Troubleshooting

### Key Not Found

If `kv_get()` returns `nil` unexpectedly:
- Verify the key name (case-sensitive)
- Check if the key was actually set
- Provide a default value: `kv_get("key", default_value)`

### Write Failures

If `kv_set()` fails:
- Check error message for details
- Verify value is JSON-serializable
- Ensure key length is under 255 characters
- Check database connectivity

### Data Type Mismatches

If you get unexpected data types:
```lua
-- Numbers may be decoded as floats
local count = kv_get("counter", 0)
count = math.floor(count) -- Ensure integer if needed

-- Booleans stored as booleans
local enabled = kv_get("enabled", false)
if enabled == true then -- Explicit comparison
    -- ...
end
```

## Migration from Workflow Variables

If you're currently using workflow variables and want to migrate to the KV store:

```lua
-- Before (workflow variable - not persistent)
set_variable("player_warnings", 3)
local warnings = get_variable("player_warnings") or 0

-- After (KV store - persistent)
kv_set("player_warnings", 3)
local warnings = kv_get("player_warnings", 0)
```

Note: Workflow variables still exist and are useful for passing data between steps in a single execution. Use the KV store when you need data to persist across multiple executions.

## Security Considerations

- **Workflow Isolation**: Each workflow's KV store is isolated - other workflows cannot access the data
- **SQL Injection**: Prevented through parameterized queries
- **Input Validation**: Validate data before storing to prevent unexpected behavior
- **Sensitive Data**: Consider encrypting sensitive data before storing if needed

```lua
-- Example: Validate before storing
local function is_valid_steam_id(id)
    return type(id) == "string" and id:match("^%d+$") ~= nil
end

local player_id = workflow.trigger_event.steam_id
if is_valid_steam_id(player_id) then
    kv_set("last_player", player_id)
else
    log_error("Invalid Steam ID format: " .. tostring(player_id))
end
```
