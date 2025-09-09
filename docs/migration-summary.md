# Summary of Unified Game Events Migration

## Files Created/Modified

### 1. New Migration Files
- **`internal/clickhouse/migrations/000002_unified_game_events.up.sql`**: Creates the unified `server_game_events_unified` table with materialized views for backwards compatibility
- **`internal/clickhouse/migrations/000003_cleanup_fragmented_tables.up.sql`**: Drops old fragmented tables and adds performance indexes  
- **`internal/clickhouse/migrations/000003_cleanup_fragmented_tables.down.sql`**: Rollback migration to recreate old tables

### 2. New Parser Implementation
- **`internal/logwatcher_manager/unified_game_parsers.go`**: New unified parsers that consolidate game events into a single table

### 3. Modified Core Files
- **`internal/logwatcher_manager/logwatcher_manager.go`**: Updated to use `GetOptimizedLogParsers()`
- **`internal/event_manager/event_types.go`**: Added `LogGameEventUnifiedData` and `EventTypeLogGameEventUnified`
- **`internal/event_manager/event_manager.go`**: Added new event type constant
- **`internal/clickhouse/ingester.go`**: 
  - Added `ingestGameEventUnified()` method
  - Removed old `ingestNewGame()` and `ingestRoundEnded()` methods
  - Updated event routing

### 4. Updated Plugins
- **`internal/plugins/discord_round_ended/discord_round_ended.go`**: Updated to handle both unified and legacy events
- **`internal/plugins/discord_round_winner/discord_round_winner.go`**: Updated to handle both unified and legacy events

### 5. Documentation  
- **`docs/unified-game-events.md`**: Comprehensive documentation with schema, examples, and migration strategy

## Key Improvements

### Before (Fragmented Approach)
```
server_new_game_events        -> Mostly empty columns for non-new-game data
server_round_ended_events     -> Mostly empty columns for non-round-end data
```
- 2 separate tables with sparse data
- Complex joins required for comprehensive analytics
- Duplicated effort in maintenance

### After (Unified Approach)  
```
server_game_events_unified    -> Single table with event_type discrimination
```
- 1 comprehensive table with efficient partitioning by event type
- Simpler queries with better performance
- JSON fields for complex/variable data
- Backwards compatibility via materialized views

## Event Types in Unified Table

1. **`TICKET_UPDATE`**: Individual ticket win/loss events
2. **`MATCH_WINNER`**: Game engine match winner determination 
3. **`ROUND_ENDED`**: Game state transition to post-match
4. **`NEW_GAME`**: New map/game initialization with correlated winner data

## Migration Benefits

- **Storage Efficiency**: Eliminates sparse columns across multiple tables
- **Query Performance**: Single table with optimized indexes and partitioning
- **Analytics Simplicity**: Complete round lifecycle in one place
- **Backwards Compatibility**: Existing queries continue to work via materialized views
- **Future Extensibility**: Easy to add new game event types

## Example Unified Query

```sql
-- Get complete round information with winner/loser details
SELECT 
    event_time,
    event_type,
    winner,
    layer,
    JSONExtract(winner_data, 'faction', 'String') as winner_faction,
    JSONExtract(winner_data, 'tickets', 'String') as winner_tickets
FROM squad_aegis.server_game_events_unified 
WHERE event_type IN ('ROUND_ENDED', 'NEW_GAME')
ORDER BY event_time DESC;
```

This migration successfully consolidates fragmented game events into a single, efficient, and extensible table structure.
