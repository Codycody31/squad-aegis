<div align="center">

<img src=".github/images/aegis_squad.png" alt="Logo" width="500"/>

# Squad Aegis

A comprehensive control panel for Squad game server administration

</div>

## Overview

**Squad Aegis** is an all-in-one control panel for managing multiple Squad game servers. Built with a Go backend, Nuxt 3 frontend, and PostgreSQL database, it provides:

- Centralized server management
- Role-based access control
- RCON command interface
- Comprehensive audit logging
- Extensible architecture through connectors and extensions

## Features

### Core Features

- **Multi-Server Management**: Control multiple Squad servers from a single dashboard
- **Role-Based Access**: Granular permission controls for different user roles
- **RCON Interface**: Execute server commands through an intuitive web interface
- **Audit System**: Track all administrative actions
- **Real-time Monitoring**: Monitor server status and player activity

### Extensibility

- **Connectors**: Integrate with external services (e.g., Discord)
- **Extensions**: Add custom functionality without modifying core code

## Installation Guide

### Prerequisites

- Docker Engine 20.10.0 or newer
- Docker Compose V2
- 2GB RAM minimum
- 10GB available storage

### Quick Start (Docker)

1. Create a new directory and download the docker-compose file:

```bash
mkdir squad-aegis && cd squad-aegis
LATEST_TAG=$(curl -s https://api.github.com/repos/Codycody31/squad-aegis/releases/latest | jq -r '.tag_name')
curl -O https://raw.githubusercontent.com/Codycody31/squad-aegis/$LATEST_TAG/docker-compose.yml
```

2. Configure the dashboard environment:

```yaml
environment:
  - INITIAL_ADMIN_USERNAME=your_admin_username
  - INITIAL_ADMIN_PASSWORD=your_secure_password
  - APP_URL=http://your_domain_or_ip:3113
  - DB_HOST=database
  - DB_PORT=5432
  - DB_NAME=squad-aegis
  - DB_USER=squad-aegis
  - DB_PASS=squad-aegis
```

3. Start the services:

```bash
docker compose up -d
```

4. Access the dashboard at `http://localhost:3113`

### Log Watcher Setup

The Log Watcher component monitors Squad server logs in real-time. To set it up:

1. Create a new directory and download the log watcher docker-compose file:

```bash
mkdir squad-aegis-log-watcher && cd squad-aegis-log-watcher
LATEST_TAG=$(curl -s https://api.github.com/repos/Codycody31/squad-aegis/releases/latest | jq -r '.tag_name')
curl -O https://raw.githubusercontent.com/Codycody31/squad-aegis/$LATEST_TAG/docker-compose.logwatcher.yml
```

2. Configure the log watcher environment:

```yaml
environment:
  - LOGWATCHER_PORT=31135
  - LOGWATCHER_AUTH_TOKEN=your_secure_token  # Must match dashboard configuration
  - LOGWATCHER_SOURCE_TYPE=local             # Options: local, sftp, ftp
  - LOGWATCHER_LOG_FILE=/path/to/SquadGame.log
  - LOGWATCHER_READ_FROM_START=false         # Optional: Set to true to read entire log file history
```

3. Start the log watcher:

```bash
docker compose -f docker-compose.logwatcher.yml up -d
```

#### Log Watcher Behavior

By default, the Log Watcher only broadcasts new log entries that appear after it starts (similar to `tail -f`). This prevents flooding clients with potentially large amounts of historical log data.

To read and process the entire log file from the beginning, set `LOGWATCHER_READ_FROM_START=true`.

### Analytics & Telemetry

Squad Aegis includes anonymous telemetry to help improve the application. By default, anonymous telemetry is enabled and collects:

- Basic system information (OS, CPU architecture, memory usage)
- Feature usage statistics
- Crash reports (if any occur)
- Performance metrics

#### Telemetry Configuration

You can configure telemetry settings through environment variables:

```yaml
environment:
  # Enable/disable telemetry completely
  - APP_TELEMETRY=true
  
  # Enable non-anonymous telemetry (includes server ID and hostname)
  - APP_NON_ANONYMOUS_TELEMETRY=false
```

#### What Data is Collected?

When telemetry is enabled, the following data is collected:

1. **Anonymous Mode** (default):
   - OS and version
   - CPU architecture
   - Memory usage
   - Disk usage
   - Application version
   - Feature usage statistics
   - Crash reports (if any)

2. **Non-Anonymous Mode** (optional):
   - All anonymous data
   - Server hostname
   - Server ID's

#### Disabling Telemetry

To completely disable telemetry, set:

```yaml
environment:
  - APP_TELEMETRY=false
```

## Configuration

### Environment Variables

#### Core Application

| Variable | Description | Default |
|----------|-------------|---------|
| APP_PORT | Dashboard web port | 3113 |
| APP_URL | Public URL | <http://localhost:3113> |
| INITIAL_ADMIN_USERNAME | First admin user | admin |
| INITIAL_ADMIN_PASSWORD | First admin password | admin |

#### Database

| Variable | Description | Default |
|----------|-------------|---------|
| DB_HOST | PostgreSQL host | database |
| DB_PORT | PostgreSQL port | 5432 |
| DB_NAME | Database name | squad-aegis |
| DB_USER | Database user | squad-aegis |
| DB_PASS | Database password | squad-aegis |

#### Log Watcher

| Variable | Description | Default |
|----------|-------------|---------|
| LOGWATCHER_PORT | gRPC server port | 31135 |
| LOGWATCHER_AUTH_TOKEN | Authentication token | (required) |
| LOGWATCHER_SOURCE_TYPE | Source type (local, sftp, ftp) | local |
| LOGWATCHER_LOG_FILE | Path to local log file | (required for local) |
| LOGWATCHER_HOST | Remote host for SFTP/FTP | (required for sftp/ftp) |
| LOGWATCHER_REMOTE_PORT | Remote port for SFTP/FTP | 22 (sftp), 21 (ftp) |
| LOGWATCHER_USERNAME | Username for SFTP/FTP | (required for sftp/ftp) |
| LOGWATCHER_PASSWORD | Password for SFTP/FTP | (required for ftp) |
| LOGWATCHER_KEY_PATH | Path to SSH key for SFTP | (optional for sftp) |
| LOGWATCHER_REMOTE_PATH | Path to remote log file | (required for sftp/ftp) |
| LOGWATCHER_POLL_FREQUENCY | Poll frequency for remote files | 5s |
| LOGWATCHER_READ_FROM_START | Read entire log from beginning | false |

## Development

For development environments, use the dev compose file:

```bash
docker compose -f docker-compose.dev.yml up -d
```

## Screenshots

![Dashboard](.github/images/dashboard.png)

## Contributing

Contributions are welcome! Please read our [Contributing Guidelines](CONTRIBUTING.md) before submitting pull requests.

## Support

- [Issue Tracker](https://github.com/Codycody31/squad-aegis/issues)
- [Discord Server](https://discord.gg/your-invite)

## License

This project is licensed under the [Apache 2.0 License](LICENSE).

## Acknowledgments

- Inspired by [milutinke/sqcp](https://github.com/milutinke/sqcp)
- Special thanks to the Squad gaming community
