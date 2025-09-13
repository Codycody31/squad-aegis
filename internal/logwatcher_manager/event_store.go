package logwatcher_manager

import (
	"sync"

	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
)

// EventStore tracks player data across the server session
type EventStore struct {
	mu           sync.RWMutex
	serverID     uuid.UUID
	disconnected map[string]*DisconnectedPlayerData // Players who disconnected, cleared on map change
	players      map[string]*PlayerData             // Persistent player data (steamId, controller, suffix)
	session      map[string]*SessionData            // Non-persistent session data
	joinRequests map[string]*JoinRequestData        // Track join requests by chainID
	roundWinner  *RoundWinnerData                   // Current round winner data
	roundLoser   *RoundLoserData                    // Current round loser data
	wonData      *WonData                           // WON event data for new game correlation
}

// NewEventStore creates a new event store for a specific server
func NewEventStore(serverID uuid.UUID) *EventStore {
	return &EventStore{
		serverID:     serverID,
		disconnected: make(map[string]*DisconnectedPlayerData),
		players:      make(map[string]*PlayerData),
		session:      make(map[string]*SessionData),
		joinRequests: make(map[string]*JoinRequestData),
	}
}

// GetServerID returns the server ID this event store is tracking
func (es *EventStore) GetServerID() uuid.UUID {
	return es.serverID
}

// StoreJoinRequest stores a join request by chainID
func (es *EventStore) StoreJoinRequest(chainID string, playerData *JoinRequestData) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.joinRequests[chainID] = playerData
}

// GetJoinRequest retrieves and removes a join request by chainID
func (es *EventStore) GetJoinRequest(chainID string) (*JoinRequestData, bool) {
	es.mu.Lock()
	defer es.mu.Unlock()

	data, exists := es.joinRequests[chainID]
	if exists {
		delete(es.joinRequests, chainID)
	}
	return data, exists
}

// StorePlayerData stores persistent player data by playerID
func (es *EventStore) StorePlayerData(playerID string, data *PlayerData) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if es.players[playerID] == nil {
		es.players[playerID] = &PlayerData{}
	}

	// Merge data by updating existing player data with new values
	existing := es.players[playerID]
	if data.PlayerController != "" {
		existing.PlayerController = data.PlayerController
	}
	if data.IP != "" {
		existing.IP = data.IP
	}
	if data.SteamID != "" {
		existing.SteamID = data.SteamID
	}
	if data.EOSID != "" {
		existing.EOSID = data.EOSID
	}
	if data.PlayerSuffix != "" {
		existing.PlayerSuffix = data.PlayerSuffix
	}
	if data.Controller != "" {
		existing.Controller = data.Controller
	}
	if data.TeamID != "" {
		existing.TeamID = data.TeamID
	}
}

// StorePlayerDataFromMap stores persistent player data by playerID from a map
func (es *EventStore) StorePlayerDataFromMap(playerID string, data map[string]interface{}) {
	playerData := &PlayerData{}
	playerData.UpdateFromMap(data)
	es.StorePlayerData(playerID, playerData)
}

// GetPlayerData retrieves persistent player data by playerID
func (es *EventStore) GetPlayerData(playerID string) (*PlayerData, bool) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	data, exists := es.players[playerID]
	if !exists {
		return nil, false
	}

	// Return a copy
	result := &PlayerData{
		PlayerController: data.PlayerController,
		IP:               data.IP,
		SteamID:          data.SteamID,
		EOSID:            data.EOSID,
		PlayerSuffix:     data.PlayerSuffix,
		Controller:       data.Controller,
		TeamID:           data.TeamID,
	}
	return result, true
}

