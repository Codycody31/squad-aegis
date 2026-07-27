package clickhouse

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
)

// EventIngester handles ingesting events from the event manager into ClickHouse
type EventIngester struct {
	client         *Client
	ctx            context.Context
	cancel         context.CancelFunc
	writeCtx       context.Context
	writeCancel    context.CancelFunc
	eventManager   *event_manager.EventManager
	subscriber     *event_manager.EventSubscriber
	batchSize      int
	flushInterval  time.Duration
	retryBackoffs  [3]time.Duration
	eventQueue     chan *IngestEvent
	wg             sync.WaitGroup
	stopOnce       sync.Once
	bufferedRows   atomic.Int64
	batchRows      atomic.Uint64
	flushFailures  atomic.Uint64
	retryCount     atomic.Uint64
	failedRows     atomic.Uint64
	invalidRows    atomic.Uint64
	degraded       atomic.Bool
	ingestOverride func(context.Context, event_manager.EventType, []*IngestEvent) error
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

// IngesterStats is a lock-free snapshot suitable for health endpoints.
type IngesterStats struct {
	QueueDepth    int    `json:"queue_depth"`
	QueueCapacity int    `json:"queue_capacity"`
	BufferedRows  int64  `json:"buffered_rows"`
	BatchRows     uint64 `json:"batch_rows"`
	FlushFailures uint64 `json:"flush_failures"`
	RetryCount    uint64 `json:"retry_count"`
	FailedRows    uint64 `json:"failed_rows"`
	InvalidRows   uint64 `json:"invalid_rows"`
	Degraded      bool   `json:"degraded"`
}

var errUnsupportedEventType = errors.New("unsupported ClickHouse ingestion event type")

// Helper functions for safe type assertions and conversions
func parseFloat32(s string) float32 {
	if f, err := strconv.ParseFloat(s, 32); err == nil {
		return float32(f)
	}
	return 0
}

func parseUint64(s string) uint64 {
	value, _ := strconv.ParseUint(s, 10, 64)
	return value
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
	writeCtx, writeCancel := context.WithCancel(context.WithoutCancel(ctx))

	ingester := &EventIngester{
		client:        client,
		ctx:           ctx,
		cancel:        cancel,
		writeCtx:      writeCtx,
		writeCancel:   writeCancel,
		eventManager:  eventManager,
		batchSize:     1000,
		flushInterval: 5 * time.Second,
		retryBackoffs: [3]time.Duration{
			100 * time.Millisecond,
			250 * time.Millisecond,
			500 * time.Millisecond,
		},
		eventQueue: make(chan *IngestEvent, 10000),
	}

	return ingester
}

// Start begins the event ingestion process
func (i *EventIngester) Start() {
	log.Info().Msg("Starting ClickHouse event ingester")

	// Subscribe to events from the event manager
	filter := event_manager.EventFilter{} // Subscribe to all events
	i.subscriber = i.eventManager.Subscribe(filter, nil, 10000)
	eventChan := i.subscriber.Channel

	i.wg.Add(2)

	// Event consumer goroutine
	go func() {
		defer i.wg.Done()
		defer close(i.eventQueue)
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
	i.stopOnce.Do(func() {
		log.Info().Msg("Stopping ClickHouse event ingester")

		if i.subscriber != nil {
			i.eventManager.Unsubscribe(i.subscriber.ID)
		}

		// The consumer owns eventQueue closure. The batch processor drains the
		// closed queue and flushes every valid per-type buffer before returning.
		i.cancel()
		i.wg.Wait()
		i.writeCancel()
	})
}

// processEvent converts an event manager event to an ingest event
func (i *EventIngester) processEvent(event event_manager.Event) {
	if !isSupportedEventType(event.Type) {
		i.invalidRows.Add(1)
		return
	}

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
	}
}

// batchProcessor processes events in batches
func (i *EventIngester) batchProcessor() {
	ticker := time.NewTicker(i.flushInterval)
	defer ticker.Stop()

	buffers := make(map[event_manager.EventType][]*IngestEvent)

	flush := func(eventType event_manager.EventType) {
		events := buffers[eventType]
		if len(events) == 0 {
			return
		}
		delete(buffers, eventType)
		i.bufferedRows.Add(-int64(len(events)))
		i.flushEventType(eventType, events)
	}

	flushAll := func() {
		for eventType := range buffers {
			flush(eventType)
		}
	}

	for {
		select {
		case <-ticker.C:
			flushAll()

		case event, ok := <-i.eventQueue:
			if !ok {
				flushAll()
				return
			}

			buffers[event.EventType] = append(buffers[event.EventType], event)
			i.bufferedRows.Add(1)
			if len(buffers[event.EventType]) >= i.batchSize {
				flush(event.EventType)
			}
		}
	}
}

func (i *EventIngester) flushEventType(eventType event_manager.EventType, events []*IngestEvent) {
	const maxAttempts = 4 // first attempt plus three retries

	var err error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(i.writeCtx, 30*time.Second)
		if i.ingestOverride != nil {
			err = i.ingestOverride(attemptCtx, eventType, events)
		} else {
			err = i.ingestEventType(attemptCtx, eventType, events)
		}
		cancel()
		if err == nil {
			i.batchRows.Add(uint64(len(events)))
			return
		}

		if attempt == maxAttempts-1 {
			break
		}

		i.retryCount.Add(1)
		timer := time.NewTimer(i.retryBackoffs[attempt])
		select {
		case <-timer.C:
		case <-i.writeCtx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			err = i.writeCtx.Err()
			attempt = maxAttempts - 1
		}
	}

	i.flushFailures.Add(1)
	i.failedRows.Add(uint64(len(events)))
	i.degraded.Store(true)
	log.Error().
		Err(err).
		Str("eventType", string(eventType)).
		Int("count", len(events)).
		Msg("ClickHouse event batch failed after retries")
}

