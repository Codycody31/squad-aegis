package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/core"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// ServerRconEvents handles subscribing to RCON events using Server-Sent Events (SSE)
func (s *Server) ServerRconEvents(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Ensure server is connected to RCON manager
	err = s.Dependencies.RconManager.ConnectToServer(serverId, server.IpAddress, server.RconPort, server.RconPassword)
	if err != nil {
		responses.BadRequest(c, "Failed to connect to RCON", &gin.H{"error": err.Error()})
		return
	}

	// Subscribe to RCON events
	eventChan := s.Dependencies.RconManager.SubscribeToEvents()
	defer s.Dependencies.RconManager.UnsubscribeFromEvents(eventChan)

	// Set headers for SSE
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
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
	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:rcon:events:connect", nil)

	// Send events to client
	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:rcon:events:disconnect", nil)
			return
		case event := <-eventChan:
			// Only send events for this server
			if event.ServerID != serverId {
				continue
			}

			// Convert event to JSON
			eventJSON, err := json.Marshal(event)
			if err != nil {
				continue
			}

			// Send event to client
			fmt.Fprintf(c.Writer, "data: %s\n\n", eventJSON)
			c.Writer.Flush()
		case <-ticker.C:
			// Send ping event
			fmt.Fprintf(c.Writer, "event: ping\ndata: %d\n\n", time.Now().Unix())
			c.Writer.Flush()
		}
	}
}
