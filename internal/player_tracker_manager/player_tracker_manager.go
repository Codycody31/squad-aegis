package player_tracker_manager

import (
	"context"
	"database/sql"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/player_tracker"
	"go.codycody31.dev/squad-aegis/internal/rcon_manager"
	valkeyClient "go.codycody31.dev/squad-aegis/internal/valkey"
)

// PlayerTrackerManager manages player trackers for multiple servers
type PlayerTrackerManager struct {
	trackers     map[uuid.UUID]*player_tracker.PlayerTracker
	rconManager  *rcon_manager.RconManager
	eventManager *event_manager.EventManager
	valkeyClient *valkeyClient.Client
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewPlayerTrackerManager creates a new player tracker manager
func NewPlayerTrackerManager(ctx context.Context, rconManager *rcon_manager.RconManager, eventManager *event_manager.EventManager, valkeyClient *valkeyClient.Client) *PlayerTrackerManager {
	ctx, cancel := context.WithCancel(ctx)

	return &PlayerTrackerManager{
		trackers:     make(map[uuid.UUID]*player_tracker.PlayerTracker),
		rconManager:  rconManager,
		eventManager: eventManager,
		valkeyClient: valkeyClient,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// CreateTrackerForServer creates and starts a player tracker for a server
func (ptm *PlayerTrackerManager) CreateTrackerForServer(serverID uuid.UUID) error {
	ptm.mu.Lock()
	defer ptm.mu.Unlock()

	// Check if tracker already exists
	if _, exists := ptm.trackers[serverID]; exists {
		log.Debug().Str("serverID", serverID.String()).Msg("Player tracker already exists for server")
		return nil
	}

	// Create new player tracker
	tracker := player_tracker.NewPlayerTracker(ptm.ctx, serverID, ptm.rconManager, ptm.eventManager, ptm.valkeyClient)

	// Start the tracker
	if err := tracker.Start(); err != nil {
		log.Error().Err(err).Str("serverID", serverID.String()).Msg("Failed to start player tracker")
		return err
	}

	ptm.trackers[serverID] = tracker

	log.Info().Str("serverID", serverID.String()).Msg("Player tracker created and started for server")
	return nil
}

// GetTracker returns the player tracker for a server
func (ptm *PlayerTrackerManager) GetTracker(serverID uuid.UUID) (*player_tracker.PlayerTracker, bool) {
	ptm.mu.RLock()
	defer ptm.mu.RUnlock()

	tracker, exists := ptm.trackers[serverID]
	return tracker, exists
}

// RemoveTracker removes and stops a player tracker for a server
func (ptm *PlayerTrackerManager) RemoveTracker(serverID uuid.UUID) error {
	ptm.mu.Lock()
	defer ptm.mu.Unlock()

	tracker, exists := ptm.trackers[serverID]
	if !exists {
		return nil // Already removed
	}

	// Stop the tracker
	tracker.Stop()

	// Remove from map
	delete(ptm.trackers, serverID)

	log.Info().Str("serverID", serverID.String()).Msg("Player tracker removed for server")
	return nil
}

// ConnectToAllServers creates player trackers for all servers in the database
func (ptm *PlayerTrackerManager) ConnectToAllServers(ctx context.Context, db *sql.DB) {
	// Get all servers from the database
	rows, err := db.QueryContext(ctx, `
		SELECT id
		FROM servers
		WHERE rcon_port > 0 AND rcon_password != ''
	`)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query servers for player tracker connections")
		return
	}
	defer rows.Close()

	// Create player trackers for each server
	for rows.Next() {
		var serverID uuid.UUID

		if err := rows.Scan(&serverID); err != nil {
			log.Error().Err(err).Msg("Failed to scan server ID for player tracker")
			continue
		}

		// Create tracker for this server
		if err := ptm.CreateTrackerForServer(serverID); err != nil {
			log.Warn().
				Err(err).
				Str("serverID", serverID.String()).
				Msg("Failed to create player tracker for server")
			continue
		}
	}

	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating server rows for player trackers")
	}
}

// GetAllTrackers returns all active player trackers
func (ptm *PlayerTrackerManager) GetAllTrackers() map[uuid.UUID]*player_tracker.PlayerTracker {
	ptm.mu.RLock()
	defer ptm.mu.RUnlock()

	// Return a copy to avoid race conditions
	trackersCopy := make(map[uuid.UUID]*player_tracker.PlayerTracker)
	for serverID, tracker := range ptm.trackers {
		trackersCopy[serverID] = tracker
	}

	return trackersCopy
}

// GetStats returns statistics for all player trackers
func (ptm *PlayerTrackerManager) GetStats() map[string]interface{} {
	ptm.mu.RLock()
	defer ptm.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["tracker_count"] = len(ptm.trackers)

	serverStats := make(map[string]interface{})
	for serverID, tracker := range ptm.trackers {
		serverStats[serverID.String()] = tracker.GetStats()
	}
	stats["servers"] = serverStats

	return stats
}

// Shutdown gracefully shuts down all player trackers
func (ptm *PlayerTrackerManager) Shutdown() {
	log.Info().Msg("Shutting down player tracker manager...")

	ptm.cancel()

	ptm.mu.Lock()
	defer ptm.mu.Unlock()

	// Stop all trackers
	for serverID, tracker := range ptm.trackers {
		tracker.Stop()
		log.Debug().Str("serverID", serverID.String()).Msg("Player tracker stopped")
	}

	// Clear the map
	ptm.trackers = make(map[uuid.UUID]*player_tracker.PlayerTracker)

	log.Info().Msg("Player tracker manager shutdown complete")
}
