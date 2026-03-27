package whitelistprogress

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"go.codycody31.dev/squad-aegis/internal/shared/utils"
)

const CurrentVersion = 1

var ErrUnknownFormat = errors.New("unknown whitelist progress format")

type State struct {
	Version int                      `json:"version"`
	Players map[string]*PlayerRecord `json:"players"`
}

type PlayerRecord struct {
	PlayerID         string    `json:"player_id"`
	SteamID          string    `json:"steam_id,omitempty"`
	EOSID            string    `json:"eos_id,omitempty"`
	QualifiedSeconds int64     `json:"qualified_seconds"`
	LifetimeSeconds  int64     `json:"lifetime_seconds"`
	LastEarnedAt     time.Time `json:"last_earned_at"`
	LastSeenAt       time.Time `json:"last_seen_at"`
}

type playerRecordJSON struct {
	PlayerID         string    `json:"player_id"`
	SteamID          string    `json:"steam_id"`
	EOSID            string    `json:"eos_id"`
	QualifiedSeconds int64     `json:"qualified_seconds"`
	LifetimeSeconds  int64     `json:"lifetime_seconds"`
	LastEarnedAt     time.Time `json:"last_earned_at"`
	LastSeenAt       time.Time `json:"last_seen_at"`
}

func (r *PlayerRecord) UnmarshalJSON(data []byte) error {
	var payload playerRecordJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	playerID := payload.PlayerID
	if playerID == "" {
		playerID = payload.SteamID
	}

	r.PlayerID = utils.NormalizePlayerID(playerID)
	r.SteamID = normalizeSteamID(payload.SteamID)
	r.EOSID = normalizeEOSID(payload.EOSID)
	applyIdentifierFallbacks(r)
	r.QualifiedSeconds = payload.QualifiedSeconds
	r.LifetimeSeconds = payload.LifetimeSeconds
	r.LastEarnedAt = payload.LastEarnedAt
	r.LastSeenAt = payload.LastSeenAt

	return nil
}

func NewState() *State {
	return &State{
		Version: CurrentVersion,
		Players: make(map[string]*PlayerRecord),
	}
}

func ParseState(raw string) (*State, error) {
	var state State
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return nil, err
	}

	if state.Version == 0 && state.Players == nil {
		return nil, ErrUnknownFormat
	}

	if state.Version != CurrentVersion {
		return nil, fmt.Errorf("unsupported whitelist progress version: %d", state.Version)
	}

	if state.Players == nil {
		state.Players = make(map[string]*PlayerRecord)
	}

	normalizedPlayers := make(map[string]*PlayerRecord, len(state.Players))
	for playerID, record := range state.Players {
		if record == nil {
			continue
		}

		normalizedPlayerID := utils.NormalizePlayerID(playerID)
		if normalizedPlayerID == "" {
			normalizedPlayerID = utils.NormalizePlayerID(record.PlayerID)
		}
		if normalizedPlayerID == "" {
			continue
		}

		if record.PlayerID == "" {
			record.PlayerID = normalizedPlayerID
		} else {
			record.PlayerID = utils.NormalizePlayerID(record.PlayerID)
		}
		record.SteamID = normalizeSteamID(record.SteamID)
		record.EOSID = normalizeEOSID(record.EOSID)
		applyIdentifierFallbacks(record)
		normalizedPlayerID = record.PlayerID
		if normalizedPlayerID == "" {
			continue
		}
		if record.QualifiedSeconds < 0 {
			record.QualifiedSeconds = 0
		}
		if record.LifetimeSeconds < 0 {
			record.LifetimeSeconds = 0
		}

		normalizedPlayers[normalizedPlayerID] = record
	}
	state.Players = normalizedPlayers

	return &state, nil
}

func MarshalPlayers(players map[string]*PlayerRecord) ([]byte, error) {
	state := NewState()
	if players != nil {
		state.Players = players
	}

	return json.Marshal(state)
}

func FindRecord(players map[string]*PlayerRecord, playerID string) (*PlayerRecord, bool) {
	playerID = utils.NormalizePlayerID(playerID)
	if playerID == "" {
		return nil, false
	}

	steamID, eosID := splitPlayerID(playerID)
	return FindRecordByIdentifiers(players, steamID, eosID)
}

func FindRecordByIdentifiers(players map[string]*PlayerRecord, steamID string, eosID string) (*PlayerRecord, bool) {
	steamID = normalizeSteamID(steamID)
	eosID = normalizeEOSID(eosID)
	if steamID == "" && eosID == "" {
		return nil, false
	}

	_, record := findRecordByIdentifiers(players, steamID, eosID)
	if record == nil {
		return nil, false
	}

	return record, true
}

