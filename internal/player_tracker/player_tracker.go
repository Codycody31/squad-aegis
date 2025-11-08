package player_tracker

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/rcon_manager"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
	valkeyClient "go.codycody31.dev/squad-aegis/internal/valkey"
)

// PlayerInfo represents comprehensive player information
type PlayerInfo struct {
	EOSID            string    `json:"eos_id"`
	SteamID          string    `json:"steam_id"`
	Name             string    `json:"name"`
	PlayerController string    `json:"player_controller"`
	PlayerSuffix     string    `json:"player_suffix"`
	TeamID           string    `json:"team_id"`
	TeamName         string    `json:"team_name"`
	SquadID          string    `json:"squad_id"`
	SquadName        string    `json:"squad_name"`
	IsConnected      bool      `json:"is_connected"`
	LastUpdated      time.Time `json:"last_updated"`
	Role             string    `json:"role"` // e.g., "SquadLeader", "TeamLeader"
}

// TeamInfo represents team information
type TeamInfo struct {
	TeamID   string `json:"team_id"`
	TeamName string `json:"team_name"`
	Faction  string `json:"faction"` // e.g., "US Army", "Russian Ground Forces"
	Tickets  int    `json:"tickets"`
}

// SquadInfo represents squad information within a team
type SquadInfo struct {
	SquadID          string `json:"squad_id"`
	SquadName        string `json:"squad_name"`
	TeamID           string `json:"team_id"`
	TeamName         string `json:"team_name"`
	Size             int    `json:"size"`
	MaxSize          int    `json:"max_size"`
	Locked           bool   `json:"locked"`
	SquadLeaderEOSID string `json:"squad_leader_eosid"`
}

// PlayerTracker maintains real-time player state information
type PlayerTracker struct {
	mu              sync.RWMutex
	serverID        uuid.UUID
	rconManager     *rcon_manager.RconManager
	squadRcon       *squadRcon.SquadRcon
	eventManager    *event_manager.EventManager
	valkeyClient    *valkeyClient.Client
	ctx             context.Context
	lastRefresh     time.Time
	refreshInterval time.Duration
	isRunning       bool
	stopChan        chan struct{}
}

// RCONManager interface defines the methods we need from the RCON manager
type RCONManager interface {
	ExecuteCommand(serverID uuid.UUID, command string) (string, error)
	ExecuteCommandWithOptions(serverID uuid.UUID, command string, options rcon_manager.CommandOptions) (string, error)
}

// NewPlayerTracker creates a new player tracker instance
func NewPlayerTracker(ctx context.Context, serverID uuid.UUID, rconManager *rcon_manager.RconManager, eventManager *event_manager.EventManager, valkeyClient *valkeyClient.Client) *PlayerTracker {
	squadRcon := squadRcon.NewSquadRcon(rconManager, serverID)

	return &PlayerTracker{
		serverID:        serverID,
		rconManager:     rconManager,
		squadRcon:       squadRcon,
		eventManager:    eventManager,
		valkeyClient:    valkeyClient,
		ctx:             ctx,
		refreshInterval: 30 * time.Second, // Refresh every 30 seconds
		stopChan:        make(chan struct{}),
	}
}

// Start begins the player tracking process
func (pt *PlayerTracker) Start() error {
	pt.mu.Lock()

	if pt.isRunning {
		pt.mu.Unlock()
		return fmt.Errorf("player tracker is already running")
	}

	pt.isRunning = true
	pt.mu.Unlock()

	// Start periodic refresh in a goroutine to avoid blocking
	go pt.refreshLoop()

	// Perform initial refresh asynchronously to avoid blocking main thread
	go func() {
		if err := pt.refreshPlayerData(); err != nil {
			log.Error().Err(err).Msg("Failed initial player data refresh")
		} else {
			log.Info().Str("serverID", pt.serverID.String()).Msg("Player tracker initial refresh completed")
		}
	}()

	log.Info().Str("serverID", pt.serverID.String()).Msg("Player tracker started")
	return nil
}

// Stop shuts down the player tracker
func (pt *PlayerTracker) Stop() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if !pt.isRunning {
		return
	}

	pt.isRunning = false
	close(pt.stopChan)

	log.Info().Str("serverID", pt.serverID.String()).Msg("Player tracker stopped")
}

// Key generation methods for Redis
func (pt *PlayerTracker) playerKey(eosID string) string {
	return fmt.Sprintf("squad-aegis:player-tracker:%s:player:%s", pt.serverID.String(), eosID)
}

func (pt *PlayerTracker) playerByNameKey(name string) string {
	return fmt.Sprintf("squad-aegis:player-tracker:%s:player-by-name:%s", pt.serverID.String(), name)
}

func (pt *PlayerTracker) playerByControllerKey(controller string) string {
	return fmt.Sprintf("squad-aegis:player-tracker:%s:player-by-controller:%s", pt.serverID.String(), controller)
}

func (pt *PlayerTracker) teamKey(teamID string) string {
	return fmt.Sprintf("squad-aegis:player-tracker:%s:team:%s", pt.serverID.String(), teamID)
}

func (pt *PlayerTracker) squadKey(squadID string) string {
	return fmt.Sprintf("squad-aegis:player-tracker:%s:squad:%s", pt.serverID.String(), squadID)
}

func (pt *PlayerTracker) squadsByTeamKey(teamID string) string {
	return fmt.Sprintf("squad-aegis:player-tracker:%s:squads-by-team:%s", pt.serverID.String(), teamID)
}