// StoreSessionData stores non-persistent session data by key (usually player name)
func (es *EventStore) StoreSessionData(key string, data *SessionData) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if es.session[key] == nil {
		es.session[key] = &SessionData{}
	}

	// Merge data by updating existing session data with new values
	existing := es.session[key]
	if data.ChainID != "" {
		existing.ChainID = data.ChainID
	}
	if data.Time != "" {
		existing.Time = data.Time
	}
	if data.WoundTime != "" {
		existing.WoundTime = data.WoundTime
	}
	if data.VictimName != "" {
		existing.VictimName = data.VictimName
	}
	if data.Damage != "" {
		existing.Damage = data.Damage
	}
	if data.AttackerName != "" {
		existing.AttackerName = data.AttackerName
	}
	if data.AttackerEOS != "" {
		existing.AttackerEOS = data.AttackerEOS
	}
	if data.AttackerSteam != "" {
		existing.AttackerSteam = data.AttackerSteam
	}
	if data.AttackerController != "" {
		existing.AttackerController = data.AttackerController
	}
	if data.Weapon != "" {
		existing.Weapon = data.Weapon
	}
	if data.TeamID != "" {
		existing.TeamID = data.TeamID
	}
	if data.EOSID != "" {
		existing.EOSID = data.EOSID
	}
}

// GetSessionData retrieves session data by key
func (es *EventStore) GetSessionData(key string) (*SessionData, bool) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	data, exists := es.session[key]
	if !exists {
		return nil, false
	}

	// Return a copy
	result := &SessionData{
		ChainID:            data.ChainID,
		Time:               data.Time,
		WoundTime:          data.WoundTime,
		VictimName:         data.VictimName,
		Damage:             data.Damage,
		AttackerName:       data.AttackerName,
		AttackerEOS:        data.AttackerEOS,
		AttackerSteam:      data.AttackerSteam,
		AttackerController: data.AttackerController,
		Weapon:             data.Weapon,
		TeamID:             data.TeamID,
		EOSID:              data.EOSID,
	}
	return result, true
}

// StoreDisconnectedPlayer stores disconnection data for a player
func (es *EventStore) StoreDisconnectedPlayer(playerID string, data *DisconnectedPlayerData) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.disconnected[playerID] = data
}

// RemoveDisconnectedPlayer removes a player from the disconnected list (when they reconnect)
func (es *EventStore) RemoveDisconnectedPlayer(playerID string) {
	es.mu.Lock()
	defer es.mu.Unlock()
	delete(es.disconnected, playerID)
}

// StoreRoundWinner stores round winner data
func (es *EventStore) StoreRoundWinner(data *RoundWinnerData) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.roundWinner = data
}

// StoreRoundLoser stores round loser data
func (es *EventStore) StoreRoundLoser(data *RoundLoserData) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.roundLoser = data
}

// GetRoundWinner retrieves and optionally removes round winner data
func (es *EventStore) GetRoundWinner(remove bool) (*RoundWinnerData, bool) {
	es.mu.Lock()
	defer es.mu.Unlock()

	data := es.roundWinner
	exists := data != nil
	if exists && remove {
		es.roundWinner = nil
	}
	return data, exists
}

// GetRoundLoser retrieves and optionally removes round loser data
func (es *EventStore) GetRoundLoser(remove bool) (*RoundLoserData, bool) {
	es.mu.Lock()
	defer es.mu.Unlock()

	data := es.roundLoser
	exists := data != nil
	if exists && remove {
		es.roundLoser = nil
	}
	return data, exists
}

// StoreWonData stores WON event data for new game correlation
func (es *EventStore) StoreWonData(data *WonData) {
	es.mu.Lock()
	defer es.mu.Unlock()

	// Check if WON already exists
	if es.wonData != nil {
		// If WON exists, store with null winner
		nullWinnerData := &WonData{
			Time:       data.Time,
			ChainID:    data.ChainID,
			Winner:     nil, // Set winner to nil
			Layer:      data.Layer,
			Team:       data.Team,
			Subfaction: data.Subfaction,
			Faction:    data.Faction,
			Action:     data.Action,
			Tickets:    data.Tickets,
			Level:      data.Level,
		}
		es.wonData = nullWinnerData
	} else {
		// Otherwise, store original data
		es.wonData = data
	}
}

// GetWonData retrieves and removes WON data
func (es *EventStore) GetWonData() (*WonData, bool) {
	es.mu.Lock()
	defer es.mu.Unlock()

	data := es.wonData
	exists := data != nil
	if exists {
		es.wonData = nil
	}
	return data, exists
}

