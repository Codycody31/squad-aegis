---
title: Auto TK Warn
icon: lucide:plug
---

# Auto TK Warn

The Auto TK Warn plugin automatically sends warning messages to players when they commit team kills (TKs), helping to maintain server discipline and encourage proper communication.

## Features

- Automatically detects team kills from game logs
- Sends customizable warning messages to attackers
- Optional warning messages to victims
- Configurable enable/disable for each message type
- Integrates with Squad's logging system

## Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `attacker_message` | Message sent to the player who committed the TK | "Please apologise for ALL TKs in ALL chat!" | No |
| `victim_message` | Message sent to the victim (leave empty to disable) | "" | No |
| `enabled` | Whether the plugin is enabled | true | No |
| `warn_attacker` | Whether to warn the attacker | true | No |
| `warn_victim` | Whether to warn the victim | false | No |

## How It Works

1. The plugin monitors game logs for team kill events
2. When a team kill is detected, it identifies both the attacker and victim
3. Based on configuration, sends appropriate warning messages to the players
4. Messages are sent through the game's chat system

## Example Configuration

```json
{
  "attacker_message": "Team kill detected! Please apologize in all chat and be more careful.",
  "victim_message": "You've been team killed. The offending player has been warned.",
  "warn_attacker": true,
  "warn_victim": true
}
```

## Tips

- Use clear, firm language in warning messages to discourage team killing
- Consider enabling victim warnings to improve player experience
- The plugin works best when combined with other moderation tools
- Messages are sent immediately when team kills are detected
- Make sure your warning messages align with your server's rules
