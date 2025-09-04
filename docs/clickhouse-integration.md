# ClickHouse Integration

Squad Aegis includes optional ClickHouse integration for storing and analyzing server events at scale. This integration captures all RCON and log events from your Squad servers and stores them in ClickHouse for analytics, metrics, and long-term storage.

## Features

- **Real-time Event Ingestion**: All events from RCON and log sources are automatically ingested into ClickHouse
- **Batch Processing**: Events are processed in batches for optimal performance
- **Comprehensive Event Types**: Supports all Squad game events including:
  - Player connections/disconnections
  - Chat messages
  - Player damage, deaths, and wounds
  - Player revivals and team changes
  - Admin actions and broadcasts
  - Round events and game state changes
  - Server performance metrics (tick rate)

## Setup

### 1. Install ClickHouse

Follow the official ClickHouse installation guide for your platform:
- [ClickHouse Installation Guide](https://clickhouse.com/docs/en/getting-started/install)

### 2. Create Database and Tables

Run the migration SQL provided in `internal/clickhouse/migrations/000001_initial.up.sql`:

```bash
clickhouse-client --multiquery < internal/clickhouse/migrations/000001_initial.up.sql
```

### 3. Configure Squad Aegis

Add the ClickHouse configuration to your Squad Aegis configuration:

```yaml
clickhouse:
  enabled: true
  host: "localhost"
  port: 9000
  database: "squad_aegis"
  username: "default"
  password: ""
  debug: false
```

Or set environment variables:
- `CLICKHOUSE_ENABLED=true`
- `CLICKHOUSE_HOST=localhost`
- `CLICKHOUSE_PORT=9000`
- `CLICKHOUSE_DATABASE=squad_aegis`
- `CLICKHOUSE_USERNAME=default`
- `CLICKHOUSE_PASSWORD=`
- `CLICKHOUSE_DEBUG=false`

### 4. Start Squad Aegis

When Squad Aegis starts with ClickHouse enabled, you'll see:

```
INFO Starting ClickHouse event ingester
INFO ClickHouse event ingestion enabled
```

## Event Types

The following events are automatically captured and stored:

### Player Events
- **Player Connected**: When players join the server
- **Player Disconnected**: When players leave the server
- **Player Damaged**: When players take damage
- **Player Died**: When players are killed
- **Player Wounded**: When players are wounded
- **Player Revived**: When players are revived
- **Player Team Change**: When players switch teams
- **Player Squad Change**: When players switch squads
- **Player Possess**: When players spawn/possess characters

### Chat Events
- **Chat Messages**: All in-game chat messages

### Game Events
- **New Game**: When a new round starts
- **Round Ended**: When a round ends with results
- **Admin Broadcast**: Admin messages to all players
- **Tick Rate**: Server performance metrics

### Infrastructure Events
- **Deployable Damaged**: When deployables (FOBs, etc.) take damage
- **Join Succeeded**: Successful player joins

## Database Schema

The ClickHouse database uses optimized table structures with:

- **Partitioning**: Tables are partitioned by month for efficient queries
- **Compression**: ZSTD compression for optimal storage
- **Indexes**: TokenBF indexes for fast text searches
- **Materialized Views**: Pre-aggregated daily statistics

### Key Tables

- `server_player_chat_messages`: All chat messages
- `server_player_connected_events`: Player connections
- `server_player_disconnected_events`: Player disconnections  
- `server_player_died_events`: Player deaths
- `server_player_damaged_events`: Player damage events
- `server_new_game_events`: Round start events
- `server_round_ended_events`: Round end events

## Querying Data

### Example Queries

**Most active players by chat messages:**
```sql
SELECT 
    player_id,
    count() as message_count
FROM squad_aegis.server_player_chat_messages 
WHERE sent_at >= now() - INTERVAL 7 DAY
GROUP BY player_id 
ORDER BY message_count DESC 
LIMIT 10;
```

**Server activity by hour:**
```sql
SELECT 
    toHour(event_time) as hour,
    count() as connections
FROM squad_aegis.server_player_connected_events 
WHERE event_time >= today()
GROUP BY hour 
ORDER BY hour;
```

**Top weapons by kills:**
```sql
SELECT 
    weapon,
    count() as kills
FROM squad_aegis.server_player_died_events 
WHERE event_time >= now() - INTERVAL 1 DAY
GROUP BY weapon 
ORDER BY kills DESC 
LIMIT 10;
```

## Performance

The ClickHouse integration is designed for high performance:

- **Batch Processing**: Events are batched (default 100 events or 5 seconds)
- **Async Processing**: Event ingestion doesn't block game server operations
- **Buffered Queues**: 1000-event buffer prevents data loss during spikes
- **Optimized Schema**: Column-oriented storage for analytical queries

## Monitoring

Monitor the integration through logs:

- Event ingestion statistics
- Batch processing metrics
- Connection status
- Error reporting

## Troubleshooting

### Connection Issues
- Verify ClickHouse is running and accessible
- Check network connectivity and firewall rules
- Validate credentials and database permissions

### Performance Issues
- Monitor batch sizes and flush intervals
- Check ClickHouse server resources
- Review query performance and add indexes if needed

### Data Issues
- Verify event data structure matches expected schema
- Check for timezone discrepancies
- Monitor for dropped events due to queue overflow
