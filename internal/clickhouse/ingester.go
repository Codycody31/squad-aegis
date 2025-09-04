package clickhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
)

// EventIngester handles ingesting events from the event manager into ClickHouse
type EventIngester struct {
	client        *Client
	ctx           context.Context
	cancel        context.CancelFunc
	eventManager  *event_manager.EventManager
	subscriber    *event_manager.EventSubscriber
	batchSize     int
	flushInterval time.Duration
	eventQueue    chan *IngestEvent
	wg            sync.WaitGroup
}

// IngestEvent represents an event ready for ingestion
type IngestEvent struct {
	ServerID  uuid.UUID
	EventType event_manager.EventType
	EventTime time.Time
	Data      map[string]interface{}
	RawData   interface{}
}

// NewEventIngester creates a new event ingester
func NewEventIngester(ctx context.Context, client *Client, eventManager *event_manager.EventManager) *EventIngester {
	ctx, cancel := context.WithCancel(ctx)

	ingester := &EventIngester{
		client:        client,
		ctx:           ctx,
		cancel:        cancel,
		eventManager:  eventManager,
		batchSize:     100,
		flushInterval: 5 * time.Second,
		eventQueue:    make(chan *IngestEvent, 1000),
	}

	return ingester
}

// Start begins the event ingestion process
func (i *EventIngester) Start() {
	log.Info().Msg("Starting ClickHouse event ingester")

	// Subscribe to events from the event manager
	filter := event_manager.EventFilter{} // Subscribe to all events
	i.subscriber = i.eventManager.Subscribe(filter, nil, 1000)
	eventChan := i.subscriber.Channel

	i.wg.Add(2)

	// Event consumer goroutine
	go func() {
		defer i.wg.Done()
		for {
			select {
			case <-i.ctx.Done():
				return
			case event, ok := <-eventChan:
				if !ok {
					return
				}
				i.processEvent(event)
			}
		}
	}()

	// Batch processor goroutine
	go func() {
		defer i.wg.Done()
		i.batchProcessor()
	}()
}

// Stop stops the event ingestion process
func (i *EventIngester) Stop() {
	log.Info().Msg("Stopping ClickHouse event ingester")

	// Unsubscribe from events
	if i.subscriber != nil {
		i.eventManager.Unsubscribe(i.subscriber.ID)
	}

	i.cancel()
	close(i.eventQueue)
	i.wg.Wait()
}

// processEvent converts an event manager event to an ingest event
func (i *EventIngester) processEvent(event event_manager.Event) {
	ingestEvent := &IngestEvent{
		ServerID:  event.ServerID,
		EventType: event.Type,
		EventTime: event.Timestamp,
		Data:      event.Data,
		RawData:   event.RawData,
	}

	select {
	case i.eventQueue <- ingestEvent:
	case <-i.ctx.Done():
		return
	default:
		log.Warn().Msg("Event queue full, dropping event")
	}
}

