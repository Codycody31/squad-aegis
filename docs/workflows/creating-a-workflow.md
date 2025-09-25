---
title: "Creating a Workflow"
---

# Creating a Workflow

This guide will walk you through creating your first workflow in Squad Aegis, from basic concepts to practical examples.

## Understanding Triggers and Conditions

### Triggers

Triggers define what events will activate your workflow. Each trigger specifies:

- **Event Type**: The type of event to listen for
- **Conditions**: Optional filters to refine when the trigger activates
- **Name**: A descriptive name for the trigger

### Conditions

Conditions allow you to filter events and control when workflows execute. They use field names from the event data and operators to compare values.

#### Condition Operators

- `equals` - Exact match
- `not_equals` - Does not match
- `contains` - String contains substring
- `not_contains` - String does not contain substring
- `starts_with` - String starts with value
- `ends_with` - String ends with value
- `regex` - Regular expression match
- `greater_than` - Numeric comparison (>)
- `less_than` - Numeric comparison (<)
- `greater_or_equal` - Numeric comparison (>=)
- `less_or_equal` - Numeric comparison (<=)

#### Field Access

Fields can be accessed using dot notation for nested data:

- `message` - Direct field access
- `victim.steam_id` - Nested object field
- `metadata.some_value` - JSON parsed field

## Example Workflows

### Example 1: Help Command Response

**Goal**: Respond to players typing "!help" with server information

**Trigger:** Player types "!help" in any chat  
**Action:** Broadcast help message

```json
{
  "name": "Help Command Response",
  "description": "Responds to !help command with server information",
  "definition": {
    "version": "1.0",
    "triggers": [
      {
        "id": "help-trigger",
        "name": "Help Command",
        "event_type": "RCON_CHAT_MESSAGE",
        "conditions": [
          {
            "field": "message",
            "operator": "contains",
            "value": "!help"
          }
        ],
        "enabled": true
      }
    ],
    "steps": [
      {
        "id": "broadcast-help",
        "name": "Broadcast Help",
        "type": "action",
        "enabled": true,
        "config": {
          "action_type": "rcon_command",
          "command": "AdminBroadcast Welcome ${trigger_event.player_name}! Visit our Discord for help: discord.gg/example"
        }
      }
    ]
  }
}
```

**Key Components:**

- **Trigger**: Listens for `RCON_CHAT_MESSAGE` events
- **Condition**: Only activates when message contains "!help"
- **Action**: Broadcasts a personalized welcome message
- **Variable**: Uses `${trigger_event.player_name}` to include the player's name

### Example 2: Admin-Only Commands

**Goal**: Handle commands that should only work in admin chat

**Trigger:** Admin types command in admin chat  
**Action:** Execute admin command and log it

```json
{
  "name": "Admin Command Handler",
  "description": "Handles admin commands in admin chat",
  "definition": {
    "version": "1.0",
    "triggers": [
      {
        "id": "admin-command",
        "name": "Admin Command",
        "event_type": "RCON_CHAT_MESSAGE",
        "conditions": [
          {
            "field": "chat_type",
            "operator": "equals",
            "value": "ChatAdmin"
          },
          {
            "field": "message",
            "operator": "starts_with",
            "value": "!admin"
          }
        ],
        "enabled": true
      }
    ],
    "steps": [
      {
        "id": "log-admin-command",
        "name": "Log Admin Command",
        "type": "action",
        "enabled": true,
        "config": {
          "action_type": "log_message",
          "message": "Admin ${trigger_event.player_name} executed: ${trigger_event.message}",
          "level": "info"
        }
      },
      {
        "id": "broadcast-admin-response",
        "name": "Acknowledge Admin Command",
        "type": "action",
        "enabled": true,
        "config": {
          "action_type": "rcon_command",
          "command": "AdminBroadcast Admin command executed by ${trigger_event.player_name}"
        }
      }
    ]
  }
}
```

**Key Components:**

- **Multiple Conditions**: Both chat type must be `ChatAdmin` AND message must start with "!admin"
- **Multiple Steps**: First logs the command, then broadcasts acknowledgment
- **Security**: Ensures only admin chat commands are processed

### Example 3: Teamkill Detection and Warning

**Goal**: Automatically warn players who teamkill

**Trigger:** Player teamkills another player  
**Action:** Warn the teamkiller and log the incident

