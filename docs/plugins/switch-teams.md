---
title: Switch Teams
icon: lucide:plug
---

## Switch Teams

The Switch Teams plugin allows players to request immediate team switches using a chat command, with built-in balance checking to prevent team imbalance exploitation.

### Features

- Player-initiated team switching via chat command
- Automatic team balance validation
- Configurable cooldown system
- Admin-only mode option
- Immediate execution (no queuing)

### Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `command` | Chat command trigger (without !) | "switch" | No |
| `cooldown_minutes` | Cooldown between uses in minutes | 60 | No |
| `team_imbalance_threshold` | Allowed team size difference | 2 | No |
| `admin_only` | Restrict to admins only | false | No |

### How It Works

1. Players type the command in chat (e.g., `!switch`)
2. The plugin checks:
   - If the player is on cooldown
   - If admin-only mode is enabled
   - Current team balance
3. If switching would maintain acceptable balance, the player is switched immediately
4. If balance would be worsened, the switch is denied
5. Success/failure messages are sent to the player

### Team Balance Logic

- The plugin allows switches that don't worsen imbalance
- If teams are already imbalanced by more than the threshold, switches from the larger to smaller team are allowed
- Switches that would increase imbalance beyond the threshold are blocked
- The goal is to maintain fair gameplay while allowing reasonable team movement

### Example Configuration

```json
{
  "command": "switchteam",
  "cooldown_minutes": 30,
  "team_imbalance_threshold": 3,
  "admin_only": false
}
```

### Usage Examples

- Player types `!switch` in chat
- If teams are balanced (e.g., 25 vs 25), switch is allowed
- If teams are imbalanced (e.g., 28 vs 22), switch from larger to smaller team is allowed
- If switch would make imbalance worse (e.g., 26 vs 24 → 25 vs 25), it's blocked

### Tips

- Use a reasonable cooldown to prevent spam switching
- The imbalance threshold should match your server's typical player counts
- Consider admin-only mode for competitive servers
- Test the balance logic during matches to ensure it works as expected
- Players receive feedback messages about switch success/failure and cooldowns
