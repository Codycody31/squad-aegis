# Changelog

## [0.3.0](https://github.com/Codycody31/squad-aegis/releases/tag/0.3.0) - 2025-11-08

### ❤️ Thanks to all contributors! ❤️

@Codycody31, @Insidious Fiddler

### ✨ Features

- feat(players): add player profile data structures and API endpoint [[#96](https://github.com/Codycody31/squad-aegis/pull/96)]
- feat: implement conditional step management in workflow [[#95](https://github.com/Codycody31/squad-aegis/pull/95)]
- feat: implement conditional step management in workflow [[#94](https://github.com/Codycody31/squad-aegis/pull/94)]
- feat: add file upload feature for importing workflow JSON [[#93](https://github.com/Codycody31/squad-aegis/pull/93)]

### 📈 Enhancement

- refactor: update/improve player actions & rules UI [[#90](https://github.com/Codycody31/squad-aegis/pull/90)]
- refactor: improve text export formatting with indentation logic [[#87](https://github.com/Codycody31/squad-aegis/pull/87)]
- feat: enhance UI with player action menus on feeds [[#86](https://github.com/Codycody31/squad-aegis/pull/86)]
- refactor: Migrate from Apache License 2.0 -> Elastic License 2.0 [[#84](https://github.com/Codycody31/squad-aegis/pull/84)]
- feat: edit/delete plugin data [[#83](https://github.com/Codycody31/squad-aegis/pull/83)]
- refactor: attempt to improve locking logic in DisconnectFromServer method [[#79](https://github.com/Codycody31/squad-aegis/pull/79)]
- refactor: server feed now shows player disconnects in connection history [[#77](https://github.com/Codycody31/squad-aegis/pull/77)]
- refactor(ui): Remove Connected Players in favor of Teams & Squads [[#76](https://github.com/Codycody31/squad-aegis/pull/76)]

### 🐛 Bug Fixes

- fix: rules are unable to be deleted [[#97](https://github.com/Codycody31/squad-aegis/pull/97)]
- fix: change ban duration from minutes to days [[#92](https://github.com/Codycody31/squad-aegis/pull/92)]
- fix(docs): remove outdated Lua scripting examples and commands [[#91](https://github.com/Codycody31/squad-aegis/pull/91)]
- refactor: remove broadcast messages for whitelist achievements [[#82](https://github.com/Codycody31/squad-aegis/pull/82)]
- fix: remove unnecessary await for fetch functions in Vue + bump squad-rcon-go [[#81](https://github.com/Codycody31/squad-aegis/pull/81)]
- fix: log watcher restart endpoint missing [[#78](https://github.com/Codycody31/squad-aegis/pull/78)]

### 📚 Documentation

- docs: update title and content structure in docs [[#89](https://github.com/Codycody31/squad-aegis/pull/89)]
- docs: v2 via fumadocs [[#88](https://github.com/Codycody31/squad-aegis/pull/88)]

### Misc

- refactor: drop lua examples in favor of docs ([12ba679](https://github.com/Codycody31/squad-aegis/commit/12ba679c73c52de33495520d349230e2c9d0d7ec))
- ci: drop broken registry [[#85](https://github.com/Codycody31/squad-aegis/pull/85)]
- chore: remove health check logic and methods [[#80](https://github.com/Codycody31/squad-aegis/pull/80)]
- ci(release-config): add option to skip commits without PRs [[#75](https://github.com/Codycody31/squad-aegis/pull/75)]
- chore: update release-helper image version [[#73](https://github.com/Codycody31/squad-aegis/pull/73)]

## [0.2.0](https://github.com/Codycody31/squad-aegis/releases/tag/0.2.0) - 2025-09-17

### ❤️ Thanks to all contributors! ❤️

@Codycody31

### ✨ Features

- wip [[#70](https://github.com/Codycody31/squad-aegis/pull/70)]
- feat: integrate ClickHouse for event logging and analytics, add log c… [[#69](https://github.com/Codycody31/squad-aegis/pull/69)]
- Refactor: cut down project into the working components [[#67](https://github.com/Codycody31/squad-aegis/pull/67)]
- feat(logwatcher): refactor log processing and client broadcasting [[#45](https://github.com/Codycody31/squad-aegis/pull/45)]

### 📈 Enhancement

- refactor: update audit log ui structure and formatting along with small bug fixes [[#62](https://github.com/Codycody31/squad-aegis/pull/62)]
- refactor: switch to squad-rcon-go v2 for RCON handling [[#58](https://github.com/Codycody31/squad-aegis/pull/58)]
- refactor: initiate RCON connection to all servers on startup [[#55](https://github.com/Codycody31/squad-aegis/pull/55)]
- enhancement(metrics): add analytics/telemetry support with metrics collection [[#52](https://github.com/Codycody31/squad-aegis/pull/52)]
- refactor(logging): update log statements to structured logging [[#50](https://github.com/Codycody31/squad-aegis/pull/50)]
- enhancement(rcon): manage active connections in RconManager [[#49](https://github.com/Codycody31/squad-aegis/pull/49)]
- enhancement: add avatar functionality to server switcher [[#47](https://github.com/Codycody31/squad-aegis/pull/47)]
- feat: add server metrics endpoint and fetching logic [[#43](https://github.com/Codycody31/squad-aegis/pull/43)]
- feat(settings): add server settings page and update routes [[#40](https://github.com/Codycody31/squad-aegis/pull/40)]

### 🐛 Bug Fixes

- fix: unable to delete server & dashboard incorrectly showing number o… [[#61](https://github.com/Codycody31/squad-aegis/pull/61)]
- fix: correct teamMap indexing for squad assignment [[#60](https://github.com/Codycody31/squad-aegis/pull/60)]
- fix(analytics): add sdk_name parameter to Countly URLs [[#53](https://github.com/Codycody31/squad-aegis/pull/53)]
- fix(logwatcher): temp directory and file creation for sources [[#46](https://github.com/Codycody31/squad-aegis/pull/46)]

### 📚 Documentation

- docs: Update README for clarity and formatting improvements [[#54](https://github.com/Codycody31/squad-aegis/pull/54)]
- docs: update README for improved configuration setup [[#42](https://github.com/Codycody31/squad-aegis/pull/42)]

## [0.1.0](https://github.com/Codycody31/squad-aegis/releases/tag/0.1.0) - 2025-03-24

### ❤️ Thanks to all contributors! ❤️

@Codycody31

### ✨ Features

- Add log parsers for game events & some more extensions [[#38](https://github.com/Codycody31/squad-aegis/pull/38)]
- Add log parsers for game events & some more extensions [[#36](https://github.com/Codycody31/squad-aegis/pull/36)]
- Log Watcher Connector & Related Extensions [[#24](https://github.com/Codycody31/squad-aegis/pull/24)]
- feat(admin-chat): add admin chat functionality and endpoints [[#21](https://github.com/Codycody31/squad-aegis/pull/21)]
- Server Connectors & Extensions [[#14](https://github.com/Codycody31/squad-aegis/pull/14)]
- Configure ACL/RBAC based on Squad permissions [[#9](https://github.com/Codycody31/squad-aegis/pull/9)]
- feat: add audit logs for most endpoints [[#4](https://github.com/Codycody31/squad-aegis/pull/4)]

### 📈 Enhancement

- refactor: remove unused chat processor implementation [[#39](https://github.com/Codycody31/squad-aegis/pull/39)]
- enhancement: add notes for extensions database records [[#22](https://github.com/Codycody31/squad-aegis/pull/22)]
- enhancement(connector): global connector deletion process [[#18](https://github.com/Codycody31/squad-aegis/pull/18)]
- RCON Manager [[#12](https://github.com/Codycody31/squad-aegis/pull/12)]
- refactor: enhance audit logging for admin actions [[#7](https://github.com/Codycody31/squad-aegis/pull/7)]
- refactor: update release config to drop unneeded sections [[#6](https://github.com/Codycody31/squad-aegis/pull/6)]

### 📚 Documentation

- docs: update README for clarity and organization [[#35](https://github.com/Codycody31/squad-aegis/pull/35)]
- docs: update README with features and extensibility details [[#20](https://github.com/Codycody31/squad-aegis/pull/20)]

### 🐛 Bug Fixes

- fix: event emitter not respecting global for server log watcher connector events. [[#30](https://github.com/Codycody31/squad-aegis/pull/30)]
- fix: add missing discord admin broadcast extension [[#29](https://github.com/Codycody31/squad-aegis/pull/29)]
- Fix log watcher docker image not using the correct binary [[#28](https://github.com/Codycody31/squad-aegis/pull/28)]
- fix(docker): add ca-certificates to final image dependencies [[#19](https://github.com/Codycody31/squad-aegis/pull/19)]
- Fix broken connector add logic [[#16](https://github.com/Codycody31/squad-aegis/pull/16)]

### 📦️ Dependency

- chore(deps): bump golang.org/x/net from 0.33.0 to 0.36.0 [[#17](https://github.com/Codycody31/squad-aegis/pull/17)]

### Misc

- Revert "Add log parsers for game events & some more extensions" [[#37](https://github.com/Codycody31/squad-aegis/pull/37)]
- chore: add Docker compose configuration for database and logwatcher [[#33](https://github.com/Codycody31/squad-aegis/pull/33)]
- chore: remove unused dependencies [[#8](https://github.com/Codycody31/squad-aegis/pull/8)]
