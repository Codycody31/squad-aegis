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
curl -O https://raw.githubusercontent.com/Codycody31/squad-aegis/main/docker-compose.yml
```

2. Create a `.env` file with your configuration:

```bash
# Core Settings
INITIAL_ADMIN_USERNAME=your_admin_username
INITIAL_ADMIN_PASSWORD=your_secure_password
APP_URL=http://your_domain_or_ip:3113

# Database Settings (optional - default values shown)
DB_HOST=database
DB_PORT=5432
DB_NAME=squad-aegis
DB_USER=squad-aegis
DB_PASS=squad-aegis
```

3. Start the services:

```bash
docker compose up -d
```

4. Access the dashboard at `http://localhost:3113`

### Log Watcher Setup

The Log Watcher component monitors Squad server logs in real-time. To set it up:

1. Create a new docker-compose file for the log watcher:

```bash
curl -O https://raw.githubusercontent.com/Codycody31/squad-aegis/main/docker-compose.logwatcher.yml
```

2. Configure the log watcher environment:

```yaml
environment:
  LOGWATCHER_PORT: 31135
  LOGWATCHER_AUTH_TOKEN: "your_secure_token"  # Must match dashboard configuration
  LOGWATCHER_LOG_FILE: "/path/to/SquadGame.log"
```

3. Start the log watcher:

```bash
docker compose -f docker-compose.logwatcher.yml up -d
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
