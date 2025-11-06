---
title: "Workflow Steps"
---

Workflow steps define the actions that are executed when a workflow is triggered. Each step has a specific type and configuration that determines what it does.

## Step Types

### Action Steps

Action steps perform operations such as sending commands, logging messages, or making HTTP requests.

#### RCON Command (`rcon_command`)

Executes an RCON command on the server.

**Configuration:**

- `command` (required) - The RCON command to execute

**Example:**

```json
{
  "name": "Broadcast Warning",
  "type": "action",
  "config": {
    "action_type": "rcon_command",
    "command": "AdminBroadcast Player ${trigger_event.player_name} has been warned for teamkilling"
  }
}
```

#### Admin Broadcast (`admin_broadcast`)

Sends a broadcast message visible to all players.

**Configuration:**

- `message` (required) - The message to broadcast

**Example:**

```json
{
  "name": "Welcome New Player",
  "type": "action",
  "config": {
    "action_type": "admin_broadcast",
    "message": "Welcome ${trigger_event.player_name} to our server! Please read the rules."
  }
}
```

#### Chat Message (`chat_message`)

Sends a private chat message to a specific player.

**Configuration:**

- `target_player` (required) - Player ID (Steam ID or player name) to send message to
- `message` (required) - The chat message to send

**Example:**

```json
{
  "name": "Auto Reply to Help",
  "type": "action",
  "config": {
    "action_type": "chat_message",
    "target_player": "${trigger_event.steam_id}",
    "message": "Hi ${trigger_event.player_name}! Type !rules for server rules."
  }
}
```

#### Kick Player (`kick_player`)

Kicks a player from the server.

**Configuration:**

- `player_id` (required) - Player ID (Steam ID or player name) to kick
- `reason` (optional) - Reason for the kick

**Example:**

```json
{
  "name": "Kick Teamkiller",
  "type": "action",
  "config": {
    "action_type": "kick_player",
    "player_id": "${trigger_event.steam_id}",
    "reason": "Repeated teamkilling"
  }
}
```

#### Ban Player (`ban_player`)

Bans a player from the server.

**Configuration:**

- `player_id` (required) - Player ID (Steam ID or player name) to ban
- `duration` (required) - Ban duration in days (0 = permanent)
- `reason` (optional) - Reason for the ban

**Example:**

```json
{
  "name": "Ban Cheater",
  "type": "action",
  "config": {
    "action_type": "ban_player",
    "player_id": "${trigger_event.steam_id}",
    "duration": 7,
    "reason": "Cheating detected"
  }
}
```

#### Warn Player (`warn_player`)

Sends a warning message to a specific player.

**Configuration:**

- `player_id` (required) - Player ID (Steam ID or player name) to warn
- `message` (required) - Warning message

**Example:**

```json
{
  "name": "Warn Teamkiller",
  "type": "action",
  "config": {
    "action_type": "warn_player",
    "player_id": "${trigger_event.steam_id}",
    "message": "Warning: Teamkilling is against server rules. Next violation will result in a kick."
  }
}
```

#### HTTP Request (`http_request`)

Makes an HTTP request to an external service.

**Configuration:**

- `url` (required) - The URL to send the request to
- `method` (optional) - HTTP method (GET, POST, PUT, DELETE) - defaults to GET
- `body` (optional) - Request body for POST/PUT requests
- `headers` (optional) - HTTP headers as key-value object
- `fail_on_error` (optional) - Whether to fail the workflow on non-2xx status codes

**Example:**

```json
{
  "name": "Notify External API",
  "type": "action",
  "config": {
    "action_type": "http_request",
    "url": "https://api.example.com/events",
    "method": "POST",
    "headers": {
      "Content-Type": "application/json",
      "Authorization": "Bearer ${webhook_token}"
    },
    "body": "{\"event\": \"player_joined\", \"player\": \"${trigger_event.player_name}\"}",
    "fail_on_error": true
  }
}
```

#### Webhook (`webhook`)

Sends a webhook notification with workflow and event data.

**Configuration:**

