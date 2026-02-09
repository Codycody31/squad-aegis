package clickhouse

import (
	"context"
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
	EventID   uuid.UUID
	ServerID  uuid.UUID
	EventType event_manager.EventType
	EventTime time.Time
	Data      event_manager.EventData
	RawData   interface{}
}

// Helper functions for safe type assertions and conversions
func parseFloat32(s string) float32 {
	if f, err := strconv.ParseFloat(s, 32); err == nil {
		return float32(f)
	}
	return 0
}

func parseTime(s string) time.Time {
	// Try different time formats that might be used in logs
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000Z",
		"2006.01.02-15.04.05:000",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t
		}
	}
	return time.Time{}
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
		EventID:   event.ID,
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
		// log.Warn().Msg("Event queue full, dropping event")
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
	case event_manager.EventTypeRconChatMessage:
		return i.ingestChatMessages(events)
	case event_manager.EventTypeRconServerInfo:
		return i.ingestServerInfo(events)
	// TODO: support ingesting rcon player warned event
	// TODO: support ingesting possessed and unpossessed admin camera
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
	case event_manager.EventTypeLogGameEventUnified:
		return i.ingestGameEventUnified(events)
	default:
		return nil
	}
}

// Specific ingestion functions for each event type
func (i *EventIngester) ingestChatMessages(events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_player_chat_messages 
		(message_id, server_id, steam_id, eos_id, player_name, sent_at, chat_type, message, ingest_ts) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*7)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")

		// Use the event ID as the message ID for consistent tracking
		messageID := event.EventID
		chatData, ok := event.Data.(*event_manager.RconChatMessageData)
		if !ok {
			continue
		}

		args = append(args,
			messageID,
			event.ServerID,
			chatData.SteamID,
			chatData.EosID,
			chatData.PlayerName,
			event.EventTime,
			chatData.ChatType,
			chatData.Message,
			time.Now(),
		)
	}

	query += strings.Join(values, ",")
	return i.client.Exec(i.ctx, query, args...)
}

