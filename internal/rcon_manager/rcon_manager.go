package rcon_manager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	rcon "github.com/SquadGO/squad-rcon-go/v2"
	"github.com/SquadGO/squad-rcon-go/v2/rconTypes"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
)

// RconEvent represents an event from the RCON server
type RconEvent struct {
	ServerID uuid.UUID
	Type     string
	Data     interface{}
	Time     time.Time
}

// RconCommand represents a command to be executed on the RCON server
type RconCommand struct {
	Command  string
	Response chan CommandResponse
}

// CommandResponse represents the response from an RCON command
type CommandResponse struct {
	Response string
	Error    error
}

// ServerConnection represents a connection to an RCON server
type ServerConnection struct {
	ServerID     uuid.UUID
	Rcon         *rcon.Rcon // Single connection for both commands and events
	CommandChan  chan RconCommand
	EventChan    chan RconEvent
	LastUsed     time.Time
	mu           sync.Mutex
	cmdSemaphore chan struct{}
}

// RconManager manages RCON connections to multiple servers
type RconManager struct {
	connections      map[uuid.UUID]*ServerConnection
	eventSubscribers []chan<- RconEvent
	eventManager     *event_manager.EventManager
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
}

// NewRconManager creates a new RCON manager
func NewRconManager(ctx context.Context, eventManager *event_manager.EventManager) *RconManager {
	ctx, cancel := context.WithCancel(ctx)
	return &RconManager{
		connections:      make(map[uuid.UUID]*ServerConnection),
		eventSubscribers: []chan<- RconEvent{},
		eventManager:     eventManager,
		ctx:              ctx,
		cancel:           cancel,
	}
}

// SubscribeToEvents subscribes to RCON events
func (m *RconManager) SubscribeToEvents() chan RconEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	eventChan := make(chan RconEvent, 100)
	m.eventSubscribers = append(m.eventSubscribers, eventChan)
	return eventChan
}

// UnsubscribeFromEvents unsubscribes from RCON events
func (m *RconManager) UnsubscribeFromEvents(eventChan chan RconEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, subscriber := range m.eventSubscribers {
		if subscriber == eventChan {
			m.eventSubscribers = append(m.eventSubscribers[:i], m.eventSubscribers[i+1:]...)
			close(eventChan)
			return
		}
	}
}

// broadcastEvent broadcasts an event to all subscribers and the centralized event manager
func (m *RconManager) broadcastEvent(event RconEvent) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Broadcast to legacy subscribers (for backward compatibility)
	for _, subscriber := range m.eventSubscribers {
		select {
		case subscriber <- event:
		default:
			// If channel is full, log and continue
			log.Warn().
				Str("serverID", event.ServerID.String()).
				Str("eventType", event.Type).
				Msg("Event channel full, dropping event")
		}
	}

	// Publish to centralized event manager
	if m.eventManager != nil && event.Data != nil {
		// Try to convert to map or use reflection
		switch data := event.Data.(type) {
		case rconTypes.Warn:
			m.eventManager.PublishEvent(event.ServerID, &event_manager.RconPlayerWarnedData{
				PlayerName: data.PlayerName,
				Message:    data.Message,
			}, data.Raw)
		case rconTypes.Ban:
			m.eventManager.PublishEvent(event.ServerID, &event_manager.RconPlayerBannedData{
				PlayerID:   data.PlayerID,
				SteamID:    data.SteamID,
				PlayerName: data.PlayerName,
				Interval:   data.Interval,
			}, data.Raw)
		case rconTypes.Kick:
			m.eventManager.PublishEvent(event.ServerID, &event_manager.RconPlayerKickedData{
				PlayerID:   data.PlayerID,
				EosID:      data.EosID,
				SteamID:    data.SteamID,
				PlayerName: data.PlayerName,
			}, data.Raw)
		case rconTypes.Message:
			m.eventManager.PublishEvent(event.ServerID, &event_manager.RconChatMessageData{
				ChatType:   data.ChatType,
				EosID:      data.EosID,
				Message:    data.Message,
				PlayerName: data.PlayerName,
				SteamID:    data.SteamID,
			}, data.Raw)
		case rconTypes.PosAdminCam:
			m.eventManager.PublishEvent(event.ServerID, &event_manager.RconAdminCameraData{
				AdminName: data.AdminName,
				EosID:     data.EosID,
				SteamID:   data.SteamID,
				Action:    "possessed",
			}, data.Raw)
		case rconTypes.UnposAdminCam:
			m.eventManager.PublishEvent(event.ServerID, &event_manager.RconAdminCameraData{
				AdminName: data.AdminName,
				EosID:     data.EosID,
				SteamID:   data.SteamID,
				Action:    "unpossessed",
			}, data.Raw)
		case rconTypes.SquadCreated:
			m.eventManager.PublishEvent(event.ServerID, &event_manager.RconSquadCreatedData{
				PlayerName: data.PlayerName,
				EosID:      data.EosID,
				SteamID:    data.SteamID,
				SquadID:    data.SquadID,
				SquadName:  data.SquadName,
				TeamName:   data.TeamName,
			}, data.Raw)
		default:
			log.Warn().
				Str("serverID", event.ServerID.String()).
				Str("eventType", event.Type).
				Msg("Unknown event data type, cannot publish to event manager")
		}
	}
}