- `url` (required) - Webhook URL
- `payload` (optional) - Custom payload data to include (merged with default data)
- `headers` (optional) - Custom HTTP headers

**Default Payload Data:**

The webhook automatically includes:

- `workflow_id` - Current workflow ID
- `execution_id` - Current execution ID
- `server_id` - Server ID
- `trigger_event` - Full trigger event data
- `variables` - Current workflow variables
- `metadata` - Workflow metadata
- `timestamp` - Unix timestamp

**Example:**

```json
{
  "name": "Discord Notification",
  "type": "action",
  "config": {
    "action_type": "webhook",
    "url": "https://discord.com/api/webhooks/...",
    "payload": {
      "content": "Player **${trigger_event.player_name}** joined the server!",
      "custom_field": "additional_data"
    },
    "headers": {
      "Authorization": "Bearer ${webhook_token}"
    }
  }
}
```

#### Discord Message (`discord_message`)

Sends a formatted message to Discord via webhook.

**Configuration:**

- `webhook_url` (required) - Discord webhook URL
- `message` (required) - Message content (supports Discord markdown)
- `username` (optional) - Custom bot username
- `avatar_url` (optional) - Custom bot avatar URL

**Example:**

```json
{
  "name": "Discord Admin Alert",
  "type": "action",
  "config": {
    "action_type": "discord_message",
    "webhook_url": "https://discord.com/api/webhooks/...",
    "message": "🚨 **Admin Alert**: ${trigger_event.player_name} was kicked for teamkilling",
    "username": "Squad Aegis",
    "avatar_url": "https://example.com/bot-avatar.png"
  }
}
```

#### Log Message (`log_message`)

Writes a message to the server logs.

**Configuration:**

- `message` (required) - Message to log
- `level` (required) - Log level (debug, info, warn, error)

**Example:**

```json
{
  "name": "Log Player Event",
  "type": "action",
  "config": {
    "action_type": "log_message",
    "message": "Player ${trigger_event.player_name} triggered workflow ${metadata.workflow_name}",
    "level": "info"
  }
}
```

#### Set Variable (`set_variable`)

Sets or updates a workflow variable.

**Configuration:**

- `variable_name` (required) - Name of the variable
- `variable_value` (required) - Value to assign

**Example:**

```json
{
  "name": "Track Last Player",
  "type": "action",
  "config": {
    "action_type": "set_variable",
    "variable_name": "last_player_name",
    "variable_value": "${trigger_event.player_name}"
  }
}
```

#### Lua Script (`lua_script`)

Executes a custom Lua script with full access to workflow data.

**Configuration:**

- `script` (required) - Lua script code
- `timeout_seconds` (optional) - Maximum execution time (default: 30)

**Available Lua Functions:**

- **Logging**: `log(message)`, `log_debug(message)`, `log_warn(message)`, `log_error(message)`
- **Variables**: `set_variable(name, value)`, `get_variable(name)`
- **Utilities**: `json_encode(table)`, `json_decode(string)`, `safe_get(table, key, default)`, `to_string(value, default)`
- **RCON Commands**: `rcon_execute(command)`, `rcon_kick(player_id, reason)`, `rcon_ban(player_id, duration, reason)`, `rcon_warn(player_id, message)`, `rcon_broadcast(message)`
- **Workflow Data**: `workflow.trigger_event`, `workflow.metadata`, `workflow.variables`, `workflow.step_results`
- **Results**: `result` table to store step output

**Example:**

```json
{
  "name": "Advanced Player Analysis",
  "type": "action",
  "config": {
    "action_type": "lua_script",
    "script": "-- Get player data\nlocal player_name = workflow.trigger_event.player_name\nlocal damage = workflow.trigger_event.damage or 0\n\n-- Log analysis\nif damage > 50 then\n  log_warn(\"High damage teamkill by \" .. player_name .. \": \" .. damage)\n  set_variable(\"high_damage_tk_count\", (get_variable(\"high_damage_tk_count\") or 0) + 1)\nelse\n  log(\"Low damage incident by \" .. player_name)\nend\n\n-- Store results\nresult.player = player_name\nresult.damage = damage\nresult.severity = damage > 50 and \"high\" or \"low\"",
    "timeout_seconds": 10
  }
}
```