func (i *EventIngester) ingestServerInfo(events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_info_metrics 
		(event_time, server_id, player_count, public_queue, reserved_queue, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*6)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?)")

		serverInfoData, ok := event.Data.(*event_manager.RconServerInfoData)
		if !ok {
			continue
		}

		args = append(args,
			event.EventTime,
			event.ServerID,
			serverInfoData.PlayerCount,
			serverInfoData.PublicQueue,
			serverInfoData.ReservedQueue,
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
		(id, event_time, server_id, chain_id, player_controller, ip, steam, eos, player_suffix, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*10)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

		// Extract data from structured event data
		var chainID, playerController, ip, steam, eos, playerSuffix string
		if connectedData, ok := event.Data.(*event_manager.LogPlayerConnectedData); ok {
			chainID = connectedData.ChainID
			playerController = connectedData.PlayerController
			ip = connectedData.IPAddress
			steam = connectedData.SteamID
			eos = connectedData.EOSID
			playerSuffix = connectedData.PlayerSuffix
		}

		args = append(args,
			event.EventID,
			event.EventTime,
			event.ServerID,
			chainID,
			playerController,
			ip,
			steam,
			eos,
			playerSuffix,
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
		(id, event_time, server_id, chain_id, player_controller, player_suffix, team, ip, steam, eos, ingest_ts) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*11)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

		// Extract data from structured event data
		var chainID, playerController, playerSuffix, team, ip, steam, eos string
		if disconnectedData, ok := event.Data.(*event_manager.LogPlayerDisconnectedData); ok {
			chainID = disconnectedData.ChainID
			playerController = disconnectedData.PlayerController
			playerSuffix = disconnectedData.PlayerSuffix
			team = disconnectedData.TeamID
			ip = disconnectedData.IP
			steam = disconnectedData.SteamID
			eos = disconnectedData.EOSID
		}

		args = append(args,
			event.EventID,
			event.EventTime,
			event.ServerID,
			chainID,
			playerController,
			playerSuffix,
			team,
			ip,
			steam,
			eos,
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
		(id, event_time, server_id, chain_id, victim_name, victim_eos, victim_steam, victim_team, victim_squad, damage, attacker_name, attacker_eos, attacker_steam, attacker_team, attacker_squad, attacker_controller, weapon, teamkill, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*19)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

		// Extract data from structured event data
		var chainID, victimName, attackerName, attackerController, weapon, attackerEOS, attackerSteam string
		var victimEOS, victimSteam, victimTeam, victimSquad, attackerTeam, attackerSquad string
		var damage float32
		var teamkill uint8

		if damagedData, ok := event.Data.(*event_manager.LogPlayerDamagedData); ok {
			chainID = damagedData.ChainID
			victimName = damagedData.VictimName
			attackerName = damagedData.AttackerName
			attackerController = damagedData.AttackerController
			weapon = damagedData.Weapon
			attackerEOS = damagedData.AttackerEOS
			attackerSteam = damagedData.AttackerSteam
			damage = parseFloat32(damagedData.Damage)
			if damagedData.Teamkill {
				teamkill = 1
			}

			// Extract victim details from PlayerInfo
			if damagedData.Victim != nil {
				victimEOS = damagedData.Victim.EOSID
				victimSteam = damagedData.Victim.SteamID
				victimTeam = damagedData.Victim.TeamID
				victimSquad = damagedData.Victim.SquadID
			}

			// Extract attacker details from PlayerInfo
			if damagedData.Attacker != nil {
				attackerTeam = damagedData.Attacker.TeamID
				attackerSquad = damagedData.Attacker.SquadID
			}
		}

		args = append(args,
			event.EventID,
			event.EventTime,
			event.ServerID,
			chainID,
			victimName,
			victimEOS,
			victimSteam,
			victimTeam,
			victimSquad,
			damage,
			attackerName,
			attackerEOS,
			attackerSteam,
			attackerTeam,
			attackerSquad,
			attackerController,
			weapon,
			teamkill,
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
		(id, event_time, wound_time, server_id, chain_id, victim_name, victim_eos, victim_steam, victim_team, victim_squad, damage, attacker_name, attacker_eos, attacker_steam, attacker_team, attacker_squad, attacker_player_controller, weapon, teamkill, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*20)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

		// Extract data from structured event data
		var chainID, victimName, attackerPlayerController, weapon, attackerEOS, attackerSteam, woundTimeStr string
		var victimEOS, victimSteam, victimTeam, victimSquad, attackerName, attackerTeam, attackerSquad string
		var damage float32
		var teamkill uint8
		var woundTimePtr *time.Time

		if diedData, ok := event.Data.(*event_manager.LogPlayerDiedData); ok {
			chainID = diedData.ChainID
			victimName = diedData.VictimName
			attackerPlayerController = diedData.AttackerPlayerController
			weapon = diedData.Weapon
			attackerEOS = diedData.AttackerEOS
			attackerSteam = diedData.AttackerSteam
			woundTimeStr = diedData.WoundTime
			damage = parseFloat32(diedData.Damage)
			if diedData.Teamkill {
				teamkill = 1
			}

			// Extract victim details from PlayerInfo
			if diedData.Victim != nil {
				victimEOS = diedData.Victim.EOSID
				victimSteam = diedData.Victim.SteamID
				victimTeam = diedData.Victim.TeamID
				victimSquad = diedData.Victim.SquadID
			}

			// Extract attacker details from PlayerInfo
			if diedData.Attacker != nil {
				attackerName = diedData.Attacker.PlayerSuffix
				attackerTeam = diedData.Attacker.TeamID
				attackerSquad = diedData.Attacker.SquadID
			}
		}

		// Parse wound time if available
		if woundTimeStr != "" {
			woundTime := parseTime(woundTimeStr)
			if !woundTime.IsZero() {
				woundTimePtr = &woundTime
			}
		}

		args = append(args,
			event.EventID,
			event.EventTime,
			woundTimePtr,
			event.ServerID,
			chainID,
			victimName,
			victimEOS,
			victimSteam,
			victimTeam,
			victimSquad,
			damage,
			attackerName,
			attackerEOS,
			attackerSteam,
			attackerTeam,
			attackerSquad,
			attackerPlayerController,
			weapon,
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
		(id, event_time, server_id, chain_id, victim_name, victim_eos, victim_steam, victim_team, victim_squad, damage, attacker_name, attacker_eos, attacker_steam, attacker_team, attacker_squad, attacker_player_controller, weapon, teamkill, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*19)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

		// Extract data from structured event data
		var chainID, victimName, attackerPlayerController, weapon, attackerEOS, attackerSteam string
		var victimEOS, victimSteam, victimTeam, victimSquad, attackerName, attackerTeam, attackerSquad string
		var damage float32
		var teamkill uint8

		if woundedData, ok := event.Data.(*event_manager.LogPlayerWoundedData); ok {
			chainID = woundedData.ChainID
			victimName = woundedData.VictimName
			attackerPlayerController = woundedData.AttackerPlayerController
			weapon = woundedData.Weapon
			attackerEOS = woundedData.AttackerEOS
			attackerSteam = woundedData.AttackerSteam
			damage = parseFloat32(woundedData.Damage)
			if woundedData.Teamkill {
				teamkill = 1
			}

			// Extract victim details from PlayerInfo
			if woundedData.Victim != nil {
				victimEOS = woundedData.Victim.EOSID
				victimSteam = woundedData.Victim.SteamID
				victimTeam = woundedData.Victim.TeamID
				victimSquad = woundedData.Victim.SquadID
			}

			// Extract attacker details from PlayerInfo
			if woundedData.Attacker != nil {
				attackerName = woundedData.Attacker.PlayerSuffix
				attackerTeam = woundedData.Attacker.TeamID
				attackerSquad = woundedData.Attacker.SquadID
			}
		}

		args = append(args,
			event.EventID,
			event.EventTime,
			event.ServerID,
			chainID,
			victimName,
			victimEOS,
			victimSteam,
			victimTeam,
			victimSquad,
			damage,
			attackerName,
			attackerEOS,
			attackerSteam,
			attackerTeam,
			attackerSquad,
			attackerPlayerController,
			weapon,
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
		(id, event_time, server_id, chain_id, reviver_name, reviver_eos, reviver_steam, reviver_team, reviver_squad, victim_name, victim_eos, victim_steam, victim_team, victim_squad, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*15)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

		// Extract data from structured event data
		var chainID, reviverName, victimName, reviverEOS, reviverSteam, victimEOS, victimSteam string
		var reviverTeam, reviverSquad, victimTeam, victimSquad string

		if revivedData, ok := event.Data.(*event_manager.LogPlayerRevivedData); ok {
			chainID = revivedData.ChainID
			reviverName = revivedData.ReviverName
			victimName = revivedData.VictimName
			reviverEOS = revivedData.ReviverEOS
			reviverSteam = revivedData.ReviverSteam
			victimEOS = revivedData.VictimEOS
			victimSteam = revivedData.VictimSteam

			// Extract reviver details from PlayerInfo
			if revivedData.Reviver != nil {
				reviverTeam = revivedData.Reviver.TeamID
				reviverSquad = revivedData.Reviver.SquadID
			}

			// Extract victim details from PlayerInfo
			if revivedData.Victim != nil {
				victimTeam = revivedData.Victim.TeamID
				victimSquad = revivedData.Victim.SquadID
			}
		}

		args = append(args,
			event.EventID,
			event.EventTime,
			event.ServerID,
			chainID,
			reviverName,
			reviverEOS,
			reviverSteam,
			reviverTeam,
			reviverSquad,
			victimName,
			victimEOS,
			victimSteam,
			victimTeam,
			victimSquad,
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
		(id, event_time, server_id, chain_id, player_suffix, possess_classname, player_eos, player_steam, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*9)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")

		// Extract data from structured event data
		var chainID, playerSuffix, possessClassname, playerEOS, playerSteam string
		if possessData, ok := event.Data.(*event_manager.LogPlayerPossessData); ok {
			chainID = possessData.ChainID
			playerSuffix = possessData.PlayerSuffix
			possessClassname = possessData.PossessClassname
			playerEOS = possessData.PlayerEOS
			playerSteam = possessData.PlayerSteam
		}

		args = append(args,
			event.EventID,
			event.EventTime,
			event.ServerID,
			chainID,
			playerSuffix,
			possessClassname,
			playerEOS,
			playerSteam,
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
		(id, event_time, server_id, chain_id, player_suffix, ip, steam, eos, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*9)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")

		// Extract data from structured event data
		var chainID, playerSuffix, ip, steam, eos string
		if joinData, ok := event.Data.(*event_manager.LogJoinSucceededData); ok {
			chainID = joinData.ChainID
			playerSuffix = joinData.PlayerSuffix
			ip = joinData.IPAddress
			steam = joinData.SteamID
			eos = joinData.EOSID
		}

		args = append(args,
			event.EventID,
			event.EventTime,
			event.ServerID,
			chainID,
			playerSuffix,
			ip,
			steam,
			eos,
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

		// Extract data from structured event data
		var chainID, message, fromUser string
		if broadcastData, ok := event.Data.(*event_manager.LogAdminBroadcastData); ok {
			chainID = broadcastData.ChainID
			message = broadcastData.Message
			fromUser = broadcastData.From
		}

		args = append(args,
			event.EventTime,
			event.ServerID,
			chainID,
			message,
			fromUser,
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
		(id, event_time, server_id, chain_id, deployable, damage, weapon, player_suffix, damage_type, health_remaining, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*11)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

		// Extract data from structured event data
		var chainID, deployable, weapon, playerSuffix, damageType string
		var damage, healthRemaining float32

		if deployableDamagedData, ok := event.Data.(*event_manager.LogDeployableDamagedData); ok {
			chainID = deployableDamagedData.ChainID
			deployable = deployableDamagedData.Deployable
			weapon = deployableDamagedData.Weapon
			playerSuffix = deployableDamagedData.PlayerSuffix
			damageType = deployableDamagedData.DamageType
			damage = parseFloat32(deployableDamagedData.Damage)
			healthRemaining = parseFloat32(deployableDamagedData.HealthRemaining)
		}

		args = append(args,
			event.EventID,
			event.EventTime,
			event.ServerID,
			chainID,
			deployable,
			damage,
			weapon,
			playerSuffix,
			damageType,
			healthRemaining,
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

		// Extract data from structured event data
		var chainID string
		var tickRate float32

		if tickRateData, ok := event.Data.(*event_manager.LogTickRateData); ok {
			chainID = tickRateData.ChainID
			tickRate = parseFloat32(tickRateData.TickRate)
		}

		args = append(args,
			event.EventTime,
			event.ServerID,
			chainID,
			tickRate,
			time.Now(),
		)
	}

	query += strings.Join(values, ",")
	return i.client.Exec(i.ctx, query, args...)
}

// ingestGameEventUnified ingests unified game events into ClickHouse
func (i *EventIngester) ingestGameEventUnified(events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_game_events_unified 
		(event_time, server_id, chain_id, event_type, winner, layer, team, subfaction, faction, action, tickets, level, dlc, map_classname, layer_classname, from_state, to_state, winner_data, loser_data, metadata, raw_log, ingested_at) VALUES`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*22)

	for _, event := range events {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

		// Extract data from structured event data
		var chainID, eventType, winner, layer, team, subfaction, faction, action, tickets, level, dlc, mapClassname, layerClassname, fromState, toState, winnerData, loserData, metadata, rawLog string

		if unifiedData, ok := event.Data.(*event_manager.LogGameEventUnifiedData); ok {
			chainID = unifiedData.ChainID
			eventType = unifiedData.EventType
			winner = unifiedData.Winner
			layer = unifiedData.Layer
			team = unifiedData.Team
			subfaction = unifiedData.Subfaction
			faction = unifiedData.Faction
			action = unifiedData.Action
			tickets = unifiedData.Tickets
			level = unifiedData.Level
			dlc = unifiedData.DLC
			mapClassname = unifiedData.MapClassname
			layerClassname = unifiedData.LayerClassname
			fromState = unifiedData.FromState
			toState = unifiedData.ToState
			winnerData = unifiedData.WinnerData
			loserData = unifiedData.LoserData
			metadata = unifiedData.Metadata
			rawLog = unifiedData.RawLog
		}

		// Convert team to uint8 if present
		var teamNum *uint8
		if team != "" {
			if teamVal, err := strconv.ParseUint(team, 10, 8); err == nil {
				teamUint8 := uint8(teamVal)
				teamNum = &teamUint8
			}
		}

		// Convert tickets to uint32 if present
		var ticketsNum *uint32
		if tickets != "" {
			if ticketsVal, err := strconv.ParseUint(tickets, 10, 32); err == nil {
				ticketsUint32 := uint32(ticketsVal)
				ticketsNum = &ticketsUint32
			}
		}

		args = append(args,
			event.EventTime,
			event.ServerID,
			chainID,
			eventType,
			winner,
			layer,
			teamNum,
			subfaction,
			faction,
			action,
			ticketsNum,
			level,
			dlc,
			mapClassname,
			layerClassname,
			fromState,
			toState,
			winnerData,
			loserData,
			metadata,
			rawLog,
			time.Now(),
		)
	}

	query += strings.Join(values, ",")
	return i.client.Exec(i.ctx, query, args...)
}
