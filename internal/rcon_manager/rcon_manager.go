package rcon_manager

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	rcon "github.com/SquadGO/squad-rcon-go/v2"
	"github.com/SquadGO/squad-rcon-go/v2/rconTypes"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
)

const (
	DefaultCommandTimeout = 30 * time.Second
	HealthCheckInterval   = 120 * time.Second
	ServerInfoInterval    = 60 * time.Second // Collect server info every 60 seconds
	MaxReconnectAttempts  = 3
	CommandQueueSize      = 1000
	EventChannelSize      = 1000
	MaxConcurrentCommands = 1

	// Command priorities
	PriorityLow      = 1
	PriorityNormal   = 5
	PriorityHigh     = 10
	PriorityCritical = 15
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
	Priority int             // Higher values = higher priority
	Timeout  time.Duration   // Per-command timeout override
	Retries  int             // Number of retries for this command
	ctx      context.Context // Command-specific context
}

// CommandResponse represents the response from an RCON command
type CommandResponse struct {
	Response string
	Error    error
}

// ServerConnection represents a connection to an RCON server
type ServerConnection struct {
	ServerID           uuid.UUID
	Rcon               *rcon.Rcon // Single connection for both commands and events
	CommandChan        chan RconCommand
	EventChan          chan RconEvent
	LastUsed           time.Time
	LastHealthCheck    time.Time
	LastServerInfoTime time.Time
	mu                 sync.Mutex
	cmdSemaphore       chan struct{}
	isHealthy          bool
	reconnectCount     int
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

// processShowServerInfoResponse processes a ShowServerInfo response and emits an event
func (m *RconManager) processShowServerInfoResponse(serverID uuid.UUID, response string) {
	// Parse the server info response using Squad's special format
	playerCount, publicQueue, reservedQueue, err := m.parseServerInfoResponse(response)
	if err != nil {
		log.Error().
			Str("serverID", serverID.String()).
			Err(err).
			Str("response", response).
			Msg("Failed to parse ShowServerInfo response")
		return
	}

	// Create and publish the server info event
	serverInfoData := &event_manager.RconServerInfoData{
		PlayerCount:     playerCount,
		PublicQueue:     publicQueue,
		ReservedQueue:   reservedQueue,
		TotalQueueCount: publicQueue + reservedQueue,
	}

	if m.eventManager != nil {
		m.eventManager.PublishEvent(serverID, serverInfoData, response)
	}
}

// parseServerInfoResponse parses the Squad RCON server info response format
func (m *RconManager) parseServerInfoResponse(response string) (playerCount, publicQueue, reservedQueue int, err error) {
	// Create a map to hold the raw JSON data
	var rawData map[string]interface{}
	if err := json.Unmarshal([]byte(response), &rawData); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Helper function to extract integer from field with _I suffix
	extractInt := func(key string) int {
		if value, exists := rawData[key]; exists {
			if strValue, ok := value.(string); ok {
				if intValue, err := strconv.Atoi(strValue); err == nil {
					return intValue
				}
			} else if numValue, ok := value.(float64); ok {
				return int(numValue)
			}
		}
		return 0
	}

	// Extract the specific fields we need
	playerCount = extractInt("PlayerCount_I")
	publicQueue = extractInt("PublicQueue_I")
	reservedQueue = extractInt("ReservedQueue_I")

	return playerCount, publicQueue, reservedQueue, nil
}

// collectServerInfo collects server info and emits a server info event
func (m *RconManager) collectServerInfo(serverID uuid.UUID, conn *ServerConnection) {
	conn.mu.Lock()
	conn.LastServerInfoTime = time.Now()
	conn.mu.Unlock()

	// Execute ShowServerInfo command
	response, err := m.ExecuteCommandWithOptions(serverID, "ShowServerInfo", CommandOptions{
		Priority: PriorityLow,
		Timeout:  10 * time.Second,
		Retries:  1,
	})

	if err != nil {
		log.Error().
			Str("serverID", serverID.String()).
			Err(err).
			Msg("Failed to collect server info")
		return
	}

	// Parse the server info response using Squad's special format
	playerCount, publicQueue, reservedQueue, err := m.parseServerInfoResponse(response)
	if err != nil {
		log.Error().
			Str("serverID", serverID.String()).
			Err(err).
			Str("response", response).
			Msg("Failed to parse server info response")
		return
	}

	// Create and publish the server info event
	serverInfoData := &event_manager.RconServerInfoData{
		PlayerCount:     playerCount,
		PublicQueue:     publicQueue,
		ReservedQueue:   reservedQueue,
		TotalQueueCount: publicQueue + reservedQueue,
	}

	if m.eventManager != nil {
		m.eventManager.PublishEvent(serverID, serverInfoData, response)
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
	cmdSemaphore := make(chan struct{}, MaxConcurrentCommands)

	conn := &ServerConnection{
		ServerID:           serverID,
		Rcon:               rconConn,
		CommandChan:        make(chan RconCommand, CommandQueueSize),
		EventChan:          make(chan RconEvent, EventChannelSize),
		LastUsed:           time.Now(),
		LastHealthCheck:    time.Now(),
		LastServerInfoTime: time.Time{}, // Initialize to zero time so it triggers immediately
		cmdSemaphore:       cmdSemaphore,
		isHealthy:          true,
		reconnectCount:     0,
	}

	m.connections[serverID] = conn

	// Start listening for events and processing commands
	go m.listenForEvents(serverID, rconConn)
	go m.processCommands(serverID, conn)
	go m.monitorConnection(serverID, conn)

	log.Info().
		Str("serverID", serverID.String()).
		Msg("Connected to RCON server")

	return nil
}

// monitorConnection monitors a connection's health and handles reconnection
func (m *RconManager) monitorConnection(serverID uuid.UUID, conn *ServerConnection) {
	// Use a shorter interval to handle both health checks and server info collection
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()

			// Check if health check is needed
			conn.mu.Lock()
			needsHealthCheck := now.Sub(conn.LastHealthCheck) >= HealthCheckInterval
			needsServerInfo := now.Sub(conn.LastServerInfoTime) >= ServerInfoInterval
			conn.mu.Unlock()

			// Perform periodic health check
			if needsHealthCheck {
				if err := m.performHealthCheck(serverID, conn); err != nil {
					log.Warn().
						Str("serverID", serverID.String()).
						Err(err).
						Msg("Periodic health check failed")

					// Attempt reconnection if configured
					conn.mu.Lock()
					if conn.reconnectCount < MaxReconnectAttempts {
						conn.reconnectCount++
						conn.mu.Unlock()

						// TODO: Implement reconnection logic if needed
						log.Info().
							Str("serverID", serverID.String()).
							Int("attempt", conn.reconnectCount).
							Msg("Connection unhealthy, may need reconnection")
					} else {
						conn.mu.Unlock()
					}
				} else {
					// Reset reconnect count on successful health check
					conn.mu.Lock()
					conn.reconnectCount = 0
					conn.mu.Unlock()
				}
			}

			// Collect server info periodically
			if needsServerInfo && conn.isHealthy {
				go m.collectServerInfo(serverID, conn)
			}

		case <-m.ctx.Done():
			log.Debug().
				Str("serverID", serverID.String()).
				Msg("Stopping connection monitor")
			return
		}
	}
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

// ExecuteCommand executes a command on an RCON server with improved error handling and performance
func (m *RconManager) ExecuteCommand(serverID uuid.UUID, command string) (string, error) {
	return m.ExecuteCommandWithOptions(serverID, command, CommandOptions{
		Priority: PriorityNormal,
		Timeout:  DefaultCommandTimeout,
		Retries:  1,
	})
}

// CommandOptions provides configuration for command execution
type CommandOptions struct {
	Priority int
	Timeout  time.Duration
	Retries  int
	Context  context.Context
}

// ExecuteCommandWithOptions executes a command with specific options
func (m *RconManager) ExecuteCommandWithOptions(serverID uuid.UUID, command string, options CommandOptions) (string, error) {
	// Validate input
	if command == "" {
		return "", errors.New("command cannot be empty")
	}

	// Set defaults
	if options.Timeout == 0 {
		options.Timeout = DefaultCommandTimeout
	}
	if options.Retries < 1 {
		options.Retries = 1
	}
	if options.Context == nil {
		options.Context = context.Background()
	}

	// Get connection with health check
	conn, err := m.getHealthyConnection(serverID)
	if err != nil {
		return "", err
	}

	// Update last used time efficiently
	conn.mu.Lock()
	conn.LastUsed = time.Now()
	conn.mu.Unlock()

	// Create command context with timeout
	ctx, cancel := context.WithTimeout(options.Context, options.Timeout)
	defer cancel()

	responseChan := make(chan CommandResponse, 1)
	defer close(responseChan)

	rconCmd := RconCommand{
		Command:  command,
		Response: responseChan,
		Priority: options.Priority,
		Timeout:  options.Timeout,
		Retries:  options.Retries,
		ctx:      ctx,
	}

	// Attempt to send command with retries
	var lastErr error
	for attempt := 0; attempt < options.Retries; attempt++ {
		if attempt > 0 {
			log.Debug().
				Str("serverID", serverID.String()).
				Str("command", command).
				Int("attempt", attempt+1).
				Msg("Retrying command execution")
		}

		// Send command to processor
		select {
		case conn.CommandChan <- rconCmd:
			// Command queued successfully
		case <-ctx.Done():
			lastErr = ctx.Err()
			if lastErr == context.DeadlineExceeded {
				lastErr = errors.New("command queue timeout")
			}
			continue
		case <-m.ctx.Done():
			return "", errors.New("rcon manager shutting down")
		}

		// Wait for response
		select {
		case response := <-responseChan:
			if response.Error != nil {
				lastErr = response.Error
				// Don't retry on certain errors
				if isNonRetryableError(response.Error) {
					break
				}
				continue
			}
			return response.Response, nil
		case <-ctx.Done():
			lastErr = ctx.Err()
			if lastErr == context.DeadlineExceeded {
				lastErr = errors.New("command execution timeout")
				// Mark connection as potentially unhealthy
				m.markConnectionUnhealthy(serverID)
			}
			continue
		case <-m.ctx.Done():
			return "", errors.New("rcon manager shutting down")
		}
	}

	log.Error().
		Str("serverID", serverID.String()).
		Str("command", command).
		Err(lastErr).
		Int("attempts", options.Retries).
		Msg("Command execution failed after all retries")

	return "", fmt.Errorf("command failed after %d attempts: %w", options.Retries, lastErr)
}

// getHealthyConnection returns a healthy connection or error
func (m *RconManager) getHealthyConnection(serverID uuid.UUID) (*ServerConnection, error) {
	m.mu.RLock()
	conn, exists := m.connections[serverID]
	m.mu.RUnlock()

	if !exists {
		return nil, errors.New("server not connected")
	}

	// Check if health check is needed
	conn.mu.Lock()
	needsHealthCheck := time.Since(conn.LastHealthCheck) > HealthCheckInterval || !conn.isHealthy
	conn.mu.Unlock()

	if needsHealthCheck {
		if err := m.performHealthCheck(serverID, conn); err != nil {
			return nil, fmt.Errorf("server unhealthy: %w", err)
		}
	}

	return conn, nil
}

// performHealthCheck performs a health check on the connection
func (m *RconManager) performHealthCheck(serverID uuid.UUID, conn *ServerConnection) error {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	// Simple ping-like command to test connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()

	// Use a lightweight command for health check
	done := make(chan bool, 1)
	var healthErr error

	go func() {
		defer func() {
			if r := recover(); r != nil {
				healthErr = fmt.Errorf("health check panic: %v", r)
			}
			done <- true
		}()

		// Try to execute a simple command
		response := conn.Rcon.Execute("ShowServerInfo")
		if response == "" {
			healthErr = errors.New("empty response from health check")
		}
	}()

	select {
	case <-done:
		// Health check completed
	case <-ctx.Done():
		healthErr = errors.New("health check timeout")
	}

	duration := time.Since(start)

	if healthErr != nil {
		conn.isHealthy = false
		log.Warn().
			Str("serverID", serverID.String()).
			Err(healthErr).
			Dur("duration", duration).
			Msg("Health check failed")
		return healthErr
	}

	conn.isHealthy = true
	conn.LastHealthCheck = time.Now()

	log.Debug().
		Str("serverID", serverID.String()).
		Dur("duration", duration).
		Msg("Health check passed")

	return nil
}

// markConnectionUnhealthy marks a connection as unhealthy
func (m *RconManager) markConnectionUnhealthy(serverID uuid.UUID) {
	m.mu.RLock()
	conn, exists := m.connections[serverID]
	m.mu.RUnlock()

	if exists {
		conn.mu.Lock()
		conn.isHealthy = false
		conn.mu.Unlock()

		log.Warn().
			Str("serverID", serverID.String()).
			Msg("Marked connection as unhealthy")
	}
}

// isNonRetryableError determines if an error should not be retried
func isNonRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	nonRetryableErrors := []string{
		"invalid command",
		"permission denied",
		"authentication failed",
		"server not connected",
		"rcon manager shutting down",
	}

	for _, nonRetryable := range nonRetryableErrors {
		if strings.Contains(strings.ToLower(errStr), nonRetryable) {
			return true
		}
	}

	return false
}

