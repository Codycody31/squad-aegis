---
title: Installation Guide
icon: lucide:download
---

# Installation Guide

This guide will walk you through installing and configuring **Squad Aegis**, the comprehensive control panel for Squad game server administration.
It includes both a quick "run with Docker Compose" path and a more detailed manual build process.

---

## Prerequisites

Before installing Squad Aegis, ensure you have:

- **Docker Engine** 20.10.0 or newer
- **Docker Compose V2**
- Minimum **1GB RAM**
- At least **10GB available storage**
- Basic knowledge of Docker and command-line operations

---

## 1. Clone the Repository

```bash
git clone https://github.com/Codycody31/squad-aegis.git
cd squad-aegis
````

---

## 2. Configure Environment Variables

Create a `.env` file in the project root (same directory as `docker-compose.yml`).
Here is a template you can customize:

```env
# Application Configuration
APP_ISDEVELOPMENT=false
APP_WEBUIPROXY=
APP_PORT=3113
APP_URL=http://localhost:3113
APP_INCONTAINER=false

# Initial Admin User
INITIAL_ADMIN_USERNAME=admin
INITIAL_ADMIN_PASSWORD=your_secure_password

# Database Configuration
DB_HOST=database
DB_PORT=5432
DB_NAME=squad-aegis
DB_USER=squad-aegis
DB_PASS=squad-aegis
DB_MIGRATE_VERBOSE=false

# ClickHouse Configuration
CLICKHOUSE_HOST=clickhouse
CLICKHOUSE_PORT=9000
CLICKHOUSE_DATABASE=default
CLICKHOUSE_USERNAME=squad_aegis
CLICKHOUSE_PASSWORD=squad_aegis
CLICKHOUSE_DEBUG=false

# Valkey Configuration
VALKEY_HOST=valkey
VALKEY_PORT=6379

# Logging Configuration
LOG_LEVEL=info
LOG_SHOWGIN=false
LOG_FILE=

# Debug Configuration
DEBUG_PRETTY=true
DEBUG_NOCOLOR=false
```

⚠️ **Important Security Note:** Always change the default `INITIAL_ADMIN_PASSWORD` and database credentials before deployment.

---

## 3. Running with Docker Compose

You have two options depending on your needs:

### Option A: Quick Start (non-dev stack)

Run everything with the production-style `docker-compose.yml`:

```bash
docker compose up -d
```

This will start the following containers:

* **PostgreSQL 14** – main application database
* **ClickHouse 25.3** – analytics database
* **Valkey 8.1.3** – caching / key-value store
* **Squad Aegis Server** – main application

### Option B: Development stack

If you want to build locally and run alongside databases:

```bash
docker compose -f docker-compose.dev.yml up -d
```

Just make sure to update the `.env` file and swap out the database connection settings.

---

## 4. (Optional) Build the Application Yourself

If you prefer to build the server and web UI locally before containerizing:

```bash
# Install dependencies
go mod tidy
go mod vendor

# Build the server (includes web UI)
make build-server

# Or build everything
make build
```

This will create binaries in the `dist/` directory.

You can also build your own Docker image:

```bash
docker build -f docker/Dockerfile.multiarch.rootless -t squad-aegis:latest .
```

Then edit `docker-compose.yml` to use:

```yaml
image: squad-aegis:latest
```

---

## 5. Database Migration

The application automatically runs database migrations on startup.
Check migration status:

```bash
docker compose logs -f squad-aegis
```

---

## 6. Verify and Access the Application

Check logs:

```bash
docker compose logs -f
```

Check container health:

```bash
docker ps
```

Then open your browser:

👉 [http://localhost:3113](http://localhost:3113)

Log in with:

* **Username:** `admin`
* **Password:** the one you set in `.env`

---

## 7. Stopping & Restarting

To stop the stack:

```bash
docker compose down
```

To restart:

```bash
docker compose up -d
```

---

## 8. Data Persistence

The following named volumes persist data between container restarts:

* `database` – PostgreSQL data
* `clickhouse` – ClickHouse data
* `valkey` – Valkey data
* `data` – Squad Aegis config

---

## 9. Post-Installation Steps

1. **Change Default Credentials** immediately after first login
2. **Configure Servers**: Add your Squad servers in the web UI
3. **Set Up Users** with appropriate permissions
4. **Configure Plugins** as needed

---

## Troubleshooting

### Common Issues

1. **Port Already in Use**

   * Change `APP_PORT` in `.env`
   * Update the port mapping in `docker-compose.yml`

2. **Database Connection Errors**

   * Verify the database container is running
   * Check connection parameters in `.env`
   * Ensure user permissions are correct

3. **Permission Issues**

   * Verify Docker volume permissions

### Viewing Logs

```bash
# View all service logs
docker compose logs

# View logs for a specific service
docker compose logs squad-aegis

# Follow logs in real-time
docker compose logs -f squad-aegis
```

---

## Support

* [Issue Tracker](https://github.com/Codycody31/squad-aegis/issues)
* Project documentation
* Contact the dev team via GitHub