### Condition Steps

Condition steps evaluate expressions and can branch the workflow execution.

**Configuration:**

- Conditions are configured through the UI with field, operator, and value selections

### Variable Steps

Variable steps perform operations on workflow variables.

**Configuration:**

- `variable_name` (required) - Name of the variable to operate on
- `variable_value` (required) - Value or operation to perform

### Delay Steps

Delay steps pause workflow execution for a specified duration.

**Configuration:**

- `delay_ms` (required) - Delay duration in milliseconds

**Example:**

```json
{
  "name": "Wait 5 Seconds",
  "type": "delay",
  "config": {
    "delay_ms": 5000
  }
}
```

### Lua Steps

Lua steps are similar to Lua Script actions but are designed for more complex logic and branching.

**Configuration:**

- `script` (required) - Lua script code
- `timeout_seconds` (optional) - Maximum execution time

## Variable Replacement

In most text fields, you can use variable replacement syntax to access dynamic data:

### Syntax

- `${trigger_event.field_name}` - Access trigger event data
- `${metadata.field_name}` - Access workflow metadata
- `${variable_name}` - Access workflow variables

### Common Trigger Event Fields

**Chat Messages:**

- `${trigger_event.player_name}` - Player who sent the message
- `${trigger_event.message}` - Message content
- `${trigger_event.chat_type}` - Type of chat (ChatAll, ChatTeam, etc.)

**Player Events:**

- `${trigger_event.player_name}` - Player name
- `${trigger_event.steam_id}` - Player's Steam ID
- `${trigger_event.eos_id}` - Player's Epic Online Services ID

**Admin Events:**

- `${trigger_event.player_name}` - Target player name
- `${trigger_event.message}` - Admin message or reason

### Metadata Fields

- `${metadata.workflow_name}` - Current workflow name
- `${metadata.workflow_id}` - Current workflow ID
- `${metadata.execution_id}` - Current execution ID
- `${metadata.server_id}` - Server ID
- `${metadata.started_at}` - Execution start time

## Error Handling

Each step can have error handling configuration:

- **Action**: What to do on error (continue, stop, retry)
- **Max Retries**: Maximum number of retry attempts
- **Retry Delay**: Delay between retries in milliseconds

## Best Practices

1. **Use descriptive names** for steps to make workflows easier to understand
2. **Test variable replacement** with simple log messages before using in complex actions
3. **Set appropriate timeouts** for Lua scripts and HTTP requests
4. **Use error handling** for steps that might fail (HTTP requests, external services)
5. **Log important events** to help with debugging and monitoring
6. **Keep Lua scripts simple** and well-commented
7. **Use variables** to share data between steps effectively

## Example Workflow

Here's a complete example workflow that demonstrates multiple step types:

```json
{
  "version": "1.0",
  "triggers": [
    {
      "name": "Teamkill Detection",
      "event_type": "LOG_PLAYER_DIED",
      "conditions": [
        {
          "field": "teamkill",
          "operator": "equals",
          "value": true,
          "type": "boolean"
        }
      ],
      "enabled": true
    }
  ],
  "variables": {
    "max_warnings": 3,
    "warning_message": "Teamkilling is against server rules!"
  },
  "steps": [
    {
      "name": "Log Teamkill Incident",
      "type": "action",
      "enabled": true,
      "config": {
        "action_type": "log_message",
        "message": "Teamkill detected: ${trigger_event.attacker_name} killed ${trigger_event.victim_name}",
        "level": "warn"
      }
    },
    {
      "name": "Analyze Damage",
      "type": "lua",
      "enabled": true,
      "config": {
        "script": "local damage = workflow.trigger_event.damage or 0\nlocal attacker = workflow.trigger_event.attacker_name\n\nif damage > 75 then\n  log_warn(\"High damage teamkill: \" .. damage)\n  set_variable(\"action_required\", \"kick\")\nelse\n  log(\"Low damage teamkill: \" .. damage)\n  set_variable(\"action_required\", \"warn\")\nend\n\nresult.damage = damage\nresult.severity = damage > 75 and \"high\" or \"low\""
      }
    },
    {
      "name": "Warn Player",
      "type": "action",
      "enabled": true,
      "config": {
        "action_type": "warn_player",
        "player_name": "${trigger_event.attacker_name}",
        "message": "${warning_message} This was a ${action_required} offense."
      }
    },
    {
      "name": "Notify Discord",
      "type": "action",
      "enabled": true,
      "config": {
        "action_type": "discord_message",
        "webhook_url": "https://discord.com/api/webhooks/...",
        "message": "⚠️ **Teamkill Alert**\n**Attacker:** ${trigger_event.attacker_name}\n**Victim:** ${trigger_event.victim_name}\n**Damage:** ${trigger_event.damage}\n**Action:** ${action_required}"
      }
    }
  ]
}
```

