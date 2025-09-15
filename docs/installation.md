---
title: Installation Guide
icon: lucide:download
---


# Installation Guide

This guide will walk you through installing and configuring Squad Aegis, the comprehensive control panel for Squad game server administration.

## Prerequisites

Before installing Squad Aegis, ensure you have the following:

- **Docker Engine** 20.10.0 or newer
- **Docker Compose V2**
- Minimum **1GB RAM**
- At least **10GB available storage**
- Basic knowledge of Docker and command-line operations

## Installation Steps

### 1. Clone the Repository

```bash
git clone https://github.com/Codycody31/squad-aegis.git
cd squad-aegis
```

### 2. Configure Environment Variables

Create a `.env` file in the project root directory with your configuration settings. Below is a template based on the configuration structure:

```env
# Application Configuration
APP_ISDEVELOPMENT=true
APP_WEBUIPROXY=
APP_PORT=3113
APP_URL=http://localhost:3113
APP_INCONTAINER=false

# Initial Admin User
INITIAL_ADMIN_USERNAME=admin
INITIAL_ADMIN_PASSWORD=your_secure_password

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_NAME=squad-aegis
DB_USER=squad-aegis
DB_PASS=your_db_password
DB_MIGRATE_VERBOSE=false

# ClickHouse Configuration 
CLICKHOUSE_HOST=localhost
CLICKHOUSE_PORT=9000
CLICKHOUSE_DATABASE=default
CLICKHOUSE_USERNAME=squad_aegis
CLICKHOUSE_PASSWORD=your_clickhouse_password
CLICKHOUSE_DEBUG=false

VALKEY_HOST=localhost
VALKEY_PORT=6379

# Logging Configuration
LOG_LEVEL=info
LOG_SHOWGIN=false
LOG_FILE=

# Debug Configuration
DEBUG_PRETTY=true
DEBUG_NOCOLOR=false
```

**Important Security Note:** Change the default `INITIAL_ADMIN_PASSWORD` and database credentials before deployment.

### 3. Build the Application

Before running with Docker, you need to build the application. The project uses a Makefile for building:

```bash
# Install dependencies
go mod tidy
go mod vendor

# Build the server (this also builds the web UI)
make build-server

# Or build everything
make build
```

This will create binaries in the `dist/` directory.

### 4. Set Up Docker Compose

The project provides Docker Compose files for deployment. For development, you can use the provided `docker-compose.dev.yml` which includes database and ClickHouse services:

```bash
docker compose -f docker-compose.dev.yml up -d
```

For production deployment, you'll need to build your own Docker image or modify the `docker-compose.yml` file to use a local build. The provided `docker-compose.yml` references a private registry image that may not be accessible.

To build and run with Docker:

```bash
# Build the Docker image
docker build -f docker/Dockerfile.multiarch.rootless -t squad-aegis:latest .

# Then modify docker-compose.yml to use your local image
# Change: image: registry.vmgware.dev/insidiousfiddler/squad-aegis:latest
# To: image: squad-aegis:latest

# Run the services
docker compose up -d
```

### 5. Database Migration

The application will automatically run database migrations on startup if configured properly. You can verify migration status by checking the logs:

```bash
docker compose logs -f squad-aegis
```

### 5. Access the Web Interface

Once the containers are running, access the Squad Aegis interface at:

```
http://localhost:3113
```

Log in with the credentials you configured in the `.env` file (default: `admin`/`admin`).

## Configuration Options

### Application Settings

| Variable | Description | Default |
|----------|-------------|---------|
| `APP_ISDEVELOPMENT` | Enable development mode | `true` |
| `APP_WEBUIPROXY` | Reverse proxy configuration | `""` |
| `APP_PORT` | Web interface port | `3113` |
| `APP_URL` | Base URL for the application | `http://localhost:3113` |
| `APP_INCONTAINER` | Indicates if running in container | `false` |

### Database Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_HOST` | Database server host | `localhost` |
| `DB_PORT` | Database server port | `5432` |
| `DB_NAME` | Database name | `squad-aegis` |
| `DB_USER` | Database username | `squad-aegis` |
| `DB_PASS` | Database password | `squad-aegis` |
| `DB_MIGRATE_VERBOSE` | Enable verbose migration logging | `false` |

### ClickHouse Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `CLICKHOUSE_ENABLED` | Enable ClickHouse integration | `false` |
| `CLICKHOUSE_HOST` | ClickHouse server host | `localhost` |
| `CLICKHOUSE_PORT` | ClickHouse server port | `9000` |
| `CLICKHOUSE_DATABASE` | ClickHouse database name | `default` |
| `CLICKHOUSE_USERNAME` | ClickHouse username | `squad_aegis` |
| `CLICKHOUSE_PASSWORD` | ClickHouse password | `squad_aegis` |
| `CLICKHOUSE_DEBUG` | Enable ClickHouse debug logging | `false` |

### Logging Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | `info` |
| `LOG_SHOWGIN` | Show Gin framework logs | `false` |
| `LOG_FILE` | Path to log file (empty for stdout) | `""` |

## Post-Installation Steps

1. **Change Default Credentials**: Immediately change the default admin password after first login
2. **Configure Servers**: Add your Squad servers through the web interface
3. **Set Up Users**: Create additional user accounts with appropriate permissions
4. **Configure Plugins**: Enable and configure any necessary plugins
5. **Set Up Backups**: Implement a backup strategy for your database

## Troubleshooting

### Common Issues

1. **Port Already in Use**
   - Change the `APP_PORT` in your `.env` file
   - Update the port mapping in `docker-compose.yml`

2. **Database Connection Errors**
   - Verify database service is running
   - Check connection parameters in `.env`
   - Ensure database user has proper permissions

3. **Permission Issues**
   - Verify Docker has proper permissions
   - Check volume mount permissions

### Viewing Logs

```bash
# View all service logs
docker compose logs

# View logs for a specific service
docker compose logs squad-aegis

# Follow logs in real-time
docker compose logs -f squad-aegis
```

## Support

For additional help:

- Check the [Issue Tracker](https://github.com/Codycody31/squad-aegis/issues)
- Review the project documentation
- Contact the development team through GitHub
