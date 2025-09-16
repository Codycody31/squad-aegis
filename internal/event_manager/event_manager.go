package event_manager

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// EventType represents the type of event
type EventType string

const (
	EventTypeAll EventType = "*"

	// RCON Events
	EventTypeRconChatMessage            EventType = "RCON_CHAT_MESSAGE"
	EventTypeRconPlayerWarned           EventType = "RCON_PLAYER_WARNED"
	EventTypeRconPlayerKicked           EventType = "RCON_PLAYER_KICKED"
	EventTypeRconPlayerBanned           EventType = "RCON_PLAYER_BANNED"
	EventTypeRconPossessedAdminCamera   EventType = "RCON_POSSESSED_ADMIN_CAMERA"
	EventTypeRconUnpossessedAdminCamera EventType = "RCON_UNPOSSESSED_ADMIN_CAMERA"
	EventTypeRconSquadCreated           EventType = "RCON_SQUAD_CREATED"
	EventTypeRconServerInfo             EventType = "RCON_SERVER_INFO"

	// Log Events
	EventTypeLogAdminBroadcast     EventType = "LOG_ADMIN_BROADCAST"
	EventTypeLogDeployableDamaged  EventType = "LOG_DEPLOYABLE_DAMAGED"
	EventTypeLogPlayerConnected    EventType = "LOG_PLAYER_CONNECTED"
	EventTypeLogPlayerDamaged      EventType = "LOG_PLAYER_DAMAGED"
	EventTypeLogPlayerDied         EventType = "LOG_PLAYER_DIED"
	EventTypeLogPlayerWounded      EventType = "LOG_PLAYER_WOUNDED"
	EventTypeLogPlayerRevived      EventType = "LOG_PLAYER_REVIVED"
	EventTypeLogPlayerPossess      EventType = "LOG_PLAYER_POSSESS"
	EventTypeLogPlayerDisconnected EventType = "LOG_PLAYER_DISCONNECTED"
	EventTypeLogJoinSucceeded      EventType = "LOG_JOIN_SUCCEEDED"
	EventTypeLogTickRate           EventType = "LOG_TICK_RATE"
	EventTypeLogGameEventUnified   EventType = "LOG_GAME_EVENT_UNIFIED"
)

// Event represents a unified event from any source
type Event struct {
	ID        uuid.UUID   `json:"id"`
	ServerID  uuid.UUID   `json:"server_id"`
	Type      EventType   `json:"type"`
	Data      EventData   `json:"data"`               // Structured event data
	RawData   interface{} `json:"raw_data,omitempty"` // Original raw data for debugging
	Timestamp time.Time   `json:"timestamp"`
}

// EventSubscriber represents a subscriber to events
type EventSubscriber struct {
	ID       uuid.UUID
	Channel  chan Event
	Filter   EventFilter
	ServerID *uuid.UUID // If nil, subscribes to all servers
}

// EventFilter allows filtering events by type and other criteria
type EventFilter struct {
	Types     []EventType `json:"types,omitempty"`
	ServerIDs []uuid.UUID `json:"server_ids,omitempty"`
}

// EventManager manages the centralized event system
type EventManager struct {
	subscribers map[uuid.UUID]*EventSubscriber
	eventQueue  chan Event
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	bufferSize  int
}

// NewEventManager creates a new event manager
func NewEventManager(ctx context.Context, bufferSize int) *EventManager {
	ctx, cancel := context.WithCancel(ctx)

	if bufferSize <= 0 {
		bufferSize = 10000 // Default buffer size
	}

	em := &EventManager{
		subscribers: make(map[uuid.UUID]*EventSubscriber),
		eventQueue:  make(chan Event, bufferSize),
		ctx:         ctx,
		cancel:      cancel,
		bufferSize:  bufferSize,
	}

	// Start event processor
	go em.processEvents()

	return em
}

// Subscribe creates a new event subscription
func (em *EventManager) Subscribe(filter EventFilter, serverID *uuid.UUID, channelSize int) *EventSubscriber {
	em.mu.Lock()
	defer em.mu.Unlock()

	if channelSize <= 0 {
		channelSize = 100 // Default channel size
	}

	subscriber := &EventSubscriber{
		ID:       uuid.New(),
		Channel:  make(chan Event, channelSize),
		Filter:   filter,
		ServerID: serverID,
	}

	em.subscribers[subscriber.ID] = subscriber

	log.Debug().
		Str("subscriberID", subscriber.ID.String()).
		Interface("filter", filter).
		Msg("New event subscriber registered")

	return subscriber
}

