package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

// ServerFeeds handles subscribing to live server events for feeds (chat, connections, teamkills)
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

	// Subscribe to events using the centralized event manager
	subscriber := s.Dependencies.EventManager.Subscribe(event_manager.EventFilter{
		Types:     eventTypes,
		ServerIDs: []uuid.UUID{serverId},
	}, &serverId, 100)
	defer s.Dependencies.EventManager.Unsubscribe(subscriber.ID)

	// Set headers for SSE
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Cache-Control")
	c.Writer.Flush()

	// Create a context that is canceled when the client disconnects
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Create a goroutine to detect client disconnection
	go func() {
		<-c.Request.Context().Done()
		cancel()
	}()

	// Send a ping event every 30 seconds to keep the connection alive
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Create audit log for connection
	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:feeds:connect",
		map[string]interface{}{"feed_types": feedTypes})

	// Send initial connection event
	fmt.Fprintf(c.Writer, "event: connected\ndata: {\"message\": \"Connected to feeds\", \"types\": %s}\n\n",
		formatStringArray(feedTypes))
	c.Writer.Flush()

	// Send events to client
	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			s.CreateAuditLog(context.Background(), &serverId, &user.Id, "server:feeds:disconnect", nil)
			return
		case event := <-subscriber.Channel:
			// Format event for feeds
			feedEvent := s.formatEventForFeed(event, feedTypes)
			if feedEvent == nil {
				continue
			}

			// Convert event to JSON
			eventJSON, err := json.Marshal(feedEvent)
			if err != nil {
				continue
			}

			// Send event to client
			fmt.Fprintf(c.Writer, "event: feed\ndata: %s\n\n", eventJSON)
			c.Writer.Flush()
		case <-ticker.C:
			// Send ping event
			fmt.Fprintf(c.Writer, "event: ping\ndata: %d\n\n", time.Now().Unix())
			c.Writer.Flush()
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
	query := `
		SELECT message_id, player_name, steam_id, eos_id, sent_at, chat_type, message
		FROM squad_aegis.server_player_chat_messages
		WHERE server_id = ?
		ORDER BY sent_at DESC
		LIMIT ?
	`

	rows, err := s.Dependencies.ClickhouseClient.Query(context.Background(), query, serverId.String(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []FeedEvent
	for rows.Next() {
		var messageId, playerName, steamId, eosId, chatType, message string
		var sentAt time.Time

		err := rows.Scan(&messageId, &playerName, &steamId, &eosId, &sentAt, &chatType, &message)
		if err != nil {
			continue
		}

		events = append(events, FeedEvent{
			ID:        messageId,
			Type:      "chat",
			Timestamp: sentAt,
			Data: map[string]interface{}{
				"player_name": playerName,
				"steam_id":    steamId,
				"eos_id":      eosId,
				"message":     message,
				"chat_type":   chatType,
			},
		})
	}

	return events, nil
}

// getHistoricalConnections retrieves connection history from ClickHouse
func (s *Server) getHistoricalConnections(serverId uuid.UUID, limit int) ([]FeedEvent, error) {
	query := `
		SELECT event_time, chain_id, player_controller, ip_address, steam_id, eos_id
		FROM squad_aegis.server_player_connected_events
		WHERE server_id = ?
		ORDER BY event_time DESC
		LIMIT ?
	`

	rows, err := s.Dependencies.ClickhouseClient.Query(context.Background(), query, serverId.String(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []FeedEvent
	for rows.Next() {
		var chainId, playerController, ipAddress, steamId, eosId string
		var eventTime time.Time

		err := rows.Scan(&eventTime, &chainId, &playerController, &ipAddress, &steamId, &eosId)
		if err != nil {
			continue
		}

		events = append(events, FeedEvent{
			ID:        chainId,
			Type:      "connection",
			Timestamp: eventTime,
			Data: map[string]interface{}{
				"player_controller": playerController,
				"ip_address":        ipAddress,
				"steam_id":          steamId,
				"eos_id":            eosId,
				"action":            "connected",
			},
		})
	}

	return events, nil
}

// getHistoricalTeamkills retrieves teamkill history from ClickHouse
func (s *Server) getHistoricalTeamkills(serverId uuid.UUID, limit int) ([]FeedEvent, error) {
	query := `
		SELECT event_time, chain_id, victim_name, attacker_player_controller, weapon, damage, attacker_steam, attacker_eos
		FROM squad_aegis.server_player_died_events
		WHERE server_id = ? AND teamkill = 1
		ORDER BY event_time DESC
		LIMIT ?
	`

	rows, err := s.Dependencies.ClickhouseClient.Query(context.Background(), query, serverId.String(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []FeedEvent
	for rows.Next() {
		var chainId, victimName, attackerController, weapon, damage, attackerSteam, attackerEos string
		var eventTime time.Time

		err := rows.Scan(&eventTime, &chainId, &victimName, &attackerController, &weapon, &damage, &attackerSteam, &attackerEos)
		if err != nil {
			continue
		}

		events = append(events, FeedEvent{
			ID:        chainId,
			Type:      "teamkill",
			Timestamp: eventTime,
			Data: map[string]interface{}{
				"victim_name":    victimName,
				"attacker_name":  extractPlayerName(attackerController),
				"attacker_steam": attackerSteam,
				"attacker_eos":   attackerEos,
				"weapon":         weapon,
				"damage":         damage,
			},
		})
	}

	return events, nil
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

func formatStringArray(arr []string) string {
	data, _ := json.Marshal(arr)
	return string(data)
}

func extractPlayerName(playerController string) string {
	// Extract player name from controller string like "BP_PlayerController_C /Game/Maps/..."
	// This is a simplified extraction - you might need to adjust based on actual data format
	if len(playerController) > 20 {
		return playerController[:20] + "..."
	}
	return playerController
}
