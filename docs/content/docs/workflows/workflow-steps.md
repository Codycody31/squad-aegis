---
title: "Workflow Steps"
---

Workflow steps are the actions a workflow runs when it triggers. Each step has a type and a `config` block. Steps run in order unless a condition step redirects flow.

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

**Common RCON Commands:**

- `AdminBroadcast <message>` - Broadcast message to all players
- `AdminWarn <player_id> <message>` - Send warning to specific player (accepts Steam ID or EOS ID)
- `AdminKick <player_id> <reason>` - Kick player from server (accepts Steam ID or EOS ID)
- `AdminBan <player_id> <duration> <reason>` - Ban player (accepts Steam ID or EOS ID, duration in days: `1d` = 1 day, `1M` = 1 month, `0` = permanent)
- `AdminForceTeamChange <player_id>` - Force player to switch teams (accepts Steam ID or EOS ID)
- `AdminChangeMap <layer>` - Change the current map layer
- `AdminSetMaxNumPlayers <number>` - Set maximum player count
- `AdminSlowMo <multiplier>` - Change game speed (admin testing)
- `AdminAlwaysValidPlacement <0|1>` - Toggle placement validation
- `AdminDisbandSquad <team_id> <squad_id>` - Disband a squad

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

- `player_id` (required) - Player ID (Steam ID, EOS ID, or player name) to kick
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

- `player_id` (required) - Player ID (Steam ID, EOS ID, or player name) to ban
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

#### Ban Player with Evidence (`ban_player_with_evidence`)

Bans a player and automatically links the triggering event as evidence in the database. This creates a complete audit trail with the event that caused the ban (chat messages, deaths, etc.) stored as evidence.

**Configuration:**

- `player_id` (required) - Player ID (Steam ID, EOS ID, or player name) to ban
- `duration` (required) - Ban duration in days (0 = permanent)
- `reason` (optional) - Reason for the ban
- `rule_id` (optional) - Server rule UUID to associate with the ban

**Supported Event Types for Evidence:**
- `RCON_CHAT_MESSAGE` - Chat message evidence
- `LOG_PLAYER_CONNECTED` - Connection event evidence
- `LOG_PLAYER_DIED` - Death event evidence
- `LOG_PLAYER_WOUNDED` - Wound event evidence
- `LOG_PLAYER_DAMAGED` - Damage event evidence

**Features:**
- Automatically creates ban record in database with UUID
- Links triggering event as evidence in ClickHouse
- Executes RCON ban command
- Kicks player immediately
- Returns ban_id, evidence_count, and success status

**Example:**

```json
{
  "name": "Ban with Offensive Language Evidence",
  "type": "action",
  "config": {
    "action_type": "ban_player_with_evidence",
    "player_id": "${trigger_event.steam_id}",
    "duration": 7,
    "reason": "Offensive language: ${trigger_event.message}",
    "rule_id": "${workflow.variables.rule_id}"
  }
}
```

