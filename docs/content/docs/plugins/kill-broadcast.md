---
title: Kill Broadcast
---

The Kill Broadcast plugin broadcasts messages to the Squad server when players get certain types of kills, such as knife kills or helicopter crashes. This adds flavor and entertainment to your server by highlighting special kill types.

## Features

- Broadcasts messages when specific weapon types are used for kills
- Customizable message layouts with template placeholders
- Random verb selection for dynamic messages
- Interval-based broadcasting to prevent message spam
- Immediate broadcasting option for instant notifications
- Seeding mode support (can enable/disable per kill type)
- Helicopter crash detection (self-kills in helicopters)
- Multiple kill type configurations

## Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `use_interval` | Use interval-based broadcasting rather than broadcasting right away | false | No |
| `interval_ms` | Interval in milliseconds for broadcasting queued messages (only used when use_interval is true) | 5000 | No |
| `broadcasts` | Array of kill type configurations for different types of kills | See example | No |

Each broadcast object in the `broadcasts` array has the following properties:

| Property | Description | Default | Required |
|----------|-------------|---------|----------|
| `enabled` | Whether this kill type is enabled | true | No |
| `heli` | Only use to specify heli kills (self-kills in helicopters) | false | No |
| `seeding` | Whether this kill type executes while on a seeding map. Set to false to disable on seeding maps | true | No |
| `layout` | Message layout with placeholders: `{{attacker}}`, `{{verb}}`, `{{victim}}`, `{{damage}}`, `{{weapon}}` | "{{attacker}} {{verb}} {{victim}}" | No |
| `ids` | Array of weapon IDs to match. Use weapon blueprint names from Squad | [] | Yes |
| `verbs` | Array of random verbs to use (leave empty if {{verb}} is not included in layout) | [] | No |

## Template Placeholders

The `layout` field supports the following placeholders:

- `{{attacker}}` - Name of the player who got the kill
- `{{verb}}` - Randomly selected verb from the `verbs` array
- `{{victim}}` - Name of the player who was killed
- `{{weapon}}` - Weapon blueprint name used for the kill
- `{{damage}}` - Damage amount dealt

## How It Works

1. The plugin monitors all player wound events from the game logs
2. When a kill is detected, it checks if the weapon matches any configured kill type
3. For helicopter kills (`heli: true`), it only triggers on self-kills (attacker and victim are the same)
4. For regular kills, it only triggers on non-teamkills
5. The plugin checks seeding mode status if `seeding: false` is set
6. If `use_interval` is enabled, messages are queued and broadcast at intervals
7. If `use_interval` is disabled, messages are broadcast immediately
8. The message is constructed using the layout template with placeholders replaced
9. A random verb is selected from the `verbs` array if available

## Example Configuration

```json
{
  "use_interval": false,
  "interval_ms": 5000,
  "broadcasts": [
    {
      "enabled": true,
      "heli": true,
      "seeding": true,
      "layout": "{{attacker}} {{verb}}",
      "ids": [
        "BP_MI8_AFU",
        "BP_MI8_VDV",
        "BP_UH1Y",
        "BP_UH60",
        "BP_UH1H_Desert",
        "BP_UH1H",
        "BP_CH178",
        "BP_MI8",
        "BP_CH146",
        "BP_MI17_MEA",
        "BP_Z8G",
        "BP_CH146_Desert",
        "BP_SA330",
        "BP_UH60_AUS",
        "BP_MRH90_Mag58",
        "BP_Z8J",
        "BP_Loach_CAS_Small",
        "BP_Loach",
        "BP_UH60_TLF_PKM",
        "BP_CH146_Raven"
      ],
      "verbs": [
        "CRASHED LANDED",
        "MADE A FLAWLESS LANDING",
        "YOU CAN'T PARK THERE"
      ]
    },
    {
      "enabled": true,
      "heli": false,
      "seeding": true,
      "layout": "{{attacker}} {{verb}} {{victim}}",
      "ids": [
        "BP_AK74Bayonet",
        "BP_AKMBayonet",
        "BP_Bayonet2000",
        "BP_G3Bayonet",
        "BP_M9Bayonet",
        "BP_OKC-3S",
        "BP_QNL-95_Bayonet",
        "BP_SA80Bayonet",
        "BP_SKS_Bayonet",
        "BP_SKS_Optic_Bayonet",
        "BP_SOCP_Knife_AUS",
        "BP_SOCP_Knife_ADF",
        "BP_VibroBlade_Knife_GC",
        "BP_MeleeUbop",
        "BP_BananaClub",
        "BP_Droid_Punch",
        "BP_MagnaGuard_Punch",
        "BP_FAMAS_Bayonet",
        "BP_FAMAS_BayonetRifle",
        "BP_HK416_Bayonet"
      ],
      "verbs": [
        "KNIFED",
        "SLICED",
        "DICED",
        "ICED",
        "CUT",
        "PAPER CUT",
        "RAZORED",
        "EDWARD SCISSOR HAND'D",
        "FRUIT NINJA'D",
        "TERMINATED",
        "DELETED",
        "ASSASSINATED"
      ]
    }
  ]
}
```

## Broadcasting Modes

### Immediate Broadcasting

When `use_interval` is `false`, messages are broadcast immediately when a matching kill is detected. This provides instant feedback but can cause message spam if multiple kills happen quickly.

### Interval-Based Broadcasting

When `use_interval` is `true`, messages are queued and broadcast at the specified interval (`interval_ms`). This prevents message spam by limiting how often broadcasts occur. The most recent message in the queue is broadcast first (LIFO - Last In, First Out).
