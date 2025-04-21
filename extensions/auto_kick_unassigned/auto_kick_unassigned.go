package auto_kick_unassigned

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/extension_manager"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
	"go.codycody31.dev/squad-aegis/shared/plug_config_schema"
)

// PlayerTracker tracks an unassigned player
type PlayerTracker struct {
	EosID       string
	Name        string
	StartTime   time.Time
	Warnings    int
	WarnTimerID *time.Ticker
	KickTimerID *time.Timer
	StopChan    chan struct{}
}

// AutoKickManager manages state for the extension
type AutoKickManager struct {
	mu                  sync.RWMutex
	BetweenRounds       bool
	TrackedPlayers      map[string]*PlayerTracker // Keyed by EOS ID
	PlayerListTicker    *time.Ticker
	CleanupTicker       *time.Ticker
	StopChan            chan struct{}
	AdminPermission     string
	WhitelistPermission string
}

// Initialize initializes the extension and sets up timers
func (e *AutoKickUnassignedExtension) Initialize(config map[string]interface{}, deps *extension_manager.Dependencies) error {
	// Set the base extension properties
	e.Definition = Define()
	e.Config = config
	e.Deps = deps

	// Validate config
	if err := e.Definition.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Fill defaults
	e.Definition.ConfigSchema.FillDefaults(config)

	// Create and setup the manager
	mgr := &AutoKickManager{
		TrackedPlayers:      make(map[string]*PlayerTracker),
		BetweenRounds:       false,
		StopChan:            make(chan struct{}),
		AdminPermission:     "canseeadminchat", // Permission for admins
		WhitelistPermission: "reserve",         // Permission for whitelist/reserved slots
	}

	// Store the manager in the extension for later use
	e.Manager = mgr

	// Set up recurring tasks
	trackingFrequency := 1 * time.Minute
	cleanupFrequency := 20 * time.Minute

	// Start the player list update ticker
	mgr.PlayerListTicker = time.NewTicker(trackingFrequency)
	go func() {
		for {
			select {
			case <-mgr.PlayerListTicker.C:
				e.updateTrackingList(false)
			case <-mgr.StopChan:
				return
			}
		}
	}()

	// Start the cleanup ticker to remove disconnected players
	mgr.CleanupTicker = time.NewTicker(cleanupFrequency)
	go func() {
		for {
			select {
			case <-mgr.CleanupTicker.C:
				e.clearDisconnectedPlayers()
			case <-mgr.StopChan:
				return
			}
		}
	}()

	return nil
}

// Shutdown stops the extension's background tasks
func (e *AutoKickUnassignedExtension) Shutdown() error {
	// Ensure Manager is initialized before accessing its fields
	if e.Manager == nil {
		log.Warn().Str("extension", e.Definition.ID).Msg("Shutdown called on uninitialized extension")
		return nil // Or return an error? For now, just log and return.
	}

	mgr := e.Manager.(*AutoKickManager)

	mgr.mu.Lock() // Lock the manager's mutex
	defer mgr.mu.Unlock()

	// Stop timers first (if they exist)
	if mgr.PlayerListTicker != nil {
		mgr.PlayerListTicker.Stop()
	}
	if mgr.CleanupTicker != nil {
		mgr.CleanupTicker.Stop()
	}

	// Stop tracking timers for all tracked players
	for eosID, tracker := range mgr.TrackedPlayers {
		if tracker.WarnTimerID != nil {
			tracker.WarnTimerID.Stop()
		}
		if tracker.KickTimerID != nil {
			tracker.KickTimerID.Stop()
		}
		// Also close the tracker's StopChan if it exists
		if tracker.StopChan != nil {
			close(tracker.StopChan)
			tracker.StopChan = nil
		}
		log.Debug().Str("eosID", eosID).Msg("Stopped timers for tracked player during shutdown")
	}
	mgr.TrackedPlayers = make(map[string]*PlayerTracker) // Clear tracked players

	// Check if manager's stopChan exists and is not nil before attempting to close
	ch := mgr.StopChan // Temporarily store the channel
	if ch != nil {
		mgr.StopChan = nil // Set manager's field to nil *before* closing
		close(ch)          // Close the stored channel
		log.Debug().Str("extension", e.Definition.ID).Msg("Manager stopChan closed successfully in Shutdown")
	} else {
		log.Debug().Str("extension", e.Definition.ID).Msg("Shutdown called but manager stopChan was already nil")
	}

	return nil
}