// ingestEventType ingests events of a specific type
func (i *EventIngester) ingestEventType(ctx context.Context, eventType event_manager.EventType, events []*IngestEvent) error {
	switch eventType {
	case event_manager.EventTypeRconChatMessage:
		return i.ingestChatMessages(ctx, events)
	case event_manager.EventTypeRconServerInfo:
		return i.ingestServerInfo(ctx, events)
	case event_manager.EventTypeLogPlayerConnected:
		return i.ingestPlayerConnected(ctx, events)
	case event_manager.EventTypeLogPlayerDisconnected:
		return i.ingestPlayerDisconnected(ctx, events)
	case event_manager.EventTypeLogPlayerDamaged:
		return i.ingestPlayerDamaged(ctx, events)
	case event_manager.EventTypeLogPlayerDied:
		return i.ingestPlayerDied(ctx, events)
	case event_manager.EventTypeLogPlayerWounded:
		return i.ingestPlayerWounded(ctx, events)
	case event_manager.EventTypeLogPlayerRevived:
		return i.ingestPlayerRevived(ctx, events)
	case event_manager.EventTypeLogPlayerPossess:
		return i.ingestPlayerPossess(ctx, events)
	case event_manager.EventTypeLogJoinSucceeded:
		return i.ingestJoinSucceeded(ctx, events)
	case event_manager.EventTypeLogAdminBroadcast:
		return i.ingestAdminBroadcast(ctx, events)
	case event_manager.EventTypeLogDeployableDamaged:
		return i.ingestDeployableDamaged(ctx, events)
	case event_manager.EventTypeLogTickRate:
		return i.ingestTickRate(ctx, events)
	case event_manager.EventTypeLogGameEventUnified:
		return i.ingestGameEventUnified(ctx, events)
	default:
		return errUnsupportedEventType
	}
}

func isSupportedEventType(eventType event_manager.EventType) bool {
	switch eventType {
	case event_manager.EventTypeRconChatMessage,
		event_manager.EventTypeRconServerInfo,
		event_manager.EventTypeLogPlayerConnected,
		event_manager.EventTypeLogPlayerDisconnected,
		event_manager.EventTypeLogPlayerDamaged,
		event_manager.EventTypeLogPlayerDied,
		event_manager.EventTypeLogPlayerWounded,
		event_manager.EventTypeLogPlayerRevived,
		event_manager.EventTypeLogPlayerPossess,
		event_manager.EventTypeLogJoinSucceeded,
		event_manager.EventTypeLogAdminBroadcast,
		event_manager.EventTypeLogDeployableDamaged,
		event_manager.EventTypeLogTickRate,
		event_manager.EventTypeLogGameEventUnified:
		return true
	default:
		return false
	}
}

