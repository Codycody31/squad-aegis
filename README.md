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

## **Roadmap**

- [ ] Log exporter - export squad logs via GRPC or some other protocol and allow events to be consumed by the dashboard
- [ ] Admin chat - just on the panel, not in discord or in-game
- [ ] Admin notes - add notes to players, bans, etc.
- [ ] Feeds - add feeds for live server events (e.g. kills, deaths, chat, etc.)
  - [ ] Live chat
  - [ ] Live team kills
  - [ ] Live connect/disconnect
  - [ ] Live admin requests
- [ ] Extensions - bringing some plugins from SquadJS to the dashboard or just any cool extensions
  - [ ] RCON command scheduling - ie: AdminChangeLayer every day at 10:00 PM
  - [ ] Automatic Kick of Unassigned Players
  - [ ] Automatic Team Kill Warning
  - [ ] Etc, from SquadJS
- [ ] Player profiles - similar to battlemetrics showing play history, statistics, chat history, warnings history

## **Some important notes on the project**

- Inspired by [milutinke/sqcp](https://github.com/milutinke/sqcp) but with a focus on being a more complete solution for server administration.
- The project is still in early development and the UI is subject to change.
- A large amount of the code was written by AI, so while it may work, I do have plans to refactor it to be more readable and maintainable rather than just functional.

## **Screenshots**

![Dashboard](.github/images/dashboard.png)

## **License**

This project is licensed under the [Apache 2.0 License](LICENSE).