// ConnectToServer connects to an RCON server
func (m *RconManager) ConnectToServer(serverID uuid.UUID, host string, port int, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	portStr := strconv.Itoa(port)

	// Check if connection already exists
	if conn, exists := m.connections[serverID]; exists {
		conn.mu.Lock()
		defer conn.mu.Unlock()

		// Connection already exists, update last used time
		conn.LastUsed = time.Now()
		return nil
	}

	// Create single RCON connection
	rconConn, err := rcon.NewRconWithContext(m.ctx, rcon.RconConfig{
		Host:               host,
		Port:               portStr,
		Password:           password,
		AutoReconnect:      true,
		AutoReconnectDelay: 5,
	})
	if err != nil {
		log.Error().
			Str("serverID", serverID.String()).
			Err(err).
			Msg("Failed to connect to RCON")
		return fmt.Errorf("failed to connect to RCON: %w", err)
	}

	// Create a semaphore to ensure only one command executes at a time
	cmdSemaphore := make(chan struct{}, 1)

	conn := &ServerConnection{
		ServerID:     serverID,
		Rcon:         rconConn,
		CommandChan:  make(chan RconCommand, 100),
		EventChan:    make(chan RconEvent, 100),
		LastUsed:     time.Now(),
		cmdSemaphore: cmdSemaphore,
	}

	m.connections[serverID] = conn

	// Start listening for events and processing commands
	go m.listenForEvents(serverID, rconConn)
	go m.processCommands(serverID, conn)

	log.Info().
		Str("serverID", serverID.String()).
		Msg("Connected to RCON server")

	return nil
}

