package logwatcher_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/valkey-io/valkey-go"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/utils"
	valkeyClient "go.codycody31.dev/squad-aegis/internal/valkey"
)

// EventStore tracks player data across the server session using Valkey as backend
type EventStore struct {
	mu       sync.RWMutex
	serverID uuid.UUID
	client   *valkeyClient.Client
}

const (
	// playerTTL is how long we keep player metadata (EOS/Steam IDs, etc.)
	playerTTL = 6 * time.Hour
	// sessionTTL is how long we keep session-specific data (damage, team, etc.)
	sessionTTL = 6 * time.Hour
	// joinRequestTTL is how long we keep join request metadata
	joinRequestTTL = 1 * time.Hour
	// roundDataTTL is how long we keep round result data
	roundDataTTL = 2 * time.Hour
)

// NewEventStore creates a new Valkey-backed event store for a specific server
func NewEventStore(serverID uuid.UUID, client *valkeyClient.Client) *EventStore {
	return &EventStore{
		serverID: serverID,
		client:   client,
	}
}

// sanitizeKey URL-encodes a key component to prevent special characters from breaking the key structure
func sanitizeKey(key string) string {
	return url.QueryEscape(key)
}

// Key generators for different data types
func (es *EventStore) playerKey(recordID string) string {
	return fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:player-record:%s", es.serverID.String(), recordID)
}

func (es *EventStore) legacyPlayerKey(recordID string) string {
	return fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:player:%s", es.serverID.String(), recordID)
}

func (es *EventStore) playerAliasKey(playerID string) string {
	return fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:player-alias:%s", es.serverID.String(), sanitizeKey(playerID))
}

func (es *EventStore) playerPattern() string {
	return fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:player-record:*", es.serverID.String())
}

func (es *EventStore) legacyPlayerPattern() string {
	return fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:player:*", es.serverID.String())
}

func (es *EventStore) sessionKey(sessionKey string) string {
	return fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:session:%s", es.serverID.String(), sanitizeKey(sessionKey))
}

func (es *EventStore) joinRequestKey(chainID string) string {
	return fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:join-request:%s", es.serverID.String(), sanitizeKey(chainID))
}

func (es *EventStore) disconnectedKey(playerID string) string {
	return fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:disconnected:%s", es.serverID.String(), sanitizeKey(playerID))
}

func (es *EventStore) roundWinnerKey() string {
	return fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:round-winner", es.serverID.String())
}

func (es *EventStore) roundLoserKey() string {
	return fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:round-loser", es.serverID.String())
}

func (es *EventStore) wonKey() string {
	return fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:won", es.serverID.String())
}

// GetServerID returns the server ID this event store is tracking
func (es *EventStore) GetServerID() uuid.UUID {
	return es.serverID
}

// StoreJoinRequest stores a join request by chainID
func (es *EventStore) StoreJoinRequest(chainID string, playerData *JoinRequestData) {

	key := es.joinRequestKey(chainID)
	data, err := json.Marshal(playerData)
	if err != nil {
		log.Error().Err(err).Str("chainID", chainID).Msg("failed to marshal join request data")
		return
	}

	// Store with joinRequestTTL expiration
	if err := es.client.Set(context.Background(), key, string(data), joinRequestTTL); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to store join request in valkey")
	}
}

// GetJoinRequest retrieves and removes a join request by chainID
func (es *EventStore) GetJoinRequest(chainID string) (*JoinRequestData, bool) {
	key := es.joinRequestKey(chainID)
	data, err := es.client.Get(context.Background(), key)
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
	if err := es.client.Del(context.Background(), key); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to delete join request from valkey")
	}

	return &joinRequest, true
}