This workflow demonstrates:

- Logging for audit trail
- Lua script for complex logic
- Variable usage and setting
- Discord integration for notifications
- Variable replacement throughout

#### Common RCON Commands

- `AdminBroadcast <message>` - Broadcast message to all players
- `AdminWarn <steam_id> <message>` - Send warning to specific player
- `AdminKick <steam_id> <reason>` - Kick player from server
- `AdminBan <steam_id> <duration> <reason>` - Ban player (duration in minutes)
- `AdminForceTeamChange <steam_id>` - Force player to switch teams
- `AdminChangeMap <layer>` - Change the current map layer
- `AdminSetMaxNumPlayers <number>` - Set maximum player count
- `AdminSlowMo <multiplier>` - Change game speed (admin testing)
- `AdminAlwaysValidPlacement <0|1>` - Toggle placement validation
- `AdminDisbandSquad <team_id> <squad_id>` - Disband a squad

### Log Message Actions

Write messages to the server logs with different severity levels.

```json
{
  "name": "Log Workflow Event",
  "type": "action",
  "enabled": true,
  "config": {
    "action_type": "log_message",
    "message": "Player ${trigger_event.player_name} triggered workflow at ${trigger_event.time}",
    "level": "info"
  }
}
```

#### Available Log Levels

- `debug` - Debug information for development
- `info` - General information about workflow execution
- `warn` - Warning messages for unusual but non-critical events
- `error` - Error messages for failed operations

### Variable Setting Actions

Set or update workflow variables for state tracking.

```json
{
  "name": "Update Help Counter",
  "type": "action",
  "enabled": true,
  "config": {
    "action_type": "set_variable",
    "variable_name": "help_requests",
    "variable_value": "${variables.help_requests + 1}"
  }
}
```

## Variable Steps (`variable`)

Variable steps provide a dedicated way to manipulate workflow variables.

### Basic Variable Setting

```json
{
  "name": "Initialize Counter",
  "type": "variable",
  "enabled": true,
  "config": {
    "action_type": "set_variable",
    "variable_name": "player_count",
    "variable_value": 0
  }
}
```

### Complex Variable Operations

```json
{
  "name": "Track Last Player",
  "type": "variable",
  "enabled": true,
  "config": {
    "action_type": "set_variable",
    "variable_name": "last_player_info",
    "variable_value": {
      "name": "${trigger_event.player_name}",
      "steam_id": "${trigger_event.steam_id}",
      "timestamp": "${trigger_event.time}"
    }
  }
}
```

## Delay Steps (`delay`)

Delay steps pause workflow execution for a specified duration.

### Basic Delay

```json
{
  "name": "Wait 5 Seconds",
  "type": "delay",
  "enabled": true,
  "config": {
    "delay_ms": 5000
  }
}
```

### Common Delay Patterns

#### Player Loading Delay

```json
{
  "name": "Wait for Player to Load",
  "type": "delay",
  "enabled": true,
  "config": {
    "delay_ms": 10000
  }
}
```