**Notes:**
- If the event_id is missing from the trigger context, the ban is created without evidence (graceful degradation)
- If the event type does not support evidence, a warning is logged and the ban proceeds without evidence
- Evidence links to ClickHouse records via the event UUID for complete audit trails
- The ban record can be queried via API with attached evidence for admin review and appeals

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
    "message": "**Admin Alert**: ${trigger_event.player_name} was kicked for teamkilling",
    "username": "Squad Aegis",
    "avatar_url": "https://example.com/bot-avatar.png"
  }
}
```

#### Log Message (`log_message`)

Writes a message to the server logs.

**Configuration:**

- `message` (required) - Message to log
- `level` (required) - Log level (`debug`, `info`, `warn`, `error`)

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

#### Lua Script Action (`lua_script`)

Executes a custom Lua script as an action step with full access to workflow data. For complex scripting, prefer the dedicated `lua` step type (see [Lua Script Steps](#step-types)).

**Configuration:**

- `script` (required) - Lua script code
- `timeout_seconds` (optional) - Maximum execution time (default: 30)

**Example:**

```json
{
  "name": "Custom Player Processing",
  "type": "action",
  "config": {
    "action_type": "lua_script",
    "script": "local player_name = workflow.trigger_event.player_name\nworkflow.log.info('Player ' .. player_name .. ' triggered workflow')\nworkflow.variables.processed_players = (workflow.variables.processed_players or 0) + 1",
    "timeout_seconds": 10
  }
}
```

### Condition Steps (`condition`)

Condition steps evaluate expressions and branch workflow execution based on the result. They support multiple conditions with AND/OR logic and can execute different sets of steps depending on whether the conditions pass or fail.

**Important:** Steps specified in `true_steps` or `false_steps` will only execute when called by the condition step, not when encountered in sequential workflow order. This prevents both branches from running.

#### Configuration Fields

- **`logic`** (string): Combination operator for multiple conditions
  - `"AND"` - All conditions must be true (default)
  - `"OR"` - At least one condition must be true

- **`conditions`** (array): List of conditions to evaluate. Each condition has:
  - `field` (string): Path to the field to evaluate (e.g., `trigger_event.player_name`)
  - `operator` (string): Comparison operator (see Available Operators below)
  - `value` (any): Value to compare against
  - `type` (string): Data type (`string`, `number`, `boolean`, `array`, `object`)

- **`true_steps`** (array): Steps to execute if conditions pass. Can contain step name strings (references to root-level steps) or full step objects (inline steps). Steps in this list are automatically skipped in sequential execution.

- **`false_steps`** (array): Steps to execute if conditions fail. Can contain step name strings (references to root-level steps) or full step objects (inline steps). Steps in this list are automatically skipped in sequential execution.

- **`continue_on_next_step_error`** (boolean): Whether to continue if a branch step fails. Default: `false` (stop on error).

#### Available Operators

- `equals` - Exact match
- `not_equals` - Not equal to
- `greater_than` - Numeric greater than
- `less_than` - Numeric less than
- `greater_than_or_equal` - Numeric greater than or equal
- `less_than_or_equal` - Numeric less than or equal
- `contains` - String contains substring
- `not_contains` - String does not contain substring
- `starts_with` - String starts with
- `ends_with` - String ends with
- `in` - Value in array
- `not_in` - Value not in array
- `is_null` - Value is null
- `is_not_null` - Value is not null
- `regex_match` - Matches regular expression

#### Inline Nested Steps

Steps can be defined directly within condition branches (inline) rather than at the root level. Inline steps are scoped to their branch and support all step types except nested conditions.

You can also mix inline steps with references to existing root-level steps within the same branch.

#### Visual Branch Indicators

In the workflow editor, condition steps label the true and false branches and show a step count for each.

#### Example: Basic Condition

```json
{
  "name": "Check Server Population",
  "type": "condition",
  "enabled": true,
  "config": {
    "logic": "AND",
    "conditions": [
      {
        "field": "trigger_event.player_count",
        "operator": "greater_than",
        "value": 50,
        "type": "number"
      }
    ],
    "true_steps": ["high-pop-action"],
    "false_steps": ["low-pop-action"]
  }
}
```

#### Example: Inline Steps

```json
{
  "name": "Check Player VIP Status",
  "type": "condition",
  "enabled": true,
  "config": {
    "logic": "AND",
    "conditions": [
      {
        "field": "trigger_event.steam_id",
        "operator": "equals",
        "value": "76561199047801300",
        "type": "string"
      }
    ],
    "true_steps": [
      {
        "id": "inline-vip-broadcast",
        "name": "VIP Welcome Broadcast",
        "type": "action",
        "enabled": true,
        "config": {
          "action_type": "admin_broadcast",
          "message": "Welcome VIP ${trigger_event.player_name}!"
        }
      },
      {
        "id": "inline-vip-log",
        "name": "Log VIP Join",
        "type": "action",
        "enabled": true,
        "config": {
          "action_type": "log_message",
          "level": "info",
          "message": "VIP player ${trigger_event.player_name} joined"
        }
      }
    ],
    "false_steps": [
      {
        "id": "inline-normal-welcome",
        "name": "Normal Welcome",
        "type": "action",
        "enabled": true,
        "config": {
          "action_type": "admin_broadcast",
          "message": "Welcome ${trigger_event.player_name}!"
        }
      }
    ]
  }
}
```

#### Example: Player Whitelist Check with `in` and `continue_on_next_step_error`

```json
{
  "name": "Check Player Whitelist",
  "type": "condition",
  "enabled": true,
  "config": {
    "logic": "AND",
    "conditions": [
      {
        "field": "trigger_event.player.steam_id",
        "operator": "in",
        "value": ["76561198012345678", "76561198087654321"],
        "type": "array"
      },
      {
        "field": "trigger_event.player.banned",
        "operator": "equals",
        "value": false,
        "type": "boolean"
      }
    ],
    "true_steps": [
      "log-whitelist-join",
      "send-welcome-message",
      "grant-permissions"
    ],
    "false_steps": [
      "log-non-whitelist-join",
      "kick-player"
    ],
    "continue_on_next_step_error": false
  }
}
```

### Variable Steps (`variable`)

Variable steps set or update a workflow variable. Use them when a step's only job is to store a value; for variable changes alongside other work, use an action step with `set_variable`.

**Configuration:**

- `action_type` (required) - Must be `set_variable`
- `variable_name` (required) - Name of the variable to set
- `variable_value` (required) - Value to assign (can be a scalar or object)

**Example: Basic variable setting**

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

**Example: Object value**

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

### Delay Steps (`delay`)

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

### Lua Script Steps (`lua`)

Lua steps run a script with the full workflow API. See [Lua Scripting](/docs/workflows/lua-scripting) for the complete function reference. The script field is a string; `\n` separates lines.

**Configuration:**

- `script` (required) - Lua script code
- `timeout_seconds` (optional) - Maximum execution time (default: 30)

**Example:**

```json
{
  "name": "Advanced Player Analysis",
  "type": "lua",
  "enabled": true,
  "config": {
    "script": "local player_name = workflow.trigger_event.player_name\nlocal damage = workflow.trigger_event.damage or 0\n\nif damage > 50 then\n  workflow.log.warn('High damage teamkill by ' .. player_name .. ': ' .. damage)\n  workflow.variable.set('high_damage_tk_count', (workflow.variable.get('high_damage_tk_count') or 0) + 1)\nelse\n  workflow.log.info('Low damage incident by ' .. player_name)\nend\n\nresult.player = player_name\nresult.damage = damage\nresult.severity = damage > 50 and 'high' or 'low'",
    "timeout_seconds": 10
  }
}
```

**Example: Player warning counter**

```json
{
  "name": "Check Player Warning Count",
  "type": "lua",
  "enabled": true,
  "config": {
    "script": "local steam_id = workflow.trigger_event.steam_id\nlocal warnings = workflow.variables['warnings_' .. steam_id] or 0\nwarnings = warnings + 1\nworkflow.variables['warnings_' .. steam_id] = warnings\nresult.warning_count = warnings\nresult.should_kick = warnings >= 3"
  }
}
```

## Variable Replacement

In most text fields, you can use variable replacement syntax to access dynamic data.

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
- `${trigger_event.steam_id}` - Player's Steam ID (may be empty for EOS-only players)
- `${trigger_event.eos_id}` - Player's Epic Online Services ID (may be empty for Steam-only players)

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

Each step can have error handling configuration.

### Step-Level Error Handling

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

### Error Handling Strategy

- **Critical Steps**: Use `stop` to prevent further execution
- **Optional Steps**: Use `continue` to proceed despite failures
- **Unreliable Steps**: Use `retry` with reasonable limits

## Best Practices

1. Use descriptive names for steps to make workflows easier to understand.
2. Test variable replacement with simple log messages before using in complex actions.
3. Set appropriate timeouts for Lua scripts and HTTP requests.
4. Use error handling for steps that might fail (HTTP requests, external services).
5. Log important events to help with debugging and monitoring.
6. Keep Lua scripts simple and well-commented.
7. Use variables to share data between steps effectively.
8. Keep variable names consistent and meaningful; prefix per-player keys with the player identifier (e.g., `warnings_${trigger_event.steam_id}`).
9. For condition steps, ensure each step appears in either `true_steps` or `false_steps`, not both.
10. Minimize delays; only use them when necessary (e.g., waiting for a player to fully load).

## Example Workflow

Complete example demonstrating multiple step types working together:

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
        "script": "local damage = workflow.trigger_event.damage or 0\nlocal attacker = workflow.trigger_event.attacker_name\n\nif damage > 75 then\n  workflow.log.warn('High damage teamkill: ' .. damage)\n  workflow.variable.set('action_required', 'kick')\nelse\n  workflow.log.info('Low damage teamkill: ' .. damage)\n  workflow.variable.set('action_required', 'warn')\nend\n\nresult.damage = damage\nresult.severity = damage > 75 and 'high' or 'low'"
      }
    },
    {
      "name": "Warn Player",
      "type": "action",
      "enabled": true,
      "config": {
        "action_type": "warn_player",
        "player_id": "${trigger_event.attacker_name}",
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
        "message": "**Teamkill Alert**\n**Attacker:** ${trigger_event.attacker_name}\n**Victim:** ${trigger_event.victim_name}\n**Damage:** ${trigger_event.damage}\n**Action:** ${action_required}"
      }
    }
  ]
}
```
