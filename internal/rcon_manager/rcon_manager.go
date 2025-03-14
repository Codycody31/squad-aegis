package rcon_manager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/rcon"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
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
	SquadRcon    *squadRcon.SquadRcon
	CommandChan  chan RconCommand
	EventChan    chan RconEvent
	Disconnected bool
	LastUsed     time.Time
	mu           sync.Mutex
}

// RconManager manages RCON connections to multiple servers
type RconManager struct {
	connections      map[uuid.UUID]*ServerConnection
	eventSubscribers []chan<- RconEvent
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
}

// NewRconManager creates a new RCON manager
func NewRconManager(ctx context.Context) *RconManager {
	ctx, cancel := context.WithCancel(ctx)
	return &RconManager{
		connections:      make(map[uuid.UUID]*ServerConnection),
		eventSubscribers: []chan<- RconEvent{},
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

// broadcastEvent broadcasts an event to all subscribers
func (m *RconManager) broadcastEvent(event RconEvent) {
	m.mu.RLock()
	defer m.mu.RUnlock()

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
}

// ConnectToServer connects to an RCON server
func (m *RconManager) ConnectToServer(serverID uuid.UUID, host string, port int, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if connection already exists
	if conn, exists := m.connections[serverID]; exists {
		conn.mu.Lock()
		defer conn.mu.Unlock()

		// If connection is disconnected, reconnect
		if conn.Disconnected {
			squadRcon, err := squadRcon.NewSquadRcon(rcon.RconConfig{
				Host:               host,
				Port:               strconv.Itoa(port),
				Password:           password,
				AutoReconnect:      true,
				AutoReconnectDelay: 5,
			})
			if err != nil {
				return fmt.Errorf("failed to connect to RCON: %w", err)
			}

			conn.SquadRcon = squadRcon
			conn.Disconnected = false
			conn.LastUsed = time.Now()

			// Start listening for events
			go m.listenForEvents(serverID, squadRcon)
			// Start command processor
			go m.processCommands(serverID, conn)

			return nil
		}

		// Connection already exists and is connected
		conn.LastUsed = time.Now()
		return nil
	}

	// Create new connection
	squadRcon, err := squadRcon.NewSquadRcon(rcon.RconConfig{
		Host:               host,
		Port:               strconv.Itoa(port),
		Password:           password,
		AutoReconnect:      true,
		AutoReconnectDelay: 5,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to RCON: %w", err)
	}

	conn := &ServerConnection{
		ServerID:     serverID,
		SquadRcon:    squadRcon,
		CommandChan:  make(chan RconCommand, 100),
		EventChan:    make(chan RconEvent, 100),
		Disconnected: false,
		LastUsed:     time.Now(),
	}

	m.connections[serverID] = conn

	// Start listening for events
	go m.listenForEvents(serverID, squadRcon)
	// Start command processor
	go m.processCommands(serverID, conn)

	return nil
}

// DisconnectFromServer disconnects from an RCON server
func (m *RconManager) DisconnectFromServer(serverID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, exists := m.connections[serverID]
	if !exists {
		return errors.New("server not connected")
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.Disconnected {
		return errors.New("server already disconnected")
	}

	conn.SquadRcon.Close()
	conn.Disconnected = true

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
	if conn.Disconnected {
		conn.mu.Unlock()
		log.Error().
			Str("serverID", serverID.String()).
			Str("command", command).
			Msg("Server disconnected")
		return "", errors.New("server disconnected")
	}
	conn.LastUsed = time.Now()
	conn.mu.Unlock()

	// Create response channel
	responseChan := make(chan CommandResponse, 1)

	// Send command to command processor
	select {
	case conn.CommandChan <- RconCommand{Command: command, Response: responseChan}:
	case <-time.After(5 * time.Second):
		log.Error().
			Str("serverID", serverID.String()).
			Str("command", command).
			Msg("Command queue full, try again later")
		return "", errors.New("command queue full, try again later")
	}

	// Wait for response
	select {
	case response := <-responseChan:
		return response.Response, response.Error
	case <-time.After(30 * time.Second):
		log.Error().
			Str("serverID", serverID.String()).
			Str("command", command).
			Msg("Command timed out")
		return "", errors.New("command timed out")
	}
}

// processCommands processes commands for a server
func (m *RconManager) processCommands(serverID uuid.UUID, conn *ServerConnection) {
	for {
		select {
		case cmd := <-conn.CommandChan:
			conn.mu.Lock()
			if conn.Disconnected {
				conn.mu.Unlock()
				cmd.Response <- CommandResponse{
					Response: "",
					Error:    errors.New("server disconnected"),
				}
				continue
			}
			conn.LastUsed = time.Now()
			conn.mu.Unlock()

			// Execute command
			response, err := conn.SquadRcon.Rcon.Execute(cmd.Command)

			// Send response
			cmd.Response <- CommandResponse{
				Response: response,
				Error:    err,
			}

		case <-m.ctx.Done():
			return
		}
	}
}

// listenForEvents listens for events from an RCON server
func (m *RconManager) listenForEvents(serverID uuid.UUID, sr *squadRcon.SquadRcon) {
	// Setup event listeners
	sr.Rcon.Emitter.On("CHAT_MESSAGE", func(data interface{}) {
		event := RconEvent{
			ServerID: serverID,
			Type:     "CHAT_MESSAGE",
			Data:     data,
			Time:     time.Now(),
		}

		m.broadcastEvent(event)
	})

	sr.Rcon.Emitter.On("PLAYER_WARNED", func(data interface{}) {
		event := RconEvent{
			ServerID: serverID,
			Type:     "PLAYER_WARNED",
			Data:     data,
			Time:     time.Now(),
		}

		m.broadcastEvent(event)
	})

	sr.Rcon.Emitter.On("PLAYER_KICKED", func(data interface{}) {
		event := RconEvent{
			ServerID: serverID,
			Type:     "PLAYER_KICKED",
			Data:     data,
			Time:     time.Now(),
		}

		m.broadcastEvent(event)
	})

	sr.Rcon.Emitter.On("POSSESSED_ADMIN_CAMERA", func(data interface{}) {
		event := RconEvent{
			ServerID: serverID,
			Type:     "POSSESSED_ADMIN_CAMERA",
			Data:     data,
			Time:     time.Now(),
		}

		m.broadcastEvent(event)
	})

	sr.Rcon.Emitter.On("UNPOSSESSED_ADMIN_CAMERA", func(data interface{}) {
		event := RconEvent{
			ServerID: serverID,
			Type:     "UNPOSSESSED_ADMIN_CAMERA",
			Data:     data,
			Time:     time.Now(),
		}

		m.broadcastEvent(event)
	})

	sr.Rcon.Emitter.On("SQUAD_CREATED", func(data interface{}) {
		event := RconEvent{
			ServerID: serverID,
			Type:     "SQUAD_CREATED",
			Data:     data,
			Time:     time.Now(),
		}

		m.broadcastEvent(event)
	})

	// Listen for connection events
	sr.Rcon.Emitter.On("close", func(data interface{}) {
		event := RconEvent{
			ServerID: serverID,
			Type:     "CONNECTION_CLOSED",
			Data:     nil,
			Time:     time.Now(),
		}

		m.broadcastEvent(event)
	})

	sr.Rcon.Emitter.On("error", func(data interface{}) {
		event := RconEvent{
			ServerID: serverID,
			Type:     "CONNECTION_ERROR",
			Data:     data,
			Time:     time.Now(),
		}

		m.broadcastEvent(event)
	})

	// Block until context is done
	<-m.ctx.Done()
}

// StartConnectionManager starts the connection manager
func (m *RconManager) StartConnectionManager() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupIdleConnections()
		case <-m.ctx.Done():
			m.cleanupAllConnections()
			return
		}
	}
}

// cleanupIdleConnections closes idle connections
func (m *RconManager) cleanupIdleConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	idleTimeout := 30 * time.Minute

	for serverID, conn := range m.connections {
		conn.mu.Lock()
		if !conn.Disconnected && now.Sub(conn.LastUsed) > idleTimeout {
			log.Info().
				Str("serverID", serverID.String()).
				Msg("Closing idle RCON connection")

			conn.SquadRcon.Close()
			conn.Disconnected = true
		}
		conn.mu.Unlock()
	}
}