// processCommands processes commands for a server with improved error handling and resource management
func (m *RconManager) processCommands(serverID uuid.UUID, conn *ServerConnection) {
	log.Debug().
		Str("serverID", serverID.String()).
		Msg("Starting command processor")

	defer func() {
		if r := recover(); r != nil {
			log.Error().
				Str("serverID", serverID.String()).
				Interface("panic", r).
				Msg("Command processor panic recovered")
		}
	}()

	for {
		select {
		case cmd := <-conn.CommandChan:
			m.processCommand(serverID, conn, cmd)

		case <-m.ctx.Done():
			log.Debug().
				Str("serverID", serverID.String()).
				Msg("Stopping command processor due to context cancellation")
			return
		}
	}
}

// processCommand processes a single command with proper resource management
func (m *RconManager) processCommand(serverID uuid.UUID, conn *ServerConnection, cmd RconCommand) {
	// Acquire the semaphore to limit concurrent commands
	select {
	case conn.cmdSemaphore <- struct{}{}:
		// Acquired semaphore
	case <-cmd.ctx.Done():
		// Command context cancelled before acquiring semaphore
		select {
		case cmd.Response <- CommandResponse{
			Response: "",
			Error:    cmd.ctx.Err(),
		}:
		default:
		}
		return
	case <-m.ctx.Done():
		// Manager shutting down
		select {
		case cmd.Response <- CommandResponse{
			Response: "",
			Error:    errors.New("rcon manager shutting down"),
		}:
		default:
		}
		return
	}

	// Ensure semaphore is released
	defer func() {
		<-conn.cmdSemaphore
	}()

	// Update last used time
	conn.mu.Lock()
	conn.LastUsed = time.Now()
	conn.mu.Unlock()

	// Execute command with proper error handling and context
	startTime := time.Now()
	responseChan := make(chan CommandResponse, 1)

	// Execute in goroutine with proper cleanup
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().
					Str("serverID", serverID.String()).
					Str("command", cmd.Command).
					Interface("panic", r).
					Msg("Command execution panic recovered")

				select {
				case responseChan <- CommandResponse{
					Response: "",
					Error:    fmt.Errorf("command execution panic: %v", r),
				}:
				default:
				}
			}
		}()

		// Check if connection is still healthy before executing
		if !conn.isHealthy {
			select {
			case responseChan <- CommandResponse{
				Response: "",
				Error:    errors.New("connection is unhealthy"),
			}:
			default:
			}
			return
		}

		// Execute the command
		response := conn.Rcon.Execute(cmd.Command)

		// Handle empty responses more gracefully
		var err error
		if response == "" {
			// Some commands legitimately return empty responses
			// Only consider it an error for certain command types
			if m.shouldHaveResponse(cmd.Command) {
				err = errors.New("empty response received")
				log.Debug().
					Str("serverID", serverID.String()).
					Str("command", cmd.Command).
					Dur("execTime", time.Since(startTime)).
					Msg("Command returned empty response")
			}
		}

		// Check if this was a ShowServerInfo command and process it
		if strings.EqualFold(strings.TrimSpace(cmd.Command), "ShowServerInfo") && response != "" && err == nil {
			go m.processShowServerInfoResponse(serverID, response)
		}

		select {
		case responseChan <- CommandResponse{
			Response: response,
			Error:    err,
		}:
			// Response sent successfully
		default:
			// Channel might be closed or full
			log.Debug().
				Str("serverID", serverID.String()).
				Str("command", cmd.Command).
				Msg("Could not send response, channel unavailable")
		}
	}()

	// Wait for response with proper timeout handling
	var cmdResponse CommandResponse

	select {
	case response := <-responseChan:
		cmdResponse = response

	case <-cmd.ctx.Done():
		// Command-specific timeout
		cmdResponse = CommandResponse{
			Response: "",
			Error:    fmt.Errorf("command timeout: %w", cmd.ctx.Err()),
		}

		// Mark connection as potentially unhealthy if timeout
		if cmd.ctx.Err() == context.DeadlineExceeded {
			conn.mu.Lock()
			conn.isHealthy = false
			conn.mu.Unlock()

			log.Debug().
				Str("serverID", serverID.String()).
				Str("command", cmd.Command).
				Msg("Command timed out, marking connection as unhealthy")
		}

	case <-m.ctx.Done():
		cmdResponse = CommandResponse{
			Response: "",
			Error:    errors.New("rcon manager shutting down"),
		}
	}

	// Send response back to caller with timeout protection
	select {
	case cmd.Response <- cmdResponse:
		// Response sent successfully
	case <-time.After(1 * time.Second):
		// Caller might have given up, log but don't block
		log.Debug().
			Str("serverID", serverID.String()).
			Str("command", cmd.Command).
			Msg("Could not send response to caller, caller may have timed out")
	case <-m.ctx.Done():
		// Manager shutting down, nothing we can do
	}
}

