---
title: Architecture
---

This page provides a detailed overview of Squad Aegis architecture with visual diagrams to help understand the data flow and component interactions.

## System Overview

Squad Aegis follows a modular architecture where different components interact through well-defined interfaces. The system is designed to be extensible through plugins while maintaining a stable core.

```mermaid
flowchart TD
    subgraph "Squad Aegis Control Panel"
        API[HTTP API Server]
        UI[Web UI]
        EventManager[Event Manager]
        RCONManager[RCON Manager]
        LogManager[Log Watcher Manager]
        PluginManager[Plugin Manager]
        DB[(PostgreSQL)]
        CH[(ClickHouse)]
    end

    subgraph "Game Servers"
        Server1[Squad Server 1]
        Server2[Squad Server 2]
        ServerN[Squad Server N]
    end

    subgraph "External Services"
        Discord[Discord]
        Other[Other Services]
    end

    UI <--> API
    API <--> DB
    API <--> EventManager
    API <--> RCONManager
    API <--> LogManager
    API <--> PluginManager
    
    EventManager <--> CH
    PluginManager <--> DB
    PluginManager <--> CH
    PluginManager <--> EventManager
    
    RCONManager <--> Server1
    RCONManager <--> Server2
    RCONManager <--> ServerN
    
    LogManager <--> Server1
    LogManager <--> Server2
    LogManager <--> ServerN
    
    PluginManager <--> Discord
    PluginManager <--> Other
    
    RCONManager --> EventManager
    LogManager --> EventManager
```

## Event Flow

Events are at the heart of Squad Aegis. The event system follows a publish-subscribe pattern, allowing components to emit events that other components can react to.

```mermaid
sequenceDiagram
    participant GameServer as Game Server
    participant RCON as RCON Manager
    participant LogWatcher as Log Watcher
    participant EventManager as Event Manager
    participant Plugins as Plugin Manager
    participant DB as ClickHouse
    
    GameServer->>RCON: RCON Event (chat, admin actions)
    RCON->>EventManager: Publish Event
    
    GameServer->>LogWatcher: Log Entry
    LogWatcher->>EventManager: Parse & Publish Event
    
    EventManager->>Plugins: Distribute Event
    EventManager->>DB: Store Event
    
    Plugins->>GameServer: Execute Commands
    Plugins->>DB: Store Plugin-specific Data
```

## Component Architecture

### Core Components

The core of Squad Aegis consists of several managers that handle different aspects of the system.

```mermaid
classDiagram
    class EventManager {
        +PublishEvent()
        +Subscribe()
        +Unsubscribe()
        -distributeEvent()
    }
    
    class RCONManager {
        +ConnectToAllServers()
        +SendCommand()
        +GetCurrentPlayers()
        -broadcastEvent()
    }
    
    class LogWatcherManager {
        +ConnectToAllServers()
        +StartConnectionManager()
        -parseLogLine()
    }
    
    class PluginManager {
        +RegisterPlugin()
        +CreatePluginInstance()
        +GetPluginInstance()
        +EnablePluginInstance()
        -handlePluginEvent()
    }
    
    EventManager <-- RCONManager : Publishes events
    EventManager <-- LogWatcherManager : Publishes events
    EventManager <-- PluginManager : Subscribes to events
```

### Plugin System

The plugin system allows for extending functionality without modifying the core codebase.

```mermaid
classDiagram
    class Plugin {
        <<interface>>
        +GetDefinition()
        +Initialize()
        +Start()
        +Stop()
        +HandleEvent()
    }
    
    class PluginDefinition {
        +ID : string
        +Name : string
        +Description : string
        +ConfigSchema : ConfigSchema
        +Events : []EventType
    }
    
    class PluginInstance {
        +ID : UUID
        +ServerID : UUID
        +PluginID : string
        +Status : PluginStatus
        +Config : map
        +Plugin : Plugin
    }
    
    class PluginAPIs {
        +ServerAPI
        +DatabaseAPI
        +RconAPI
        +AdminAPI
        +EventAPI
        +ConnectorAPI
        +LogAPI
    }
    
    Plugin <-- PluginInstance : Contains
    PluginDefinition <-- Plugin : Defines
    PluginAPIs <-- Plugin : Uses
```

### Database Architecture

Squad Aegis uses two database systems: PostgreSQL for relational data and ClickHouse for analytics.

