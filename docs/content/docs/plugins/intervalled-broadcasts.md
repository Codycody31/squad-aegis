---
title: Intervalled Broadcasts
---

The Intervalled Broadcasts plugin allows you to set up automated broadcast messages that are sent to all players at regular intervals, perfect for server rules, announcements, or promotional messages.

## Features

- Automated periodic broadcasts to all players
- Multiple messages with sequential or random rotation
- Configurable broadcast intervals
- Enable/disable control
- Message shuffling option

## Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `broadcasts` | Array of messages to broadcast | ["This server is powered by Squad Aegis."] | No |
| `interval_ms` | Broadcast frequency in milliseconds | 300000 | No |
| `enabled` | Whether the plugin is enabled | true | No |
| `shuffle_messages` | Random message order | false | No |

## How It Works

1. The plugin maintains a list of broadcast messages
2. At configured intervals, it sends the next message to all players
3. Messages can be sent in order or randomly shuffled
4. The cycle repeats when all messages have been sent
5. Continues until disabled or server shutdown

## Example Configuration

```json
{
  "broadcasts": [
    "Welcome to our Squad server! Please read the rules.",
    "Join our Discord: https://discord.gg/example",
    "Report issues to admins using @admin in chat.",
    "Have fun and play fair!"
  ],
  "interval_ms": 600000,
  "shuffle_messages": true
}
```

## Tips

- Use intervals of 5-15 minutes for optimal player experience
- Keep messages concise and relevant
- Include important server information like rules and contact details
- Test message timing to avoid spam
- Use shuffle for variety or sequential for structured announcements
- Consider player feedback when setting broadcast frequency