func EnsureRecord(players map[string]*PlayerRecord, steamID string, eosID string, now time.Time) *PlayerRecord {
	steamID = normalizeSteamID(steamID)
	eosID = normalizeEOSID(eosID)

	canonicalPlayerID := chooseCanonicalPlayerID(steamID, eosID)
	if canonicalPlayerID == "" {
		return nil
	}

	recordKey, record := findRecordByIdentifiers(players, steamID, eosID)
	if record == nil {
		record = &PlayerRecord{
			PlayerID:     canonicalPlayerID,
			SteamID:      steamID,
			EOSID:        eosID,
			LastEarnedAt: now,
			LastSeenAt:   now,
		}
		applyIdentifierFallbacks(record)
		players[record.PlayerID] = record
		return record
	}

	if steamID != "" {
		record.SteamID = steamID
	}
	if eosID != "" {
		record.EOSID = eosID
	}
	if record.PlayerID == "" {
		if recordKey != "" {
			record.PlayerID = recordKey
		} else {
			record.PlayerID = canonicalPlayerID
		}
	}
	applyIdentifierFallbacks(record)

	if recordKey != "" && recordKey != record.PlayerID {
		delete(players, recordKey)
	}

	mergeDuplicateRecord(players, record, steamID)
	mergeDuplicateRecord(players, record, eosID)
	players[record.PlayerID] = record

	return record
}

func normalizeSteamID(steamID string) string {
	steamID = utils.NormalizePlayerID(steamID)
	if !utils.IsSteamID(steamID) {
		return ""
	}
	return steamID
}

func normalizeEOSID(eosID string) string {
	eosID = utils.NormalizePlayerID(eosID)
	if !utils.IsEOSID(eosID) {
		return ""
	}
	return eosID
}

func splitPlayerID(playerID string) (steamID string, eosID string) {
	playerID = utils.NormalizePlayerID(playerID)
	switch {
	case utils.IsSteamID(playerID):
		return playerID, ""
	case utils.IsEOSID(playerID):
		return "", playerID
	default:
		return "", ""
	}
}

func chooseCanonicalPlayerID(steamID string, eosID string) string {
	if steamID != "" {
		return steamID
	}
	return eosID
}

func applyIdentifierFallbacks(record *PlayerRecord) {
	if record == nil {
		return
	}

	canonicalPlayerID := chooseCanonicalPlayerID(record.SteamID, record.EOSID)
	if canonicalPlayerID != "" {
		record.PlayerID = canonicalPlayerID
	} else {
		record.PlayerID = utils.NormalizePlayerID(record.PlayerID)
	}

	if record.SteamID == "" && utils.IsSteamID(record.PlayerID) {
		record.SteamID = record.PlayerID
	}
	if record.EOSID == "" && utils.IsEOSID(record.PlayerID) {
		record.EOSID = record.PlayerID
	}
}

func recordMatches(record *PlayerRecord, steamID string, eosID string) bool {
	if record == nil {
		return false
	}

	if steamID != "" && (record.SteamID == steamID || record.PlayerID == steamID) {
		return true
	}
	if eosID != "" && (record.EOSID == eosID || record.PlayerID == eosID) {
		return true
	}

	return false
}

func findRecordByIdentifiers(players map[string]*PlayerRecord, steamID string, eosID string) (string, *PlayerRecord) {
	if steamID != "" {
		if record, exists := players[steamID]; exists && record != nil {
			return steamID, record
		}
	}
	if eosID != "" {
		if record, exists := players[eosID]; exists && record != nil {
			return eosID, record
		}
	}

	for key, record := range players {
		if record == nil {
			continue
		}
		if recordMatches(record, steamID, eosID) {
			return key, record
		}
	}

	return "", nil
}

func mergeDuplicateRecord(players map[string]*PlayerRecord, target *PlayerRecord, alias string) {
	if target == nil || alias == "" {
		return
	}

	duplicate, exists := players[alias]
	if !exists || duplicate == nil || duplicate == target {
		return
	}

	target.QualifiedSeconds += duplicate.QualifiedSeconds
	target.LifetimeSeconds += duplicate.LifetimeSeconds
	if duplicate.LastEarnedAt.After(target.LastEarnedAt) {
		target.LastEarnedAt = duplicate.LastEarnedAt
	}
	if duplicate.LastSeenAt.After(target.LastSeenAt) {
		target.LastSeenAt = duplicate.LastSeenAt
	}
	if target.SteamID == "" {
		target.SteamID = duplicate.SteamID
	}
	if target.EOSID == "" {
		target.EOSID = duplicate.EOSID
	}
	applyIdentifierFallbacks(target)
	delete(players, alias)
	delete(players, duplicate.PlayerID)
	players[target.PlayerID] = target
}

func RequiredSeconds(hours int) int64 {
	if hours <= 0 {
		return 0
	}
	return int64(hours) * int64(time.Hour/time.Second)
}

func Percent(qualifiedSeconds, requiredSeconds int64) float64 {
	if requiredSeconds <= 0 {
		return 0
	}
	return float64(qualifiedSeconds) / float64(requiredSeconds) * 100.0
}

func IsQualified(qualifiedSeconds, requiredSeconds int64) bool {
	if requiredSeconds <= 0 {
		return false
	}
	return qualifiedSeconds >= requiredSeconds
}

func LegacyPercentToSeconds(percent float64, hoursToWhitelist int) int64 {
	requiredSeconds := RequiredSeconds(hoursToWhitelist)
	if percent <= 0 || requiredSeconds <= 0 {
		return 0
	}

	seconds := percent / 100.0 * float64(requiredSeconds)
	return int64(math.Round(seconds))
}

func SecondsToHours(seconds int64) float64 {
	if seconds <= 0 {
		return 0
	}
	return float64(seconds) / 3600.0
}

func DecayQualifiedSeconds(current, decay int64) int64 {
	if current <= 0 || decay <= 0 {
		return maxInt64(current, 0)
	}
	if decay >= current {
		return 0
	}
	return current - decay
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
