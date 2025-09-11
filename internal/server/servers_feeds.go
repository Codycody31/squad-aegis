package server

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// FeedEvent represents a formatted event for the feeds
type FeedEvent struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// WebSocket upgrader with permissive settings for development
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from any origin (adjust for production)
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// ServerFeeds handles subscribing to live server events for feeds (chat, connections, teamkills) via WebSocket
func (s *Server) ServerFeeds(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Get feed types from query parameter (default to all)
	feedTypes := c.QueryArray("types")
	if len(feedTypes) == 0 {
		feedTypes = []string{"chat", "connections", "teamkills"}
	}

	// Map feed types to event types
	eventTypes := []event_manager.EventType{}
	for _, feedType := range feedTypes {
		switch feedType {
		case "chat":
			eventTypes = append(eventTypes, event_manager.EventTypeRconChatMessage)
		case "connections":
			eventTypes = append(eventTypes,
				event_manager.EventTypeLogPlayerConnected,
				event_manager.EventTypeLogJoinSucceeded,
			)
		case "teamkills":
			eventTypes = append(eventTypes, event_manager.EventTypeLogPlayerDied)
		}
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		responses.BadRequest(c, "Failed to upgrade to WebSocket", &gin.H{"error": err.Error()})
		return
	}
	defer conn.Close()

	// Subscribe to events using the centralized event manager
	subscriber := s.Dependencies.EventManager.Subscribe(event_manager.EventFilter{
		Types:     eventTypes,
		ServerIDs: []uuid.UUID{serverId},
	}, &serverId, 100)
	defer s.Dependencies.EventManager.Unsubscribe(subscriber.ID)

	// Create a context that is canceled when the connection is closed
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Send initial connection message
	connectMsg := map[string]interface{}{
		"type":    "connected",
		"message": "Connected to feeds",
		"types":   feedTypes,
	}
	if err := conn.WriteJSON(connectMsg); err != nil {
		log.Error().Err(err).Msg("Failed to send initial connection message")
		return
	}

	// Set up ping/pong handlers for connection health
	conn.SetPingHandler(func(appData string) error {
		return conn.WriteMessage(websocket.PongMessage, []byte(appData))
	})

	// Send ping every 30 seconds to keep connection alive
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Start a goroutine to handle client messages (for connection health)
	go func() {
		defer cancel()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					// Log unexpected close
				}
				return
			}
		}
	}()

	// Send events to client
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-subscriber.Channel:
			// Format event for feeds
			feedEvent := s.formatEventForFeed(event, feedTypes)
			if feedEvent == nil {
				continue
			}

			// Send event to client
			if err := conn.WriteJSON(feedEvent); err != nil {
				// Connection likely closed
				log.Error().Err(err).Msg("Failed to send event to WebSocket client")
				return
			}
		case <-ticker.C:
			// Send ping to keep connection alive
			if err := conn.WriteMessage(websocket.PingMessage, []byte("ping")); err != nil {
				// Connection likely closed
				return
			}
		}
	}
}