func (pt *PlayerTracker) allPlayersKey() string {
	return fmt.Sprintf("squad-aegis:player-tracker:%s:all-players", pt.serverID.String())
}

func (pt *PlayerTracker) allTeamsKey() string {
	return fmt.Sprintf("squad-aegis:player-tracker:%s:all-teams", pt.serverID.String())
}

func (pt *PlayerTracker) allSquadsKey() string {
	return fmt.Sprintf("squad-aegis:player-tracker:%s:all-squads", pt.serverID.String())
}

// Helper methods for JSON serialization/deserialization
func (pt *PlayerTracker) marshalPlayer(player *PlayerInfo) (string, error) {
	data, err := json.Marshal(player)
	if err != nil {
		return "", fmt.Errorf("failed to marshal player info: %w", err)
	}
	return string(data), nil
}

func (pt *PlayerTracker) unmarshalPlayer(data string) (*PlayerInfo, error) {
	var player PlayerInfo
	if err := json.Unmarshal([]byte(data), &player); err != nil {
		return nil, fmt.Errorf("failed to unmarshal player info: %w", err)
	}
	return &player, nil
}

func (pt *PlayerTracker) marshalTeam(team *TeamInfo) (string, error) {
	data, err := json.Marshal(team)
	if err != nil {
		return "", fmt.Errorf("failed to marshal team info: %w", err)
	}
	return string(data), nil
}

func (pt *PlayerTracker) unmarshalTeam(data string) (*TeamInfo, error) {
	var team TeamInfo
	if err := json.Unmarshal([]byte(data), &team); err != nil {
		return nil, fmt.Errorf("failed to unmarshal team info: %w", err)
	}
	return &team, nil
}

func (pt *PlayerTracker) marshalSquad(squad *SquadInfo) (string, error) {
	data, err := json.Marshal(squad)
	if err != nil {
		return "", fmt.Errorf("failed to marshal squad info: %w", err)
	}
	return string(data), nil
}

func (pt *PlayerTracker) unmarshalSquad(data string) (*SquadInfo, error) {
	var squad SquadInfo
	if err := json.Unmarshal([]byte(data), &squad); err != nil {
		return nil, fmt.Errorf("failed to unmarshal squad info: %w", err)
	}
	return &squad, nil
}

// refreshLoop periodically refreshes player data
func (pt *PlayerTracker) refreshLoop() {
	ticker := time.NewTicker(pt.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := pt.refreshPlayerData(); err != nil {
				log.Error().Err(err).Msg("Failed to refresh player data")
			}
		case <-pt.stopChan:
			return
		}
	}
}

// refreshPlayerData fetches current player information via SquadRcon
func (pt *PlayerTracker) refreshPlayerData() error {
	// Get teams and squads data using SquadRcon (no lock needed for this)
	teams, err := pt.squadRcon.GetTeamsAndSquads()
	if err != nil {
		return fmt.Errorf("failed to get teams and squads: %w", err)
	}

	// Parse and update data with minimal lock scope
	if err := pt.updateFromSquadRconDataWithLock(teams); err != nil {
		return fmt.Errorf("failed to update from SquadRcon data: %w", err)
	}

	// Update last refresh time with lock
	pt.mu.Lock()
	pt.lastRefresh = time.Now()
	pt.mu.Unlock()

	// Get counts from Redis for the event (no lock needed for Redis operations)
	playerCount, err := pt.getPlayerCount()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get player count from Redis")
		playerCount = 0
	}

	teamCount, err := pt.getTeamCount()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get team count from Redis")
		teamCount = 0
	}

	squadCount, err := pt.getSquadCount()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get squad count from Redis")
		squadCount = 0
	}

	// Publish player list updated event (no lock needed)
	if pt.eventManager != nil {
		pt.eventManager.PublishEvent(pt.serverID, &event_manager.PlayerListUpdatedData{
			PlayerCount: playerCount,
			TeamCount:   teamCount,
			SquadCount:  squadCount,
			Timestamp:   pt.lastRefresh,
		}, "")
	}

	return nil
}

