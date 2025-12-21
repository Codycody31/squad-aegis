package logwatcher_manager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/player_tracker_manager"
	valkeyClient "go.codycody31.dev/squad-aegis/internal/valkey"
)

// ServerLogConnection represents a log connection to a server
type ServerLogConnection struct {
	ServerID          uuid.UUID
	LogSource         LogSource
	Config            LogSourceConfig
	EventStore        EventStoreInterface
	Metrics           *LogParsingMetrics
	Connected         bool
	LastUsed          time.Time
	mu                sync.Mutex
	cancel            context.CancelFunc
	reconnectAttempts int
	lastReconnectTime time.Time
}

// LogwatcherManager manages log connections to multiple servers
type LogwatcherManager struct {
	connections          map[uuid.UUID]*ServerLogConnection
	eventManager         *event_manager.EventManager
	parsers              []LogParser
	valkeyClient         *valkeyClient.Client
	playerTrackerManager *player_tracker_manager.PlayerTrackerManager
	mu                   sync.RWMutex
	ctx                  context.Context
	cancel               context.CancelFunc
}

// NewLogwatcherManager creates a new logwatcher manager
func NewLogwatcherManager(ctx context.Context, eventManager *event_manager.EventManager, valkeyClient *valkeyClient.Client, playerTrackerManager *player_tracker_manager.PlayerTrackerManager) *LogwatcherManager {
	ctx, cancel := context.WithCancel(ctx)

	return &LogwatcherManager{
		connections:          make(map[uuid.UUID]*ServerLogConnection),
		eventManager:         eventManager,
		parsers:              GetOptimizedLogParsers(), // Use the unified parsers
		valkeyClient:         valkeyClient,
		playerTrackerManager: playerTrackerManager,
		ctx:                  ctx,
		cancel:               cancel,
	}
}

// ConnectToServer connects to a server's log source
func (m *LogwatcherManager) ConnectToServer(serverID uuid.UUID, config LogSourceConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if connection already exists
	if conn, exists := m.connections[serverID]; exists {
		conn.mu.Lock()
		defer conn.mu.Unlock()

		// If connection is disconnected, try to reconnect with backoff
		if !conn.Connected {
			// Calculate reconnection delay with exponential backoff
			delay := m.calculateReconnectDelay(conn.reconnectAttempts)

			// Check if enough time has passed since last reconnect attempt
			if time.Since(conn.lastReconnectTime) < delay {
				remainingDelay := delay - time.Since(conn.lastReconnectTime)
				log.Debug().
					Str("serverID", serverID.String()).
					Dur("remainingDelay", remainingDelay).
					Int("attempts", conn.reconnectAttempts).
					Msg("Log reconnection attempt too soon, waiting")
				return fmt.Errorf("reconnection delayed, try again in %v", remainingDelay)
			}

			conn.reconnectAttempts++
			conn.lastReconnectTime = time.Now()

			log.Debug().
				Str("serverID", serverID.String()).
				Int("attempts", conn.reconnectAttempts).
				Dur("delay", delay).
				Msg("Reconnecting to log source")

			// Create new log source
			logSource, err := m.createLogSource(config)
			if err != nil {
				log.Error().
					Str("serverID", serverID.String()).
					Err(err).
					Int("attempts", conn.reconnectAttempts).
					Msg("Failed to reconnect to log source")
				return fmt.Errorf("failed to reconnect to log source: %w", err)
			}

			// Close old source if it exists
			if conn.LogSource != nil {
				conn.LogSource.Close()
			}

			conn.LogSource = logSource
			conn.Config = config
			conn.Connected = true
			conn.LastUsed = time.Now()
			// Reset reconnect attempts on successful connection
			conn.reconnectAttempts = 0

			// Start watching logs
			go m.watchLogs(m.ctx, serverID, conn)

			log.Info().
				Str("serverID", serverID.String()).
				Msg("Successfully reconnected to log source")

			return nil
		}

		// Connection already exists and is connected
		conn.LastUsed = time.Now()
		return nil
	}

	// Create new log source
	logSource, err := m.createLogSource(config)
	if err != nil {
		log.Error().
			Str("serverID", serverID.String()).
			Err(err).
			Msg("Failed to create log source")
		return fmt.Errorf("failed to create log source: %w", err)
	}

	// Create connection context
	ctx, cancel := context.WithCancel(m.ctx)

	conn := &ServerLogConnection{
		ServerID:          serverID,
		LogSource:         logSource,
		Config:            config,
		EventStore:        NewEventStore(serverID, m.valkeyClient),
		Metrics:           NewLogParsingMetrics(),
		Connected:         true,
		LastUsed:          time.Now(),
		cancel:            cancel,
		reconnectAttempts: 0,
		lastReconnectTime: time.Time{},
	}

	m.connections[serverID] = conn

	// Start watching logs
	go m.watchLogs(ctx, serverID, conn)

	log.Info().
		Str("serverID", serverID.String()).
		Str("sourceType", string(config.Type)).
		Msg("Connected to log source")

	return nil
}

