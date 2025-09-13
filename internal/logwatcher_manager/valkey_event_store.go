package logwatcher_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/valkey-io/valkey-go"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	valkeyClient "go.codycody31.dev/squad-aegis/internal/valkey"
)

// ValkeyEventStore tracks player data across the server session using Valkey as backend
type ValkeyEventStore struct {
	mu       sync.RWMutex
	serverID uuid.UUID
	client   *valkeyClient.Client
	ctx      context.Context
}

// NewValkeyEventStore creates a new Valkey-backed event store for a specific server
func NewValkeyEventStore(ctx context.Context, serverID uuid.UUID, client *valkeyClient.Client) *ValkeyEventStore {
	return &ValkeyEventStore{
		serverID: serverID,
		client:   client,
		ctx:      ctx,
	}
}

// Key generators for different data types
func (es *ValkeyEventStore) playerKey(playerID string) string {
	return fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:player:%s", es.serverID.String(), playerID)
}

func (es *ValkeyEventStore) sessionKey(sessionKey string) string {
	return fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:session:%s", es.serverID.String(), sessionKey)
}

func (es *ValkeyEventStore) joinRequestKey(chainID string) string {
	return fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:join-request:%s", es.serverID.String(), chainID)
}

func (es *ValkeyEventStore) disconnectedKey(playerID string) string {
	return fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:disconnected:%s", es.serverID.String(), playerID)
}

func (es *ValkeyEventStore) roundWinnerKey() string {
	return fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:round-winner", es.serverID.String())
}

func (es *ValkeyEventStore) roundLoserKey() string {
	return fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:round-loser", es.serverID.String())
}

func (es *ValkeyEventStore) wonKey() string {
	return fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:won", es.serverID.String())
}

// GetServerID returns the server ID this event store is tracking
func (es *ValkeyEventStore) GetServerID() uuid.UUID {
	return es.serverID
}

// StoreJoinRequest stores a join request by chainID
func (es *ValkeyEventStore) StoreJoinRequest(chainID string, playerData *JoinRequestData) {

	key := es.joinRequestKey(chainID)
	data, err := json.Marshal(playerData)
	if err != nil {
		log.Error().Err(err).Str("chainID", chainID).Msg("failed to marshal join request data")
		return
	}

	// Store with 1 hour expiration
	if err := es.client.Set(es.ctx, key, string(data), time.Hour); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to store join request in valkey")
	}
}

// GetJoinRequest retrieves and removes a join request by chainID
func (es *ValkeyEventStore) GetJoinRequest(chainID string) (*JoinRequestData, bool) {
	key := es.joinRequestKey(chainID)
	data, err := es.client.Get(es.ctx, key)
	if err != nil {
		if err != valkey.Nil {
			log.Error().Err(err).Str("key", key).Msg("failed to get join request from valkey")
		}
		return nil, false
	}

	var joinRequest JoinRequestData
	if err := json.Unmarshal([]byte(data), &joinRequest); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to unmarshal join request data")
		return nil, false
	}

	// Remove the key after retrieving
	if err := es.client.Del(es.ctx, key); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to delete join request from valkey")
	}

	return &joinRequest, true
}

// StorePlayerData stores persistent player data by playerID
func (es *ValkeyEventStore) StorePlayerData(playerID string, data *PlayerData) {

	es.mu.Lock()
	defer es.mu.Unlock()

	key := es.playerKey(playerID)

	// Get existing data first to merge
	existing := &PlayerData{}
	if existingData, err := es.client.Get(es.ctx, key); err == nil {
		if err := json.Unmarshal([]byte(existingData), existing); err != nil {
			log.Error().Err(err).Str("key", key).Msg("failed to unmarshal existing player data")
		}
	}

	// Merge data by updating existing player data with new values
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

	// Store merged data with no expiration (persistent)
	mergedData, err := json.Marshal(existing)
	if err != nil {
		log.Error().Err(err).Str("playerID", playerID).Msg("failed to marshal player data")
		return
	}

	if err := es.client.Set(es.ctx, key, string(mergedData), 0); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to store player data in valkey")
	}
}