// updateFromSquadRconDataWithLock updates player tracker data with minimal lock scope
func (pt *PlayerTracker) updateFromSquadRconDataWithLock(teams []squadRcon.Team) error {
	// Clear team and squad data from Redis (but preserve player data) - no lock needed
	if err := pt.clearTeamsAndSquadsData(); err != nil {
		return fmt.Errorf("failed to clear teams and squads data from Redis: %w", err)
	}

	// Get all existing players to preserve their custom data - needs read lock
	existingPlayers := pt.GetAllPlayers()

	// Track current connected players to clean up disconnected ones
	currentConnectedPlayers := make(map[string]bool)

	// Process teams and squads - no lock needed for Redis operations
	for _, team := range teams {
		// Convert SquadRcon Team to PlayerTracker TeamInfo
		teamIDStr := strconv.Itoa(team.ID)
		teamInfo := &TeamInfo{
			TeamID:   teamIDStr,
			TeamName: team.Name,
			Faction:  "", // Not available in SquadRcon Team
			Tickets:  0,  // Not available in SquadRcon Team
		}

		// Store team in Redis
		if err := pt.storeTeam(teamInfo); err != nil {
			log.Error().Err(err).Str("teamID", teamIDStr).Msg("Failed to store team in Redis")
			continue
		}

		// Process squads in this team
		for _, squad := range team.Squads {
			// Convert SquadRcon Squad to PlayerTracker SquadInfo
			squadIDStr := strconv.Itoa(squad.ID)
			teamIDStr := strconv.Itoa(squad.TeamId)
			squadInfo := &SquadInfo{
				SquadID:          squadIDStr,
				SquadName:        squad.Name,
				TeamID:           teamIDStr,
				TeamName:         team.Name,
				Size:             squad.Size,
				MaxSize:          9, // Default max size for Squad squads
				Locked:           squad.Locked,
				SquadLeaderEOSID: "", // Will be set from squad leader
			}

			// Set squad leader EOSID if available
			if squad.Leader != nil {
				squadInfo.SquadLeaderEOSID = squad.Leader.EosId
			}

			// Store squad in Redis
			if err := pt.storeSquad(squadInfo); err != nil {
				log.Error().Err(err).Str("squadID", squadIDStr).Msg("Failed to store squad in Redis")
				continue
			}

			// Add squad to team's squad list
			if err := pt.addSquadToTeam(teamIDStr, squadIDStr); err != nil {
				log.Error().Err(err).Str("teamID", teamIDStr).Str("squadID", squadIDStr).Msg("Failed to add squad to team in Redis")
			}

			// Process players in this squad
			for _, player := range squad.Players {
				// Convert SquadRcon Player to PlayerTracker PlayerInfo
				teamIDStr := strconv.Itoa(player.TeamId)
				squadIDStr := strconv.Itoa(player.SquadId)

				// Check if player already exists and preserve custom data
				var playerInfo *PlayerInfo
				if existingPlayer, exists := existingPlayers[player.EosId]; exists {
					// Preserve existing player data but update team/squad info
					playerInfo = existingPlayer
					playerInfo.TeamID = teamIDStr
					playerInfo.TeamName = team.Name
					playerInfo.SquadID = squadIDStr
					playerInfo.SquadName = squad.Name
					playerInfo.IsConnected = true
					playerInfo.LastUpdated = time.Now()
					playerInfo.Role = player.Role

					// Update name if different (but preserve controller/suffix)
					if playerInfo.Name != player.Name {
						// Remove old name index
						if playerInfo.Name != "" {
							pt.valkeyClient.Del(pt.ctx, pt.playerByNameKey(playerInfo.Name))
						}
						playerInfo.Name = player.Name
					}
				} else {
					// Create new player
					playerInfo = &PlayerInfo{
						EOSID:       player.EosId,
						SteamID:     player.SteamId,
						Name:        player.Name,
						TeamID:      teamIDStr,
						TeamName:    team.Name,
						SquadID:     squadIDStr,
						SquadName:   squad.Name,
						IsConnected: true, // All players in GetServerPlayers are online
						LastUpdated: time.Now(),
						Role:        player.Role,
					}
				}

				// Store player in Redis
				if err := pt.storePlayer(playerInfo); err != nil {
					log.Error().Err(err).Str("eosID", player.EosId).Msg("Failed to store player in Redis")
					continue
				}

				currentConnectedPlayers[player.EosId] = true
			}
		}

		// Process unassigned players in this team
		for _, player := range team.Players {
			// Convert SquadRcon Player to PlayerTracker PlayerInfo (unassigned)
			teamIDStr := strconv.Itoa(player.TeamId)

			// Check if player already exists and preserve custom data
			var playerInfo *PlayerInfo
			if existingPlayer, exists := existingPlayers[player.EosId]; exists {
				// Preserve existing player data but update team info
				playerInfo = existingPlayer
				playerInfo.TeamID = teamIDStr
				playerInfo.TeamName = team.Name
				playerInfo.SquadID = "" // Unassigned
				playerInfo.SquadName = ""
				playerInfo.IsConnected = true
				playerInfo.LastUpdated = time.Now()
				playerInfo.Role = player.Role

				// Update name if different (but preserve controller/suffix)
				if playerInfo.Name != player.Name {
					// Remove old name index
					if playerInfo.Name != "" {
						pt.valkeyClient.Del(pt.ctx, pt.playerByNameKey(playerInfo.Name))
					}
					playerInfo.Name = player.Name
				}
			} else {
				// Create new player
				playerInfo = &PlayerInfo{
					EOSID:       player.EosId,
					SteamID:     player.SteamId,
					Name:        player.Name,
					TeamID:      teamIDStr,
					TeamName:    team.Name,
					SquadID:     "", // Unassigned players have no squad
					SquadName:   "",
					IsConnected: true, // All players in GetServerPlayers are online
					LastUpdated: time.Now(),
					Role:        player.Role,
				}
			}

			// Store player in Redis
			if err := pt.storePlayer(playerInfo); err != nil {
				log.Error().Err(err).Str("eosID", player.EosId).Msg("Failed to store unassigned player in Redis")
				continue
			}

			currentConnectedPlayers[player.EosId] = true
		}
	}

	// Clean up disconnected players that are no longer in the current data - no lock needed
	if err := pt.cleanupDisconnectedPlayers(currentConnectedPlayers); err != nil {
		log.Error().Err(err).Msg("Failed to cleanup disconnected players")
	}

	return nil
}

// Helper methods to get counts from Redis
func (pt *PlayerTracker) getPlayerCount() (int, error) {
	keys, err := pt.valkeyClient.Keys(pt.ctx, fmt.Sprintf("squad-aegis:player-tracker:%s:player:*", pt.serverID.String()))
	if err != nil {
		return 0, err
	}
	return len(keys), nil
}