// batchProcessor processes events in batches
func (i *EventIngester) batchProcessor() {
	ticker := time.NewTicker(i.flushInterval)
	defer ticker.Stop()

	batch := make([]*IngestEvent, 0, i.batchSize)

	for {
		select {
		case <-i.ctx.Done():
			// Flush remaining events
			if len(batch) > 0 {
				i.ingestBatch(batch)
			}
			return

		case <-ticker.C:
			// Flush on timer
			if len(batch) > 0 {
				i.ingestBatch(batch)
				batch = batch[:0]
			}

		case event, ok := <-i.eventQueue:
			if !ok {
				// Channel closed, flush remaining and exit
				if len(batch) > 0 {
					i.ingestBatch(batch)
				}
				return
			}

			batch = append(batch, event)
			if len(batch) >= i.batchSize {
				i.ingestBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

// ingestBatch ingests a batch of events into ClickHouse
func (i *EventIngester) ingestBatch(events []*IngestEvent) {
	if len(events) == 0 {
		return
	}

	// Group events by type for efficient insertion
	eventGroups := make(map[event_manager.EventType][]*IngestEvent)
	for _, event := range events {
		eventGroups[event.EventType] = append(eventGroups[event.EventType], event)
	}

	// Process each event type
	for eventType, typeEvents := range eventGroups {
		if err := i.ingestEventType(eventType, typeEvents); err != nil {
			log.Error().
				Err(err).
				Str("eventType", string(eventType)).
				Int("count", len(typeEvents)).
				Msg("Failed to ingest events")
		}
	}
}

// ingestEventType ingests events of a specific type
func (i *EventIngester) ingestEventType(eventType event_manager.EventType, events []*IngestEvent) error {
	switch eventType {
	case event_manager.EventTypeLogChatMessage:
		return i.ingestChatMessages(events)
	case event_manager.EventTypeLogPlayerConnected:
		return i.ingestPlayerConnected(events)
	case event_manager.EventTypeLogPlayerDisconnected:
		return i.ingestPlayerDisconnected(events)
	case event_manager.EventTypeLogPlayerDamaged:
		return i.ingestPlayerDamaged(events)
	case event_manager.EventTypeLogPlayerDied:
		return i.ingestPlayerDied(events)
	case event_manager.EventTypeLogPlayerWounded:
		return i.ingestPlayerWounded(events)
	case event_manager.EventTypeLogPlayerRevived:
		return i.ingestPlayerRevived(events)
	case event_manager.EventTypeLogNewGame:
		return i.ingestNewGame(events)
	case event_manager.EventTypeLogRoundEnded:
		return i.ingestRoundEnded(events)
	case event_manager.EventTypeLogPlayerSquadChange:
		return i.ingestPlayerSquadChange(events)
	case event_manager.EventTypeLogPlayerTeamChange:
		return i.ingestPlayerTeamChange(events)
	case event_manager.EventTypeLogPlayerPossess:
		return i.ingestPlayerPossess(events)
	case event_manager.EventTypeLogJoinSucceeded:
		return i.ingestJoinSucceeded(events)
	case event_manager.EventTypeLogAdminBroadcast:
		return i.ingestAdminBroadcast(events)
	case event_manager.EventTypeLogDeployableDamaged:
		return i.ingestDeployableDamaged(events)
	case event_manager.EventTypeLogTickRate:
		return i.ingestTickRate(events)
	default:
		log.Debug().
			Str("eventType", string(eventType)).
			Msg("Unhandled event type for ClickHouse ingestion")
		return nil
	}
}

// Helper functions for extracting values from event data
func getStringValue(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
		return fmt.Sprintf("%v", val)
	}
	return ""
}

func getFloat32Value(data map[string]interface{}, key string) float32 {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case float64:
			return float32(v)
		case float32:
			return v
		case int:
			return float32(v)
		case string:
			if f, err := strconv.ParseFloat(v, 32); err == nil {
				return float32(f)
			}
		}
	}
	return 0
}

func getTimeValue(data map[string]interface{}, key string) time.Time {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case time.Time:
			return v
		case string:
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

// Specific ingestion functions for each event type
func (i *EventIngester) ingestChatMessages(events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_player_chat_messages 
		(message_id, server_id, player_id, sent_at, type, message, ingest_ts) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*7)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?)")

		// Generate a UUID for the message
		messageID := uuid.New()

		// Try to extract player ID from steam ID if available
		var playerID uuid.UUID
		if steamIDStr := getStringValue(event.Data, "steamId"); steamIDStr != "" {
			// For now, use a deterministic UUID based on steam ID
			// In a real system, you'd look up the player ID from the players table
			playerID = uuid.NewSHA1(uuid.NameSpaceDNS, []byte("steam:"+steamIDStr))
		} else {
			playerID = uuid.New() // Fallback
		}

		args = append(args,
			messageID,
			event.ServerID,
			playerID,
			event.EventTime,
			1, // Chat message type
			getStringValue(event.Data, "message"),
			time.Now(),
		)
	}

	query += strings.Join(values, ",")
	return i.client.Exec(i.ctx, query, args...)
}

func (i *EventIngester) ingestPlayerConnected(events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_player_connected_events 
		(event_time, server_id, chain_id, player_controller, ip, steam, eos, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*8)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			event.EventTime,
			event.ServerID,
			getStringValue(event.Data, "chainID"),
			getStringValue(event.Data, "playerController"),
			getStringValue(event.Data, "ip"),
			getStringValue(event.Data, "steam"),
			getStringValue(event.Data, "eos"),
			time.Now(),
		)
	}

	query += strings.Join(values, ",")
	return i.client.Exec(i.ctx, query, args...)
}

func (i *EventIngester) ingestPlayerDisconnected(events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_player_disconnected_events 
		(event_time, server_id, chain_id, ip, player_controller, eos_id, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*7)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			event.EventTime,
			event.ServerID,
			getStringValue(event.Data, "chainID"),
			getStringValue(event.Data, "ip"),
			getStringValue(event.Data, "playerController"),
			getStringValue(event.Data, "eosId"),
			time.Now(),
		)
	}

	query += strings.Join(values, ",")
	return i.client.Exec(i.ctx, query, args...)
}

func (i *EventIngester) ingestPlayerDamaged(events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_player_damaged_events 
		(event_time, server_id, chain_id, victim_name, damage, attacker_name, attacker_controller, weapon, attacker_eos, attacker_steam, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*11)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			event.EventTime,
			event.ServerID,
			getStringValue(event.Data, "chainID"),
			getStringValue(event.Data, "victimName"),
			getFloat32Value(event.Data, "damage"),
			getStringValue(event.Data, "attackerName"),
			getStringValue(event.Data, "attackerController"),
			getStringValue(event.Data, "weapon"),
			getStringValue(event.Data, "attackerEos"),
			getStringValue(event.Data, "attackerSteam"),
			time.Now(),
		)
	}

	query += strings.Join(values, ",")
	return i.client.Exec(i.ctx, query, args...)
}

