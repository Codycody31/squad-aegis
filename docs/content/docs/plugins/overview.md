---
title: Overview
---

Squad Aegis features a comprehensive plugin system that allows you to extend the functionality of your server administration panel. Plugins can be enabled, disabled, and configured through the web interface.

# Available Plugins

## Core Plugins

- **[Auto Kick Unassigned](/docs/plugins/auto-kick-unassigned)** - Automatically kicks players who are not in a squad after a specified time
- **[Auto TK Warn](/docs/plugins/auto-tk-warn)** - Warns players for team kills
- **[Chat Commands](/docs/plugins/chat-commands)** - Allows configuration of custom chat commands that broadcast or warn players
- **[Command Scheduler](/docs/plugins/command-scheduler)** - Schedules commands to be executed at specific intervals or events
- **[Fog of War](/docs/plugins/fog-of-war)** - Manages fog of war settings
- **[Intervalled Broadcasts](/docs/plugins/intervalled-broadcasts)** - Sends periodic broadcast messages to players
- **[Seeding Mode](/docs/plugins/seeding-mode)** - Broadcasts seeding rules based on player count
- **[Server Seeder Whitelist](/docs/plugins/server-seeder-whitelist)** - Manages whitelist for server seeders
- **[Squad Leader Whitelist](/docs/plugins/squad-leader-whitelist)** - Manages whitelist for squad leaders
- **[Switch Teams](/docs/plugins/switch-teams)** - Allows players to request team switches
- **[Team Randomizer](/docs/plugins/team-randomizer)** - Randomizes team assignments

## Discord Integration Plugins

- **[Discord Admin Broadcast](/docs/plugins/discord-admin-broadcast)** - Broadcasts admin messages to Discord
- **[Discord Admin Camera Logs](/docs/plugins/discord-admin-cam-logs)** - Logs admin camera usage to Discord
- **[Discord Admin Requests](/docs/plugins/discord-admin-request)** - Handles admin requests via Discord
- **[Discord Chat](/docs/plugins/discord-chat)** - Logs in-game chat to Discord
- **[Discord FOB/HAB Explosion Damage](/docs/plugins/discord-fob-hab-explosion-damage)** - Logs FOB/HAB damage to Discord
- **[Discord Kill Feed](/docs/plugins/discord-kill-feed)** - Logs kill feed to Discord
- **[Discord Round Ended](/docs/plugins/discord-round-ended)** - Logs round end events to Discord
- **[Discord Round Winner](/docs/plugins/discord-round-winner)** - Logs round winner to Discord
- **[Discord Server Status](/docs/plugins/discord-server-status)** - Logs server status changes to Discord
- **[Discord Squad Created](/docs/plugins/discord-squad-created)** - Logs squad creation to Discord

## Information Plugins

- **[Community Ban List Info](/docs/plugins/cbl-info)** - Provides Community Ban List information

# Plugin Configuration

Each plugin can be configured through the web interface under the Plugins section. Configuration options include:

- Enable/disable plugins
- Set plugin-specific parameters
- Configure Discord channel IDs for Discord plugins
- Set timers, thresholds, and messages

# Required Connectors

Some plugins require additional connectors to be configured:

- **Discord Connector**: Required for all Discord-related plugins
  - Configure your Discord bot token
  - Set up Discord channels
  - Enable necessary permissions

# Plugin Development

Plugins are written in Go and follow a standardized interface. If you're interested in developing custom plugins, refer to the plugin development documentation in the source code.

# Compatibility

Squad Aegis plugins are designed to be compatible with SquadJS plugins where possible, making migration easier for existing SquadJS users.