// Unsubscribe removes an event subscription
func (em *EventManager) Unsubscribe(subscriberID uuid.UUID) {
	em.mu.Lock()
	defer em.mu.Unlock()

	if subscriber, exists := em.subscribers[subscriberID]; exists {
		close(subscriber.Channel)
		delete(em.subscribers, subscriberID)

		log.Debug().
			Str("subscriberID", subscriberID.String()).
			Msg("Event subscriber unregistered")
	}
}

// PublishEvent publishes an event to the event queue with structured data
func (em *EventManager) PublishEvent(serverID uuid.UUID, data EventData, rawData interface{}) {
	event := Event{
		ID:        uuid.New(),
		ServerID:  serverID,
		Type:      data.GetEventType(),
		Data:      data,
		RawData:   rawData,
		Timestamp: time.Now(),
	}

	select {
	case em.eventQueue <- event:
		// Event queued successfully
	default:
		// Queue is full, log warning and drop event
		log.Warn().
			Str("eventID", event.ID.String()).
			Str("serverID", serverID.String()).
			Str("eventType", string(event.Type)).
			Msg("Event queue full, dropping event")
	}
}

// processEvents processes events from the queue and distributes to subscribers
func (em *EventManager) processEvents() {
	log.Info().Msg("Event processor started")
	defer log.Info().Msg("Event processor stopped")

	for {
		select {
		case <-em.ctx.Done():
			return
		case event := <-em.eventQueue:
			em.distributeEvent(event)
		}
	}
}

// distributeEvent distributes an event to matching subscribers
func (em *EventManager) distributeEvent(event Event) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	for _, subscriber := range em.subscribers {
		if em.eventMatchesFilter(event, subscriber) {
			select {
			case subscriber.Channel <- event:
				// Event sent successfully
			default:
				// Subscriber channel is full, log warning
				log.Warn().
					Str("subscriberID", subscriber.ID.String()).
					Str("eventID", event.ID.String()).
					Str("eventType", string(event.Type)).
					Msg("Subscriber channel full, dropping event")
			}
		}
	}
}

// eventMatchesFilter checks if an event matches a subscriber's filter
func (em *EventManager) eventMatchesFilter(event Event, subscriber *EventSubscriber) bool {
	// Check server filter
	if subscriber.ServerID != nil && *subscriber.ServerID != event.ServerID {
		return false
	}

	// Check server IDs in filter
	if len(subscriber.Filter.ServerIDs) > 0 {
		found := false
		for _, serverID := range subscriber.Filter.ServerIDs {
			if serverID == event.ServerID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if (len(subscriber.Filter.Types) == 1 && subscriber.Filter.Types[0] == EventTypeAll) || len(subscriber.Filter.Types) == 0 {
		return true
	}

	// Check event types in filter
	if len(subscriber.Filter.Types) > 0 {
		found := false
		for _, eventType := range subscriber.Filter.Types {
			if eventType == event.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// GetEventStats returns statistics about the event system
func (em *EventManager) GetEventStats() map[string]interface{} {
	em.mu.RLock()
	defer em.mu.RUnlock()

	return map[string]interface{}{
		"subscribers":    len(em.subscribers),
		"queue_size":     len(em.eventQueue),
		"queue_capacity": cap(em.eventQueue),
		"buffer_size":    em.bufferSize,
	}
}

// Shutdown gracefully shuts down the event manager
func (em *EventManager) Shutdown() {
	log.Info().Msg("Shutting down event manager...")

	em.cancel()

	// Close all subscriber channels
	em.mu.Lock()
	for _, subscriber := range em.subscribers {
		close(subscriber.Channel)
	}
	em.subscribers = make(map[uuid.UUID]*EventSubscriber)
	em.mu.Unlock()

	log.Info().Msg("Event manager shutdown complete")
}

// ConvertToJSON converts event data to JSON string
func (e *Event) ConvertToJSON() (string, error) {
	jsonBytes, err := json.Marshal(e.Data)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// GetEventTypeFromString converts string to EventType
func GetEventTypeFromString(s string) EventType {
	return EventType(s)
}

// IsRconEvent checks if the event type is an RCON event
func (et EventType) IsRconEvent() bool {
	switch et {
	case EventTypeRconChatMessage, EventTypeRconPlayerWarned,
		EventTypeRconPlayerKicked, EventTypeRconPlayerBanned, EventTypeRconPossessedAdminCamera, EventTypeRconUnpossessedAdminCamera,
		EventTypeRconSquadCreated:
		return true
	default:
		return false
	}
}

// IsLogEvent checks if the event type is a log event
func (et EventType) IsLogEvent() bool {
	return !et.IsRconEvent()
}