func (i *EventIngester) ingestPlayerDied(events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_player_died_events 
		(event_time, wound_time, server_id, chain_id, victim_name, damage, attacker_player_controller, weapon, attacker_eos, attacker_steam, teamkill, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*12)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

		woundTime := getTimeValue(event.Data, "woundTime")
		var woundTimePtr *time.Time
		if !woundTime.IsZero() {
			woundTimePtr = &woundTime
		}

		teamkill := uint8(0)
		if tk, ok := event.Data["teamkill"].(bool); ok && tk {
			teamkill = 1
		}

		args = append(args,
			event.EventTime,
			woundTimePtr,
			event.ServerID,
			getStringValue(event.Data, "chainID"),
			getStringValue(event.Data, "victimName"),
			getFloat32Value(event.Data, "damage"),
			getStringValue(event.Data, "attackerPlayerController"),
			getStringValue(event.Data, "weapon"),
			getStringValue(event.Data, "attackerEos"),
			getStringValue(event.Data, "attackerSteam"),
			teamkill,
			time.Now(),
		)
	}

	query += strings.Join(values, ",")
	return i.client.Exec(i.ctx, query, args...)
}

func (i *EventIngester) ingestPlayerWounded(events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_player_wounded_events 
		(event_time, server_id, chain_id, victim_name, damage, attacker_player_controller, weapon, attacker_eos, attacker_steam, teamkill, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*11)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

		teamkill := uint8(0)
		if tk, ok := event.Data["teamkill"].(bool); ok && tk {
			teamkill = 1
		}

		args = append(args,
			event.EventTime,
			event.ServerID,
			getStringValue(event.Data, "chainID"),
			getStringValue(event.Data, "victimName"),
			getFloat32Value(event.Data, "damage"),
			getStringValue(event.Data, "attackerPlayerController"),
			getStringValue(event.Data, "weapon"),
			getStringValue(event.Data, "attackerEos"),
			getStringValue(event.Data, "attackerSteam"),
			teamkill,
			time.Now(),
		)
	}

	query += strings.Join(values, ",")
	return i.client.Exec(i.ctx, query, args...)
}

func (i *EventIngester) ingestPlayerRevived(events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_player_revived_events 
		(event_time, server_id, chain_id, reviver_name, victim_name, reviver_eos, reviver_steam, victim_eos, victim_steam, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*10)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			event.EventTime,
			event.ServerID,
			getStringValue(event.Data, "chainID"),
			getStringValue(event.Data, "reviverName"),
			getStringValue(event.Data, "victimName"),
			getStringValue(event.Data, "reviverEos"),
			getStringValue(event.Data, "reviverSteam"),
			getStringValue(event.Data, "victimEos"),
			getStringValue(event.Data, "victimSteam"),
			time.Now(),
		)
	}

	query += strings.Join(values, ",")
	return i.client.Exec(i.ctx, query, args...)
}

func (i *EventIngester) ingestNewGame(events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_new_game_events 
		(event_time, server_id, chain_id, team, subfaction, faction, action, tickets, layer, level, dlc, map_classname, layer_classname, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*14)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			event.EventTime,
			event.ServerID,
			getStringValue(event.Data, "chainID"),
			getStringValue(event.Data, "team"),
			getStringValue(event.Data, "subfaction"),
			getStringValue(event.Data, "faction"),
			getStringValue(event.Data, "action"),
			getStringValue(event.Data, "tickets"),
			getStringValue(event.Data, "layer"),
			getStringValue(event.Data, "level"),
			getStringValue(event.Data, "dlc"),
			getStringValue(event.Data, "mapClassname"),
			getStringValue(event.Data, "layerClassname"),
			time.Now(),
		)
	}

	query += strings.Join(values, ",")
	return i.client.Exec(i.ctx, query, args...)
}

func (i *EventIngester) ingestRoundEnded(events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_round_ended_events 
		(event_time, server_id, chain_id, winner, layer, winner_json, loser_json, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*8)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?)")

		// Convert winner/loser data to JSON strings if they exist
		var winnerJSON, loserJSON string
		if winner, ok := event.Data["winner"]; ok && winner != nil {
			if winnerBytes, err := json.Marshal(winner); err == nil {
				winnerJSON = string(winnerBytes)
			}
		}
		if loser, ok := event.Data["loser"]; ok && loser != nil {
			if loserBytes, err := json.Marshal(loser); err == nil {
				loserJSON = string(loserBytes)
			}
		}

		args = append(args,
			event.EventTime,
			event.ServerID,
			getStringValue(event.Data, "chainID"),
			getStringValue(event.Data, "winner"),
			getStringValue(event.Data, "layer"),
			winnerJSON,
			loserJSON,
			time.Now(),
		)
	}

	query += strings.Join(values, ",")
	return i.client.Exec(i.ctx, query, args...)
}