func (pt *PlayerTracker) getTeamCount() (int, error) {
	keys, err := pt.valkeyClient.Keys(pt.ctx, fmt.Sprintf("squad-aegis:player-tracker:%s:team:*", pt.serverID.String()))
	if err != nil {
		return 0, err
	}
	return len(keys), nil
}

func (pt *PlayerTracker) getSquadCount() (int, error) {
	keys, err := pt.valkeyClient.Keys(pt.ctx, fmt.Sprintf("squad-aegis:player-tracker:%s:squad:*", pt.serverID.String()))
	if err != nil {
		return 0, err
	}
	return len(keys), nil
}

// parseSquadList parses the ListSquads command response
func (pt *PlayerTracker) parseSquadList(response string) error {
	lines := strings.Split(response, "\n")

	// Clear existing squads from Redis
	squadKeys, err := pt.valkeyClient.Keys(pt.ctx, fmt.Sprintf("squad-aegis:player-tracker:%s:squad:*", pt.serverID.String()))
	if err == nil && len(squadKeys) > 0 {
		pt.valkeyClient.Del(pt.ctx, squadKeys...)
	}

	// Clear squads-by-team hashes
	squadsByTeamKeys, err := pt.valkeyClient.Keys(pt.ctx, fmt.Sprintf("squad-aegis:player-tracker:%s:squads-by-team:*", pt.serverID.String()))
	if err == nil && len(squadsByTeamKeys) > 0 {
		pt.valkeyClient.Del(pt.ctx, squadsByTeamKeys...)
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Current Squads:") || strings.HasPrefix(line, "Team:") {
			continue
		}

		squadInfo := pt.parseSquadLine(line)
		if squadInfo != nil {
			// Store squad in Redis
			if err := pt.storeSquad(squadInfo); err != nil {
				log.Error().Err(err).Str("squadID", squadInfo.SquadID).Msg("Failed to store parsed squad")
				continue
			}

			// Add to team-based index
			if err := pt.addSquadToTeam(squadInfo.TeamID, squadInfo.SquadID); err != nil {
				log.Error().Err(err).Str("teamID", squadInfo.TeamID).Str("squadID", squadInfo.SquadID).Msg("Failed to add squad to team")
			}
		}
	}

	return nil
}

// parseSquadLine parses a single squad line from ListSquads output
func (pt *PlayerTracker) parseSquadLine(line string) *SquadInfo {
	squad := &SquadInfo{}

	// Parse squad line format: Team ID: X | Team Name: TeamName | Squad ID: Y | Squad Name: SquadName | Size: Z/Max | Locked: Yes/No | Leader: LeaderName
	pairs := strings.Split(line, "|")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		kv := strings.SplitN(pair, ":", 2)
		if len(kv) != 2 {
			continue
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		switch strings.ToLower(key) {
		case "team id":
			squad.TeamID = value
		case "team name":
			squad.TeamName = value
			// Also add to teams map in Redis
			teamInfo := &TeamInfo{
				TeamID:   value,
				TeamName: value,
			}
			if err := pt.storeTeam(teamInfo); err != nil {
				log.Error().Err(err).Str("teamID", value).Msg("Failed to store team from squad line")
			}
		case "squad id":
			squad.SquadID = value
		case "squad name":
			squad.SquadName = value
		case "size":
			// Parse "current/max" format
			sizeParts := strings.Split(value, "/")
			if len(sizeParts) == 2 {
				if size, err := strconv.Atoi(strings.TrimSpace(sizeParts[0])); err == nil {
					squad.Size = size
				}
				if maxSize, err := strconv.Atoi(strings.TrimSpace(sizeParts[1])); err == nil {
					squad.MaxSize = maxSize
				}
			}
		case "locked":
			squad.Locked = strings.ToLower(value) == "yes" || strings.ToLower(value) == "true"
		case "leader":
			// Try to find leader by name and get their EOSID
			eosID, err := pt.valkeyClient.Get(pt.ctx, pt.playerByNameKey(value))
			if err == nil {
				squad.SquadLeaderEOSID = eosID
			}
		}
	}

	// Skip if essential info is missing
	if squad.SquadID == "" || squad.TeamID == "" {
		return nil
	}

	return squad
}