// DisconnectFromServer disconnects from a server's log source
func (m *LogwatcherManager) DisconnectFromServer(serverID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, exists := m.connections[serverID]
	if !exists {
		return errors.New("server log connection not found")
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	if !conn.Connected {
		return errors.New("server log connection already disconnected")
	}

	// Cancel the context to stop log watching
	conn.cancel()

	// Close the log source
	if conn.LogSource != nil {
		conn.LogSource.Close()
	}

	conn.Connected = false

	log.Info().
		Str("serverID", serverID.String()).
		Msg("Disconnected from log source")

	return nil
}

// createLogSource creates a log source based on configuration
func (m *LogwatcherManager) createLogSource(config LogSourceConfig) (LogSource, error) {
	switch config.Type {
	case LogSourceTypeLocal:
		if config.FilePath == "" {
			return nil, errors.New("file path is required for local log source")
		}
		return NewLocalFileSource(config.FilePath), nil

	case LogSourceTypeSFTP:
		if config.Host == "" || config.Username == "" || config.FilePath == "" {
			return nil, errors.New("host, username, and file path are required for SFTP log source")
		}
		if config.Password == "" {
			return nil, errors.New("password is required for SFTP log source")
		}
		if config.Port == 0 {
			config.Port = 22 // Default SFTP port
		}
		if config.PollFrequency == 0 {
			config.PollFrequency = 5 * time.Second // Default poll frequency
		}
		return NewSFTPSource(config.Host, config.Port, config.Username, config.Password,
			config.FilePath, config.PollFrequency, config.ReadFromStart), nil

	case LogSourceTypeFTP:
		if config.Host == "" || config.Username == "" || config.Password == "" || config.FilePath == "" {
			return nil, errors.New("host, username, password, and file path are required for FTP log source")
		}
		if config.Port == 0 {
			config.Port = 21 // Default FTP port
		}
		if config.PollFrequency == 0 {
			config.PollFrequency = 5 * time.Second // Default poll frequency
		}
		return NewFTPSource(config.Host, config.Port, config.Username, config.Password,
			config.FilePath, config.PollFrequency, config.ReadFromStart), nil

	default:
		return nil, fmt.Errorf("unsupported log source type: %s", config.Type)
	}
}

// watchLogs watches logs from a server and processes events
func (m *LogwatcherManager) watchLogs(ctx context.Context, serverID uuid.UUID, conn *ServerLogConnection) {
	log.Debug().
		Str("serverID", serverID.String()).
		Msg("Starting log watcher")

	defer func() {
		log.Debug().
			Str("serverID", serverID.String()).
			Msg("Log watcher stopped")
	}()

	// Start watching logs
	logChan, err := conn.LogSource.Watch(ctx)
	if err != nil {
		log.Error().
			Str("serverID", serverID.String()).
			Err(err).
			Msg("Failed to start watching logs")

		// Mark connection as disconnected
		conn.mu.Lock()
		conn.Connected = false
		conn.mu.Unlock()
		return
	}

	// Process log lines
	for {
		select {
		case <-ctx.Done():
			return
		case logLine, ok := <-logChan:
			if !ok {
				// Channel closed, connection lost
				log.Warn().
					Str("serverID", serverID.String()).
					Msg("Log channel closed, connection lost")

				conn.mu.Lock()
				conn.Connected = false
				conn.mu.Unlock()
				return
			}

			// Update last used time
			conn.mu.Lock()
			conn.LastUsed = time.Now()
			conn.mu.Unlock()

			// Process the log line for events
			// For now, pass nil as playerTracker since we don't have per-server player tracking yet
			tracker, exists := m.playerTrackerManager.GetTracker(serverID)
			if exists {
				ProcessLogForEventsWithMetrics(logLine, serverID, m.parsers, m.eventManager, conn.EventStore, tracker, conn.Metrics)
			} else {
				ProcessLogForEventsWithMetrics(logLine, serverID, m.parsers, m.eventManager, conn.EventStore, nil, conn.Metrics)
			}
		}
	}
}

// calculateReconnectDelay calculates the delay for reconnection attempts using exponential backoff
func (m *LogwatcherManager) calculateReconnectDelay(attempts int) time.Duration {
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

// ConnectToAllServers connects to all servers in the database that have log configuration
func (m *LogwatcherManager) ConnectToAllServers(ctx context.Context, db *sql.DB) {
	// Get all servers from the database with log configuration
	rows, err := db.QueryContext(ctx, `
		SELECT id, log_source_type, log_file_path, log_host, log_port, log_username,
		       log_password, log_poll_frequency, log_read_from_start
		FROM servers
		WHERE log_source_type IS NOT NULL AND log_file_path IS NOT NULL AND log_file_path != ''
	`)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query servers for log connections")
		return
	}
	defer rows.Close()

	// Connect to each server
	for rows.Next() {
		var id uuid.UUID
		var logSourceType, logFilePath *string
		var logHost, logUsername, logPassword *string
		var logPort *int
		var logPollFrequency *int // in seconds
		var logReadFromStart *bool

		if err := rows.Scan(&id, &logSourceType, &logFilePath, &logHost, &logPort,
			&logUsername, &logPassword, &logPollFrequency, &logReadFromStart); err != nil {
			log.Error().Err(err).Msg("Failed to scan server log configuration")
			continue
		}

		// Skip if essential fields are missing
		if logSourceType == nil || logFilePath == nil {
			continue
		}

		// Build log source config
		config := LogSourceConfig{
			Type:          LogSourceType(*logSourceType),
			FilePath:      *logFilePath,
			ReadFromStart: false, // Default value
		}

		if logHost != nil {
			config.Host = *logHost
		}
		if logPort != nil {
			config.Port = *logPort
		}
		if logUsername != nil {
			config.Username = *logUsername
		}
		if logPassword != nil {
			config.Password = *logPassword
		}
		if logPollFrequency != nil {
			config.PollFrequency = time.Duration(*logPollFrequency) * time.Second
		}
		if logReadFromStart != nil {
			config.ReadFromStart = *logReadFromStart
		}

		// Try to connect to the server
		err := m.ConnectToServer(id, config)
		if err != nil {
			log.Warn().
				Err(err).
				Str("serverID", id.String()).
				Str("sourceType", string(config.Type)).
				Msg("Failed to connect to server log source")
			continue
		}

		log.Info().
			Str("serverID", id.String()).
			Str("sourceType", string(config.Type)).
			Msg("Connected to server log source")
	}

	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating server log configuration rows")
	}
}