// ClearNewGameData clears session and disconnected data for new game
func (es *EventStore) ClearNewGameData() {
	es.mu.Lock()
	defer es.mu.Unlock()

	// Clear session and disconnected data for new game
	es.session = make(map[string]*SessionData)
	es.disconnected = make(map[string]*DisconnectedPlayerData)
	// Note: We don't clear players or joinRequests as they persist across map changes
}

// CheckTeamkill checks if an action is a teamkill based on victim and attacker data
func (es *EventStore) CheckTeamkill(victimName string, attackerEOSID string) bool {
	es.mu.RLock()
	defer es.mu.RUnlock()

	// Look up victim team ID
	var victimTeamID string
	if victimData, exists := es.session[victimName]; exists {
		victimTeamID = victimData.TeamID
	}

	// Look up attacker team ID if we have their EOS ID
	var attackerTeamID string
	if attackerEOSID != "" {
		if attackerData, exists := es.players[attackerEOSID]; exists {
			attackerTeamID = attackerData.TeamID
		}
	}

	// Check for teamkill: same team but different players
	if victimTeamID != "" && attackerTeamID != "" && victimTeamID == attackerTeamID {
		// Ensure this isn't self-damage by checking if victim has EOS ID
		var victimEOSID string
		if victimData, exists := es.session[victimName]; exists {
			victimEOSID = victimData.EOSID
		}

		// If we have both EOSIDs and they're different, but teams are the same, it's a teamkill
		if victimEOSID != "" && attackerEOSID != "" && victimEOSID != attackerEOSID {
			return true
		}
	}

	return false
}

// GetPlayerInfoByName finds a player by their name and returns PlayerInfo for event manager
func (es *EventStore) GetPlayerInfoByName(name string) (*event_manager.PlayerInfo, bool) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	// Check session data first
	if sessionData, exists := es.session[name]; exists {
		// Convert SessionData to PlayerInfo
		playerInfo := &event_manager.PlayerInfo{
			TeamID: sessionData.TeamID,
			EOSID:  sessionData.EOSID,
		}
		return playerInfo, true
	}

	// Check if any player data has this name
	for _, playerData := range es.players {
		if playerData.PlayerSuffix == name {
			// Convert PlayerData to PlayerInfo
			playerInfo := &event_manager.PlayerInfo{
				PlayerController: playerData.PlayerController,
				IP:               playerData.IP,
				SteamID:          playerData.SteamID,
				EOSID:            playerData.EOSID,
				PlayerSuffix:     playerData.PlayerSuffix,
				Controller:       playerData.Controller,
				TeamID:           playerData.TeamID,
			}

			// Also include any session data for this player
			if sessionData, hasSession := es.session[name]; hasSession {
				if sessionData.TeamID != "" {
					playerInfo.TeamID = sessionData.TeamID
				}
				if sessionData.EOSID != "" {
					playerInfo.EOSID = sessionData.EOSID
				}
			}
			return playerInfo, true
		}
	}

	return nil, false
}

// GetPlayerInfoByEOSID finds a player by their EOS ID and returns PlayerInfo for event manager
func (es *EventStore) GetPlayerInfoByEOSID(eosID string) (*event_manager.PlayerInfo, bool) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if eosID == "" {
		return nil, false
	}

	if data, exists := es.players[eosID]; exists {
		// Convert PlayerData to PlayerInfo
		playerInfo := &event_manager.PlayerInfo{
			PlayerController: data.PlayerController,
			IP:               data.IP,
			SteamID:          data.SteamID,
			EOSID:            data.EOSID,
			PlayerSuffix:     data.PlayerSuffix,
			Controller:       data.Controller,
			TeamID:           data.TeamID,
		}
		return playerInfo, true
	}

	return nil, false
}