// shouldHaveResponse determines if a command should return a non-empty response
func (m *RconManager) shouldHaveResponse(command string) bool {
	// Commands that typically return empty responses
	emptyResponseCommands := []string{
		"AdminWarn",
		"AdminKick",
		"AdminBan",
		"AdminBroadcast",
		"AdminForceTeamChange",
		"AdminEndMatch",
		"AdminSetMaxNumPlayers",
		"AdminSlomo",
	}

	commandLower := strings.ToLower(command)
	for _, emptyCmd := range emptyResponseCommands {
		if strings.HasPrefix(commandLower, strings.ToLower(emptyCmd)) {
			return false
		}
	}

	return true
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

// Convenience methods for common operations

// ExecuteCommandAsync executes a command asynchronously and returns immediately
func (m *RconManager) ExecuteCommandAsync(serverID uuid.UUID, command string, callback func(string, error)) {
	go func() {
		response, err := m.ExecuteCommand(serverID, command)
		if callback != nil {
			callback(response, err)
		}
	}()
}

// ExecuteHighPriorityCommand executes a command with high priority
func (m *RconManager) ExecuteHighPriorityCommand(serverID uuid.UUID, command string) (string, error) {
	return m.ExecuteCommandWithOptions(serverID, command, CommandOptions{
		Priority: PriorityHigh,
		Timeout:  DefaultCommandTimeout,
		Retries:  2,
	})
}

// ExecuteCriticalCommand executes a command with critical priority and extended timeout
func (m *RconManager) ExecuteCriticalCommand(serverID uuid.UUID, command string) (string, error) {
	return m.ExecuteCommandWithOptions(serverID, command, CommandOptions{
		Priority: PriorityCritical,
		Timeout:  60 * time.Second,
		Retries:  3,
	})
}

// ExecuteCommandWithTimeout executes a command with a specific timeout
func (m *RconManager) ExecuteCommandWithTimeout(serverID uuid.UUID, command string, timeout time.Duration) (string, error) {
	return m.ExecuteCommandWithOptions(serverID, command, CommandOptions{
		Priority: PriorityNormal,
		Timeout:  timeout,
		Retries:  1,
	})
}

// GetConnectionStats returns statistics about a connection
func (m *RconManager) GetConnectionStats(serverID uuid.UUID) (ConnectionStats, error) {
	m.mu.RLock()
	conn, exists := m.connections[serverID]
	m.mu.RUnlock()

	if !exists {
		return ConnectionStats{}, errors.New("server not connected")
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	return ConnectionStats{
		ServerID:        serverID,
		LastUsed:        conn.LastUsed,
		LastHealthCheck: conn.LastHealthCheck,
		IsHealthy:       conn.isHealthy,
		ReconnectCount:  conn.reconnectCount,
		QueueLength:     len(conn.CommandChan),
	}, nil
}

// ConnectionStats represents statistics about a connection
type ConnectionStats struct {
	ServerID        uuid.UUID
	LastUsed        time.Time
	LastHealthCheck time.Time
	IsHealthy       bool
	ReconnectCount  int
	QueueLength     int
}