// StorePlayerDataFromMap stores persistent player data by playerID from a map
func (es *ValkeyEventStore) StorePlayerDataFromMap(playerID string, data map[string]interface{}) {
	playerData := &PlayerData{}
	playerData.UpdateFromMap(data)
	es.StorePlayerData(playerID, playerData)
}

// GetPlayerData retrieves persistent player data by playerID
func (es *ValkeyEventStore) GetPlayerData(playerID string) (*PlayerData, bool) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	key := es.playerKey(playerID)
	data, err := es.client.Get(es.ctx, key)
	if err != nil {
		if err != valkey.Nil {
			log.Error().Err(err).Str("key", key).Msg("failed to get player data from valkey")
		}
		return nil, false
	}

	var playerData PlayerData
	if err := json.Unmarshal([]byte(data), &playerData); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to unmarshal player data")
		return nil, false
	}

	return &playerData, true
}

// StoreSessionData stores non-persistent session data by key (usually player name)
func (es *ValkeyEventStore) StoreSessionData(key string, data *SessionData) {

	es.mu.Lock()
	defer es.mu.Unlock()

	valkeyKey := es.sessionKey(key)

	// Get existing data first to merge
	existing := &SessionData{}
	if existingData, err := es.client.Get(es.ctx, valkeyKey); err == nil {
		if err := json.Unmarshal([]byte(existingData), existing); err != nil {
			log.Error().Err(err).Str("key", valkeyKey).Msg("failed to unmarshal existing session data")
		}
	}

	// Merge data by updating existing session data with new values
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

	// Store merged data with 24 hour expiration (session data)
	mergedData, err := json.Marshal(existing)
	if err != nil {
		log.Error().Err(err).Str("sessionKey", key).Msg("failed to marshal session data")
		return
	}

	if err := es.client.Set(es.ctx, valkeyKey, string(mergedData), 24*time.Hour); err != nil {
		log.Error().Err(err).Str("key", valkeyKey).Msg("failed to store session data in valkey")
	}
}

// GetSessionData retrieves session data by key
func (es *ValkeyEventStore) GetSessionData(key string) (*SessionData, bool) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	valkeyKey := es.sessionKey(key)
	data, err := es.client.Get(es.ctx, valkeyKey)
	if err != nil {
		if err != valkey.Nil {
			log.Error().Err(err).Str("key", valkeyKey).Msg("failed to get session data from valkey")
		}
		return nil, false
	}

	var sessionData SessionData
	if err := json.Unmarshal([]byte(data), &sessionData); err != nil {
		log.Error().Err(err).Str("key", valkeyKey).Msg("failed to unmarshal session data")
		return nil, false
	}

	return &sessionData, true
}

// StoreDisconnectedPlayer stores disconnection data for a player
func (es *ValkeyEventStore) StoreDisconnectedPlayer(playerID string, data *DisconnectedPlayerData) {

	key := es.disconnectedKey(playerID)
	dataBytes, err := json.Marshal(data)
	if err != nil {
		log.Error().Err(err).Str("playerID", playerID).Msg("failed to marshal disconnected player data")
		return
	}

	// Store with 24 hour expiration
	if err := es.client.Set(es.ctx, key, string(dataBytes), 24*time.Hour); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to store disconnected player in valkey")
	}
}

// RemoveDisconnectedPlayer removes a player from the disconnected list (when they reconnect)
func (es *ValkeyEventStore) RemoveDisconnectedPlayer(playerID string) {

	key := es.disconnectedKey(playerID)
	if err := es.client.Del(es.ctx, key); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to remove disconnected player from valkey")
	}
}