// Stats returns the current ingestion queue, batching, and failure state.
func (i *EventIngester) Stats() IngesterStats {
	return IngesterStats{
		QueueDepth:    len(i.eventQueue),
		QueueCapacity: cap(i.eventQueue),
		BufferedRows:  i.bufferedRows.Load(),
		BatchRows:     i.batchRows.Load(),
		FlushFailures: i.flushFailures.Load(),
		RetryCount:    i.retryCount.Load(),
		FailedRows:    i.failedRows.Load(),
		InvalidRows:   i.invalidRows.Load(),
		Degraded:      i.degraded.Load(),
	}
}

func (i *EventIngester) invalidPayload() {
	i.invalidRows.Add(1)
}

func (i *EventIngester) sendRows(ctx context.Context, query string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}

	batch, err := i.client.PrepareBatch(ctx, query)
	if err != nil {
		return err
	}

	for _, row := range rows {
		if err := batch.Append(row...); err != nil {
			_ = batch.Abort()
			return err
		}
	}

	return batch.Send()
}

// Specific ingestion functions for each event type
func (i *EventIngester) ingestChatMessages(ctx context.Context, events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_player_chat_messages 
		(message_id, server_id, steam_id, eos_id, player_name, sent_at, chat_type, message, ingest_ts)`

	rows := make([][]any, 0, len(events))

	for _, event := range events {
		chatData, ok := event.Data.(*event_manager.RconChatMessageData)
		if !ok {
			i.invalidPayload()
			continue
		}

		rows = append(rows, []any{
			event.EventID,
			event.ServerID,
			parseUint64(chatData.SteamID),
			chatData.EosID,
			chatData.PlayerName,
			event.EventTime,
			chatData.ChatType,
			chatData.Message,
			time.Now(),
		})
	}

	return i.sendRows(ctx, query, rows)
}

func (i *EventIngester) ingestServerInfo(ctx context.Context, events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_info_metrics 
		(event_time, server_id, player_count, public_queue, reserved_queue, ingested_at)`

	rows := make([][]any, 0, len(events))

	for _, event := range events {
		serverInfoData, ok := event.Data.(*event_manager.RconServerInfoData)
		if !ok {
			i.invalidPayload()
			continue
		}

		rows = append(rows, []any{
			event.EventTime,
			event.ServerID,
			serverInfoData.PlayerCount,
			serverInfoData.PublicQueue,
			serverInfoData.ReservedQueue,
			time.Now(),
		})
	}

	return i.sendRows(ctx, query, rows)
}

func (i *EventIngester) ingestPlayerConnected(ctx context.Context, events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_player_connected_events
		(id, event_time, server_id, chain_id, player_controller, ip, steam, eos, epic, player_suffix, ingested_at)`

	rows := make([][]any, 0, len(events))

	for _, event := range events {
		connectedData, ok := event.Data.(*event_manager.LogPlayerConnectedData)
		if !ok {
			i.invalidPayload()
			continue
		}

		rows = append(rows, []any{
			event.EventID,
			event.EventTime,
			event.ServerID,
			connectedData.ChainID,
			connectedData.PlayerController,
			connectedData.IPAddress,
			connectedData.SteamID,
			connectedData.EOSID,
			connectedData.EpicID,
			connectedData.PlayerSuffix,
			time.Now(),
		})
	}

	return i.sendRows(ctx, query, rows)
}

func (i *EventIngester) ingestPlayerDisconnected(ctx context.Context, events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_player_disconnected_events
		(id, event_time, server_id, chain_id, player_controller, player_suffix, team, ip, steam, eos, epic, ingest_ts)`

	rows := make([][]any, 0, len(events))

	for _, event := range events {
		disconnectedData, ok := event.Data.(*event_manager.LogPlayerDisconnectedData)
		if !ok {
			i.invalidPayload()
			continue
		}

		rows = append(rows, []any{
			event.EventID,
			event.EventTime,
			event.ServerID,
			disconnectedData.ChainID,
			disconnectedData.PlayerController,
			disconnectedData.PlayerSuffix,
			disconnectedData.TeamID,
			disconnectedData.IP,
			disconnectedData.SteamID,
			disconnectedData.EOSID,
			disconnectedData.EpicID,
			time.Now(),
		})
	}

	return i.sendRows(ctx, query, rows)
}