// cleanupAllConnections closes all connections
func (m *RconManager) cleanupAllConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for serverID, conn := range m.connections {
		conn.mu.Lock()
		if !conn.Disconnected {
			log.Info().
				Str("serverID", serverID.String()).
				Msg("Closing RCON connection during shutdown")

			conn.SquadRcon.Close()
			conn.Disconnected = true
		}
		conn.mu.Unlock()
	}
}

// Shutdown shuts down the RCON manager
func (m *RconManager) Shutdown() {
	m.cancel()
}

// ConnectToAllServers connects to all servers in the database
func (m *RconManager) ConnectToAllServers(ctx context.Context, db *sql.DB) {
	// Get all servers from the database
	rows, err := db.QueryContext(ctx, `
		SELECT id, ip_address, rcon_port, rcon_password
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
		var rconPort int
		var rconPassword string

		if err := rows.Scan(&id, &ipAddress, &rconPort, &rconPassword); err != nil {
			log.Error().Err(err).Msg("Failed to scan server row")
			continue
		}

		// Try to connect to the server
		err := m.ConnectToServer(id, ipAddress, rconPort, rconPassword)
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
func (m *RconManager) ProcessChatMessages(ctx context.Context, messageHandler func(serverID uuid.UUID, message rcon.Message)) {
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
					if message, ok := event.Data.(rcon.Message); ok {
						// Call the message handler
						messageHandler(event.ServerID, message)
					}
				}
			}
		}
	}()
}