#### Rate Limiting Delay

```json
{
  "name": "Rate Limit Commands",
  "type": "delay",
  "enabled": true,
  "config": {
    "delay_ms": 2000
  }
}
```

## Lua Script Steps (`lua`)

Execute custom Lua scripts for complex logic and data manipulation.

### Basic Lua Script

```json
{
  "name": "Custom Player Processing",
  "type": "lua",
  "enabled": true,
  "config": {
    "script": "local player_name = workflow.trigger_event.player_name\nlog('Player ' .. player_name .. ' triggered workflow')\nworkflow.variables.processed_players = workflow.variables.processed_players + 1"
  }
}
```

### Advanced Lua Script with Conditionals

```json
{
  "name": "Advanced Player Analysis",
  "type": "lua",
  "enabled": true,
  "config": {
    "script": "-- Get player info\nlocal player_name = workflow.trigger_event.player_name\nlocal steam_id = workflow.trigger_event.steam_id\n\n-- Check if this is a returning player\nif workflow.variables.known_players == nil then\n    workflow.variables.known_players = {}\nend\n\nif workflow.variables.known_players[steam_id] then\n    log('Returning player: ' .. player_name)\n    set_variable('player_status', 'returning')\nelse\n    log('New player: ' .. player_name)\n    workflow.variables.known_players[steam_id] = player_name\n    set_variable('player_status', 'new')\nend\n\n-- Set result for next steps\nresult.player_type = get_variable('player_status')\nresult.message = 'Processed player: ' .. player_name"
  }
}
```

### Available Lua Functions

#### Logging Functions

`log(level, message)` - Log messages at specified level (`debug`, `info`, `warn`, `error`)

#### Variable Functions

- `set_variable(name, value)` - Set a workflow variable
- `get_variable(name)` - Get a workflow variable value

#### Utility Functions

- `json_encode(table)` - Convert Lua table to JSON string
- `json_decode(string)` - Parse JSON string to Lua table

#### Access to Workflow Data

- `workflow.trigger_event` - Access to trigger event data
- `workflow.variables` - Access to workflow variables
- `workflow.step_results` - Access to previous step results
- `workflow.execution_id` - Current execution ID
- `result` - Table to store step results

## Condition Steps (`condition`)

*Note: Condition steps are a planned feature for future releases.*

Condition steps will allow branching workflow execution based on runtime conditions.

### Planned Condition Step Structure

```json
{
  "name": "Check Server Population",
  "type": "condition",
  "enabled": true,
  "config": {
    "condition": {
      "field": "trigger_event.player_count",
      "operator": "greater_than",
      "value": 50
    },
    "true_steps": ["high-pop-action"],
    "false_steps": ["low-pop-action"]
  }
}
```

## Error Handling

Configure how steps handle failures and errors.

### Step-Level Error Handling

Each step can define its own error handling behavior:

```json
{
  "name": "Potentially Failing Operation",
  "type": "action",
  "enabled": true,
  "config": {
    "action_type": "rcon_command",
    "command": "AdminBroadcast ${trigger_event.player_name} joined!"
  },
  "on_error": {
    "action": "retry",
    "max_retries": 3,
    "retry_delay_ms": 1000
  }
}
```

### Available Error Actions

- `continue` - Continue to the next step despite the error
- `stop` - Stop workflow execution immediately
- `retry` - Retry the failed step with specified parameters

### Workflow-Level Error Handling

Set default error handling for the entire workflow:

```json
{
  "error_handling": {
    "default_action": "continue",
    "max_retries": 3,
    "retry_delay_ms": 1000
  }
}
```

## Best Practices for Steps

### Naming Conventions

Use clear, descriptive names for steps:

```json
{
  "name": "Broadcast Welcome Message to New Player",
  "type": "action"
}
```

### Logical Grouping

Group related steps together and use consistent naming:

```json
[
  {
    "id": "validate-player-input",
    "name": "Validate Player Input"
  },
  {
    "id": "process-player-command",
    "name": "Process Player Command"
  },
  {
    "id": "respond-to-player",
    "name": "Send Response to Player"
  }
]
```