// updateFromSquadRconData updates player tracker data from SquadRcon structures
func (pt *PlayerTracker) updateFromSquadRconData(teams []squadRcon.Team) error {
	// Clear team and squad data from Redis (but preserve player data)
	if err := pt.clearTeamsAndSquadsData(); err != nil {
		return fmt.Errorf("failed to clear teams and squads data from Redis: %w", err)
	}

	// Track current connected players to clean up disconnected ones
	currentConnectedPlayers := make(map[string]bool)

	// Get all existing players to preserve their custom data
	existingPlayers := pt.GetAllPlayers()

	// Process teams and squads
	for _, team := range teams {
		// Convert SquadRcon Team to PlayerTracker TeamInfo
		teamIDStr := strconv.Itoa(team.ID)
		teamInfo := &TeamInfo{
			TeamID:   teamIDStr,
			TeamName: team.Name,
			Faction:  "", // Not available in SquadRcon Team
			Tickets:  0,  // Not available in SquadRcon Team
		}

		// Store team in Redis
		if err := pt.storeTeam(teamInfo); err != nil {
			log.Error().Err(err).Str("teamID", teamIDStr).Msg("Failed to store team in Redis")
			continue
		}

		// Process squads in this team
		for _, squad := range team.Squads {
			// Convert SquadRcon Squad to PlayerTracker SquadInfo
			squadIDStr := strconv.Itoa(squad.ID)
			teamIDStr := strconv.Itoa(squad.TeamId)
			squadInfo := &SquadInfo{
				SquadID:          squadIDStr,
				SquadName:        squad.Name,
				TeamID:           teamIDStr,
				TeamName:         team.Name,
				Size:             squad.Size,
				MaxSize:          9, // Default max size for Squad squads
				Locked:           squad.Locked,
				SquadLeaderEOSID: "", // Will be set from squad leader
			}

			// Set squad leader EOSID if available
			if squad.Leader != nil {
				squadInfo.SquadLeaderEOSID = squad.Leader.EosId
			}

			// Store squad in Redis
			if err := pt.storeSquad(squadInfo); err != nil {
				log.Error().Err(err).Str("squadID", squadIDStr).Msg("Failed to store squad in Redis")
				continue
			}

			// Add squad to team's squad list
			if err := pt.addSquadToTeam(teamIDStr, squadIDStr); err != nil {
				log.Error().Err(err).Str("teamID", teamIDStr).Str("squadID", squadIDStr).Msg("Failed to add squad to team in Redis")
			}

			// Process players in this squad
			for _, player := range squad.Players {
				// Convert SquadRcon Player to PlayerTracker PlayerInfo
				teamIDStr := strconv.Itoa(player.TeamId)
				squadIDStr := strconv.Itoa(player.SquadId)

				// Check if player already exists and preserve custom data
				var playerInfo *PlayerInfo
				if existingPlayer, exists := existingPlayers[player.EosId]; exists {
					// Preserve existing player data but update team/squad info
					playerInfo = existingPlayer
					playerInfo.TeamID = teamIDStr
					playerInfo.TeamName = team.Name
					playerInfo.SquadID = squadIDStr
					playerInfo.SquadName = squad.Name
					playerInfo.IsConnected = true
					playerInfo.LastUpdated = time.Now()
					playerInfo.Role = player.Role

					// Update name if different (but preserve controller/suffix)
					if playerInfo.Name != player.Name {
						// Remove old name index
						if playerInfo.Name != "" {
							pt.valkeyClient.Del(pt.ctx, pt.playerByNameKey(playerInfo.Name))
						}
						playerInfo.Name = player.Name
					}
				} else {
					// Create new player
					playerInfo = &PlayerInfo{
						EOSID:       player.EosId,
						SteamID:     player.SteamId,
						Name:        player.Name,
						TeamID:      teamIDStr,
						TeamName:    team.Name,
						SquadID:     squadIDStr,
						SquadName:   squad.Name,
						IsConnected: true, // All players in GetServerPlayers are online
						LastUpdated: time.Now(),
						Role:        player.Role,
					}
				}

				// Store player in Redis
				if err := pt.storePlayer(playerInfo); err != nil {
					log.Error().Err(err).Str("eosID", player.EosId).Msg("Failed to store player in Redis")
					continue
				}

				currentConnectedPlayers[player.EosId] = true
			}
		}

		// Process unassigned players in this team
		for _, player := range team.Players {
			// Convert SquadRcon Player to PlayerTracker PlayerInfo (unassigned)
			teamIDStr := strconv.Itoa(player.TeamId)

			// Check if player already exists and preserve custom data
			var playerInfo *PlayerInfo
			if existingPlayer, exists := existingPlayers[player.EosId]; exists {
				// Preserve existing player data but update team info
				playerInfo = existingPlayer
				playerInfo.TeamID = teamIDStr
				playerInfo.TeamName = team.Name
				playerInfo.SquadID = "" // Unassigned
				playerInfo.SquadName = ""
				playerInfo.IsConnected = true
				playerInfo.LastUpdated = time.Now()
				playerInfo.Role = player.Role

				// Update name if different (but preserve controller/suffix)
				if playerInfo.Name != player.Name {
					// Remove old name index
					if playerInfo.Name != "" {
						pt.valkeyClient.Del(pt.ctx, pt.playerByNameKey(playerInfo.Name))
					}
					playerInfo.Name = player.Name
				}
			} else {
				// Create new player
				playerInfo = &PlayerInfo{
					EOSID:       player.EosId,
					SteamID:     player.SteamId,
					Name:        player.Name,
					TeamID:      teamIDStr,
					TeamName:    team.Name,
					SquadID:     "", // Unassigned players have no squad
					SquadName:   "",
					IsConnected: true, // All players in GetServerPlayers are online
					LastUpdated: time.Now(),
					Role:        player.Role,
				}
			}

			// Store player in Redis
			if err := pt.storePlayer(playerInfo); err != nil {
				log.Error().Err(err).Str("eosID", player.EosId).Msg("Failed to store unassigned player in Redis")
				continue
			}

			currentConnectedPlayers[player.EosId] = true
		}
	}

	// Clean up disconnected players that are no longer in the current data
	if err := pt.cleanupDisconnectedPlayers(currentConnectedPlayers); err != nil {
		log.Error().Err(err).Msg("Failed to cleanup disconnected players")
	}

	return nil
}

