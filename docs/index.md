---
title: Introduction to Squad Aegis
icon: lucide:shield-check
---

# Introduction to Squad Aegis

**Squad Aegis** is a comprehensive control panel designed for efficient administration of Squad game servers. It provides server administrators with a centralized interface for managing multiple Squad servers through an intuitive web interface.

## Key Features

- **Multi-Server Management**: Supervise and control multiple Squad servers from a unified dashboard  
- **Role-Based Access Control**: Granular permission system ensuring secure administrative operations  
- **Web-Based RCON Interface**: Execute server commands through an intuitive graphical interface  
- **Comprehensive Audit Logging**: Track all administrative actions for security and accountability  
- **Extensible Plugin Architecture**: Customize functionality through modular plugins  

## System Requirements

- Docker Engine 20.10.0 or newer
- Docker Compose V2
- Minimum 1GB RAM
- At least 10GB available storage

## Technical Foundation

Squad Aegis builds upon several open-source projects:

- `sqcp` for core architecture inspiration
- `squad-rcon-go` for RCON communication
- `SquadJS` for log parsing and plugin compatibility
- Specialized plugins like the Squad Lead Whitelister

## Getting Started

For installation instructions, refer to our [Installation Guide](/installation). The setup process involves:

1. Cloning the repository
2. Configuring environment variables
3. Building the application
4. Setting up Docker services
5. Accessing the web interface
