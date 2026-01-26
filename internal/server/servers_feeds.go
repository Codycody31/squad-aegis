package server

import (
	"context"
	"fmt"
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
				event_manager.EventTypeLogPlayerDisconnected,
			)
		case "teamkills":
			eventTypes = append(eventTypes, event_manager.EventTypeLogPlayerWounded)
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

	case event_manager.EventTypeLogPlayerWounded:
		if !contains(feedTypes, "teamkills") {
			return nil
		}
		if woundedData, ok := event.Data.(*event_manager.LogPlayerWoundedData); ok {
			// Only include teamkills
			if !woundedData.Teamkill {
				return nil
			}
			feedEvent.Type = "teamkill"

			// Use Steam ID as fallback if attacker name is empty
			attackerName := woundedData.AttackerName
			if attackerName == "" && woundedData.AttackerSteam != "" {
				attackerName = woundedData.AttackerSteam
			}

			feedEvent.Data = map[string]interface{}{
				"victim_name":    woundedData.VictimName,
				"attacker_name":  attackerName,
				"attacker_steam": woundedData.AttackerSteam,
				"attacker_eos":   woundedData.AttackerEOS,
				"weapon":         woundedData.Weapon,
				"damage":         woundedData.Damage,
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

	// Support pagination with 'before' parameter for loading older events
	beforeStr := c.Query("before")
	var beforeTime *time.Time
	if beforeStr != "" {
		if parsed, err := time.Parse(time.RFC3339, beforeStr); err == nil {
			beforeTime = &parsed
		}
	}

	var events []FeedEvent

	// Get historical data from ClickHouse through PluginManager
	if s.Dependencies.Clickhouse != nil {
		switch feedType {
		case "chat":
			events, err = s.getHistoricalChatMessagesWithPagination(serverId, limit, beforeTime)
		case "connections":
			events, err = s.getHistoricalConnectionsWithPagination(serverId, limit, beforeTime)
		case "teamkills":
			events, err = s.getHistoricalTeamkillsWithPagination(serverId, limit, beforeTime)
		default:
			// Get all types
			chatEvents, _ := s.getHistoricalChatMessagesWithPagination(serverId, limit/3, beforeTime)
			connEvents, _ := s.getHistoricalConnectionsWithPagination(serverId, limit/3, beforeTime)
			tkEvents, _ := s.getHistoricalTeamkillsWithPagination(serverId, limit/3, beforeTime)

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

	// Reverse events to show oldest first (like logs)
	for i := 0; i < len(events)/2; i++ {
		events[i], events[len(events)-1-i] = events[len(events)-1-i], events[i]
	}

	responses.Success(c, "Feed history retrieved successfully", &gin.H{
		"events": events,
		"type":   feedType,
		"limit":  limit,
		"before": beforeStr,
	})
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

// getHistoricalChatMessagesWithPagination retrieves chat message history with pagination
func (s *Server) getHistoricalChatMessagesWithPagination(serverId uuid.UUID, limit int, beforeTime *time.Time) ([]FeedEvent, error) {
	if s.Dependencies.Clickhouse == nil {
		return []FeedEvent{}, nil
	}

	query := `
		SELECT 
			message_id,
			sent_at,
			player_name,
			steam_id,
			eos_id,
			message,
			chat_type
		FROM squad_aegis.server_player_chat_messages 
		WHERE server_id = ?`

	args := []interface{}{serverId}

	if beforeTime != nil {
		query += " AND sent_at < ?"
		args = append(args, *beforeTime)
	}

	query += " ORDER BY sent_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.Dependencies.Clickhouse.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []FeedEvent
	for rows.Next() {
		var messageId string
		var sentAt time.Time
		var playerName string
		var steamId uint64
		var eosId string
		var message string
		var chatType string

		err := rows.Scan(&messageId, &sentAt, &playerName, &steamId, &eosId, &message, &chatType)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan chat message row")
			continue
		}

		events = append(events, FeedEvent{
			ID:        messageId,
			Type:      "chat",
			Timestamp: sentAt,
			Data: map[string]interface{}{
				"player_name": playerName,
				"steam_id":    fmt.Sprintf("%d", steamId),
				"eos_id":      eosId,
				"message":     message,
				"chat_type":   chatType,
			},
		})
	}

	return events, nil
}

// getHistoricalConnectionsWithPagination retrieves connection history with pagination
func (s *Server) getHistoricalConnectionsWithPagination(serverId uuid.UUID, limit int, beforeTime *time.Time) ([]FeedEvent, error) {
	if s.Dependencies.Clickhouse == nil {
		return []FeedEvent{}, nil
	}

	// Union query to get both connected and join succeeded events
	var query string
	var args []interface{}

	if beforeTime != nil {
		query = `
			SELECT * FROM (
				SELECT 
					chain_id as id,
					event_time,
					'connected' as action,
					player_controller,
					ip,
					steam,
					eos
				FROM squad_aegis.server_player_connected_events 
				WHERE server_id = ? AND event_time < ?
				UNION ALL
				SELECT 
					chain_id as id,
					event_time,
					'joined' as action,
					player_suffix as player_controller,
					ip,
					steam,
					eos
				FROM squad_aegis.server_join_succeeded_events 
				WHERE server_id = ? AND event_time < ?
				UNION ALL
				SELECT 
					chain_id as id,
					event_time,
					'disconnected' as action,
					player_suffix as player_controller,
					ip,
					steam,
					eos
				FROM squad_aegis.server_player_disconnected_events 
				WHERE server_id = ? AND event_time < ?
			) ORDER BY event_time DESC LIMIT ?`
		args = []interface{}{serverId, *beforeTime, serverId, *beforeTime, serverId, *beforeTime, limit}
	} else {
		query = `
			SELECT * FROM (
				SELECT 
					chain_id as id,
					event_time,
					'connected' as action,
					player_controller,
					ip,
					steam,
					eos
				FROM squad_aegis.server_player_connected_events 
				WHERE server_id = ?
				UNION ALL
				SELECT 
					chain_id as id,
					event_time,
					'joined' as action,
					player_suffix as player_controller,
					ip,
					steam,
					eos
				FROM squad_aegis.server_join_succeeded_events 
				WHERE server_id = ?
				UNION ALL
				SELECT 
					chain_id as id,
					event_time,
					'disconnected' as action,
					player_suffix as player_controller,
					ip,
					steam,
					eos
				FROM squad_aegis.server_player_disconnected_events 
				WHERE server_id = ?
			) ORDER BY event_time DESC LIMIT ?`
		args = []interface{}{serverId, serverId, serverId, limit}
	}

	rows, err := s.Dependencies.Clickhouse.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []FeedEvent
	for rows.Next() {
		var id string
		var eventTime time.Time
		var action string
		var playerController string
		var ip string
		var steam, eos *string

		err := rows.Scan(&id, &eventTime, &action, &playerController, &ip, &steam, &eos)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan connection row")
			continue
		}

		// Build data map with proper null handling
		data := map[string]interface{}{
			"action":     action,
			"ip_address": ip,
		}

		if action == "connected" {
			data["player_controller"] = playerController
		} else {
			data["player_suffix"] = playerController
		}

		if steam != nil {
			data["steam_id"] = *steam
		}
		if eos != nil {
			data["eos_id"] = *eos
		}

		events = append(events, FeedEvent{
			ID:        id,
			Type:      "connection",
			Timestamp: eventTime,
			Data:      data,
		})
	}

	return events, nil
}

// ServerRecentJoins returns the most recent player joins for a server
func (s *Server) ServerRecentJoins(c *gin.Context) {
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

	// Get limit from query, default to 5
	limitStr := c.DefaultQuery("limit", "5")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 50 {
		limit = 5
	}

	joins, err := s.getRecentServerJoins(serverId, limit)
	if err != nil {
		responses.BadRequest(c, "Failed to retrieve recent joins", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Recent joins retrieved successfully", &gin.H{
		"joins": joins,
		"count": len(joins),
	})
}

// RecentJoin represents a recent player join event
type RecentJoin struct {
	ID         string    `json:"id"`
	PlayerName string    `json:"player_name"`
	SteamID    string    `json:"steam_id,omitempty"`
	EOSID      string    `json:"eos_id,omitempty"`
	JoinedAt   time.Time `json:"joined_at"`
}

// getRecentServerJoins retrieves the most recent player joins from ClickHouse
func (s *Server) getRecentServerJoins(serverId uuid.UUID, limit int) ([]RecentJoin, error) {
	if s.Dependencies.Clickhouse == nil {
		return []RecentJoin{}, nil
	}

	query := `
		SELECT
			chain_id,
			player_suffix,
			steam,
			eos,
			event_time
		FROM squad_aegis.server_join_succeeded_events
		WHERE server_id = ?
		ORDER BY event_time DESC
		LIMIT ?`

	rows, err := s.Dependencies.Clickhouse.Query(context.Background(), query, serverId, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var joins []RecentJoin
	for rows.Next() {
		var chainId string
		var playerSuffix string
		var steam, eos *string
		var eventTime time.Time

		err := rows.Scan(&chainId, &playerSuffix, &steam, &eos, &eventTime)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan recent join row")
			continue
		}

		join := RecentJoin{
			ID:         chainId,
			PlayerName: playerSuffix,
			JoinedAt:   eventTime,
		}

		if steam != nil {
			join.SteamID = *steam
		}
		if eos != nil {
			join.EOSID = *eos
		}

		joins = append(joins, join)
	}

	return joins, nil
}

// getHistoricalTeamkillsWithPagination retrieves teamkill history with pagination
func (s *Server) getHistoricalTeamkillsWithPagination(serverId uuid.UUID, limit int, beforeTime *time.Time) ([]FeedEvent, error) {
	if s.Dependencies.Clickhouse == nil {
		return []FeedEvent{}, nil
	}

	query := `
		SELECT 
			chain_id,
			event_time,
			victim_name,
			attacker_name,
			attacker_steam,
			attacker_eos,
			weapon,
			damage
		FROM squad_aegis.server_player_wounded_events 
		WHERE server_id = ? AND teamkill = 1`

	args := []interface{}{serverId}

	if beforeTime != nil {
		query += " AND event_time < ?"
		args = append(args, *beforeTime)
	}

	query += " ORDER BY event_time DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.Dependencies.Clickhouse.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []FeedEvent
	for rows.Next() {
		var chainId string
		var eventTime time.Time
		var victimName string
		var attackerName string
		var attackerSteam, attackerEos *string
		var weapon string
		var damage float32

		err := rows.Scan(&chainId, &eventTime, &victimName, &attackerName, &attackerSteam, &attackerEos, &weapon, &damage)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan teamkill row")
			continue
		}

		// Use Steam ID as fallback if attacker name is empty
		if attackerName == "" && attackerSteam != nil && *attackerSteam != "" {
			attackerName = *attackerSteam
		}

		data := map[string]interface{}{
			"victim_name":   victimName,
			"attacker_name": attackerName,
			"weapon":        weapon,
			"damage":        damage,
		}

		if attackerSteam != nil {
			data["attacker_steam"] = *attackerSteam
		}
		if attackerEos != nil {
			data["attacker_eos"] = *attackerEos
		}

		events = append(events, FeedEvent{
			ID:        chainId,
			Type:      "teamkill",
			Timestamp: eventTime,
			Data:      data,
		})
	}

	return events, nil
}