// Helper methods for Redis operations
func (pt *PlayerTracker) clearTeamsAndSquadsData() error {
	// Clear all team keys
	teamKeys, err := pt.valkeyClient.Keys(pt.ctx, fmt.Sprintf("squad-aegis:player-tracker:%s:team:*", pt.serverID.String()))
	if err != nil {
		return err
	}
	if len(teamKeys) > 0 {
		if err := pt.valkeyClient.Del(pt.ctx, teamKeys...); err != nil {
			return err
		}
	}

	// Clear all squad keys
	squadKeys, err := pt.valkeyClient.Keys(pt.ctx, fmt.Sprintf("squad-aegis:player-tracker:%s:squad:*", pt.serverID.String()))
	if err != nil {
		return err
	}
	if len(squadKeys) > 0 {
		if err := pt.valkeyClient.Del(pt.ctx, squadKeys...); err != nil {
			return err
		}
	}

	// Clear all squads-by-team keys
	squadsByTeamKeys, err := pt.valkeyClient.Keys(pt.ctx, fmt.Sprintf("squad-aegis:player-tracker:%s:squads-by-team:*", pt.serverID.String()))
	if err != nil {
		return err
	}
	if len(squadsByTeamKeys) > 0 {
		if err := pt.valkeyClient.Del(pt.ctx, squadsByTeamKeys...); err != nil {
			return err
		}
	}

	return nil
}

func (pt *PlayerTracker) storePlayer(player *PlayerInfo) error {
	// Store player by EOSID
	playerData, err := pt.marshalPlayer(player)
	if err != nil {
		return err
	}
	if err := pt.valkeyClient.Set(pt.ctx, pt.playerKey(player.EOSID), playerData, 0); err != nil {
		return err
	}

	// Store player by name
	if player.Name != "" {
		if err := pt.valkeyClient.Set(pt.ctx, pt.playerByNameKey(player.Name), player.EOSID, 0); err != nil {
			return err
		}
	}

	// Store player by controller
	if player.PlayerController != "" {
		if err := pt.valkeyClient.Set(pt.ctx, pt.playerByControllerKey(player.PlayerController), player.EOSID, 0); err != nil {
			return err
		}
	}

	return nil
}

func (pt *PlayerTracker) storeTeam(team *TeamInfo) error {
	teamData, err := pt.marshalTeam(team)
	if err != nil {
		return err
	}
	return pt.valkeyClient.Set(pt.ctx, pt.teamKey(team.TeamID), teamData, 0)
}

func (pt *PlayerTracker) storeSquad(squad *SquadInfo) error {
	squadData, err := pt.marshalSquad(squad)
	if err != nil {
		return err
	}
	return pt.valkeyClient.Set(pt.ctx, pt.squadKey(squad.SquadID), squadData, 0)
}

func (pt *PlayerTracker) addSquadToTeam(teamID, squadID string) error {
	return pt.valkeyClient.HSet(pt.ctx, pt.squadsByTeamKey(teamID), squadID, squadID)
}

func (pt *PlayerTracker) cleanupDisconnectedPlayers(currentConnectedPlayers map[string]bool) error {
	// Get all existing player keys
	playerKeys, err := pt.valkeyClient.Keys(pt.ctx, fmt.Sprintf("squad-aegis:player-tracker:%s:player:*", pt.serverID.String()))
	if err != nil {
		return err
	}

	for _, key := range playerKeys {
		// Extract EOSID from key
		parts := strings.Split(key, ":")
		if len(parts) < 6 {
			continue
		}
		eosID := parts[5]

		// Check if this player is currently connected
		if !currentConnectedPlayers[eosID] {
			// Get player data
			playerData, err := pt.valkeyClient.Get(pt.ctx, key)
			if err != nil {
				continue
			}

			player, err := pt.unmarshalPlayer(playerData)
			if err != nil {
				continue
			}

			// Mark as disconnected
			player.IsConnected = false
			player.LastUpdated = time.Now()

			// Update player data
			if err := pt.storePlayer(player); err != nil {
				log.Error().Err(err).Str("eosID", eosID).Msg("Failed to update disconnected player in Redis")
			}
		}
	}

	return nil
}

// GetPlayerByEOSID retrieves player information by EOS ID
func (pt *PlayerTracker) GetPlayerByEOSID(eosID string) (*PlayerInfo, bool) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	playerData, err := pt.valkeyClient.Get(pt.ctx, pt.playerKey(eosID))
	if err != nil {
		return nil, false
	}

	player, err := pt.unmarshalPlayer(playerData)
	if err != nil {
		log.Error().Err(err).Str("eosID", eosID).Msg("Failed to unmarshal player data")
		return nil, false
	}

	// Return a copy to avoid race conditions
	playerCopy := *player
	return &playerCopy, true
}

// GetPlayerByName retrieves player information by name
func (pt *PlayerTracker) GetPlayerByName(name string) (*PlayerInfo, bool) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	eosID, err := pt.valkeyClient.Get(pt.ctx, pt.playerByNameKey(name))
	if err != nil {
		return nil, false
	}

	return pt.GetPlayerByEOSID(eosID)
}

// GetPlayerByController retrieves player information by player controller
func (pt *PlayerTracker) GetPlayerByController(controller string) (*PlayerInfo, bool) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	eosID, err := pt.valkeyClient.Get(pt.ctx, pt.playerByControllerKey(controller))
	if err != nil {
		return nil, false
	}

	return pt.GetPlayerByEOSID(eosID)
}