func (i *EventIngester) ingestPlayerDamaged(ctx context.Context, events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_player_damaged_events
		(id, event_time, server_id, chain_id, victim_name, victim_eos, victim_steam, victim_team, victim_squad, damage, attacker_name, attacker_eos, attacker_steam, attacker_team, attacker_squad, attacker_controller, weapon, teamkill, ingested_at)`

	rows := make([][]any, 0, len(events))

	for _, event := range events {
		// Extract data from structured event data
		var chainID, victimName, attackerName, attackerController, weapon, attackerEOS, attackerSteam string
		var victimEOS, victimSteam, victimTeam, victimSquad, attackerTeam, attackerSquad string
		var damage float32
		var teamkill uint8

		damagedData, ok := event.Data.(*event_manager.LogPlayerDamagedData)
		if !ok {
			i.invalidPayload()
			continue
		}
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

		if damagedData.Victim != nil {
			victimEOS = damagedData.Victim.EOSID
			victimSteam = damagedData.Victim.SteamID
			victimTeam = damagedData.Victim.TeamID
			victimSquad = damagedData.Victim.SquadID
		}

		if damagedData.Attacker != nil {
			attackerTeam = damagedData.Attacker.TeamID
			attackerSquad = damagedData.Attacker.SquadID
		}

		rows = append(rows, []any{
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
		})
	}

	return i.sendRows(ctx, query, rows)
}

func (i *EventIngester) ingestPlayerDied(ctx context.Context, events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_player_died_events
		(id, event_time, wound_time, server_id, chain_id, victim_name, victim_eos, victim_steam, victim_team, victim_squad, damage, attacker_name, attacker_eos, attacker_steam, attacker_team, attacker_squad, attacker_player_controller, weapon, teamkill, ingested_at)`

	rows := make([][]any, 0, len(events))

	for _, event := range events {
		// Extract data from structured event data
		var chainID, victimName, attackerPlayerController, weapon, attackerEOS, attackerSteam, woundTimeStr string
		var victimEOS, victimSteam, victimTeam, victimSquad, attackerName, attackerTeam, attackerSquad string
		var damage float32
		var teamkill uint8
		var woundTimePtr *time.Time

		diedData, ok := event.Data.(*event_manager.LogPlayerDiedData)
		if !ok {
			i.invalidPayload()
			continue
		}
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

		if diedData.Victim != nil {
			victimEOS = diedData.Victim.EOSID
			victimSteam = diedData.Victim.SteamID
			victimTeam = diedData.Victim.TeamID
			victimSquad = diedData.Victim.SquadID
		}

		if diedData.Attacker != nil {
			attackerName = diedData.Attacker.PlayerSuffix
			attackerTeam = diedData.Attacker.TeamID
			attackerSquad = diedData.Attacker.SquadID
		}

		// Parse wound time if available
		if woundTimeStr != "" {
			woundTime := parseTime(woundTimeStr)
			if !woundTime.IsZero() {
				woundTimePtr = &woundTime
			}
		}

		rows = append(rows, []any{
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
		})
	}

	return i.sendRows(ctx, query, rows)
}

