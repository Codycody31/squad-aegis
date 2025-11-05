---
title: "Basic Concepts"
---

## What are Workflows?

Workflows in Squad Aegis provide a powerful automation system that allows you to create custom responses to in-game events. When specific events occur (like a player sending a chat message or a player joining the server), workflows can automatically execute a series of actions such as sending RCON commands, logging messages, or triggering notifications.

## Basic Concepts

### Workflow Components

A workflow consists of:

- **Triggers**: Define what events will start the workflow
- **Conditions**: Optional filters to determine if the workflow should run
- **Steps**: The actions to execute when triggered
- **Variables**: Data storage for workflow execution
- **Error Handling**: How to handle failures

### Execution Flow

1. An in-game event occurs (e.g., player sends chat message)
2. Squad Aegis checks if any workflows have matching triggers
3. If conditions are met, the workflow executes its steps in order
4. Results are logged and can be used by subsequent steps

## Event Types and Available Fields

### Chat Message Events (`RCON_CHAT_MESSAGE`)

Triggered when a player sends a chat message.

**Available Fields:**

- `chat_type` - The type of chat (see Chat Types below)
- `eos_id` - Player's Epic Online Services ID
- `steam_id` - Player's Steam ID
- `player_name` - Player's display name
- `message` - The actual message content

**Chat Types:**

- `ChatAll` - Public chat visible to all players
- `ChatTeam` - Team-only chat
- `ChatSquad` - Squad-only chat
- `ChatAdmin` - Admin chat channel

### Player Events

#### Player Connected (`LOG_PLAYER_CONNECTED`)

**Available Fields:**

- `time` - Timestamp of the event
- `chain_id` - Unique event chain identifier
- `player_controller` - Player controller identifier
- `ip_address` - Player's IP address
- `steam_id` - Player's Steam ID
- `eos_id` - Player's Epic Online Services ID

#### Player Disconnected (`LOG_PLAYER_DISCONNECTED`)

**Available Fields:**

- `time` - Timestamp of the event
- `chain_id` - Unique event chain identifier
- `ip` - Player's IP address
- `player_controller` - Player controller identifier
- `player_suffix` - Player name suffix
- `team_id` - Team the player was on
- `steam_id` - Player's Steam ID
- `eos_id` - Player's Epic Online Services ID

#### Player Died (`LOG_PLAYER_DIED`)

**Available Fields:**

- `time` - Timestamp of the event
- `wound_time` - When the player was wounded (if applicable)
- `chain_id` - Unique event chain identifier
- `victim_name` - Name of the player who died
- `damage` - Damage amount that caused death
- `attacker_player_controller` - Attacker's controller ID
- `weapon` - Weapon used for the kill
- `attacker_eos` - Attacker's Epic Online Services ID
- `attacker_steam` - Attacker's Steam ID
- `victim` - Detailed victim information object
- `attacker` - Detailed attacker information object
- `teamkill` - Boolean indicating if this was a teamkill

#### Player Wounded (`LOG_PLAYER_WOUNDED`)

**Available Fields:**

- `time` - Timestamp of the event
- `chain_id` - Unique event chain identifier
- `victim_name` - Name of the wounded player
- `damage` - Damage amount
- `attacker_player_controller` - Attacker's controller ID
- `weapon` - Weapon used
- `attacker_eos` - Attacker's Epic Online Services ID
- `attacker_steam` - Attacker's Steam ID
- `victim` - Detailed victim information object
- `attacker` - Detailed attacker information object
- `teamkill` - Boolean indicating if this was a teamkill

#### Player Revived (`LOG_PLAYER_REVIVED`)

**Available Fields:**

- `time` - Timestamp of the event
- `chain_id` - Unique event chain identifier
- `reviver_name` - Name of the player who performed the revive
- `victim_name` - Name of the player who was revived
- `reviver_eos` - Reviver's Epic Online Services ID
- `reviver_steam` - Reviver's Steam ID
- `victim_eos` - Victim's Epic Online Services ID
- `victim_steam` - Victim's Steam ID

### Admin Events

#### Player Warned (`RCON_PLAYER_WARNED`)

**Available Fields:**

- `player_name` - Name of the warned player
- `message` - Warning message content

#### Player Kicked (`RCON_PLAYER_KICKED`)

**Available Fields:**

- `player_id` - Internal player ID
- `eos_id` - Player's Epic Online Services ID
- `steam_id` - Player's Steam ID
- `player_name` - Player's display name

#### Player Banned (`RCON_PLAYER_BANNED`)

**Available Fields:**

- `player_id` - Internal player ID
- `steam_id` - Player's Steam ID
- `player_name` - Player's display name
- `interval` - Ban duration in minutes

#### Admin Broadcast (`LOG_ADMIN_BROADCAST`)

**Available Fields:**

- `time` - Timestamp of the broadcast
- `chain_id` - Unique event chain identifier
- `message` - Broadcast message content
- `from` - Admin who sent the broadcast

### Squad Events

#### Squad Created (`RCON_SQUAD_CREATED`)

**Available Fields:**

- `player_name` - Name of the squad leader
- `eos_id` - Squad leader's Epic Online Services ID
- `steam_id` - Squad leader's Steam ID
- `squad_id` - Unique squad identifier
- `squad_name` - Name of the squad
- `team_name` - Team the squad belongs to

### Server Events

#### Server Info (`RCON_SERVER_INFO`)

**Available Fields:**

- `player_count` - Current number of players
- `public_queue` - Number of players in public queue
- `reserved_queue` - Number of players in reserved queue
- `total_queue_count` - Total players in queue

### Game Events

#### Game Event Unified (`LOG_GAME_EVENT_UNIFIED`)

**Available Fields:**

- `time` - Timestamp of the event
- `chain_id` - Unique event chain identifier
- `event_type` - Type of game event (see Game Event Types below)
- `winner` - Winning team/faction
- `layer` - Current map layer
- `team` - Team identifier
- `subfaction` - Subfaction name
- `faction` - Faction name
- `action` - Action performed ("won" or "lost")
- `tickets` - Ticket count
- `level` - Map/level name
- `dlc` - DLC information
- `map_classname` - Internal map class name
- `layer_classname` - Internal layer class name
- `from_state` - Previous game state
- `to_state` - New game state
- `winner_data` - Additional winner information (JSON)
- `loser_data` - Additional loser information (JSON)
- `metadata` - Additional event metadata (JSON)
- `raw_log` - Original log line

**Game Event Types:**

- `ROUND_ENDED` - A round has concluded
- `NEW_GAME` - A new game/round is starting
- `MATCH_WINNER` - Match winner declared
- `TICKET_UPDATE` - Ticket count updated
