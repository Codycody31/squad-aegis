<div align="center">

<img src=".github/images/aegis_squad.png" alt="Logo" width="500"/>

# Squad Aegis

A comprehensive control panel for Squad game server administration

</div>

## Notice

The project is currently in the early stages of development and is not ready for production use. Expect breaking changes and bugs as I work on making this stable and not a bunch of ai written spaghetti code from the PoC.

## Overview

**Squad Aegis** is an all-in-one control panel designed to manage multiple Squad game servers efficiently. Whether you're running a single server or a complex cluster, Squad Aegis provides a centralized interface to keep everything under control.

## Acknowledgments

- [milutinke/sqcp](https://github.com/milutinke/sqcp): This project draws inspiration sqcp and we extend gratitude to the Squad gaming community for their support and feedback.
  - License: [MIT License](https://github.com/milutinke/sqcp/blob/master/LICENSE)
- [SquadGO/squad-rcon-go](https://github.com/SquadGO/squad-rcon-go): For the RCON library used in this project with some modifications of our own to suit our needs.
  - License: [MIT License](https://github.com/SquadGO/squad-rcon-go/tree/v2.0.5/LICENSE)
- [Team-Silver-Sphere/SquadJS](https://github.com/Team-Silver-Sphere/SquadJS/tree/v4.2.0): Big thanks to Team Silver Sphere for their SquadJS project which provided a solid foundation for our log parsing logic along plenty of designs for plugins! We do our best to be feature compat with SquadJS to make a easy and simple switch.
  - License: [BSL-1.0 License](https://github.com/Team-Silver-Sphere/SquadJS/tree/v4.2.0/LICENSE)
- [mikebjoyce/squadjs-squadlead-whitelister](https://github.com/mikebjoyce/squadjs-squadlead-whitelister): For the Squad Lead Whitelister plugin which we used as a base for our Server Seeder Whitelist plugin.

## Features

### Core Features

- **Multi-Server Management**: Central hub allowing supervision and control of multiple servers from a single interface.
- **Role-Based Access**: Define specific permissions for users to ensure only authorized actions are executed.
- **RCON Interface**: Engage with server commands using a user-friendly web-based interface.
- **Audit System**: Detailed logging of administrative activities for transparency and security.
- **Plugin Architecture**: Extend functionality with a modular plugin system, allowing for custom features and integrations.

## Installation Guide

To set up **Squad Aegis**, follow these steps:

### Prerequisites

Ensure you have the following installed on your system:

- **Docker Engine** 20.10.0 or newer: Container platform to deploy the application.
- **Docker Compose V2**: Tool for defining and running multi-container Docker applications.
- **Go 1.21+**: Required for building the application.
- **Node.js 18+**: Required for building the web interface.
- **Git**: For cloning the repository.
- Minimum **1GB RAM**: For efficient performance.
- At least **10GB available storage**: To store data and logs.

### Quick Start

1. **Clone the Repository**

   ```bash
   git clone https://github.com/Codycody31/squad-aegis.git
   cd squad-aegis
   ```

2. **Install Dependencies**

   ```bash
   go mod tidy
   go mod vendor
   ```

3. **Build the Application**

   ```bash
   make build-server
   ```

4. **Configure Environment**
   Create a `.env` file with your configuration (see [Installation Guide](docs/installation.md) for details).

5. **Start Services**

   ```bash
   docker compose -f docker-compose.dev.yml up -d
   ```

6. **Access the Web Interface**
   Open <http://localhost:3113> in your browser.

For detailed installation instructions, see the [Installation Guide](docs/installation.md).

## Development

For development, you can use the provided development compose file to set up the environment:

```bash
docker compose -f docker-compose.dev.yml up -d
```

The development environment includes:

- PostgreSQL database
- ClickHouse for analytics (optional)
- ClickHouse UI for database management

To build the application during development:

```bash
# Build the server
make build-server

# Build everything including log watcher
make build

# Build only the web UI
make build-web
```

## Screenshots

![Dashboard](.github/images/dashboard.png)

## Contributing

We welcome contributions!

## Support

For help and support, you can refer to the resources below:

- **Issue Tracker**: [Submit bug reports and feature requests](https://github.com/Codycody31/squad-aegis/issues)

## License

This project is available under the [Apache 2.0 License](LICENSE).

Ensure to check directories for any additional licenses or third-party components.