func (i *EventIngester) ingestPlayerSquadChange(events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_player_squad_change_events 
		(event_time, server_id, chain_id, name, team_id, squad_id, old_team_id, old_squad_id, player_eos, player_steam, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*11)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			event.EventTime,
			event.ServerID,
			getStringValue(event.Data, "chainID"),
			getStringValue(event.Data, "name"),
			getStringValue(event.Data, "teamId"),
			getStringValue(event.Data, "squadId"),
			getStringValue(event.Data, "oldTeamId"),
			getStringValue(event.Data, "oldSquadId"),
			getStringValue(event.Data, "playerEos"),
			getStringValue(event.Data, "playerSteam"),
			time.Now(),
		)
	}

	query += strings.Join(values, ",")
	return i.client.Exec(i.ctx, query, args...)
}

func (i *EventIngester) ingestPlayerTeamChange(events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_player_team_change_events 
		(event_time, server_id, chain_id, name, new_team_id, old_team_id, player_eos, player_steam, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*9)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			event.EventTime,
			event.ServerID,
			getStringValue(event.Data, "chainID"),
			getStringValue(event.Data, "name"),
			getStringValue(event.Data, "newTeamId"),
			getStringValue(event.Data, "oldTeamId"),
			getStringValue(event.Data, "playerEos"),
			getStringValue(event.Data, "playerSteam"),
			time.Now(),
		)
	}

	query += strings.Join(values, ",")
	return i.client.Exec(i.ctx, query, args...)
}

func (i *EventIngester) ingestPlayerPossess(events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_player_possess_events 
		(event_time, server_id, chain_id, player_suffix, possess_classname, player_eos, player_steam, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*8)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			event.EventTime,
			event.ServerID,
			getStringValue(event.Data, "chainID"),
			getStringValue(event.Data, "playerSuffix"),
			getStringValue(event.Data, "possessClassname"),
			getStringValue(event.Data, "playerEos"),
			getStringValue(event.Data, "playerSteam"),
			time.Now(),
		)
	}

	query += strings.Join(values, ",")
	return i.client.Exec(i.ctx, query, args...)
}

func (i *EventIngester) ingestJoinSucceeded(events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_join_succeeded_events 
		(event_time, server_id, chain_id, player_suffix, ip, steam, eos, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*8)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			event.EventTime,
			event.ServerID,
			getStringValue(event.Data, "chainID"),
			getStringValue(event.Data, "playerSuffix"),
			getStringValue(event.Data, "ip"),
			getStringValue(event.Data, "steam"),
			getStringValue(event.Data, "eos"),
			time.Now(),
		)
	}

	query += strings.Join(values, ",")
	return i.client.Exec(i.ctx, query, args...)
}

func (i *EventIngester) ingestAdminBroadcast(events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_admin_broadcast_events 
		(event_time, server_id, chain_id, message, from_user, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*6)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?)")
		args = append(args,
			event.EventTime,
			event.ServerID,
			getStringValue(event.Data, "chainID"),
			getStringValue(event.Data, "message"),
			getStringValue(event.Data, "fromUser"),
			time.Now(),
		)
	}

	query += strings.Join(values, ",")
	return i.client.Exec(i.ctx, query, args...)
}

func (i *EventIngester) ingestDeployableDamaged(events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_deployable_damaged_events 
		(event_time, server_id, chain_id, deployable, damage, weapon, player_suffix, damage_type, health_remaining, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*10)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			event.EventTime,
			event.ServerID,
			getStringValue(event.Data, "chainID"),
			getStringValue(event.Data, "deployable"),
			getFloat32Value(event.Data, "damage"),
			getStringValue(event.Data, "weapon"),
			getStringValue(event.Data, "playerSuffix"),
			getStringValue(event.Data, "damageType"),
			getFloat32Value(event.Data, "healthRemaining"),
			time.Now(),
		)
	}

	query += strings.Join(values, ",")
	return i.client.Exec(i.ctx, query, args...)
}

func (i *EventIngester) ingestTickRate(events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_tick_rate_events 
		(event_time, server_id, chain_id, tick_rate, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*5)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?)")
		args = append(args,
			event.EventTime,
			event.ServerID,
			getStringValue(event.Data, "chainID"),
			getFloat32Value(event.Data, "tickRate"),
			time.Now(),
		)
	}

	query += strings.Join(values, ",")
	return i.client.Exec(i.ctx, query, args...)
}