// GetPlayerInfoByController finds a player by their controller ID and returns PlayerInfo for event manager
func (es *EventStore) GetPlayerInfoByController(controllerID string) (*event_manager.PlayerInfo, bool) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if controllerID == "" {
		return nil, false
	}

	// Check all players for matching controller ID
	for _, playerData := range es.players {
		if playerData.Controller == controllerID || playerData.PlayerController == controllerID {
			// Convert PlayerData to PlayerInfo
			playerInfo := &event_manager.PlayerInfo{
				PlayerController: playerData.PlayerController,
				IP:               playerData.IP,
				SteamID:          playerData.SteamID,
				EOSID:            playerData.EOSID,
				PlayerSuffix:     playerData.PlayerSuffix,
				Controller:       playerData.Controller,
				TeamID:           playerData.TeamID,
			}
			return playerInfo, true
		}
	}

	return nil, false
}

// Legacy methods for backwards compatibility during migration - these return maps
// TODO: Remove these once all code is migrated to use the struct versions

// GetPlayerByName finds a player by their name in session data (legacy)
func (es *EventStore) GetPlayerByName(name string) (map[string]interface{}, bool) {
	playerInfo, exists := es.GetPlayerInfoByName(name)
	if !exists {
		return nil, false
	}

	// Convert PlayerInfo back to map for compatibility
	result := make(map[string]interface{})
	if playerInfo.PlayerController != "" {
		result["playercontroller"] = playerInfo.PlayerController
	}
	if playerInfo.IP != "" {
		result["ip"] = playerInfo.IP
	}
	if playerInfo.SteamID != "" {
		result["steam"] = playerInfo.SteamID
	}
	if playerInfo.EOSID != "" {
		result["eos"] = playerInfo.EOSID
	}
	if playerInfo.PlayerSuffix != "" {
		result["playerSuffix"] = playerInfo.PlayerSuffix
	}
	if playerInfo.Controller != "" {
		result["controller"] = playerInfo.Controller
	}
	if playerInfo.TeamID != "" {
		result["teamID"] = playerInfo.TeamID
	}
	return result, true
}

// GetPlayerByEOSID finds a player by their EOS ID (legacy)
func (es *EventStore) GetPlayerByEOSID(eosID string) (map[string]interface{}, bool) {
	playerInfo, exists := es.GetPlayerInfoByEOSID(eosID)
	if !exists {
		return nil, false
	}

	// Convert PlayerInfo back to map for compatibility
	result := make(map[string]interface{})
	if playerInfo.PlayerController != "" {
		result["playercontroller"] = playerInfo.PlayerController
	}
	if playerInfo.IP != "" {
		result["ip"] = playerInfo.IP
	}
	if playerInfo.SteamID != "" {
		result["steam"] = playerInfo.SteamID
	}
	if playerInfo.EOSID != "" {
		result["eos"] = playerInfo.EOSID
	}
	if playerInfo.PlayerSuffix != "" {
		result["playerSuffix"] = playerInfo.PlayerSuffix
	}
	if playerInfo.Controller != "" {
		result["controller"] = playerInfo.Controller
	}
	if playerInfo.TeamID != "" {
		result["teamID"] = playerInfo.TeamID
	}
	return result, true
}

// GetPlayerByController finds a player by their controller ID (legacy)
func (es *EventStore) GetPlayerByController(controllerID string) (map[string]interface{}, bool) {
	playerInfo, exists := es.GetPlayerInfoByController(controllerID)
	if !exists {
		return nil, false
	}

	// Convert PlayerInfo back to map for compatibility
	result := make(map[string]interface{})
	if playerInfo.PlayerController != "" {
		result["playercontroller"] = playerInfo.PlayerController
	}
	if playerInfo.IP != "" {
		result["ip"] = playerInfo.IP
	}
	if playerInfo.SteamID != "" {
		result["steam"] = playerInfo.SteamID
	}
	if playerInfo.EOSID != "" {
		result["eos"] = playerInfo.EOSID
	}
	if playerInfo.PlayerSuffix != "" {
		result["playerSuffix"] = playerInfo.PlayerSuffix
	}
	if playerInfo.Controller != "" {
		result["controller"] = playerInfo.Controller
	}
	if playerInfo.TeamID != "" {
		result["teamID"] = playerInfo.TeamID
	}
	return result, true
}
