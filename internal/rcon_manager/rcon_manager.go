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
	ServerID             uuid.UUID
	Rcon                 *rcon.Rcon // Single connection for both commands and events
	CommandChan          chan RconCommand
	EventChan            chan RconEvent
	Disconnected         bool
	LastUsed             time.Time
	mu                   sync.Mutex
	cmdSemaphore         chan struct{}
	host                 string
	port                 string
	password             string
	wasForceDisconnected bool
	reconnectAttempts    int
	lastReconnectTime    time.Time
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

// calculateReconnectDelay calculates the delay for reconnection attempts using exponential backoff
// Starting at 5 seconds, doubling each attempt, capped at 1 minute
func (m *RconManager) calculateReconnectDelay(attempts int) time.Duration {
	const (
		baseDelay = 5 * time.Second
		maxDelay  = 60 * time.Second
	)

	if attempts == 0 {
		return 0 // First attempt has no delay
	}

	// Calculate exponential backoff: 5s, 10s, 20s, 40s, 60s (capped)
	delay := baseDelay * time.Duration(1<<uint(attempts-1))
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
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

		// If connection is disconnected, reconnect with backoff
		if conn.Disconnected {
			// Calculate reconnection delay with exponential backoff
			delay := m.calculateReconnectDelay(conn.reconnectAttempts)

			// Check if enough time has passed since last reconnect attempt
			if time.Since(conn.lastReconnectTime) < delay {
				remainingDelay := delay - time.Since(conn.lastReconnectTime)
				log.Debug().
					Str("serverID", serverID.String()).
					Dur("remainingDelay", remainingDelay).
					Int("attempts", conn.reconnectAttempts).
					Msg("Reconnection attempt too soon, waiting")
				return fmt.Errorf("reconnection delayed, try again in %v", remainingDelay)
			}

			conn.reconnectAttempts++
			conn.lastReconnectTime = time.Now()

			if conn.wasForceDisconnected {
				log.Debug().
					Str("serverID", serverID.String()).
					Int("attempts", conn.reconnectAttempts).
					Dur("delay", delay).
					Msg("Reconnecting to force-disconnected RCON server")
			} else {
				log.Debug().
					Str("serverID", serverID.String()).
					Int("attempts", conn.reconnectAttempts).
					Dur("delay", delay).
					Msg("Reconnecting to disconnected RCON server")
			}

			// Create single RCON connection
			rconConn, err := rcon.NewRcon(rcon.RconConfig{
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
					Int("attempts", conn.reconnectAttempts).
					Msg("Failed to connect to RCON")
				return fmt.Errorf("failed to connect to RCON: %w", err)
			}

			conn.Rcon = rconConn
			conn.Disconnected = false
			conn.LastUsed = time.Now()
			// Store connection details
			conn.host = host
			conn.port = portStr
			conn.password = password
			// Reset reconnect attempts on successful connection
			conn.reconnectAttempts = 0
			conn.wasForceDisconnected = false

			// Start listening for events and processing commands
			go m.listenForEvents(serverID, rconConn)
			go m.processCommands(serverID, conn)

			log.Info().
				Str("serverID", serverID.String()).
				Msg("Successfully reconnected to RCON server")

			return nil
		}

		// Connection already exists and is connected
		conn.LastUsed = time.Now()

		return nil
	}

	// Create single RCON connection
	rconConn, err := rcon.NewRcon(rcon.RconConfig{
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
		ServerID:          serverID,
		Rcon:              rconConn,
		CommandChan:       make(chan RconCommand, 100),
		EventChan:         make(chan RconEvent, 100),
		Disconnected:      false,
		LastUsed:          time.Now(),
		cmdSemaphore:      cmdSemaphore,
		host:              host,
		port:              portStr,
		password:          password,
		reconnectAttempts: 0,
		lastReconnectTime: time.Time{},
	}

	m.connections[serverID] = conn

	// Start listening for events and processing commands
	go m.listenForEvents(serverID, rconConn)
	go m.processCommands(serverID, conn)

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

	if conn.Disconnected {
		return errors.New("server already disconnected")
	}

	if force {
		conn.Rcon.Close()
		conn.Disconnected = true
		conn.wasForceDisconnected = true
		return nil
	}

	// Close the connection
	conn.Rcon.Close()
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

	// Remove unnecessary debug log for every command
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
	}

	// Wait for response
	select {
	case response := <-responseChan:
		// Only log debug on errors, not on every command success
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
	}
}

// processCommands processes commands for a server
func (m *RconManager) processCommands(serverID uuid.UUID, conn *ServerConnection) {
	// Log startup once, not for every command processor
	log.Debug().
		Str("serverID", serverID.String()).
		Msg("Starting command processor")

	for {
		select {
		case cmd := <-conn.CommandChan:
			// Acquire the semaphore
			conn.cmdSemaphore <- struct{}{}

			// Check connection status
			conn.mu.Lock()
			if conn.Disconnected {
				conn.mu.Unlock()
				// This is already logged at the outer level
				cmd.Response <- CommandResponse{
					Response: "",
					Error:    errors.New("server disconnected"),
				}
				// Release the semaphore
				<-conn.cmdSemaphore
				continue
			}

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
						Msg("Command execution returned error")
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
			}

			// Send response back to caller
			cmd.Response <- cmdResponse

			// Release the semaphore
			<-conn.cmdSemaphore

			// Remove duplicate logging - already logged in execute command
			// Avoid logging every successful command

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
		log.Warn(). // Use Warn level for unexpected closures
				Str("serverID", serverID.String()).
				Interface("data", data).
				Msg("RCON event connection closed")

		// Mark connection as disconnected and reset reconnect attempts
		m.mu.RLock()
		conn, exists := m.connections[serverID]
		m.mu.RUnlock()

		if exists {
			conn.mu.Lock()
			conn.Disconnected = true
			// Don't reset reconnect attempts here - let them accumulate for backoff
			conn.mu.Unlock()
		}

		updateAndBroadcast("CONNECTION_CLOSED", data)
	})

	sr.Emitter.On("error", func(data interface{}) {
		log.Error(). // Use Error level for connection errors
				Str("serverID", serverID.String()).
				Interface("data", data). // Often the error itself
				Msg("RCON event connection error")

		// Mark connection as disconnected on error
		m.mu.RLock()
		conn, exists := m.connections[serverID]
		m.mu.RUnlock()

		if exists {
			conn.mu.Lock()
			conn.Disconnected = true
			// Don't reset reconnect attempts here - let them accumulate for backoff
			conn.mu.Unlock()
		}

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
		if !conn.Disconnected {
			// A single log for shutdown is better than per-connection logs
			conn.Rcon.Close()
			conn.Disconnected = true
		}
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