// handleNewGame handles new game events
func (e *AutoKickUnassignedExtension) handleNewGame(data interface{}) error {
	mgr := e.Manager.(*AutoKickManager)

	// Set between rounds flag
	mgr.mu.Lock()
	mgr.BetweenRounds = true
	mgr.mu.Unlock()

	// Update tracking list to clear all tracked players
	e.updateTrackingList(false)

	// Set timer to reset between rounds flag after grace period
	roundStartDelay := plug_config_schema.GetIntValue(e.Config, "round_start_delay")
	time.AfterFunc(time.Duration(roundStartDelay)*time.Second, func() {
		mgr.mu.Lock()
		mgr.BetweenRounds = false
		mgr.mu.Unlock()
	})

	return nil
}

// handlePlayerSquadChange handles player squad change events
func (e *AutoKickUnassignedExtension) handlePlayerSquadChange(data interface{}) error {
	// Extract event data
	eventMap, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid event data format")
	}

	// Extract the event data JSON string
	eventDataStr, ok := eventMap["Data"].(string)
	if !ok {
		return fmt.Errorf("event data is not a string")
	}

	// Parse the JSON data into a map
	var eventData map[string]interface{}
	if err := json.Unmarshal([]byte(eventDataStr), &eventData); err != nil {
		return fmt.Errorf("failed to parse event data: %w", err)
	}

	// Get the player's EOS ID and squad ID
	playerEosID, _ := eventData["playerEos"].(string)
	squadID, _ := eventData["squadID"].(string)

	if playerEosID == "" {
		return fmt.Errorf("missing player EOS ID in squad change event")
	}

	mgr := e.Manager.(*AutoKickManager)

	mgr.mu.RLock()
	_, isTracked := mgr.TrackedPlayers[playerEosID]
	mgr.mu.RUnlock()

	// If player is tracked and joined a squad, untrack them
	if isTracked && squadID != "" && squadID != "null" {
		e.untrackPlayer(playerEosID)
	}

	return nil
}

// updateTrackingList updates the list of tracked players
func (e *AutoKickUnassignedExtension) updateTrackingList(forceUpdate bool) error {
	mgr := e.Manager.(*AutoKickManager)

	mgr.mu.RLock()
	betweenRounds := mgr.BetweenRounds
	mgr.mu.RUnlock()

	// Get server players
	r := squadRcon.NewSquadRcon(e.Deps.RconManager, e.Deps.Server.Id)
	players, err := r.GetServerPlayers()
	if err != nil {
		log.Error().
			Err(err).
			Str("serverID", e.Deps.Server.Id.String()).
			Msg("Failed to get server players")
		return err
	}

	playerThreshold := plug_config_schema.GetIntValue(e.Config, "player_threshold")
	ignoreAdmins := plug_config_schema.GetBoolValue(e.Config, "ignore_admins")
	ignoreWhitelist := plug_config_schema.GetBoolValue(e.Config, "ignore_whitelist")

	// Check if we should run the update
	run := !(betweenRounds || (playerThreshold > 0 && len(players.OnlinePlayers) < playerThreshold))

	log.Debug().
		Str("extension", "auto_kick_unassigned").
		Bool("run", run).
		Bool("betweenRounds", betweenRounds).
		Int("playerCount", len(players.OnlinePlayers)).
		Int("threshold", playerThreshold).
		Msg("Update tracking list check")

	// If not running, untrack all players
	if !run {
		mgr.mu.Lock()
		for eosID := range mgr.TrackedPlayers {
			e.untrackPlayer(eosID)
		}
		mgr.mu.Unlock()
		return nil
	}

	// Get admins and whitelist players
	var admins []string
	var whitelist []string

	// TODO: Implement admin and whitelist permission checking from server list
	// This is a placeholder until we have access to the actual admin list

	// Track unassigned players
	for _, player := range players.OnlinePlayers {
		isTracked := false

		mgr.mu.RLock()
		_, isTracked = mgr.TrackedPlayers[player.EosId]
		mgr.mu.RUnlock()

		isUnassigned := player.SquadId == 0 // Assuming 0 means unassigned
		isAdmin := contains(admins, player.EosId)
		isWhitelisted := contains(whitelist, player.EosId)

		// If player is in a squad and tracked, untrack them
		if !isUnassigned && isTracked {
			e.untrackPlayer(player.EosId)
			continue
		}

		// Skip if player is in a squad
		if !isUnassigned {
			continue
		}

		// Log and skip admins if configured
		if isAdmin {
			log.Debug().
				Str("extension", "auto_kick_unassigned").
				Str("player", player.Name).
				Str("eosID", player.EosId).
				Msg("Admin is unassigned")

			if ignoreAdmins {
				continue
			}
		}

		// Log and skip whitelisted players if configured
		if isWhitelisted {
			log.Debug().
				Str("extension", "auto_kick_unassigned").
				Str("player", player.Name).
				Str("eosID", player.EosId).
				Msg("Whitelisted player is unassigned")

			if ignoreWhitelist {
				continue
			}
		}

		// Start tracking player if not already tracked
		if !isTracked {
			e.trackPlayer(player.EosId, player.Name)
		}
	}

	return nil
}