func (i *EventIngester) ingestPlayerWounded(ctx context.Context, events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_player_wounded_events
		(id, event_time, server_id, chain_id, victim_name, victim_eos, victim_steam, victim_team, victim_squad, damage, attacker_name, attacker_eos, attacker_steam, attacker_team, attacker_squad, attacker_player_controller, weapon, teamkill, ingested_at)`

	rows := make([][]any, 0, len(events))

	for _, event := range events {
		// Extract data from structured event data
		var chainID, victimName, attackerPlayerController, weapon, attackerEOS, attackerSteam string
		var victimEOS, victimSteam, victimTeam, victimSquad, attackerName, attackerTeam, attackerSquad string
		var damage float32
		var teamkill uint8

		woundedData, ok := event.Data.(*event_manager.LogPlayerWoundedData)
		if !ok {
			i.invalidPayload()
			continue
		}
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

		if woundedData.Victim != nil {
			victimEOS = woundedData.Victim.EOSID
			victimSteam = woundedData.Victim.SteamID
			victimTeam = woundedData.Victim.TeamID
			victimSquad = woundedData.Victim.SquadID
		}

		if woundedData.Attacker != nil {
			attackerName = woundedData.Attacker.PlayerSuffix
			attackerTeam = woundedData.Attacker.TeamID
			attackerSquad = woundedData.Attacker.SquadID
		}

		rows = append(rows, []any{
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
		})
	}

	return i.sendRows(ctx, query, rows)
}

func (i *EventIngester) ingestPlayerRevived(ctx context.Context, events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_player_revived_events
		(id, event_time, server_id, chain_id, reviver_name, reviver_eos, reviver_steam, reviver_team, reviver_squad, victim_name, victim_eos, victim_steam, victim_team, victim_squad, ingested_at)`

	rows := make([][]any, 0, len(events))

	for _, event := range events {
		// Extract data from structured event data
		var chainID, reviverName, victimName, reviverEOS, reviverSteam, victimEOS, victimSteam string
		var reviverTeam, reviverSquad, victimTeam, victimSquad string

		revivedData, ok := event.Data.(*event_manager.LogPlayerRevivedData)
		if !ok {
			i.invalidPayload()
			continue
		}
		chainID = revivedData.ChainID
		reviverName = revivedData.ReviverName
		victimName = revivedData.VictimName
		reviverEOS = revivedData.ReviverEOS
		reviverSteam = revivedData.ReviverSteam
		victimEOS = revivedData.VictimEOS
		victimSteam = revivedData.VictimSteam

		if revivedData.Reviver != nil {
			reviverTeam = revivedData.Reviver.TeamID
			reviverSquad = revivedData.Reviver.SquadID
		}

		if revivedData.Victim != nil {
			victimTeam = revivedData.Victim.TeamID
			victimSquad = revivedData.Victim.SquadID
		}

		rows = append(rows, []any{
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
		})
	}

	return i.sendRows(ctx, query, rows)
}

func (i *EventIngester) ingestPlayerPossess(ctx context.Context, events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_player_possess_events
		(id, event_time, server_id, chain_id, player_suffix, possess_classname, player_eos, player_steam, player_epic, ingested_at)`

	rows := make([][]any, 0, len(events))

	for _, event := range events {
		possessData, ok := event.Data.(*event_manager.LogPlayerPossessData)
		if !ok {
			i.invalidPayload()
			continue
		}

		rows = append(rows, []any{
			event.EventID,
			event.EventTime,
			event.ServerID,
			possessData.ChainID,
			possessData.PlayerSuffix,
			possessData.PossessClassname,
			possessData.PlayerEOS,
			possessData.PlayerSteam,
			possessData.PlayerEpic,
			time.Now(),
		})
	}

	return i.sendRows(ctx, query, rows)
}

func (i *EventIngester) ingestJoinSucceeded(ctx context.Context, events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_join_succeeded_events
		(id, event_time, server_id, chain_id, player_suffix, ip, steam, eos, epic, ingested_at)`

	rows := make([][]any, 0, len(events))

	for _, event := range events {
		joinData, ok := event.Data.(*event_manager.LogJoinSucceededData)
		if !ok {
			i.invalidPayload()
			continue
		}

		rows = append(rows, []any{
			event.EventID,
			event.EventTime,
			event.ServerID,
			joinData.ChainID,
			joinData.PlayerSuffix,
			joinData.IPAddress,
			joinData.SteamID,
			joinData.EOSID,
			joinData.EpicID,
			time.Now(),
		})
	}

	return i.sendRows(ctx, query, rows)
}

