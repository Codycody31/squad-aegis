# Changelog

## [0.5.1](https://github.com/Codycody31/squad-aegis/releases/tag/0.5.1) - 2026-01-26

### ❤️ Thanks to all contributors! ❤️

@Insidious Fiddler

### Misc

- fix: enhance cookie handling for session authentication and improve player search input ([8eadb53](https://github.com/Codycody31/squad-aegis/commit/8eadb53c363d236199745d522709a526efcf67ed))
- revert: remove BattleMetrics fallback for player name lookup (too slow) ([48db48c](https://github.com/Codycody31/squad-aegis/commit/48db48c3de8bf8c2a592d195ddc759af0d49a324))
- fix: player name lookup and steam_id JSON encoding for ban list ([235e661](https://github.com/Codycody31/squad-aegis/commit/235e6613257dcfd7a4c481ce9d4ce51e2c166db3))
- refactor: api calls to handle auth failures & changesto the banned players page ([3053dc3](https://github.com/Codycody31/squad-aegis/commit/3053dc3e55161ccb877b8bc803169ec798f40a39))
- feat: add ban-with-evidence system for workflows and plugins ([1a1314d](https://github.com/Codycody31/squad-aegis/commit/1a1314d70b4dca9ab2a847ac6c4388018bce5c8a))
- fix: missed setup for /cfg that allowed wrong format ([858fc0c](https://github.com/Codycody31/squad-aegis/commit/858fc0c1cd724bc5d1fb3b46bd5d27c5e1ce1201))
- Revert "wip" ([510bac7](https://github.com/Codycody31/squad-aegis/commit/510bac7c82a0bacad254a21ee50894651b3a3f77))
- fix: remote ban list file being incorrect format (?) ([85c602f](https://github.com/Codycody31/squad-aegis/commit/85c602f06425d65d87ca6cfdae20c9f901a66f38))
- wip ([bdbafb0](https://github.com/Codycody31/squad-aegis/commit/bdbafb0115ce7e4360676cb104a31dc0ad41325b))

## [0.5.0](https://github.com/Codycody31/squad-aegis/releases/tag/0.5.0) - 2025-12-24

### ❤️ Thanks to all contributors! ❤️

@Codycody31, @Insidious Fiddler

### 📈 Enhancement

- refactor: enforce manageserver permission on server rules actions [[#125](https://github.com/Codycody31/squad-aegis/pull/125)]

### 🐛 Bug Fixes

- fix(discord-admin-requests): trim whitespace from permit and admin ro… [[#127](https://github.com/Codycody31/squad-aegis/pull/127)]
- fix(discord-admin-requests): update online admin count logic with rol… [[#126](https://github.com/Codycody31/squad-aegis/pull/126)]

### Misc

- feat(plugins): plugin command system with command execution and status tracking features ([ea1b3c7](https://github.com/Codycody31/squad-aegis/commit/ea1b3c73be5faf21a9397189252fdabd4c9a1893))
- refactor: introduce TTL constants for player, session, join request, and round data storage for event_store & player_tracker ([59ab186](https://github.com/Codycody31/squad-aegis/commit/59ab186d2ea893ebcabe63084ca1af5a4d0e60fb))
- docs: bump min ram ([3699240](https://github.com/Codycody31/squad-aegis/commit/3699240d8cc65eb0cd16ce76adb2234fbd070e09))
- fix(bans): missing selective file deletion and querying existing evidence files resulting in large unexpected deletions ([df619c8](https://github.com/Codycody31/squad-aegis/commit/df619c8ffa824f6a5ce721a7110a12c7e956cf9a))
- feat(sudo): implement semi-okay admin dashboard with metrics, system health, storage management, and session control features ([3103225](https://github.com/Codycody31/squad-aegis/commit/3103225ae2eb2376ecb3ca0c93ed1409b2dccef1))
- feat(plugins): implement Squad Creation Blocker plugin to prevent custom squad creation during specified periods with rate limiting and cooldown features ([b9818ab](https://github.com/Codycody31/squad-aegis/commit/b9818abe811e2c808cf9c00a7021f1c63b4da970))
- refactor: add context menu for selecting multiple players ([e0addfa](https://github.com/Codycody31/squad-aegis/commit/e0addfaa9876c276885feb4a47d37625a85069b6))
- docs(plugins): update auto warn & kick sl wrong kit and cbl ([7f72b12](https://github.com/Codycody31/squad-aegis/commit/7f72b122493baaf2e6ff108962b1be3957b44478))
- feat(plugins): introduce Team Balancer plugin for tracking win streaks and managing team scrambles to ensure balanced matches ([f61238d](https://github.com/Codycody31/squad-aegis/commit/f61238d31ec67391f01613465f03e2acf40dddb1))
- feat(plugins): add log level configuration for plugin instances with database migrations and API updates ([06e6457](https://github.com/Codycody31/squad-aegis/commit/06e64574e9ba1e9403a0a903a1b33fcfe0406722))
- fix(workflow): add trigger_data field to ServerWorkflowExecution and update related database queries and migrations ([584a842](https://github.com/Codycody31/squad-aegis/commit/584a8425eddc0c00df2913421d83de9cbd4df48a))
- feat(workflow): add Workflow Execution Timeline component and enhance WorkflowEditor with null checks for triggers and steps ([d49870d](https://github.com/Codycody31/squad-aegis/commit/d49870d9a5cf47f66421ca069cc14e127a366069))
- fix(discord_teamkill): update attacker and victim data extraction in teamkill embed ([4df9a00](https://github.com/Codycody31/squad-aegis/commit/4df9a00c5085926785ea35fde34ae58d726cbfdf))
- feat(plugins): add Discord Teamkill plugin for logging teamkill events ([bd72796](https://github.com/Codycody31/squad-aegis/commit/bd72796a7cfe5ca3739250fe28bffa515946842a))
- fix(cbl): remove unnecessary debug logs for player checks in CBL ([bfed345](https://github.com/Codycody31/squad-aegis/commit/bfed345efe027b61264d33d1e515641e630713a1))
- refactor(cbl): add configuration option to ignore specific Steam IDs from CBL checks ([2e91b7a](https://github.com/Codycody31/squad-aegis/commit/2e91b7a3685efff4c019759c32ce9e0225d22c6c))
- refactor(cbl): implement Community Ban List plugin for player monitoring and alerts ([65b426a](https://github.com/Codycody31/squad-aegis/commit/65b426a3a54a6947831fedab0b9113589ea04a9b))
- fix(cbl_info): ensure proper context usage in player connection handling ([82b7828](https://github.com/Codycody31/squad-aegis/commit/82b7828627eef556e74dfe7ebf8825ddf1ee2848))
- chore(logwatcher): add error handling for player data removal on disconnect ([2f5a5bd](https://github.com/Codycody31/squad-aegis/commit/2f5a5bd92d03b2529c0fcb9ff909735f81605658))
- chore(logwatcher): attempt to imrpove event data by populating attacker and victim names from player data ([db1be0c](https://github.com/Codycody31/squad-aegis/commit/db1be0c2fe37c5eb0af64bdabc1213562ff55ab1))
- enhance(logwatcher): event store and log parser to include player names for attackers and victims ([23d4939](https://github.com/Codycody31/squad-aegis/commit/23d49394d993000b6a32536d2e13c2f332a550e0))
- refactor(metrics): implement custom date range selection for server metrics and enhance metrics fetching logic ([2564a05](https://github.com/Codycody31/squad-aegis/commit/2564a05e0937df7a1802fae565d4dc83b940d7dd))
- feat(server-roles): add is_admin flag to server roles and update role management functionality ([d3807b2](https://github.com/Codycody31/squad-aegis/commit/d3807b23a1c4d379db1b05c3dcfee91821ccf3d0))
- docs(plugins): add auto-warn-sl-wrong-kit & kill-broadcast ([e72171a](https://github.com/Codycody31/squad-aegis/commit/e72171a54f19ea6cd0daac6719a9649b0a1651a4))
- feat(plugin): add Kill Broadcast plugin to announce player kills ([d2be527](https://github.com/Codycody31/squad-aegis/commit/d2be527ee7a72b3a52c96f9fdfd8f1d7c934cc20))
- feat(plugin): add functionality to remove players from squad without kicking ([c491995](https://github.com/Codycody31/squad-aegis/commit/c491995a9d534edf789083a041e7fe4de6cf9ca4))
- feat(plugin): add Auto Kick SL Wrong Kit plugin and enhance PlayerInfo structure ([d00f111](https://github.com/Codycody31/squad-aegis/commit/d00f1116c758b66cbde7425e51a249dc0ffb8819))
- refactor(discord-admin-requests): improve admin request handling ([54e1ae4](https://github.com/Codycody31/squad-aegis/commit/54e1ae4ad2684cb53837256f094931e4f9d07224))
- chore: make pages mobile & desktop friendly [[#122](https://github.com/Codycody31/squad-aegis/pull/122)]

## [0.4.0](https://github.com/Codycody31/squad-aegis/releases/tag/0.4.0) - 2025-12-03

### ❤️ Thanks to all contributors! ❤️

@Codycody31

### ✨ Features

- feat(events): simplify query logic and enhance player_connected handling [[#119](https://github.com/Codycody31/squad-aegis/pull/119)]
- feat: add storage volume and set timezone to UTC [[#118](https://github.com/Codycody31/squad-aegis/pull/118)]
- feature: ban evidence [[#117](https://github.com/Codycody31/squad-aegis/pull/117)]
- feat: live viewing of plugin logs [[#110](https://github.com/Codycody31/squad-aegis/pull/110)]
- feat: add JSON editor for workflow definition management [[#107](https://github.com/Codycody31/squad-aegis/pull/107)]
- feat(workflow): workflow conditions can have nested steps [[#104](https://github.com/Codycody31/squad-aegis/pull/104)]

### 📈 Enhancement

- refactor: add option to hide expired bans in banned players list [[#120](https://github.com/Codycody31/squad-aegis/pull/120)]
- refactor(ui): add demote commander btn [[#116](https://github.com/Codycody31/squad-aegis/pull/116)]
- refactor: add CBL info + steam & battlemetrics links to player profile [[#115](https://github.com/Codycody31/squad-aegis/pull/115)]
- refactor: improve player violations tab [[#109](https://github.com/Codycody31/squad-aegis/pull/109)]
- refactor: add player profile link and fetch name if empty for players… [[#102](https://github.com/Codycody31/squad-aegis/pull/102)]

### 🐛 Bug Fixes

- fix: ban duration of 0 not working [[#114](https://github.com/Codycody31/squad-aegis/pull/114)]
- feat(workflow): implement workflow kv store [[#113](https://github.com/Codycody31/squad-aegis/pull/113)]
- fix: ban cfg format not correct [[#112](https://github.com/Codycody31/squad-aegis/pull/112)]
- fix: auto tk warn plugin & live plugin logs [[#111](https://github.com/Codycody31/squad-aegis/pull/111)]
- fix: missing RCON_SERVER_INFO workflow event in UI [[#108](https://github.com/Codycody31/squad-aegis/pull/108)]
- fix(feeds): teamkill feed not showing attacker name [[#105](https://github.com/Codycody31/squad-aegis/pull/105)]

### 📦️ Dependency

- chore(deps): update fumadocs and tailwind dependencies to latest vers… [[#106](https://github.com/Codycody31/squad-aegis/pull/106)]

### Misc

- chore: update squad aegis icon [[#121](https://github.com/Codycody31/squad-aegis/pull/121)]

## [0.3.1](https://github.com/Codycody31/squad-aegis/releases/tag/0.3.1) - 2025-11-08

### ❤️ Thanks to all contributors! ❤️

@Codycody31

### 🐛 Bug Fixes

- fix: update paths for web output in Dockerfile and embed [[#100](https://github.com/Codycody31/squad-aegis/pull/100)]

## [0.3.0](https://github.com/Codycody31/squad-aegis/releases/tag/0.3.0) - 2025-11-08

### ❤️ Thanks to all contributors! ❤️

@Codycody31, @Insidious Fiddler

### ✨ Features

- feat: teamkill tracking + more advanced player tracking [[#98](https://github.com/Codycody31/squad-aegis/pull/98)]
- feat(players): add player profile data structures and API endpoint [[#96](https://github.com/Codycody31/squad-aegis/pull/96)]
- feat: implement conditional step management in workflow [[#95](https://github.com/Codycody31/squad-aegis/pull/95)]
- feat: implement conditional step management in workflow [[#94](https://github.com/Codycody31/squad-aegis/pull/94)]
- feat: add file upload feature for importing workflow JSON [[#93](https://github.com/Codycody31/squad-aegis/pull/93)]
- feat: enhance UI with player action menus on feeds [[#86](https://github.com/Codycody31/squad-aegis/pull/86)]
- feat: edit/delete plugin data [[#83](https://github.com/Codycody31/squad-aegis/pull/83)]

### 📈 Enhancement

- refactor: update/improve player actions & rules UI [[#90](https://github.com/Codycody31/squad-aegis/pull/90)]
- refactor: improve text export formatting with indentation logic [[#87](https://github.com/Codycody31/squad-aegis/pull/87)]
- refactor: Migrate from Apache License 2.0 -> Elastic License 2.0 [[#84](https://github.com/Codycody31/squad-aegis/pull/84)]
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

- docs: add Elastic License 2.0 documentation and entry [[#99](https://github.com/Codycody31/squad-aegis/pull/99)]
- docs: update title and content structure in docs [[#89](https://github.com/Codycody31/squad-aegis/pull/89)]
- docs: v2 via fumadocs [[#88](https://github.com/Codycody31/squad-aegis/pull/88)]

### Misc

- ci(docker): attempt to clean up docker image and fix build issues ([a70f320](https://github.com/Codycody31/squad-aegis/commit/a70f3204aadb5b17193d18d704796714533248d0))
- chore: assets/_nuxt -> assets/nuxt ([c6d0711](https://github.com/Codycody31/squad-aegis/commit/c6d0711c4f22657fc2c75b32442ff26815a927da))
- chore: move nuxt to assets/_nuxt ([ed52a92](https://github.com/Codycody31/squad-aegis/commit/ed52a920cc529ba819a3a4f49cc9a36129879221))
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