// clearDisconnectedPlayers removes players who have disconnected from the tracking list
func (e *AutoKickUnassignedExtension) clearDisconnectedPlayers() error {
	mgr := e.Manager.(*AutoKickManager)

	// Get server players
	r := squadRcon.NewSquadRcon(e.Deps.RconManager, e.Deps.Server.Id)
	players, err := r.GetServerPlayers()
	if err != nil {
		log.Error().
			Err(err).
			Str("serverID", e.Deps.Server.Id.String()).
			Msg("Failed to get server players")
		return err
	}

	// Build map of connected players for fast lookup
	connectedPlayers := make(map[string]bool)
	for _, player := range players.OnlinePlayers {
		connectedPlayers[player.EosId] = true
	}

	// Untrack players that are no longer connected
	mgr.mu.Lock()
	for eosID := range mgr.TrackedPlayers {
		if !connectedPlayers[eosID] {
			e.untrackPlayer(eosID)
		}
	}
	mgr.mu.Unlock()

	return nil
}

// trackPlayer starts tracking an unassigned player
func (e *AutoKickUnassignedExtension) trackPlayer(eosID, playerName string) {
	mgr := e.Manager.(*AutoKickManager)

	log.Debug().
		Str("extension", "auto_kick_unassigned").
		Str("player", playerName).
		Str("eosID", eosID).
		Msg("Tracking unassigned player")

	warningFrequency := plug_config_schema.GetIntValue(e.Config, "frequency_of_warnings")
	kickTimeout := plug_config_schema.GetIntValue(e.Config, "unassigned_timer")
	warningMessage := plug_config_schema.GetStringValue(e.Config, "warning_message")
	kickMessage := plug_config_schema.GetStringValue(e.Config, "kick_message")

	// Create a tracker for the player
	tracker := &PlayerTracker{
		EosID:     eosID,
		Name:      playerName,
		StartTime: time.Now(),
		Warnings:  0,
		StopChan:  make(chan struct{}),
	}

	// Create a warning ticker
	warningInterval := time.Duration(warningFrequency) * time.Second
	tracker.WarnTimerID = time.NewTicker(warningInterval)

	// Start warning goroutine
	go func() {
		for {
			select {
			case <-tracker.WarnTimerID.C:
				// Calculate time left
				elapsedTime := time.Since(tracker.StartTime)
				timeLeft := time.Duration(kickTimeout)*time.Second - elapsedTime
				timeLeftStr := formatDuration(timeLeft)

				// Stop timer on last warning
				if timeLeft < warningInterval {
					tracker.WarnTimerID.Stop()
				}

				// Warn the player
				r := squadRcon.NewSquadRcon(e.Deps.RconManager, e.Deps.Server.Id)
				_, err := r.ExecuteRaw(fmt.Sprintf("AdminWarn %s %s - %s", eosID, warningMessage, timeLeftStr))
				if err != nil {
					log.Error().
						Str("extension", "auto_kick_unassigned").
						Str("player", playerName).
						Str("eosID", eosID).
						Err(err).
						Msg("Failed to warn unassigned player")
				}

				log.Debug().
					Str("extension", "auto_kick_unassigned").
					Str("player", playerName).
					Str("eosID", eosID).
					Str("timeLeft", timeLeftStr).
					Msg("Warning unassigned player")

				mgr.mu.Lock()
				if tracker, exists := mgr.TrackedPlayers[eosID]; exists {
					tracker.Warnings++
				}
				mgr.mu.Unlock()

			case <-tracker.StopChan:
				return
			}
		}
	}()

	// Create a kick timer
	kickDuration := time.Duration(kickTimeout) * time.Second
	tracker.KickTimerID = time.NewTimer(kickDuration)

	// Start kick goroutine
	go func() {
		select {
		case <-tracker.KickTimerID.C:
			// Force update tracking list to ensure player is still unassigned
			e.updateTrackingList(true)

			// Check if player is still tracked
			mgr.mu.RLock()
			stillTracked := false
			var storedTracker *PlayerTracker
			if t, exists := mgr.TrackedPlayers[eosID]; exists {
				stillTracked = true
				storedTracker = t
			}
			mgr.mu.RUnlock()

			if !stillTracked {
				return
			}

			// Kick the player
			r := squadRcon.NewSquadRcon(e.Deps.RconManager, e.Deps.Server.Id)
			_, err := r.ExecuteRaw(fmt.Sprintf("AdminKick %s %s", eosID, kickMessage))
			if err != nil {
				log.Error().
					Str("extension", "auto_kick_unassigned").
					Str("player", playerName).
					Str("eosID", eosID).
					Err(err).
					Msg("Failed to kick unassigned player")
			}

			log.Info().
				Str("extension", "auto_kick_unassigned").
				Str("player", playerName).
				Str("eosID", eosID).
				Int("warnings", storedTracker.Warnings).
				Msg("Kicked unassigned player")

			// Emit an event for the kick
			e.emitPlayerAutoKicked(eosID, playerName, storedTracker.Warnings, storedTracker.StartTime)

			// Untrack the player
			e.untrackPlayer(eosID)

		case <-tracker.StopChan:
			return
		}
	}()

	// Add tracker to managed list
	mgr.mu.Lock()
	mgr.TrackedPlayers[eosID] = tracker
	mgr.mu.Unlock()
}