// StoreRoundWinner stores round winner data
func (es *ValkeyEventStore) StoreRoundWinner(data *RoundWinnerData) {

	key := es.roundWinnerKey()
	dataBytes, err := json.Marshal(data)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal round winner data")
		return
	}

	// Store with 2 hour expiration
	if err := es.client.Set(es.ctx, key, string(dataBytes), 2*time.Hour); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to store round winner in valkey")
	}
}

// StoreRoundLoser stores round loser data
func (es *ValkeyEventStore) StoreRoundLoser(data *RoundLoserData) {

	key := es.roundLoserKey()
	dataBytes, err := json.Marshal(data)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal round loser data")
		return
	}

	// Store with 2 hour expiration
	if err := es.client.Set(es.ctx, key, string(dataBytes), 2*time.Hour); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to store round loser in valkey")
	}
}

// GetRoundWinner retrieves and optionally removes round winner data
func (es *ValkeyEventStore) GetRoundWinner(remove bool) (*RoundWinnerData, bool) {
	key := es.roundWinnerKey()
	data, err := es.client.Get(es.ctx, key)
	if err != nil {
		if err != valkey.Nil {
			log.Error().Err(err).Str("key", key).Msg("failed to get round winner from valkey")
		}
		return nil, false
	}

	var roundWinner RoundWinnerData
	if err := json.Unmarshal([]byte(data), &roundWinner); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to unmarshal round winner data")
		return nil, false
	}

	if remove {
		if err := es.client.Del(es.ctx, key); err != nil {
			log.Error().Err(err).Str("key", key).Msg("failed to delete round winner from valkey")
		}
	}

	return &roundWinner, true
}

// GetRoundLoser retrieves and optionally removes round loser data
func (es *ValkeyEventStore) GetRoundLoser(remove bool) (*RoundLoserData, bool) {
	key := es.roundLoserKey()
	data, err := es.client.Get(es.ctx, key)
	if err != nil {
		if err != valkey.Nil {
			log.Error().Err(err).Str("key", key).Msg("failed to get round loser from valkey")
		}
		return nil, false
	}

	var roundLoser RoundLoserData
	if err := json.Unmarshal([]byte(data), &roundLoser); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to unmarshal round loser data")
		return nil, false
	}

	if remove {
		if err := es.client.Del(es.ctx, key); err != nil {
			log.Error().Err(err).Str("key", key).Msg("failed to delete round loser from valkey")
		}
	}

	return &roundLoser, true
}

// StoreWonData stores WON event data for new game correlation
func (es *ValkeyEventStore) StoreWonData(data *WonData) {

	key := es.wonKey()

	// Check if WON already exists
	if existingData, err := es.client.Get(es.ctx, key); err == nil && existingData != "" {
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
		data = nullWinnerData
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal won data")
		return
	}

	// Store with 2 hour expiration
	if err := es.client.Set(es.ctx, key, string(dataBytes), 2*time.Hour); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to store won data in valkey")
	}
}

// GetWonData retrieves and removes WON data
func (es *ValkeyEventStore) GetWonData() (*WonData, bool) {
	key := es.wonKey()
	data, err := es.client.Get(es.ctx, key)
	if err != nil {
		if err != valkey.Nil {
			log.Error().Err(err).Str("key", key).Msg("failed to get won data from valkey")
		}
		return nil, false
	}

	var wonData WonData
	if err := json.Unmarshal([]byte(data), &wonData); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to unmarshal won data")
		return nil, false
	}

	// Remove the key after retrieving
	if err := es.client.Del(es.ctx, key); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to delete won data from valkey")
	}

	return &wonData, true
}