// DisconnectFromServer disconnects from an RCON server
func (m *RconManager) DisconnectFromServer(serverID uuid.UUID, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, exists := m.connections[serverID]
	if !exists {
		return errors.New("server not connected")
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	// Close the connection
	conn.Rcon.Close()

	// Remove the connection from the map
	delete(m.connections, serverID)

	log.Info().
		Str("serverID", serverID.String()).
		Msg("Disconnected from RCON server")

	return nil
}

// ExecuteCommand executes a command on an RCON server
func (m *RconManager) ExecuteCommand(serverID uuid.UUID, command string) (string, error) {
	m.mu.RLock()
	conn, exists := m.connections[serverID]
	m.mu.RUnlock()

	if !exists {
		log.Error().
			Str("serverID", serverID.String()).
			Str("command", command).
			Msg("Server not connected")
		return "", errors.New("server not connected")
	}

	conn.mu.Lock()
	conn.LastUsed = time.Now()
	conn.mu.Unlock()

	responseChan := make(chan CommandResponse, 1)

	// Send command to command processor
	select {
	case conn.CommandChan <- RconCommand{Command: command, Response: responseChan}:
		// Command queued successfully
	case <-time.After(30 * time.Second):
		log.Error().
			Str("serverID", serverID.String()).
			Str("command", command).
			Msg("Command queue full, try again later")
		return "", errors.New("command queue full, try again later")
	case <-m.ctx.Done():
		return "", errors.New("rcon manager shutting down")
	}

	// Wait for response
	select {
	case response := <-responseChan:
		if response.Error != nil {
			log.Debug().
				Str("serverID", serverID.String()).
				Str("command", command).
				Err(response.Error).
				Msg("Command execution failed")
		}
		return response.Response, response.Error
	case <-time.After(30 * time.Second):
		log.Error().
			Str("serverID", serverID.String()).
			Str("command", command).
			Msg("Command timed out")
		return "", errors.New("command timed out")
	case <-m.ctx.Done():
		return "", errors.New("rcon manager shutting down")
	}
}

// processCommands processes commands for a server
func (m *RconManager) processCommands(serverID uuid.UUID, conn *ServerConnection) {
	log.Debug().
		Str("serverID", serverID.String()).
		Msg("Starting command processor")

	for {
		select {
		case cmd := <-conn.CommandChan:
			// Acquire the semaphore
			conn.cmdSemaphore <- struct{}{}

			// Update last used time
			conn.mu.Lock()
			conn.LastUsed = time.Now()
			conn.mu.Unlock()

			// Execute command with timeout
			responseChan := make(chan CommandResponse, 1)

			startTime := time.Now()
			go func() {
				// Execute command using the single RCON connection
				response := conn.Rcon.Execute(cmd.Command)
				// Only log on errors, not every execution time
				if response == "" {
					execTime := time.Since(startTime)
					log.Debug().
						Str("serverID", serverID.String()).
						Str("command", cmd.Command).
						Err(errors.New("empty response")).
						Dur("execTime", execTime).
						Msg("Command execution returned empty response")
				}
				select {
				case responseChan <- CommandResponse{
					Response: response,
				}:
					// Response sent
				default:
					// Only log on failure
					log.Debug().
						Str("serverID", serverID.String()).
						Str("command", cmd.Command).
						Msg("Could not send response to channel, might be closed")
				}
			}()

			// Wait for response with timeout
			var cmdResponse CommandResponse
			select {
			case response := <-responseChan:
				cmdResponse = response
			case <-time.After(30 * time.Second):
				cmdResponse = CommandResponse{
					Response: "",
					Error:    errors.New("command execution timed out"),
				}
				log.Debug().
					Str("serverID", serverID.String()).
					Str("command", cmd.Command).
					Msg("Command execution timed out internally")
			case <-m.ctx.Done():
				cmdResponse = CommandResponse{
					Response: "",
					Error:    errors.New("rcon manager shutting down"),
				}
			}

			// Send response back to caller
			select {
			case cmd.Response <- cmdResponse:
				// Response sent
			case <-m.ctx.Done():
				// Manager is shutting down
			}

			// Release the semaphore
			<-conn.cmdSemaphore

		case <-m.ctx.Done():
			log.Debug().
				Str("serverID", serverID.String()).
				Msg("Stopping command processor due to context cancellation")
			return
		}
	}
}

// listenForEvents listens for events from an RCON server
func (m *RconManager) listenForEvents(serverID uuid.UUID, sr *rcon.Rcon) {
	// Helper function to update LastUsed and broadcast event
	updateAndBroadcast := func(eventType string, data interface{}) {
		m.mu.RLock()
		conn, exists := m.connections[serverID]
		m.mu.RUnlock()

		if exists {
			conn.mu.Lock()
			conn.LastUsed = time.Now()
			conn.mu.Unlock()
		} else {
			log.Warn().Str("serverID", serverID.String()).Msg("Connection not found for event, cannot update LastUsed")
		}

		event := RconEvent{
			ServerID: serverID,
			Type:     eventType,
			Data:     data,
			Time:     time.Now(),
		}
		m.broadcastEvent(event)
	}

	// Setup event listeners
	sr.Emitter.On("CHAT_MESSAGE", func(data interface{}) {
		updateAndBroadcast("CHAT_MESSAGE", data)
	})

	sr.Emitter.On("CHAT_COMMAND", func(data interface{}) {
		updateAndBroadcast("CHAT_COMMAND", data)
	})

	sr.Emitter.On("PLAYER_WARNED", func(data interface{}) {
		updateAndBroadcast("PLAYER_WARNED", data)
	})

	sr.Emitter.On("PLAYER_KICKED", func(data interface{}) {
		updateAndBroadcast("PLAYER_KICKED", data)
	})

	sr.Emitter.On("POSSESSED_ADMIN_CAMERA", func(data interface{}) {
		updateAndBroadcast("POSSESSED_ADMIN_CAMERA", data)
	})

	sr.Emitter.On("UNPOSSESSED_ADMIN_CAMERA", func(data interface{}) {
		updateAndBroadcast("UNPOSSESSED_ADMIN_CAMERA", data)
	})

	sr.Emitter.On("SQUAD_CREATED", func(data interface{}) {
		updateAndBroadcast("SQUAD_CREATED", data)
	})

	// Listen for connection events
	sr.Emitter.On("close", func(data interface{}) {
		log.Warn().
			Str("serverID", serverID.String()).
			Interface("data", data).
			Msg("RCON event connection closed")

		updateAndBroadcast("CONNECTION_CLOSED", data)
	})

	sr.Emitter.On("error", func(data interface{}) {
		log.Error().
			Str("serverID", serverID.String()).
			Interface("data", data).
			Msg("RCON event connection error")

		updateAndBroadcast("CONNECTION_ERROR", data)
	})

	// Block until context is done
	<-m.ctx.Done()
}

// StartConnectionManager starts the connection manager
func (m *RconManager) StartConnectionManager() {
	<-m.ctx.Done()
	m.cleanupAllConnections()
}

// cleanupAllConnections closes all connections
func (m *RconManager) cleanupAllConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, conn := range m.connections {
		conn.mu.Lock()
		conn.Rcon.Close()
		conn.mu.Unlock()
	}

	log.Debug().Msg("All RCON connections closed during shutdown")
}

