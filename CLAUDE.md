# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Squad Aegis is a comprehensive control panel for Squad game server administration. It provides multi-server management, RCON interface, plugin architecture, role-based access, and audit logging.

## Tech Stack

- **Backend**: Go 1.24+ with Gin HTTP framework
- **Frontend**: Nuxt 3 (Vue 3) with TypeScript, Tailwind CSS, shadcn-vue components
- **Databases**: PostgreSQL (primary), ClickHouse (analytics/events), Valkey (caching)
- **Documentation**: Next.js with Fumadocs

## Development Commands

### Backend (Go)
```bash
make build           # Build web UI and server binary
make build-server    # Build server only (requires web UI built first)
make build-web       # Build web UI only
make test            # Run tests with coverage
make lint            # Run golangci-lint
make format          # Format code with gofumpt
make generate        # Run code generation
make vendor          # Update vendor directory
```

### Frontend (web/)
```bash
cd web && pnpm install    # Install dependencies
cd web && pnpm dev        # Development server (localhost:3000)
cd web && pnpm build      # Production build
cd web && pnpm generate   # Generate static site (used by make build-web)
```

### Documentation (docs/)
```bash
cd docs && pnpm install   # Install dependencies
cd docs && pnpm dev       # Development server
cd docs && pnpm build     # Production build
```

### Run with Docker
```bash
docker-compose up         # Start all services (postgres, clickhouse, valkey, server)
```

## Architecture

### Backend Structure (`internal/`)

- **cmd/server/main.go**: Application entry point; initializes all managers and starts HTTP server
- **server/**: HTTP handlers and API routes (Gin framework)
- **core/**: Business logic for users, sessions, servers, bans
- **models/**: Data models/structs
- **db/**: Database interface and PostgreSQL migrations (`db/migrations/`)
- **clickhouse/**: ClickHouse client, migrations, and event ingestion
- **valkey/**: Valkey (Redis-compatible) client for caching

### Plugin System

The plugin system allows extending server functionality:

- **plugin_manager/**: Plugin lifecycle management, interfaces, and APIs
  - `Plugin` interface: `GetDefinition()`, `Initialize()`, `Start()`, `Stop()`, `HandleEvent()`
  - `PluginAPIs`: Provides `ServerAPI`, `RconAPI`, `DatabaseAPI`, `AdminAPI`, `EventAPI`, `ConnectorAPI`, `LogAPI`
- **plugins/**: Individual plugin implementations (one directory per plugin)
- **connectors/**: Global service connectors (e.g., Discord integration in `connectors/discord/`)
- **plugin_registry/**: Registers all available plugins and connectors

Each plugin defines via `Define()` function returning `PluginDefinition` with ID, config schema, and event subscriptions.

### Event System

- **event_manager/**: Centralized pub/sub event system
  - Event types: RCON events (chat, kicks, bans), Log events (kills, connections), Player tracker events
  - Plugins subscribe to specific event types via `EventFilter`

### Managers

- **rcon_manager/**: Manages RCON connections to Squad servers
- **logwatcher_manager/**: Watches server log files for events
- **player_tracker_manager/**: Tracks player state across servers
- **workflow_manager/**: Executes automated workflows

### Frontend Structure (`web/`)

- **pages/**: Nuxt page routes
- **components/**: Vue components (using shadcn-vue)
- **stores/**: Pinia state stores
- **composables/**: Vue composables
- **web.go** / **web_embed.go**: Embeds built frontend into Go binary

## Configuration

Configuration loaded from environment variables or `/etc/squad-aegis/config.yaml`:

Key env vars (see `.env.example`):
- `APP_PORT`: HTTP server port (default 3113)
- `DB_*`: PostgreSQL connection
- `CLICK_HOUSE_*`: ClickHouse connection
- `VALKEY_*`: Valkey connection
- `INITIAL_ADMIN_*`: Initial admin credentials

## Creating a New Plugin

1. Create directory: `internal/plugins/my_plugin/`
2. Create `my_plugin.go` implementing `plugin_manager.Plugin` interface
3. Define plugin metadata via `Define()` function returning `PluginDefinition`
4. Register in `internal/plugin_registry/` by adding to registration list
5. Handle events via `HandleEvent()` method based on subscribed event types