// ClearNewGameData clears session and disconnected data for new game
func (es *ValkeyEventStore) ClearNewGameData() {

	// Clear session data
	sessionPattern := fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:session:*", es.serverID.String())
	if keys, err := es.client.Keys(es.ctx, sessionPattern); err == nil && len(keys) > 0 {
		if err := es.client.Del(es.ctx, keys...); err != nil {
			log.Error().Err(err).Msg("failed to clear session data from valkey")
		}
	}

	// Clear disconnected data
	disconnectedPattern := fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:disconnected:*", es.serverID.String())
	if keys, err := es.client.Keys(es.ctx, disconnectedPattern); err == nil && len(keys) > 0 {
		if err := es.client.Del(es.ctx, keys...); err != nil {
			log.Error().Err(err).Msg("failed to clear disconnected data from valkey")
		}
	}
}

// CheckTeamkill checks if an action is a teamkill based on victim and attacker data
func (es *ValkeyEventStore) CheckTeamkill(victimName string, attackerEOSID string) bool {
	// Look up victim team ID
	var victimTeamID string
	if victimData, exists := es.GetSessionData(victimName); exists {
		victimTeamID = victimData.TeamID
	}

	// Look up attacker team ID if we have their EOS ID
	var attackerTeamID string
	if attackerEOSID != "" {
		if attackerData, exists := es.GetPlayerData(attackerEOSID); exists {
			attackerTeamID = attackerData.TeamID
		}
	}

	// Check for teamkill: same team but different players
	if victimTeamID != "" && attackerTeamID != "" && victimTeamID == attackerTeamID {
		// Ensure this isn't self-damage by checking if victim has EOS ID
		var victimEOSID string
		if victimData, exists := es.GetSessionData(victimName); exists {
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
func (es *ValkeyEventStore) GetPlayerInfoByName(name string) (*event_manager.PlayerInfo, bool) {
	// Check session data first
	if sessionData, exists := es.GetSessionData(name); exists {
		// Convert SessionData to PlayerInfo
		playerInfo := &event_manager.PlayerInfo{
			TeamID: sessionData.TeamID,
			EOSID:  sessionData.EOSID,
		}
		return playerInfo, true
	}

	// Check if any player data has this name by searching all player keys
	playerPattern := fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:player:*", es.serverID.String())
	if keys, err := es.client.Keys(es.ctx, playerPattern); err == nil {
		for _, key := range keys {
			if data, err := es.client.Get(es.ctx, key); err == nil {
				var playerData PlayerData
				if err := json.Unmarshal([]byte(data), &playerData); err == nil {
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
						if sessionData, hasSession := es.GetSessionData(name); hasSession {
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
			}
		}
	}

	return nil, false
}

// GetPlayerInfoByEOSID finds a player by their EOS ID and returns PlayerInfo for event manager
func (es *ValkeyEventStore) GetPlayerInfoByEOSID(eosID string) (*event_manager.PlayerInfo, bool) {
	if eosID == "" {
		return nil, false
	}

	if data, exists := es.GetPlayerData(eosID); exists {
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
func (es *ValkeyEventStore) GetPlayerInfoByController(controllerID string) (*event_manager.PlayerInfo, bool) {
	if controllerID == "" {
		return nil, false
	}

	// Check all players for matching controller ID by searching all player keys
	playerPattern := fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:player:*", es.serverID.String())
	if keys, err := es.client.Keys(es.ctx, playerPattern); err == nil {
		for _, key := range keys {
			if data, err := es.client.Get(es.ctx, key); err == nil {
				var playerData PlayerData
				if err := json.Unmarshal([]byte(data), &playerData); err == nil {
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
			}
		}
	}

	return nil, false
}

// Legacy methods for backwards compatibility during migration - these return maps
// TODO: Remove these once all code is migrated to use the struct versions

// GetPlayerByName finds a player by their name in session data (legacy)
func (es *ValkeyEventStore) GetPlayerByName(name string) (map[string]interface{}, bool) {
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
func (es *ValkeyEventStore) GetPlayerByEOSID(eosID string) (map[string]interface{}, bool) {
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
func (es *ValkeyEventStore) GetPlayerByController(controllerID string) (map[string]interface{}, bool) {
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