// Shutdown shuts down the RCON manager
func (m *RconManager) Shutdown() {
	m.cancel()
}

// ConnectToAllServers connects to all servers in the database
func (m *RconManager) ConnectToAllServers(ctx context.Context, db *sql.DB) {
	// Get all servers from the database
	rows, err := db.QueryContext(ctx, `
		SELECT id, ip_address, rcon_ip_address, rcon_port, rcon_password
		FROM servers
		WHERE rcon_port > 0 AND rcon_password != ''
	`)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query servers for RCON connections")
		return
	}
	defer rows.Close()

	// Connect to each server
	for rows.Next() {
		var id uuid.UUID
		var ipAddress string
		var rconIpAddress *string
		var rconPort int
		var rconPassword string

		if err := rows.Scan(&id, &ipAddress, &rconIpAddress, &rconPort, &rconPassword); err != nil {
			log.Error().Err(err).Msg("Failed to scan server row")
			continue
		}

		ipAddressForRcon := ipAddress
		if rconIpAddress != nil && *rconIpAddress != "" {
			ipAddressForRcon = *rconIpAddress
		}

		// Try to connect to the server
		err := m.ConnectToServer(id, ipAddressForRcon, rconPort, rconPassword)
		if err != nil {
			log.Warn().
				Err(err).
				Str("serverID", id.String()).
				Str("ipAddress", ipAddress).
				Int("rconPort", rconPort).
				Msg("Failed to connect to server RCON")
			continue
		}

		log.Info().
			Str("serverID", id.String()).
			Str("ipAddress", ipAddress).
			Int("rconPort", rconPort).
			Msg("Connected to server RCON")
	}

	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating server rows")
	}
}

// ProcessChatMessages starts processing chat messages for all connected servers
func (m *RconManager) ProcessChatMessages(ctx context.Context, messageHandler func(serverID uuid.UUID, message rconTypes.Message)) {
	// Create a channel to receive chat events
	eventChan := m.SubscribeToEvents()

	// Start a goroutine to process events
	go func() {
		defer m.UnsubscribeFromEvents(eventChan)

		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("Stopping chat message processor")
				return
			case event := <-eventChan:
				// Only process chat messages
				if event.Type == "CHAT_MESSAGE" {
					if message, ok := event.Data.(rconTypes.Message); ok {
						// Call the message handler
						messageHandler(event.ServerID, message)
					}
				}
			}
		}
	}()
}