func (i *EventIngester) ingestAdminBroadcast(ctx context.Context, events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_admin_broadcast_events 
		(event_time, server_id, chain_id, message, from_user, ingested_at)`

	rows := make([][]any, 0, len(events))

	for _, event := range events {
		broadcastData, ok := event.Data.(*event_manager.LogAdminBroadcastData)
		if !ok {
			i.invalidPayload()
			continue
		}

		rows = append(rows, []any{
			event.EventTime,
			event.ServerID,
			broadcastData.ChainID,
			broadcastData.Message,
			broadcastData.From,
			time.Now(),
		})
	}

	return i.sendRows(ctx, query, rows)
}

func (i *EventIngester) ingestDeployableDamaged(ctx context.Context, events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_deployable_damaged_events
		(id, event_time, server_id, chain_id, deployable, damage, weapon, player_suffix, damage_type, health_remaining, ingested_at)`

	rows := make([][]any, 0, len(events))

	for _, event := range events {
		deployableDamagedData, ok := event.Data.(*event_manager.LogDeployableDamagedData)
		if !ok {
			i.invalidPayload()
			continue
		}

		rows = append(rows, []any{
			event.EventID,
			event.EventTime,
			event.ServerID,
			deployableDamagedData.ChainID,
			deployableDamagedData.Deployable,
			parseFloat32(deployableDamagedData.Damage),
			deployableDamagedData.Weapon,
			deployableDamagedData.PlayerSuffix,
			deployableDamagedData.DamageType,
			parseFloat32(deployableDamagedData.HealthRemaining),
			time.Now(),
		})
	}

	return i.sendRows(ctx, query, rows)
}

func (i *EventIngester) ingestTickRate(ctx context.Context, events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_tick_rate_events 
		(event_time, server_id, chain_id, tick_rate, ingested_at)`

	rows := make([][]any, 0, len(events))

	for _, event := range events {
		tickRateData, ok := event.Data.(*event_manager.LogTickRateData)
		if !ok {
			i.invalidPayload()
			continue
		}

		rows = append(rows, []any{
			event.EventTime,
			event.ServerID,
			tickRateData.ChainID,
			parseFloat32(tickRateData.TickRate),
			time.Now(),
		})
	}

	return i.sendRows(ctx, query, rows)
}

// ingestGameEventUnified ingests unified game events into ClickHouse
func (i *EventIngester) ingestGameEventUnified(ctx context.Context, events []*IngestEvent) error {
	if len(events) == 0 {
		return nil
	}

	query := `INSERT INTO squad_aegis.server_game_events_unified 
		(event_time, server_id, chain_id, event_type, winner, layer, team, subfaction, faction, action, tickets, level, dlc, map_classname, layer_classname, from_state, to_state, winner_data, loser_data, metadata, raw_log, ingested_at)`

	rows := make([][]any, 0, len(events))

	for _, event := range events {
		unifiedData, ok := event.Data.(*event_manager.LogGameEventUnifiedData)
		if !ok {
			i.invalidPayload()
			continue
		}

		// Convert team to uint8 if present
		var teamNum *uint8
		if unifiedData.Team != "" {
			if teamVal, err := strconv.ParseUint(unifiedData.Team, 10, 8); err == nil {
				teamUint8 := uint8(teamVal)
				teamNum = &teamUint8
			}
		}

		// Convert tickets to uint32 if present
		var ticketsNum *uint32
		if unifiedData.Tickets != "" {
			if ticketsVal, err := strconv.ParseUint(unifiedData.Tickets, 10, 32); err == nil {
				ticketsUint32 := uint32(ticketsVal)
				ticketsNum = &ticketsUint32
			}
		}

		rows = append(rows, []any{
			event.EventTime,
			event.ServerID,
			unifiedData.ChainID,
			unifiedData.EventType,
			unifiedData.Winner,
			unifiedData.Layer,
			teamNum,
			unifiedData.Subfaction,
			unifiedData.Faction,
			unifiedData.Action,
			ticketsNum,
			unifiedData.Level,
			unifiedData.DLC,
			unifiedData.MapClassname,
			unifiedData.LayerClassname,
			unifiedData.FromState,
			unifiedData.ToState,
			unifiedData.WinnerData,
			unifiedData.LoserData,
			unifiedData.Metadata,
			unifiedData.RawLog,
			time.Now(),
		})
	}

	return i.sendRows(ctx, query, rows)
}