// StorePlayerData stores persistent player data by playerID
func (es *EventStore) StorePlayerData(playerID string, data *PlayerData) {
	es.mu.Lock()
	defer es.mu.Unlock()

	identifiers := utils.NormalizePlayerIdentifiers(playerID, data.SteamID, data.EOSID)
	if identifiers.PlayerID == "" {
		return
	}

	recordID, existing, exists := es.lookupPlayerRecord(identifiers.StorageIDs()...)
	if !exists {
		recordID = identifiers.PlayerID
		existing = &PlayerData{}
	}
	existingIdentifiers := utils.NormalizePlayerIdentifiers("", existing.SteamID, existing.EOSID)
	resolvedIdentifiers := utils.MergePlayerIdentifiers(existingIdentifiers, identifiers)
	oldIdentifiers := existingIdentifiers.StorageIDs()

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
	if data.EpicID != "" {
		existing.EpicID = data.EpicID
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

	existing.SteamID = resolvedIdentifiers.SteamID
	existing.EOSID = resolvedIdentifiers.EOSID

	// Store merged data with playerTTL expiration
	mergedData, err := json.Marshal(existing)
	if err != nil {
		log.Error().Err(err).Str("playerID", playerID).Msg("failed to marshal player data")
		return
	}

	key := es.playerKey(recordID)
	if err := es.client.Set(context.Background(), key, string(mergedData), playerTTL); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to store player data in valkey")
		return
	}
	if err := es.client.Del(context.Background(), es.legacyPlayerKey(recordID)); err != nil {
		log.Error().Err(err).Str("playerID", recordID).Msg("failed to remove legacy player record")
	}

	if exists {
		for _, oldIdentifier := range oldIdentifiers {
			if utils.ContainsIdentifier(resolvedIdentifiers.StorageIDs(), oldIdentifier) {
				continue
			}
			if err := es.client.Del(context.Background(), es.playerAliasKey(oldIdentifier)); err != nil {
				log.Error().Err(err).Str("playerID", oldIdentifier).Msg("failed to remove stale player alias")
			}
		}
	}

	for _, alias := range resolvedIdentifiers.StorageIDs() {
		if err := es.client.Set(context.Background(), es.playerAliasKey(alias), recordID, playerTTL); err != nil {
			log.Error().Err(err).Str("playerID", alias).Msg("failed to store player alias in valkey")
		}
	}
}

// GetPlayerData retrieves persistent player data by playerID.
// Hold the read lock so alias lookup and record fetch stay consistent while a
// local writer is updating the multi-key player record.
func (es *EventStore) GetPlayerData(playerID string) (*PlayerData, bool) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	recordID, exists := es.resolvePlayerRecordID(playerID)
	if !exists {
		return nil, false
	}

	return es.getPlayerDataByRecordID(recordID)
}

// RemovePlayerData removes persistent player data by playerID
func (es *EventStore) RemovePlayerData(playerID string) error {
	es.mu.Lock()
	defer es.mu.Unlock()

	recordID, exists := es.resolvePlayerRecordID(playerID)
	if !exists {
		return nil
	}

	existing, ok := es.getPlayerDataByRecordID(recordID)
	if ok {
		for _, alias := range utils.NormalizePlayerIdentifiers("", existing.SteamID, existing.EOSID).StorageIDs() {
			if err := es.client.Del(context.Background(), es.playerAliasKey(alias)); err != nil {
				return err
			}
		}
	}

	if err := es.client.Del(context.Background(), es.playerKey(recordID), es.legacyPlayerKey(recordID)); err != nil {
		return err
	}

	return nil
}

// StoreSessionData stores non-persistent session data by key (usually player name)
func (es *EventStore) StoreSessionData(key string, data *SessionData) {

	es.mu.Lock()
	defer es.mu.Unlock()

	valkeyKey := es.sessionKey(key)

	// Get existing data first to merge
	existing := &SessionData{}
	if existingData, err := es.client.Get(context.Background(), valkeyKey); err == nil {
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

	// Store merged data with sessionTTL expiration
	mergedData, err := json.Marshal(existing)
	if err != nil {
		log.Error().Err(err).Str("sessionKey", key).Msg("failed to marshal session data")
		return
	}

	if err := es.client.Set(context.Background(), valkeyKey, string(mergedData), sessionTTL); err != nil {
		log.Error().Err(err).Str("key", valkeyKey).Msg("failed to store session data in valkey")
	}
}

// GetSessionData retrieves session data by key.
// No Go mutex needed: single atomic Redis GET.
func (es *EventStore) GetSessionData(key string) (*SessionData, bool) {
	valkeyKey := es.sessionKey(key)
	data, err := es.client.Get(context.Background(), valkeyKey)
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

// StoreRoundWinner stores round winner data
func (es *EventStore) StoreRoundWinner(data *RoundWinnerData) {

	key := es.roundWinnerKey()
	dataBytes, err := json.Marshal(data)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal round winner data")
		return
	}

	// Store with roundDataTTL expiration
	if err := es.client.Set(context.Background(), key, string(dataBytes), roundDataTTL); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to store round winner in valkey")
	}
}

