# Unified Game Events Architecture

## Overview

The unified game events architecture consolidates fragmented game-related events (round endings, new games, match winners, and game state changes) into a single comprehensive ClickHouse table. This approach eliminates sparse data across multiple tables and simplifies analytics queries.

## Schema Design

### Unified Table: `server_game_events_unified`

```sql
CREATE TABLE squad_aegis.server_game_events_unified (
    event_id        UUID DEFAULT generateUUIDv4(),
    event_time      DateTime64(3, 'UTC'),
    server_id       UUID,
    chain_id        String,
    event_type      LowCardinality(String), -- 'ROUND_ENDED', 'NEW_GAME', 'MATCH_WINNER', 'TICKET_UPDATE'
    
    -- Round/Match data
    winner          Nullable(String),
    layer           Nullable(String),
    team            Nullable(UInt8),
    subfaction      Nullable(String),
    faction         Nullable(String),
    action          Nullable(String), -- 'won', 'lost'
    tickets         Nullable(UInt32),
    level           Nullable(String),
    
    -- New Game data
    dlc             Nullable(String),
    map_classname   Nullable(String),
    layer_classname Nullable(String),
    
    -- Game state data
    from_state      Nullable(String),
    to_state        Nullable(String),
    
    -- JSON fields for complex data
    winner_data     Nullable(String), -- JSON string for winner team data
    loser_data      Nullable(String), -- JSON string for loser team data
    metadata        Nullable(String), -- JSON string for additional event-specific data
    
    -- Raw log line for debugging
    raw_log         String CODEC(ZSTD(5)),
    
    ingested_at     DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY (toYYYYMM(event_time), event_type)
ORDER BY (server_id, event_time, event_type, event_id);
```

## Event Types

### 1. TICKET_UPDATE
Individual ticket events when teams win/lose with specific ticket counts.
- Populated fields: `team`, `subfaction`, `faction`, `action`, `tickets`, `layer`, `level`

### 2. MATCH_WINNER
When the game determines a match winner.
- Populated fields: `winner`, `layer`

### 3. ROUND_ENDED
When match state changes from InProgress to WaitingPostMatch.
- Populated fields: `from_state`, `to_state`, `winner_data`, `loser_data`, `winner`, `layer`

### 4. NEW_GAME
When a new map/game starts.
- Populated fields: `dlc`, `map_classname`, `layer_classname` + any correlated winner data

## Benefits

1. **Reduced Storage**: Eliminates sparse columns across multiple tables
2. **Simplified Queries**: Single table for all game-related analytics
3. **Better Performance**: Optimized partitioning by event type and time
4. **Correlation**: Events can reference related data through metadata and timestamps

## Example Queries

### Get complete round information
```sql
SELECT 
    event_time,
    server_id,
    event_type,
    winner,
    layer,
    JSONExtract(winner_data, 'faction', 'String') as winner_faction,
    JSONExtract(winner_data, 'tickets', 'String') as winner_tickets,
    JSONExtract(loser_data, 'faction', 'String') as loser_faction,
    JSONExtract(loser_data, 'tickets', 'String') as loser_tickets
FROM squad_aegis.server_game_events_unified 
WHERE event_type = 'ROUND_ENDED'
AND event_time >= now() - INTERVAL 1 DAY
ORDER BY event_time DESC;
```

### Track map popularity
```sql
SELECT 
    layer_classname,
    count() as games_played,
    countIf(event_type = 'ROUND_ENDED') as rounds_completed
FROM squad_aegis.server_game_events_unified 
WHERE event_type IN ('NEW_GAME', 'ROUND_ENDED')
AND event_time >= now() - INTERVAL 7 DAY
GROUP BY layer_classname
ORDER BY games_played DESC;
```

### Analyze faction win rates
```sql
WITH winner_stats AS (
    SELECT 
        JSONExtract(winner_data, 'faction', 'String') as faction,
        count() as wins
    FROM squad_aegis.server_game_events_unified 
    WHERE event_type = 'ROUND_ENDED' 
    AND winner_data IS NOT NULL
    GROUP BY faction
),
total_stats AS (
    SELECT 
        faction,
        count() as total_rounds
    FROM (
        SELECT JSONExtract(winner_data, 'faction', 'String') as faction
        FROM squad_aegis.server_game_events_unified 
        WHERE event_type = 'ROUND_ENDED' AND winner_data IS NOT NULL
        UNION ALL
        SELECT JSONExtract(loser_data, 'faction', 'String') as faction  
        FROM squad_aegis.server_game_events_unified 
        WHERE event_type = 'ROUND_ENDED' AND loser_data IS NOT NULL
    )
    GROUP BY faction
)
SELECT 
    w.faction,
    w.wins,
    t.total_rounds,
    round(w.wins / t.total_rounds * 100, 2) as win_rate_percent
FROM winner_stats w
JOIN total_stats t ON w.faction = t.faction
ORDER BY win_rate_percent DESC;
```

## Migration Strategy

1. Deploy the unified table schema
2. Create materialized views for backwards compatibility
3. Update event ingestion to use unified events
4. Gradually migrate existing queries to use the new table
5. Eventually deprecate old fragmented tables

## Backwards Compatibility

Materialized views automatically populate the old table structures from the unified table, ensuring existing queries continue to work during the migration period.
