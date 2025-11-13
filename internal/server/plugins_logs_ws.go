package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
)

// ServerPluginLogsWebSocket handles WebSocket connections for real-time plugin logs for a specific plugin instance
func (s *Server) ServerPluginLogsWebSocket(c *gin.Context) {
	if s.Dependencies.PluginManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "plugin manager not available"})
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	instanceID, err := uuid.Parse(c.Param("pluginId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plugin instance ID"})
		return
	}

	// Verify plugin instance exists
	_, err = s.Dependencies.PluginManager.GetPluginInstance(serverID, instanceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin instance not found"})
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to upgrade to WebSocket"})
		return
	}
	defer conn.Close()

	// Subscribe to plugin log events for this specific instance
	subscriber := s.Dependencies.EventManager.Subscribe(event_manager.EventFilter{
		Types:     []event_manager.EventType{event_manager.EventTypePluginLog},
		ServerIDs: []uuid.UUID{serverID},
	}, &serverID, 100)
	defer s.Dependencies.EventManager.Unsubscribe(subscriber.ID)

	// Create a context that is canceled when the connection is closed
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Send initial connection message
	connectMsg := map[string]interface{}{
		"type":               "connected",
		"message":            "Connected to plugin logs",
		"plugin_instance_id": instanceID.String(),
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
					log.Debug().Err(err).Msg("WebSocket connection closed unexpectedly")
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
			// Filter events to only include logs for this specific plugin instance
			if event.Type == event_manager.EventTypePluginLog {
				if logData, ok := event.Data.(event_manager.PluginLogEventData); ok {
					if logData.PluginInstanceID == instanceID.String() {
						// Format log event for client
						logEvent := map[string]interface{}{
							"type":               "log",
							"id":                 event.ID.String(),
							"timestamp":          logData.Timestamp,
							"level":              logData.Level,
							"message":            logData.Message,
							"error_message":      logData.ErrorMessage,
							"fields":             logData.Fields,
							"plugin_instance_id": logData.PluginInstanceID,
							"plugin_name":        logData.PluginName,
							"plugin_id":          logData.PluginID,
						}

						// Send event to client
						if err := conn.WriteJSON(logEvent); err != nil {
							log.Error().Err(err).Msg("Failed to send log event to WebSocket client")
							return
						}
					}
				}
			}
		case <-ticker.C:
			// Send ping to keep connection alive
			if err := conn.WriteMessage(websocket.PingMessage, []byte("ping")); err != nil {
				return
			}
		}
	}
}

// ServerPluginLogsAllWebSocket handles WebSocket connections for real-time logs from all plugin instances on a server
func (s *Server) ServerPluginLogsAllWebSocket(c *gin.Context) {
	if s.Dependencies.PluginManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "plugin manager not available"})
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to upgrade to WebSocket"})
		return
	}
	defer conn.Close()

	// Subscribe to plugin log events for all instances on this server
	subscriber := s.Dependencies.EventManager.Subscribe(event_manager.EventFilter{
		Types:     []event_manager.EventType{event_manager.EventTypePluginLog},
		ServerIDs: []uuid.UUID{serverID},
	}, &serverID, 100)
	defer s.Dependencies.EventManager.Unsubscribe(subscriber.ID)

	// Create a context that is canceled when the connection is closed
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Send initial connection message
	connectMsg := map[string]interface{}{
		"type":    "connected",
		"message": "Connected to all plugin logs",
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
					log.Debug().Err(err).Msg("WebSocket connection closed unexpectedly")
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
			// Process plugin log events
			if event.Type == event_manager.EventTypePluginLog {
				if logData, ok := event.Data.(event_manager.PluginLogEventData); ok {
					// Format log event for client
					logEvent := map[string]interface{}{
						"type":               "log",
						"id":                 event.ID.String(),
						"timestamp":          logData.Timestamp,
						"level":              logData.Level,
						"message":            logData.Message,
						"error_message":      logData.ErrorMessage,
						"fields":             logData.Fields,
						"plugin_instance_id": logData.PluginInstanceID,
						"plugin_name":        logData.PluginName,
						"plugin_id":          logData.PluginID,
					}

					// Send event to client
					if err := conn.WriteJSON(logEvent); err != nil {
						log.Error().Err(err).Msg("Failed to send log event to WebSocket client")
						return
					}
				}
			}
		case <-ticker.C:
			// Send ping to keep connection alive
			if err := conn.WriteMessage(websocket.PingMessage, []byte("ping")); err != nil {
				return
			}
		}
	}
}