### Error Handling Strategy

Always consider what should happen if a step fails:

- **Critical Steps**: Use `stop` to prevent further execution
- **Optional Steps**: Use `continue` to proceed despite failures
- **Unreliable Steps**: Use `retry` with reasonable limits

### Variable Management

Keep variable names consistent and meaningful:

```json
{
  "config": {
    "action_type": "set_variable",
    "variable_name": "teamkill_warning_count_player_${trigger_event.steam_id}",
    "variable_value": "${variables.teamkill_warning_count_player_${trigger_event.steam_id} + 1}"
  }
}
```

### Performance Considerations

1. **Minimize Delays**: Only use delays when necessary
2. **Efficient Lua Scripts**: Keep Lua scripts simple and fast
3. **Conditional Execution**: Use conditions to avoid unnecessary processing
4. **Variable Cleanup**: Periodically clean up unused variables

### Security Considerations

1. **Input Validation**: Always validate data from trigger events
2. **Command Injection**: Be careful with dynamic RCON commands
3. **Rate Limiting**: Implement delays to prevent spam
4. **Admin Verification**: Verify admin permissions for sensitive operations

## Advanced Examples

### Multi-Step Player Warning System

```json
{
  "steps": [
    {
      "name": "Check Player Warning Count",
      "type": "lua",
      "enabled": true,
      "config": {
        "script": "local steam_id = workflow.trigger_event.steam_id\nlocal warnings = workflow.variables['warnings_' .. steam_id] or 0\nwarnings = warnings + 1\nworkflow.variables['warnings_' .. steam_id] = warnings\nresult.warning_count = warnings\nresult.should_kick = warnings >= 3"
      }
    },
    {
      "name": "Send Warning to Player",
      "type": "action",
      "enabled": true,
      "config": {
        "action_type": "rcon_command",
        "command": "AdminWarn ${trigger_event.steam_id} Warning ${step_results.check-warning-count.warning_count}/3: Please follow server rules!"
      }
    },
    {
      "name": "Kick Player if Max Warnings Reached",
      "type": "lua",
      "enabled": true,
      "config": {
        "script": "if workflow.step_results['check-warning-count'].should_kick then\n    -- This would ideally be a conditional step\n    log('Player ' .. workflow.trigger_event.player_name .. ' reached maximum warnings')\nend"
      }
    }
  ]
}
```

### Dynamic Server Management

```json
{
  "steps": [
    {
      "name": "Get Current Server Status",
      "type": "action",
      "enabled": true,
      "config": {
        "action_type": "rcon_command",
        "command": "ShowCurrentMap"
      }
    },
    {
      "name": "Process Server Information",
      "type": "lua",
      "enabled": true,
      "config": {
        "script": "local player_count = workflow.trigger_event.player_count\nlocal peak_hours = workflow.variables.peak_hours or {19, 20, 21, 22}\nlocal current_hour = tonumber(os.date('%H'))\n\nlocal is_peak = false\nfor _, hour in ipairs(peak_hours) do\n    if hour == current_hour then\n        is_peak = true\n        break\n    end\nend\n\nresult.is_peak_time = is_peak\nresult.needs_map_rotation = (is_peak and player_count > 60) or (not is_peak and player_count < 20)"
      }
    },
    {
      "name": "Rotate Map if Conditions Met",
      "type": "lua",
      "enabled": true,
      "config": {
        "script": "if workflow.step_results['process-server-info'].needs_map_rotation then\n    local maps = {'Gorodok_RAAS_v1', 'Yehorivka_RAAS_v2', 'Tallil_RAAS_v1'}\n    local current_map = workflow.variables.current_map_index or 1\n    local next_map = maps[current_map + 1] or maps[1]\n    \n    workflow.variables.current_map_index = (current_map % #maps) + 1\n    result.next_map = next_map\n    result.should_rotate = true\nelse\n    result.should_rotate = false\nend"
      }
    }
  ]
}
```