// untrackPlayer stops tracking a player
func (e *AutoKickUnassignedExtension) untrackPlayer(eosID string) {
	mgr := e.Manager.(*AutoKickManager)

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	tracker, exists := mgr.TrackedPlayers[eosID]
	if !exists {
		return
	}

	log.Debug().
		Str("extension", "auto_kick_unassigned").
		Str("player", tracker.Name).
		Str("eosID", eosID).
		Msg("Untracking player")

	// Stop timers and goroutines
	tracker.WarnTimerID.Stop()
	tracker.KickTimerID.Stop()
	close(tracker.StopChan)

	// Remove from tracked list
	delete(mgr.TrackedPlayers, eosID)
}

// emitPlayerAutoKicked emits a player auto kicked event
func (e *AutoKickUnassignedExtension) emitPlayerAutoKicked(eosID, playerName string, warnings int, startTime time.Time) {
	// TODO: Implement event emission once we have an event system
	// This is a placeholder for future implementation

	log.Info().
		Str("extension", "auto_kick_unassigned").
		Str("player", playerName).
		Str("eosID", eosID).
		Int("warnings", warnings).
		Time("startTime", startTime).
		Msg("PLAYER_AUTO_KICKED")
}

// formatDuration formats a duration in MM:SS format
func formatDuration(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60

	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

// contains checks if a string slice contains a value
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// GetDefinition returns the extension's definition
func (e *AutoKickUnassignedExtension) GetDefinition() extension_manager.ExtensionDefinition {
	return e.Definition
}
