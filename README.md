<div align="center">

<img src=".github/images/aegis_squad.png" alt="Logo" width="500"/>

#### Squad Aegis

</div>

## **About**

**Squad Aegis** is a comprehensive control panel solution for administrating multiple **Squad** game servers in one place. Built on a **Go** backend, **Nuxt 3** frontend, and a **PostgreSQL** database, it streamlines player management, role-based permissions, and RCON controls into a single, easy-to-use web interface.

## **Features**

- **Multiple Servers**: Support for managing multiple Squad servers from a single interface.
- **Role-Based Permissions**: Set permissions for each role to control access to various features.
- **RCON Controls**: Execute commands on your servers with the click of a button.
- **Audit Logs**: Track all actions taken by dashboard users.
- **Connectors**: Extend the dashboard's functionality by adding custom connectors.
- **Extensions**: Extend the dashboard's functionality by adding extensions that may work with connectors.

## **Connectors & Extensions**

The dashboard is designed to be extensible through connectors and extensions.

### **Connectors**

Connectors are used to extend the dashboard's connectivity to external services for example `Discord`.

### **Extensions**

Extensions are used to extend the dashboard's functionality for example `DiscordAdminCamLogs`, however extensions do not affect the UI of the dashboard and are primarily used to modify the backend logic of the dashboard, for example supporting custom commands in the chat.

## **Getting Started**

### **Prerequisites**

- **Docker**: [Install Docker](https://docs.docker.com/get-docker/)
- **Docker Compose**: [Install Docker Compose](https://docs.docker.com/compose/install/)

### **Server/Squad Aegis Installation**

It should be noted that you can skip steps 1 and 2 if you have already cloned the repository or if you just have the `docker-compose.yml` file, as the `docker-compose.yml` file will automatically pull the latest image from my self-hosted registry.

1. Clone the repository:

```bash
git clone https://github.com/Codycody31/squad-aegis.git -o codycody31-squad-aegis
```

2. Change into the project directory:

```bash
cd codycody31-squad-aegis
```

3. Edit the `docker-compose.yml` file and update the following environment variables:

- `INITIAL_ADMIN_USERNAME`: The initial admin username.
- `INITIAL_ADMIN_PASSWORD`: The initial admin password.

If you have a external PostgreSQL database, you can update the following environment variables:

- `DB_HOST`: The PostgreSQL database host.
- `DB_PORT`: The PostgreSQL database port.
- `DB_USER`: The PostgreSQL database user.
- `DB_PASS`: The PostgreSQL database password.
- `DB_NAME`: The PostgreSQL database name.

4. Start the project:

```bash
docker-compose up
```

5. Access the dashboard at `http://localhost:3113`.

### **Log Watcher Installation**

No instructions yet, a docker image should exist for `registry.vmgware.dev/insidiousfiddler/squad-aegis-logwatcher:<latest|next>`, you can use the `docker-compose.logwatcher.yml` file to run the log watcher.

## **Some important notes on the project**

- Inspired by [milutinke/sqcp](https://github.com/milutinke/sqcp) but with a focus on being a more complete solution for server administration.
- The project is still in early development and the UI is subject to change.
- A large amount of the code was written by AI, so while it may work, I do have plans to refactor it to be more readable and maintainable rather than just functional.

## **Screenshots**

![Dashboard](.github/images/dashboard.png)

## **License**

This project is licensed under the [Apache 2.0 License](LICENSE).