// GetAllPlayers returns all current players
func (pt *PlayerTracker) GetAllPlayers() map[string]*PlayerInfo {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	players := make(map[string]*PlayerInfo)

	// Get all player keys
	playerKeys, err := pt.valkeyClient.Keys(pt.ctx, fmt.Sprintf("squad-aegis:player-tracker:%s:player:*", pt.serverID.String()))
	if err != nil {
		log.Error().Err(err).Msg("Failed to get player keys from Redis")
		return players
	}

	// Get each player's data
	for _, key := range playerKeys {
		playerData, err := pt.valkeyClient.Get(pt.ctx, key)
		if err != nil {
			log.Error().Err(err).Str("key", key).Msg("Failed to get player data from Redis")
			continue
		}

		player, err := pt.unmarshalPlayer(playerData)
		if err != nil {
			log.Error().Err(err).Str("key", key).Msg("Failed to unmarshal player data")
			continue
		}

		// Extract EOSID from key
		parts := strings.Split(key, ":")
		if len(parts) >= 6 {
			eosID := parts[5]
			players[eosID] = player
		}
	}

	return players
}

// GetTeamInfo retrieves team information
func (pt *PlayerTracker) GetTeamInfo(teamID string) (*TeamInfo, bool) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	teamData, err := pt.valkeyClient.Get(pt.ctx, pt.teamKey(teamID))
	if err != nil {
		return nil, false
	}

	team, err := pt.unmarshalTeam(teamData)
	if err != nil {
		log.Error().Err(err).Str("teamID", teamID).Msg("Failed to unmarshal team data")
		return nil, false
	}

	// Return a copy to avoid race conditions
	teamCopy := *team
	return &teamCopy, true
}

// GetSquadInfo retrieves squad information
func (pt *PlayerTracker) GetSquadInfo(squadID string) (*SquadInfo, bool) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	squadData, err := pt.valkeyClient.Get(pt.ctx, pt.squadKey(squadID))
	if err != nil {
		return nil, false
	}

	squad, err := pt.unmarshalSquad(squadData)
	if err != nil {
		log.Error().Err(err).Str("squadID", squadID).Msg("Failed to unmarshal squad data")
		return nil, false
	}

	// Return a copy to avoid race conditions
	squadCopy := *squad
	return &squadCopy, true
}

// GetPlayersByTeam returns all players in a specific team
func (pt *PlayerTracker) GetPlayersByTeam(teamID string) []*PlayerInfo {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	var teamPlayers []*PlayerInfo

	// Get all players and filter by team
	allPlayers := pt.GetAllPlayers()
	for _, player := range allPlayers {
		if player.TeamID == teamID {
			playerCopy := *player
			teamPlayers = append(teamPlayers, &playerCopy)
		}
	}

	return teamPlayers
}

// GetPlayersBySquad returns all players in a specific squad
func (pt *PlayerTracker) GetPlayersBySquad(squadID string) []*PlayerInfo {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	var squadPlayers []*PlayerInfo

	// Get all players and filter by squad
	allPlayers := pt.GetAllPlayers()
	for _, player := range allPlayers {
		if player.SquadID == squadID {
			playerCopy := *player
			squadPlayers = append(squadPlayers, &playerCopy)
		}
	}

	return squadPlayers
}

// IsTeamkill checks if damage between two players was a teamkill
func (pt *PlayerTracker) IsTeamkill(attackerEOSID, victimEOSID string) (bool, *PlayerInfo, *PlayerInfo) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	// Skip if same player
	if attackerEOSID == victimEOSID {
		return false, nil, nil
	}

	attacker, attackerExists := pt.GetPlayerByEOSID(attackerEOSID)
	victim, victimExists := pt.GetPlayerByEOSID(victimEOSID)

	if !attackerExists || !victimExists {
		return false, nil, nil
	}

	// Check if they're on the same team
	isTeamkill := attacker.TeamID != "" && victim.TeamID != "" && attacker.TeamID == victim.TeamID

	if isTeamkill {
		// Return copies to avoid race conditions
		attackerCopy := *attacker
		victimCopy := *victim
		return true, &attackerCopy, &victimCopy
	}

	return false, nil, nil
}

// GetLastRefreshTime returns when the data was last refreshed
func (pt *PlayerTracker) GetLastRefreshTime() time.Time {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	return pt.lastRefresh
}

// GetStats returns tracker statistics
func (pt *PlayerTracker) GetStats() map[string]interface{} {
	// Get counts from Redis (no lock needed for Redis operations)
	playerCount, err := pt.getPlayerCount()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get player count for stats")
		playerCount = 0
	}

	teamCount, err := pt.getTeamCount()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get team count for stats")
		teamCount = 0
	}

	squadCount, err := pt.getSquadCount()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get squad count for stats")
		squadCount = 0
	}

	// Get state variables that need lock protection
	pt.mu.RLock()
	lastRefresh := pt.lastRefresh
	isRunning := pt.isRunning
	refreshInterval := pt.refreshInterval
	pt.mu.RUnlock()

	return map[string]interface{}{
		"player_count":     playerCount,
		"team_count":       teamCount,
		"squad_count":      squadCount,
		"last_refresh":     lastRefresh,
		"is_running":       isRunning,
		"refresh_interval": refreshInterval,
	}
}

// ForceRefresh forces an immediate refresh of player data
func (pt *PlayerTracker) ForceRefresh() error {
	return pt.refreshPlayerData()
}