// StoreRoundLoser stores round loser data
func (es *EventStore) StoreRoundLoser(data *RoundLoserData) {

	key := es.roundLoserKey()
	dataBytes, err := json.Marshal(data)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal round loser data")
		return
	}

	// Store with roundDataTTL expiration
	if err := es.client.Set(context.Background(), key, string(dataBytes), roundDataTTL); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to store round loser in valkey")
	}
}

// GetRoundWinner retrieves and optionally removes round winner data
func (es *EventStore) GetRoundWinner(remove bool) (*RoundWinnerData, bool) {
	key := es.roundWinnerKey()
	data, err := es.client.Get(context.Background(), key)
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
		if err := es.client.Del(context.Background(), key); err != nil {
			log.Error().Err(err).Str("key", key).Msg("failed to delete round winner from valkey")
		}
	}

	return &roundWinner, true
}

// GetRoundLoser retrieves and optionally removes round loser data
func (es *EventStore) GetRoundLoser(remove bool) (*RoundLoserData, bool) {
	key := es.roundLoserKey()
	data, err := es.client.Get(context.Background(), key)
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
		if err := es.client.Del(context.Background(), key); err != nil {
			log.Error().Err(err).Str("key", key).Msg("failed to delete round loser from valkey")
		}
	}

	return &roundLoser, true
}

// StoreWonData stores WON event data for new game correlation
func (es *EventStore) StoreWonData(data *WonData) {

	key := es.wonKey()

	// Check if WON already exists
	if existingData, err := es.client.Get(context.Background(), key); err == nil && existingData != "" {
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

	// Store with roundDataTTL expiration
	if err := es.client.Set(context.Background(), key, string(dataBytes), roundDataTTL); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to store won data in valkey")
	}
}

// GetWonData retrieves and removes WON data
func (es *EventStore) GetWonData() (*WonData, bool) {
	key := es.wonKey()
	data, err := es.client.Get(context.Background(), key)
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
	if err := es.client.Del(context.Background(), key); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to delete won data from valkey")
	}

	return &wonData, true
}

// ClearNewGameData clears session and disconnected data for new game
func (es *EventStore) ClearNewGameData() {

	// Clear session data
	sessionPattern := fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:session:*", es.serverID.String())
	if keys, err := es.client.Scan(context.Background(), sessionPattern); err == nil && len(keys) > 0 {
		if err := es.client.Del(context.Background(), keys...); err != nil {
			log.Error().Err(err).Msg("failed to clear session data from valkey")
		}
	}

	// Clear disconnected data
	disconnectedPattern := fmt.Sprintf("squad-aegis:logwatcher:%s:event-store:disconnected:*", es.serverID.String())
	if keys, err := es.client.Scan(context.Background(), disconnectedPattern); err == nil && len(keys) > 0 {
		if err := es.client.Del(context.Background(), keys...); err != nil {
			log.Error().Err(err).Msg("failed to clear disconnected data from valkey")
		}
	}
}

func (es *EventStore) resolvePlayerRecordID(playerID string) (string, bool) {
	rawPlayerID := strings.TrimSpace(playerID)
	playerID = utils.NormalizePlayerID(playerID)
	if playerID == "" {
		return "", false
	}

	recordID, err := es.client.Get(context.Background(), es.playerAliasKey(playerID))
	if err == nil && recordID != "" {
		return recordID, true
	}

	if exists, err := es.client.Exists(context.Background(), es.playerKey(playerID), es.legacyPlayerKey(playerID)); err == nil && exists > 0 {
		return playerID, true
	}

	if rawPlayerID != "" && rawPlayerID != playerID {
		if exists, err := es.client.Exists(context.Background(), es.playerKey(rawPlayerID), es.legacyPlayerKey(rawPlayerID)); err == nil && exists > 0 {
			return rawPlayerID, true
		}
	}

	return "", false
}

func (es *EventStore) getPlayerDataByRecordID(recordID string) (*PlayerData, bool) {
	if recordID == "" {
		return nil, false
	}

	for _, key := range []string{es.playerKey(recordID), es.legacyPlayerKey(recordID)} {
		data, err := es.client.Get(context.Background(), key)
		if err != nil {
			if err != valkey.Nil {
				log.Error().Err(err).Str("key", key).Msg("failed to get player data from valkey")
			}
			continue
		}

		var playerData PlayerData
		if err := json.Unmarshal([]byte(data), &playerData); err != nil {
			log.Error().Err(err).Str("key", key).Msg("failed to unmarshal player data")
			return nil, false
		}

		return &playerData, true
	}

	return nil, false
}