```json
{
  "name": "Teamkill Warning System",
  "description": "Warns players who commit teamkills",
  "definition": {
    "version": "1.0",
    "triggers": [
      {
        "id": "teamkill-trigger",
        "name": "Teamkill Detection",
        "event_type": "LOG_PLAYER_DIED",
        "conditions": [
          {
            "field": "teamkill",
            "operator": "equals",
            "value": true
          }
        ],
        "enabled": true
      }
    ],
    "variables": {
      "teamkill_count": 0
    },
    "steps": [
      {
        "id": "increment-counter",
        "name": "Increment Teamkill Counter",
        "type": "variable",
        "enabled": true,
        "config": {
          "action_type": "set_variable",
          "variable_name": "teamkill_count",
          "variable_value": "${variables.teamkill_count + 1}"
        }
      },
      {
        "id": "warn-teamkiller",
        "name": "Warn Teamkiller",
        "type": "action",
        "enabled": true,
        "config": {
          "action_type": "rcon_command",
          "command": "AdminWarn ${trigger_event.attacker_steam} Teamkilling is not allowed! This is warning ${variables.teamkill_count}"
        }
      },
      {
        "id": "log-teamkill",
        "name": "Log Teamkill",
        "type": "action",
        "enabled": true,
        "config": {
          "action_type": "log_message",
          "message": "Teamkill: ${trigger_event.attacker_name} killed ${trigger_event.victim_name} with ${trigger_event.weapon}",
          "level": "warn"
        }
      }
    ]
  }
}
```

**Key Components:**

- **Death Event**: Listens for `LOG_PLAYER_DIED` events
- **Teamkill Filter**: Only processes when `teamkill` is true
- **Variable Tracking**: Counts teamkills across workflow executions
- **Multiple Actions**: Increments counter, warns player, and logs incident

### Example 4: Player Welcome System

**Goal**: Welcome new players when they join

**Trigger:** Player connects to server  
**Action:** Welcome them with a delayed message

```json
{
  "name": "Player Welcome System",
  "description": "Welcomes new players when they join",
  "definition": {
    "version": "1.0",
    "triggers": [
      {
        "id": "player-join",
        "name": "Player Connected",
        "event_type": "LOG_PLAYER_CONNECTED",
        "conditions": [],
        "enabled": true
      }
    ],
    "steps": [
      {
        "id": "wait-for-load",
        "name": "Wait for Player to Load",
        "type": "delay",
        "enabled": true,
        "config": {
          "delay_ms": 10000
        }
      },
      {
        "id": "welcome-message",
        "name": "Send Welcome Message",
        "type": "action",
        "enabled": true,
        "config": {
          "action_type": "rcon_command",
          "command": "AdminWarn ${trigger_event.steam_id} Welcome to our server! Type !help for commands and !rules for server rules."
        }
      }
    ]
  }
}
```

**Key Components:**

- **Connection Event**: Listens for `LOG_PLAYER_CONNECTED` events
- **No Conditions**: Activates for all player connections
- **Delay Step**: Waits 10 seconds for the player to fully load
- **Personal Welcome**: Sends a private message to the connecting player

## Common Condition Examples

### Message contains help command

```json
{
  "field": "message",
  "operator": "contains",
  "value": "!help"
}
```

### Admin chat only

```json
{
  "field": "chat_type",
  "operator": "equals",
  "value": "ChatAdmin"
}
```

### Specific player

```json
{
  "field": "steam_id",
  "operator": "equals",
  "value": "76561198000000000"
}
```

### Message starts with command prefix

```json
{
  "field": "message",
  "operator": "starts_with",
  "value": "!"
}
```

### High damage teamkill

```json
[
  {
    "field": "teamkill",
    "operator": "equals",
    "value": true
  },
  {
    "field": "damage",
    "operator": "greater_than",
    "value": "50"
  }
]
```

## Variables

Variables allow workflows to store and share data across executions. They can be used in step configurations using `${variable_name}` syntax.

### Built-in Variables

#### Trigger Event Data

Access trigger event fields using `${trigger_event.field_name}`:

- `${trigger_event.player_name}` - Player who triggered the event
- `${trigger_event.steam_id}` - Player's Steam ID
- `${trigger_event.message}` - Chat message content
- `${trigger_event.chat_type}` - Type of chat

#### Step Results

Access results from previous steps using `${step_results.step_id}`:

- `${step_results.rcon_response.command}` - RCON command that was executed
- `${step_results.rcon_response.response}` - RCON command response

#### Workflow Variables

Access custom variables using `${variables.variable_name}`:

- `${variables.help_count}` - Custom counter
- `${variables.last_player}` - Last player who triggered workflow

### Variable Examples

**Player Name in Message:**

```json
{
  "command": "AdminBroadcast Hello ${trigger_event.player_name}!"
}
```

**Conditional Logic:**

```json
{
  "command": "AdminWarn ${trigger_event.steam_id} Warning ${variables.warning_count} of 3"
}
```

## Error Handling

Configure how workflows handle failures and errors.

### Error Actions

- `continue` - Continue to next step
- `stop` - Stop workflow execution
- `retry` - Retry the failed step

### Workflow-Level Error Handling

```json
{
  "error_handling": {
    "default_action": "continue",
    "max_retries": 3,
    "retry_delay_ms": 1000
  }
}
```

### Step-Level Error Handling

```json
{
  "on_error": {
    "action": "retry",
    "max_retries": 2,
    "retry_delay_ms": 500
  }
}
```