```mermaid
erDiagram
    SERVERS {
        UUID id PK
        string name
        string ip_address
        int game_port
        string rcon_password
    }
    
    USERS {
        UUID id PK
        string username
        string password_hash
        boolean super_admin
    }
    
    SERVER_ADMINS {
        UUID id PK
        UUID server_id FK
        UUID user_id FK
        string role
        timestamp expires_at
    }
    
    PLUGIN_INSTANCES {
        UUID id PK
        UUID server_id FK
        string plugin_id
        jsonb config
        boolean enabled
    }
    
    CONNECTOR_INSTANCES {
        string id PK
        jsonb config
        boolean enabled
    }
    
    SERVERS ||--o{ SERVER_ADMINS : "has admins"
    USERS ||--o{ SERVER_ADMINS : "has access to"
    SERVERS ||--o{ PLUGIN_INSTANCES : "has plugins"
```

## Data Flow

This diagram shows how data flows through the system from various sources to the end user.

```mermaid
flowchart TD
    subgraph "Data Sources"
        RCON[RCON Events]
        Logs[Server Logs]
        API_Requests[API Requests]
    end
    
    subgraph "Processing"
        RCONManager[RCON Manager]
        LogWatcher[Log Watcher]
        APIServer[API Server]
        
        EventManager[Event Manager]
        PluginManager[Plugin Manager]
    end
    
    subgraph "Storage"
        Postgres[(PostgreSQL)]
        ClickHouse[(ClickHouse)]
    end
    
    subgraph "Output"
        WebUI[Web UI]
        WebHooks[WebHooks]
        Discord[Discord]
    end
    
    RCON --> RCONManager
    Logs --> LogWatcher
    API_Requests --> APIServer
    
    RCONManager --> EventManager
    LogWatcher --> EventManager
    APIServer --> EventManager
    
    EventManager --> PluginManager
    PluginManager --> Postgres
    PluginManager --> ClickHouse
    EventManager --> ClickHouse
    APIServer --> Postgres
    
    PluginManager --> WebHooks
    PluginManager --> Discord
    Postgres --> APIServer
    ClickHouse --> APIServer
    APIServer --> WebUI
```

## Plugin Data Flow

This diagram illustrates how data flows through a plugin from receiving an event to taking actions.

```mermaid
sequenceDiagram
    participant EventManager
    participant PluginManager
    participant Plugin
    participant RCON
    participant DB
    participant External
    
    EventManager->>PluginManager: Event
    PluginManager->>Plugin: HandleEvent()
    
    alt Process Event
        Plugin->>Plugin: Process Event
        Plugin->>RCON: Execute Command
        RCON-->>Plugin: Command Result
        Plugin->>DB: Store Data
        DB-->>Plugin: Stored Data
        Plugin->>External: Notify External Service
        External-->>Plugin: External Service Response
    end
    
    Plugin-->>PluginManager: Event Handled
```

## Deployment Architecture

Squad Aegis is designed to be deployed using Docker containers, making it easy to set up and maintain.

```mermaid
flowchart TD
    subgraph "Docker Environment"
        subgraph "Squad Aegis Container"
            Server[Server]
            API[API]
        end
        
        subgraph "Database Containers"
            Postgres[(PostgreSQL)]
            ClickHouse[(ClickHouse)]
        end
        
        subgraph "Frontend Container"
            WebUI[Web UI]
        end
        
        subgraph "Proxy Container"
            Nginx[Nginx]
        end
    end
    
    subgraph "External"
        SquadServers[Squad Servers]
        Users[Users]
    end
    
    Server <--> Postgres
    Server <--> ClickHouse
    API <--> Server
    WebUI <--> API
    Nginx <--> WebUI
    Nginx <--> API
    
    Server <--> SquadServers
    Users <--> Nginx
```

## Development Considerations

When developing for Squad Aegis, keep these architectural principles in mind:

1. **Event-Driven Design**: Most functionality is triggered by events. New features should integrate with the event system.

2. **Modularity**: Keep components modular and with well-defined interfaces.

3. **Plugin Extensibility**: When possible, implement new features as plugins rather than modifying core code.

4. **Scalability**: The system should handle multiple servers and high event volumes efficiently.

5. **Security**: Be mindful of security considerations, especially in the RCON and API components.

By understanding these diagrams and principles, developers can effectively contribute to and extend Squad Aegis.