// GetConnectionStats returns statistics about log connections
func (m *LogwatcherManager) GetConnectionStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	connectedCount := 0
	disconnectedCount := 0
	sourceTypes := make(map[string]int)
	serverMetrics := make(map[string]interface{})

	totalLinesPerMinute := 0.0
	totalMatchingLinesPerMinute := 0.0
	totalMatchingLatency := 0.0
	activeConnections := 0

	for serverID, conn := range m.connections {
		conn.mu.Lock()
		if conn.Connected {
			connectedCount++

			// Get metrics for this connection
			if conn.Metrics != nil {
				metrics := conn.Metrics.GetMetrics()
				serverMetrics[serverID.String()] = metrics

				// Aggregate metrics
				if lpm, ok := metrics["linesPerMinute"].(float64); ok {
					totalLinesPerMinute += lpm
				}
				if mlpm, ok := metrics["matchingLinesPerMinute"].(float64); ok {
					totalMatchingLinesPerMinute += mlpm
				}
				if ml, ok := metrics["matchingLatency"].(float64); ok && ml > 0 {
					totalMatchingLatency += ml
					activeConnections++
				}
			}
		} else {
			disconnectedCount++
		}
		sourceTypes[string(conn.Config.Type)]++
		conn.mu.Unlock()
	}

	// Calculate average matching latency
	averageMatchingLatency := 0.0
	if activeConnections > 0 {
		averageMatchingLatency = totalMatchingLatency / float64(activeConnections)
	}

	return map[string]interface{}{
		"total_connections":        len(m.connections),
		"connected_connections":    connectedCount,
		"disconnected_connections": disconnectedCount,
		"source_types":             sourceTypes,
		"server_metrics":           serverMetrics,
		"aggregate_metrics": map[string]interface{}{
			"total_lines_per_minute":          totalLinesPerMinute,
			"total_matching_lines_per_minute": totalMatchingLinesPerMinute,
			"average_matching_latency":        averageMatchingLatency,
		},
	}
}

// GetServerMetrics returns parsing metrics for a specific server
func (m *LogwatcherManager) GetServerMetrics(serverID uuid.UUID) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, exists := m.connections[serverID]
	if !exists {
		return nil, errors.New("server connection not found")
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	if !conn.Connected || conn.Metrics == nil {
		return nil, errors.New("server not connected or metrics not available")
	}

	return conn.Metrics.GetMetrics(), nil
}

// StartConnectionManager starts the connection manager
func (m *LogwatcherManager) StartConnectionManager() {
	log.Info().Msg("Logwatcher connection manager started")
	<-m.ctx.Done()
	m.cleanupAllConnections()
	log.Info().Msg("Logwatcher connection manager stopped")
}

// cleanupAllConnections closes all connections
func (m *LogwatcherManager) cleanupAllConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for serverID, conn := range m.connections {
		conn.mu.Lock()
		if conn.Connected {
			conn.cancel()
			if conn.LogSource != nil {
				conn.LogSource.Close()
			}
			conn.Connected = false
		}
		conn.mu.Unlock()

		log.Debug().
			Str("serverID", serverID.String()).
			Msg("Closed log connection during shutdown")
	}

	log.Info().Msg("All log connections closed during shutdown")
}

// Shutdown shuts down the logwatcher manager
func (m *LogwatcherManager) Shutdown() {
	log.Info().Msg("Shutting down logwatcher manager...")
	m.cancel()
	log.Info().Msg("Logwatcher manager shutdown complete")
}