// formatEventForFeed formats an event for the feeds interface
func (s *Server) formatEventForFeed(event event_manager.Event, feedTypes []string) *FeedEvent {
	feedEvent := &FeedEvent{
		ID:        event.ID.String(),
		Timestamp: event.Timestamp,
		Data:      make(map[string]interface{}),
	}

	switch event.Type {
	case event_manager.EventTypeRconChatMessage:
		if !contains(feedTypes, "chat") {
			return nil
		}
		feedEvent.Type = "chat"
		if chatData, ok := event.Data.(*event_manager.RconChatMessageData); ok {
			feedEvent.Data = map[string]interface{}{
				"player_name": chatData.PlayerName,
				"steam_id":    chatData.SteamID,
				"eos_id":      chatData.EosID,
				"message":     chatData.Message,
				"chat_type":   chatData.ChatType,
			}
		}

	case event_manager.EventTypeLogPlayerConnected:
		if !contains(feedTypes, "connections") {
			return nil
		}
		feedEvent.Type = "connection"
		if connData, ok := event.Data.(*event_manager.LogPlayerConnectedData); ok {
			feedEvent.Data = map[string]interface{}{
				"player_controller": connData.PlayerController,
				"ip_address":        connData.IPAddress,
				"steam_id":          connData.SteamID,
				"eos_id":            connData.EOSID,
				"action":            "connected",
			}
		}

	case event_manager.EventTypeLogJoinSucceeded:
		if !contains(feedTypes, "connections") {
			return nil
		}
		feedEvent.Type = "connection"
		if joinData, ok := event.Data.(*event_manager.LogJoinSucceededData); ok {
			feedEvent.Data = map[string]interface{}{
				"player_suffix": joinData.PlayerSuffix,
				"steam_id":      joinData.SteamID,
				"eos_id":        joinData.EOSID,
				"ip_address":    joinData.IPAddress,
				"action":        "joined",
			}
		}

	case event_manager.EventTypeLogPlayerDied:
		if !contains(feedTypes, "teamkills") {
			return nil
		}
		if diedData, ok := event.Data.(*event_manager.LogPlayerDiedData); ok {
			// Only include teamkills
			if !diedData.Teamkill {
				return nil
			}
			feedEvent.Type = "teamkill"
			feedEvent.Data = map[string]interface{}{
				"victim_name":    diedData.VictimName,
				"attacker_name":  extractPlayerName(diedData.AttackerPlayerController),
				"attacker_steam": diedData.AttackerSteam,
				"attacker_eos":   diedData.AttackerEOS,
				"weapon":         diedData.Weapon,
				"damage":         diedData.Damage,
			}
		}

	default:
		return nil
	}

	return feedEvent
}

// ServerFeedsHistory returns historical feed data
func (s *Server) ServerFeedsHistory(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Get parameters
	feedType := c.Query("type")
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 1000 {
		limit = 50
	}

	var events []FeedEvent

	// Get historical data from ClickHouse through PluginManager
	if s.Dependencies.PluginManager != nil {
		switch feedType {
		case "chat":
			events, err = s.getHistoricalChatMessages(serverId, limit)
		case "connections":
			events, err = s.getHistoricalConnections(serverId, limit)
		case "teamkills":
			events, err = s.getHistoricalTeamkills(serverId, limit)
		default:
			// Get all types
			chatEvents, _ := s.getHistoricalChatMessages(serverId, limit/3)
			connEvents, _ := s.getHistoricalConnections(serverId, limit/3)
			tkEvents, _ := s.getHistoricalTeamkills(serverId, limit/3)

			events = append(events, chatEvents...)
			events = append(events, connEvents...)
			events = append(events, tkEvents...)

			// Sort by timestamp descending
			for i := 0; i < len(events)-1; i++ {
				for j := i + 1; j < len(events); j++ {
					if events[i].Timestamp.Before(events[j].Timestamp) {
						events[i], events[j] = events[j], events[i]
					}
				}
			}

			if len(events) > limit {
				events = events[:limit]
			}
		}

		if err != nil {
			responses.BadRequest(c, "Failed to retrieve historical data", &gin.H{"error": err.Error()})
			return
		}
	}

	responses.Success(c, "Feed history retrieved successfully", &gin.H{
		"events": events,
		"type":   feedType,
		"limit":  limit,
	})
}

// getHistoricalChatMessages retrieves chat message history from ClickHouse
func (s *Server) getHistoricalChatMessages(serverId uuid.UUID, limit int) ([]FeedEvent, error) {
	// TODO: Implement ClickHouse access for historical chat messages
	// For now, return empty results
	return []FeedEvent{}, nil
}

// getHistoricalConnections retrieves connection history from ClickHouse
func (s *Server) getHistoricalConnections(serverId uuid.UUID, limit int) ([]FeedEvent, error) {
	// TODO: Implement ClickHouse access for historical connections
	// For now, return empty results
	return []FeedEvent{}, nil
}

// getHistoricalTeamkills retrieves teamkill history from ClickHouse
func (s *Server) getHistoricalTeamkills(serverId uuid.UUID, limit int) ([]FeedEvent, error) {
	// TODO: Implement ClickHouse access for historical teamkills
	// For now, return empty results
	return []FeedEvent{}, nil
}

// Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func extractPlayerName(playerController string) string {
	// Extract player name from controller string like "BP_PlayerController_C /Game/Maps/..."
	// This is a simplified extraction - you might need to adjust based on actual data format
	if len(playerController) > 20 {
		return playerController[:20] + "..."
	}
	return playerController
}
