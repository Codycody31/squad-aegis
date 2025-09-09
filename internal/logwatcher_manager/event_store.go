package logwatcher_manager

import (
	"sync"

	"github.com/google/uuid"
)

// EventStore tracks player data across the server session
type EventStore struct {
	mu           sync.RWMutex
	serverID     uuid.UUID
	disconnected map[string]map[string]interface{} // Players who disconnected, cleared on map change
	players      map[string]map[string]interface{} // Persistent player data (steamId, controller, suffix)
	session      map[string]map[string]interface{} // Non-persistent session data
	joinRequests map[string]map[string]interface{} // Track join requests by chainID
}

// NewEventStore creates a new event store for a specific server
func NewEventStore(serverID uuid.UUID) *EventStore {
	return &EventStore{
		serverID:     serverID,
		disconnected: make(map[string]map[string]interface{}),
		players:      make(map[string]map[string]interface{}),
		session:      make(map[string]map[string]interface{}),
		joinRequests: make(map[string]map[string]interface{}),
	}
}

// GetServerID returns the server ID this event store is tracking
func (es *EventStore) GetServerID() uuid.UUID {
	return es.serverID
}

// StoreJoinRequest stores a join request by chainID
func (es *EventStore) StoreJoinRequest(chainID string, playerData map[string]interface{}) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.joinRequests[chainID] = playerData
}

// GetJoinRequest retrieves and removes a join request by chainID
func (es *EventStore) GetJoinRequest(chainID string) (map[string]interface{}, bool) {
	es.mu.Lock()
	defer es.mu.Unlock()

	data, exists := es.joinRequests[chainID]
	if exists {
		delete(es.joinRequests, chainID)
	}
	return data, exists
}

// StorePlayerData stores persistent player data by playerID
func (es *EventStore) StorePlayerData(playerID string, data map[string]interface{}) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if es.players[playerID] == nil {
		es.players[playerID] = make(map[string]interface{})
	}

	// Merge data
	for k, v := range data {
		es.players[playerID][k] = v
	}
}

// GetPlayerData retrieves persistent player data by playerID
func (es *EventStore) GetPlayerData(playerID string) (map[string]interface{}, bool) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	data, exists := es.players[playerID]
	if !exists {
		return nil, false
	}

	// Return a copy
	result := make(map[string]interface{})
	for k, v := range data {
		result[k] = v
	}
	return result, true
}

// StoreSessionData stores non-persistent session data by key (usually player name)
func (es *EventStore) StoreSessionData(key string, data map[string]interface{}) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if es.session[key] == nil {
		es.session[key] = make(map[string]interface{})
	}

	// Merge data
	for k, v := range data {
		es.session[key][k] = v
	}
}

// GetSessionData retrieves session data by key
func (es *EventStore) GetSessionData(key string) (map[string]interface{}, bool) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	data, exists := es.session[key]
	if !exists {
		return nil, false
	}

	// Return a copy
	result := make(map[string]interface{})
	for k, v := range data {
		result[k] = v
	}
	return result, true
}

// StoreDisconnectedPlayer stores disconnection data for a player
func (es *EventStore) StoreDisconnectedPlayer(playerID string, data map[string]interface{}) {
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
func (es *EventStore) StoreRoundWinner(data map[string]interface{}) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.session["ROUND_WINNER"] = data
}

// StoreRoundLoser stores round loser data
func (es *EventStore) StoreRoundLoser(data map[string]interface{}) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.session["ROUND_LOSER"] = data
}

// GetRoundWinner retrieves and optionally removes round winner data
func (es *EventStore) GetRoundWinner(remove bool) (map[string]interface{}, bool) {
	es.mu.Lock()
	defer es.mu.Unlock()

	data, exists := es.session["ROUND_WINNER"]
	if exists && remove {
		delete(es.session, "ROUND_WINNER")
	}
	return data, exists
}

// GetRoundLoser retrieves and optionally removes round loser data
func (es *EventStore) GetRoundLoser(remove bool) (map[string]interface{}, bool) {
	es.mu.Lock()
	defer es.mu.Unlock()

	data, exists := es.session["ROUND_LOSER"]
	if exists && remove {
		delete(es.session, "ROUND_LOSER")
	}
	return data, exists
}

// StoreWonData stores WON event data for new game correlation
func (es *EventStore) StoreWonData(data map[string]interface{}) {
	es.mu.Lock()
	defer es.mu.Unlock()

	// Check if WON already exists
	_, wonExists := es.session["WON"]
	if wonExists {
		// If WON exists, store with null winner
		nullWinnerData := make(map[string]interface{})
		for k, v := range data {
			nullWinnerData[k] = v
		}
		nullWinnerData["winner"] = nil
		es.session["WON"] = nullWinnerData
	} else {
		// Otherwise, store original data
		es.session["WON"] = data
	}
}

// GetWonData retrieves and removes WON data
func (es *EventStore) GetWonData() (map[string]interface{}, bool) {
	es.mu.Lock()
	defer es.mu.Unlock()

	data, exists := es.session["WON"]
	if exists {
		delete(es.session, "WON")
	}
	return data, exists
}

// ClearNewGameData clears session and disconnected data for new game
func (es *EventStore) ClearNewGameData() {
	es.mu.Lock()
	defer es.mu.Unlock()

	// Clear session and disconnected data for new game
	es.session = make(map[string]map[string]interface{})
	es.disconnected = make(map[string]map[string]interface{})
	// Note: We don't clear players or joinRequests as they persist across map changes
}

// CheckTeamkill checks if an action is a teamkill based on victim and attacker data
func (es *EventStore) CheckTeamkill(victimName string, attackerEOSID string) bool {
	es.mu.RLock()
	defer es.mu.RUnlock()

	// Look up victim team ID
	var victimTeamID string
	if victimData, exists := es.session[victimName]; exists {
		if teamID, hasTeam := victimData["teamID"].(string); hasTeam {
			victimTeamID = teamID
		}
	}

	// Look up attacker team ID if we have their EOS ID
	var attackerTeamID string
	if attackerEOSID != "" {
		if attackerData, exists := es.players[attackerEOSID]; exists {
			if teamID, hasTeam := attackerData["teamID"].(string); hasTeam {
				attackerTeamID = teamID
			}
		}
	}

	// Check for teamkill: same team but different players
	if victimTeamID != "" && attackerTeamID != "" && victimTeamID == attackerTeamID {
		// Ensure this isn't self-damage by checking if victim has EOS ID
		var victimEOSID string
		if victimData, exists := es.session[victimName]; exists {
			if eosID, hasEOS := victimData["eosID"].(string); hasEOS {
				victimEOSID = eosID
			}
		}

		// If we have both EOSIDs and they're different, but teams are the same, it's a teamkill
		if victimEOSID != "" && attackerEOSID != "" && victimEOSID != attackerEOSID {
			return true
		}
	}

	return false
}
