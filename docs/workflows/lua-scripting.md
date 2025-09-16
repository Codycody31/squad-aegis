---
title: "Lua Scripting"
---

# Lua Scripting in Workflows

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

#### `rcon_chat_message(player_id, message)`

Sends a private chat message to a specific player.

**Parameters:**

- `player_id` (string): Player's Steam ID or name
- `message` (string): Chat message

**Returns:** `success, response`

```lua
local success, response = rcon_chat_message(player_steam_id, "Welcome to our server!")
if success then
    log("Chat message sent to player")
else
    log_error("Failed to send chat message: " .. response)
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

### Player Warning System

```lua
-- Get player info
local steam_id = safe_get(workflow.trigger_event, "steam_id", "")
if steam_id == "" then
    log_error("No Steam ID available")
    return
end

-- Track warnings per player
local warning_key = "warnings_" .. steam_id
local current_warnings = get_variable(warning_key) or 0
current_warnings = current_warnings + 1
set_variable(warning_key, current_warnings)

-- Take action based on warning count
if current_warnings >= 3 then
    local success, response = rcon_kick(steam_id, "Too many warnings")
    if success then
        log("Player kicked after " .. current_warnings .. " warnings")
        -- Reset warning count
        set_variable(warning_key, 0)
    else
        log_error("Failed to kick player: " .. response)
    end
else
    local success, response = rcon_warn(steam_id, "Warning " .. current_warnings .. "/3: Please follow server rules")
    if success then
        log("Warning " .. current_warnings .. " sent to player")
    end
end

-- Store results
result.warning_count = current_warnings
result.action_taken = current_warnings >= 3 and "kick" or "warn"
```

### Dynamic Server Management

```lua
-- Get current server state
local player_count = safe_get(workflow.trigger_event, "player_count", 0)
local current_hour = tonumber(os.date("%H"))

-- Define peak hours
local peak_hours = {19, 20, 21, 22, 23}
local is_peak_time = false

for _, hour in ipairs(peak_hours) do
    if hour == current_hour then
        is_peak_time = true
        break
    end
end

-- Dynamic messaging based on conditions
if is_peak_time and player_count > 70 then
    rcon_broadcast("Server is busy! Consider joining our secondary server.")
elseif not is_peak_time and player_count < 20 then
    rcon_broadcast("Join us for some fun gameplay! Invite your friends!")
end

-- Store analysis results
result.is_peak_time = is_peak_time
result.player_count = player_count
result.server_status = player_count > 70 and "full" or player_count > 40 and "busy" or "quiet"
```

### Teamkill Analysis

```lua
-- Get teamkill data
local damage = safe_get(workflow.trigger_event, "damage", 0)
local attacker_name = safe_get(workflow.trigger_event, "attacker_name", "Unknown")
local victim_name = safe_get(workflow.trigger_event, "victim_name", "Unknown")
local weapon = safe_get(workflow.trigger_event, "weapon", "Unknown")

-- Analyze severity
local severity = "low"
local action = "warn"

if damage >= 90 then
    severity = "critical"
    action = "kick"
elseif damage >= 60 then
    severity = "high"
    action = "warn"
elseif damage >= 30 then
    severity = "medium"
    action = "warn"
end

-- Log the incident
log_warn(string.format("Teamkill: %s -> %s (%d damage, %s, severity: %s)", 
    attacker_name, victim_name, damage, weapon, severity))

-- Take appropriate action
local attacker_steam = safe_get(workflow.trigger_event, "attacker_steam", "")
if attacker_steam ~= "" then
    if action == "kick" then
        local success, response = rcon_kick(attacker_steam, "High damage teamkill")
        log("Kicked player for high damage teamkill: " .. tostring(success))
    else
        local success, response = rcon_warn(attacker_steam, 
            "Teamkilling is against server rules. Damage: " .. damage)
        log("Warned player for teamkill: " .. tostring(success))
    end
end

-- Store results for other steps
result.damage = damage
result.severity = severity
result.action_taken = action
result.weapon_used = weapon
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
    local help_text = "Available commands: !help, !rules, !discord, !time"
    rcon_chat_message(steam_id, help_text)
    log("Help command used by " .. player_name)
    
elseif command == "rules" then
    local rules = "1. No teamkilling 2. Follow squad leader 3. Communicate 4. Have fun!"
    rcon_chat_message(steam_id, rules)
    log("Rules command used by " .. player_name)
    
elseif command == "discord" then
    local discord_link = "Join our Discord: https://discord.gg/example"
    rcon_chat_message(steam_id, discord_link)
    log("Discord command used by " .. player_name)
    
elseif command == "time" then
    local server_time = os.date("%H:%M UTC")
    rcon_chat_message(steam_id, "Server time: " .. server_time)
    log("Time command used by " .. player_name)
    
else
    rcon_chat_message(steam_id, "Unknown command. Type !help for available commands.")
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

Always check return values from RCON functions:

```lua
local success, response = rcon_kick(player_id, reason)
if not success then
    log_error("Failed to kick player: " .. response)
    return
end
```

### Performance Considerations

1. **Keep scripts short** - Long scripts can block workflow execution
2. **Use timeouts** - Set appropriate timeout values for your scripts
3. **Avoid infinite loops** - Always have exit conditions
4. **Cache expensive operations** - Store results in variables when possible

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