// UpdatePlayerFromLog updates player information based on log events
func (pt *PlayerTracker) UpdatePlayerFromLog(eosID, steamID, name, playerController, playerSuffix string) {
	// Try to get existing player by EOSID - no lock needed for Redis operations
	playerData, err := pt.valkeyClient.Get(pt.ctx, pt.playerKey(eosID))
	if err == nil {
		// Player exists, update it
		player, err := pt.unmarshalPlayer(playerData)
		if err != nil {
			log.Error().Err(err).Str("eosID", eosID).Msg("Failed to unmarshal existing player for update")
			return
		}

		// Update existing player
		updated := false
		if name != "" && player.Name != name {
			// Name changed, remove old name index
			if player.Name != "" {
				pt.valkeyClient.Del(pt.ctx, pt.playerByNameKey(player.Name))
			}
			player.Name = name
			updated = true
		}
		if steamID != "" && player.SteamID == "" {
			player.SteamID = steamID
			updated = true
		}
		if playerController != "" && player.PlayerController != playerController {
			// Controller changed, remove old controller index
			if player.PlayerController != "" {
				pt.valkeyClient.Del(pt.ctx, pt.playerByControllerKey(player.PlayerController))
			}
			player.PlayerController = playerController
			updated = true
		}
		if playerSuffix != "" && player.PlayerSuffix != playerSuffix {
			player.PlayerSuffix = playerSuffix
			updated = true
		}

		if updated {
			player.LastUpdated = time.Now()
			if err := pt.storePlayer(player); err != nil {
				log.Error().Err(err).Str("eosID", eosID).Msg("Failed to store updated player")
			}
		}
	} else {
		// Player doesn't exist, try to find by Steam ID
		if steamID != "" {
			// Get all players and check for Steam ID match (needs read lock)
			allPlayers := pt.GetAllPlayers()
			for _, existingPlayer := range allPlayers {
				if existingPlayer.SteamID == steamID {
					// Found by Steam ID, update EOSID and other info
					if eosID != "" && existingPlayer.EOSID != eosID {
						// Remove old EOSID key
						pt.valkeyClient.Del(pt.ctx, pt.playerKey(existingPlayer.EOSID))
						existingPlayer.EOSID = eosID
					}
					if name != "" && existingPlayer.Name != name {
						if existingPlayer.Name != "" {
							pt.valkeyClient.Del(pt.ctx, pt.playerByNameKey(existingPlayer.Name))
						}
						existingPlayer.Name = name
					}
					if playerController != "" && existingPlayer.PlayerController != playerController {
						// Controller changed, remove old controller index
						if existingPlayer.PlayerController != "" {
							pt.valkeyClient.Del(pt.ctx, pt.playerByControllerKey(existingPlayer.PlayerController))
						}
						existingPlayer.PlayerController = playerController
					}
					if playerSuffix != "" && existingPlayer.PlayerSuffix != playerSuffix {
						existingPlayer.PlayerSuffix = playerSuffix
					}
					existingPlayer.LastUpdated = time.Now()

					if err := pt.storePlayer(existingPlayer); err != nil {
						log.Error().Err(err).Str("eosID", eosID).Msg("Failed to store player found by Steam ID")
					}
					return
				}
			}
		}

		// Create new player with minimal info
		newPlayer := &PlayerInfo{
			EOSID:            eosID,
			SteamID:          steamID,
			Name:             name,
			PlayerController: playerController,
			PlayerSuffix:     playerSuffix,
			IsConnected:      true,
			LastUpdated:      time.Now(),
		}

		if err := pt.storePlayer(newPlayer); err != nil {
			log.Error().Err(err).Str("eosID", eosID).Msg("Failed to store new player")
		}
	}
}

// MarkPlayerDisconnected marks a player as disconnected
func (pt *PlayerTracker) MarkPlayerDisconnected(eosID string) {
	// Get player data - no lock needed for Redis operations
	playerData, err := pt.valkeyClient.Get(pt.ctx, pt.playerKey(eosID))
	if err != nil {
		// Player not found
		return
	}

	player, err := pt.unmarshalPlayer(playerData)
	if err != nil {
		log.Error().Err(err).Str("eosID", eosID).Msg("Failed to unmarshal player for disconnect")
		return
	}

	// Mark as disconnected
	player.IsConnected = false
	player.LastUpdated = time.Now()

	// Update player data
	if err := pt.storePlayer(player); err != nil {
		log.Error().Err(err).Str("eosID", eosID).Msg("Failed to store disconnected player")
	}

	// Don't remove from Redis immediately, keep for a while in case they reconnect
	// The periodic refresh will clean up disconnected players
}

// ExportData exports all player data as JSON (useful for debugging)
func (pt *PlayerTracker) ExportData() (string, error) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	// Get all data from Redis
	players := pt.GetAllPlayers()

	teams := make(map[string]*TeamInfo)
	teamKeys, err := pt.valkeyClient.Keys(pt.ctx, fmt.Sprintf("squad-aegis:player-tracker:%s:team:*", pt.serverID.String()))
	if err == nil {
		for _, key := range teamKeys {
			teamData, err := pt.valkeyClient.Get(pt.ctx, key)
			if err != nil {
				continue
			}
			team, err := pt.unmarshalTeam(teamData)
			if err != nil {
				continue
			}
			teams[team.TeamID] = team
		}
	}

	squads := make(map[string]*SquadInfo)
	squadKeys, err := pt.valkeyClient.Keys(pt.ctx, fmt.Sprintf("squad-aegis:player-tracker:%s:squad:*", pt.serverID.String()))
	if err == nil {
		for _, key := range squadKeys {
			squadData, err := pt.valkeyClient.Get(pt.ctx, key)
			if err != nil {
				continue
			}
			squad, err := pt.unmarshalSquad(squadData)
			if err != nil {
				continue
			}
			squads[squad.SquadID] = squad
		}
	}

	data := map[string]interface{}{
		"players":   players,
		"teams":     teams,
		"squads":    squads,
		"timestamp": pt.lastRefresh,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal player data: %w", err)
	}

	return string(jsonData), nil
}
