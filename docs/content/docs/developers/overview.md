---
title: Overview
---

Squad Aegis is a comprehensive control panel for managing Squad game servers. This section provides technical documentation for developers who want to understand the codebase, contribute to the project, or extend its functionality.

## Technology Stack

Squad Aegis is built using the following technologies:

- **Backend**: Go (v1.21+)
  - Gin web framework for API endpoints
  - GORM for database access
  - Squad-RCON for game server communication
  - Zerolog for structured logging

- **Frontend**: Vue.js/Nuxt.js
  - Tailwind CSS for styling
  - Pinia for state management
  - TypeScript for type safety

- **Data Storage**:
  - PostgreSQL for primary relational data storage
  - ClickHouse for analytics and event log storage

- **Containerization**:
  - Docker for deployment and development environments

## Project Structure

The project follows a standard Go project layout with several key directories:

- `/cmd` - Main applications for the project
  - `/server` - The main server application

- `/internal` - Private application and library code
  - `/clickhouse` - ClickHouse client and integration
  - `/commands` - Command-line commands
  - `/connectors` - Integration with external services
  - `/core` - Core business logic
  - `/db` - Database access and migrations
  - `/event_manager` - Central event distribution system
  - `/logwatcher_manager` - Game log parsing and processing
  - `/models` - Data models
  - `/plugin_manager` - Plugin system implementation
  - `/plugin_registry` - Plugin registration
  - `/plugins` - Individual plugin implementations
  - `/rcon_manager` - RCON connection management
  - `/server` - HTTP server and API endpoints
  - `/shared` - Shared utilities and helpers
  - `/squad-rcon` - RCON protocol implementation

- `/web` - Web frontend code
  - Vue.js/Nuxt.js application

- `/docs` - Documentation

## Development Workflow

To get started with development:

1. **Clone the repository**:

   ```bash
   git clone https://github.com/Codycody31/squad-aegis.git
   cd squad-aegis
   ```

2. **Set up the development environment**:

   ```bash
   docker compose -f docker-compose.dev.yml up -d
   ```

3. **Install dependencies**:

   ```bash
   go mod tidy
   go mod vendor
   ```

4. **Build the application**:

   ```bash
   make build-server # Builds just the server
   make build-web    # Builds just the web UI
   make build        # Builds everything
   ```

5. **Run tests**:

   ```bash
   make test
   ```

## Core Concepts

### Event System

The event system is central to Squad Aegis. It uses a publish-subscribe pattern to distribute events from various sources (RCON, log files, etc.) to interested components like plugins.

### Plugin Architecture

The plugin system allows for extending functionality without modifying the core codebase. Plugins can subscribe to events, execute commands, and store data.

### RCON Integration

Squad Aegis communicates with Squad servers using the RCON protocol, allowing for command execution and event subscription.

### Log Parsing

The log watcher parses Squad server log files in real-time to extract events and information.

See the [Architecture](architecture) page for more detailed information about how these components interact.