func (es *EventStore) lookupPlayerRecord(ids ...string) (string, *PlayerData, bool) {
	for _, id := range ids {
		recordID, exists := es.resolvePlayerRecordID(id)
		if !exists {
			continue
		}

		playerData, ok := es.getPlayerDataByRecordID(recordID)
		if !ok {
			continue
		}

		return recordID, playerData, true
	}

	return "", nil, false
}

func (es *EventStore) scanPlayerRecordKeys() ([]string, error) {
	patterns := []string{es.playerPattern(), es.legacyPlayerPattern()}
	seen := make(map[string]struct{})
	keys := make([]string, 0)

	for _, pattern := range patterns {
		matchedKeys, err := es.client.Scan(context.Background(), pattern)
		if err != nil {
			return nil, err
		}
		for _, key := range matchedKeys {
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}

	return keys, nil
}

// CheckTeamkill checks if an action is a teamkill based on victim and attacker data
func (es *EventStore) CheckTeamkill(victimName string, attackerEOSID string) bool {
	// Look up victim session data once to avoid TOCTOU race
	victimData, victimExists := es.GetSessionData(victimName)
	if !victimExists || victimData.TeamID == "" {
		return false
	}

	// Look up attacker team ID if we have their EOS ID
	if attackerEOSID == "" {
		return false
	}
	attackerData, attackerExists := es.GetPlayerData(attackerEOSID)
	if !attackerExists || attackerData.TeamID == "" {
		return false
	}

	// Check for teamkill: same team but different players
	if victimData.TeamID == attackerData.TeamID {
		if victimData.EOSID != "" && victimData.EOSID != attackerEOSID {
			return true
		}
	}

	return false
}

// GetPlayerInfoByName finds a player by their name and returns PlayerInfo for event manager
func (es *EventStore) GetPlayerInfoByName(name string) (*event_manager.PlayerInfo, bool) {
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
	if keys, err := es.scanPlayerRecordKeys(); err == nil {
		for _, key := range keys {
			if data, err := es.client.Get(context.Background(), key); err == nil {
				var playerData PlayerData
				if err := json.Unmarshal([]byte(data), &playerData); err == nil {
					if playerData.PlayerSuffix == name {
						// Convert PlayerData to PlayerInfo
						playerInfo := &event_manager.PlayerInfo{
							PlayerController: playerData.PlayerController,
							IP:               playerData.IP,
							SteamID:          playerData.SteamID,
							EOSID:            playerData.EOSID,
							EpicID:           playerData.EpicID,
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

func (es *EventStore) GetPlayerInfoByIdentifier(playerID string) (*event_manager.PlayerInfo, bool) {
	if playerID == "" {
		return nil, false
	}

	if data, exists := es.GetPlayerData(playerID); exists {
		playerInfo := &event_manager.PlayerInfo{
			PlayerController: data.PlayerController,
			IP:               data.IP,
			SteamID:          data.SteamID,
			EOSID:            data.EOSID,
			EpicID:           data.EpicID,
			PlayerSuffix:     data.PlayerSuffix,
			Controller:       data.Controller,
			TeamID:           data.TeamID,
		}
		return playerInfo, true
	}

	return nil, false
}

// GetPlayerInfoByEOSID finds a player by their EOS ID and returns PlayerInfo for event manager
func (es *EventStore) GetPlayerInfoByEOSID(eosID string) (*event_manager.PlayerInfo, bool) {
	return es.GetPlayerInfoByIdentifier(eosID)
}

// GetPlayerInfoByController finds a player by their controller ID and returns PlayerInfo for event manager
func (es *EventStore) GetPlayerInfoByController(controllerID string) (*event_manager.PlayerInfo, bool) {
	if controllerID == "" {
		return nil, false
	}

	// Check all players for matching controller ID by searching all player keys
	if keys, err := es.scanPlayerRecordKeys(); err == nil {
		for _, key := range keys {
			if data, err := es.client.Get(context.Background(), key); err == nil {
				var playerData PlayerData
				if err := json.Unmarshal([]byte(data), &playerData); err == nil {
					if playerData.Controller == controllerID || playerData.PlayerController == controllerID {
						// Convert PlayerData to PlayerInfo
						playerInfo := &event_manager.PlayerInfo{
							PlayerController: playerData.PlayerController,
							IP:               playerData.IP,
							SteamID:          playerData.SteamID,
							EOSID:            playerData.EOSID,
							EpicID:           playerData.EpicID,
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
