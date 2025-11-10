---
title: "Creating a Workflow"
---

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
  "version": "1.0",
  "triggers": [
    {
      "id": "c58f5356-d59e-4334-ad55-50500588b906",
      "name": "help-trigger",
      "event_type": "RCON_CHAT_MESSAGE",
      "conditions": [
        {
          "field": "message",
          "operator": "starts_with",
          "value": "!help",
          "type": "string"
        }
      ],
      "enabled": true
    }
  ],
  "variables": {},
  "steps": [
    {
      "id": "0b5bb151-b091-4a22-943e-11ec8e8abbaf",
      "name": "broadcast-help",
      "type": "action",
      "enabled": true,
      "config": {
        "action_type": "rcon_command",
        "command": "AdminBroadcast Welcome ${trigger_event.player_name}! Visit our Discord for help: discord.gg/example"
      },
      "on_error": {
        "action": "stop",
        "max_retries": 3,
        "retry_delay_ms": 1000
      }
    }
  ],
  "error_handling": {
    "default_action": "stop",
    "max_retries": 3,
    "retry_delay_ms": 1000
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
  "version": "1.0",
  "triggers": [
    {
      "id": "2ddef5ab-c3dd-4285-8980-fd8dcfa760ba",
      "name": "admin-command",
      "event_type": "RCON_CHAT_MESSAGE",
      "conditions": [
        {
          "field": "chat_type",
          "operator": "equals",
          "value": "ChatAdmin",
          "type": "string"
        },
        {
          "field": "message",
          "operator": "starts_with",
          "value": "!admin",
          "type": "string"
        }
      ],
      "enabled": true
    }
  ],
  "variables": {},
  "steps": [
    {
      "id": "c040d259-8ea1-4fe9-94ce-2c9059f81415",
      "name": "log-admin-command",
      "type": "action",
      "enabled": true,
      "config": {
        "action_type": "log_message",
        "level": "info",
        "message": "Admin ${trigger_event.player_name} executed: ${trigger_event.message}"
      },
      "on_error": {
        "action": "stop",
        "max_retries": 3,
        "retry_delay_ms": 1000
      }
    },
    {
      "id": "43f7d438-bf9f-46e0-a5ed-4f01ca5c3cc2",
      "name": "broadcast-admin-response",
      "type": "action",
      "enabled": true,
      "config": {
        "action_type": "rcon_command",
        "command": "AdminBroadcast Admin command executed by ${trigger_event.player_name}"
      },
      "on_error": {
        "action": "stop",
        "max_retries": 3,
        "retry_delay_ms": 1000
      }
    }
  ],
  "error_handling": {
    "default_action": "stop",
    "max_retries": 3,
    "retry_delay_ms": 1000
  }
}
```

**Key Components:**

- **Multiple Conditions**: Both chat type must be `ChatAdmin` AND message must start with "!admin"
- **Multiple Steps**: First logs the command, then broadcasts acknowledgment
- **Security**: Ensures only admin chat commands are processed

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

## Conditional Branching

Condition steps allow you to create workflows that make decisions based on runtime data. They evaluate conditions and execute different sets of steps depending on whether the conditions are true or false.

### How Condition Steps Work

**Important:** Condition steps prevent sequential execution of conditional branches. When you specify steps in `true_steps` or `false_steps`, those steps will **only** execute when called by the condition - they won't run when encountered in the normal workflow order.

This means:
- If a condition is TRUE: Only the `true_steps` execute, `false_steps` are skipped
- If a condition is FALSE: Only the `false_steps` execute, `true_steps` are skipped
- You never get both branches executing

### Example 3: VIP Player Detection (with Nested Steps)

**Goal**: Give VIP players special treatment when they join using inline nested steps

**Trigger:** Player connects to server
**Action:** Check if they're VIP and respond accordingly

This example demonstrates using **inline nested steps** directly within condition branches:

```json
{
  "version": "1.0",
  "triggers": [
    {
      "id": "c4f602f5-4f2b-40b5-98e3-494856d7a939",
      "name": "player-join-succeeded",
      "event_type": "LOG_JOIN_SUCCEEDED",
      "enabled": true
    }
  ],
  "variables": {},
  "steps": [
    {
      "id": "2f76a8f1-c8a5-45c3-9b45-a952337eb5cb",
      "name": "wait",
      "type": "delay",
      "enabled": true,
      "config": {
        "delay_ms": 30000
      },
      "on_error": {
        "action": "stop",
        "max_retries": 3,
        "retry_delay_ms": 1000
      }
    },
    {
      "id": "34f1b9fc-ce43-4086-b6c2-941d2ae27a6e",
      "name": "check-vip-status",
      "type": "condition",
      "enabled": true,
      "config": {
        "conditions": [
          {
            "field": "trigger_event.steam_id",
            "operator": "equals",
            "type": "string",
            "value": "76561199047801300"
          }
        ],
        "logic": "AND",
        "true_steps": [
          {
            "id": "inline-vip-broadcast",
            "name": "VIP Welcome Broadcast",
            "type": "action",
            "enabled": true,
            "config": {
              "action_type": "admin_broadcast",
              "message": "Welcome VIP ${trigger_event.player_suffix}! Thank you for your support!"
            }
          },
          {
            "id": "inline-vip-discord",
            "name": "Notify Discord VIP Join",
            "type": "action",
            "enabled": true,
            "config": {
              "action_type": "discord_message",
              "webhook_url": "https://discord.com/api/webhooks/...",
              "message": "🎉 VIP player **${trigger_event.player_suffix}** joined the server!"
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
              "action_type": "warn_player",
              "message": "Welcome ${trigger_event.player_suffix}! Type !vip to learn about VIP benefits.",
              "player_id": "${trigger_event.steam_id}"
            }
          }
        ]
      },
      "on_error": {
        "action": "stop",
        "max_retries": 3,
        "retry_delay_ms": 1000
      }
    },
    {
      "id": "0fc7e804-e205-4599-a0be-b1db5f285d31",
      "name": "log-player-join",
      "type": "action",
      "enabled": true,
      "config": {
        "action_type": "log_message",
        "level": "info",
        "message": "Player ${trigger_event.player_suffix} connected"
      },
      "on_error": {
        "action": "stop",
        "max_retries": 3,
        "retry_delay_ms": 1000
      }
    }
  ],
  "error_handling": {
    "default_action": "stop",
    "max_retries": 3,
    "retry_delay_ms": 1000
  }
}
```

**Key Features:**

- **Inline Steps**: VIP-specific steps are defined directly in the `true_steps` array
- **Cleaner Structure**: No need for root-level steps that are only used conditionally
- **Visual Clarity**: The editor shows ✓ for true branch (2 steps) and ✕ for false branch (1 step)
- **Execution Flow**: Steps execute in order within their branch, then workflow continues sequentially

**Execution Flow:**

1. Player connects (trigger activates)
2. Wait 30 seconds for player to fully load
3. "Check VIP Status" condition evaluates
4. **If VIP (true):**
   - Executes inline "VIP Welcome Broadcast" step
   - Executes inline "Notify Discord VIP Join" step
   - Skips "normal-welcome" inline step
   - Executes "Log Player Join" (continues sequentially)
5. **If Not VIP (false):**
   - Skips both VIP inline steps
   - Executes inline "Normal Welcome" step
   - Executes "Log Player Join" (continues sequentially)

### Configuring Condition Steps in the UI

When creating a condition step in the Squad Aegis UI:

1. **Set Step Type**: Select "Condition" as the step type
2. **Configure Logic**: Choose AND or OR for multiple conditions
3. **Add Conditions**: Use the condition builder to add evaluation rules
4. **Configure True Branch Steps**:
   - Click "Add Inline Step" to create steps directly within the true branch
   - Use the step selector dropdown to reference existing root-level steps
   - Reorder steps using the up/down arrows (hover over steps to see controls)
5. **Configure False Branch Steps**:
   - Click "Add Inline Step" to create steps directly within the false branch
   - Use the step selector dropdown to reference existing root-level steps
   - Reorder steps using the up/down arrows (hover over steps to see controls)
6. **Visual Feedback**: The editor shows visual indicators (✓ for true, ✕ for false) with step counts
7. **Test Both Branches**: Always test workflows to ensure both paths work correctly

#### Inline Steps vs Step References

**Inline Steps** are best for:
- Actions that are specific to a single branch
- Keeping workflows clean and organized
- Steps that don't need to be reused elsewhere

**Step References** are best for:
- Steps that are used in multiple places
- Shared logging or common actions
- Steps that need to be easily found and modified

### Best Practices for Conditional Workflows

1. **Name Steps Clearly**: Use descriptive names that indicate the step's purpose
2. **Use Inline Steps for Branch-Specific Actions**: Create inline nested steps for actions that only make sense within a specific branch
3. **Reference Shared Steps**: Use step references for actions that are needed in multiple places
4. **Reorder Steps Within Branches**: Use the reorder controls to ensure steps execute in the correct order
5. **One Branch Per Step**: Don't put the same step in both true and false branches
6. **Handle Both Outcomes**: Consider what should happen in both true and false scenarios
7. **Keep Branches Simple**: If logic becomes complex, split into multiple workflows
8. **Test Thoroughly**: Test both the true and false paths before deploying
9. **Use Visual Indicators**: The editor's visual branch indicators help you quickly understand workflow flow
