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
	QualifiedSeconds int64     `json:"qualified_seconds"`
	LifetimeSeconds  int64     `json:"lifetime_seconds"`
	LastEarnedAt     time.Time `json:"last_earned_at"`
	LastSeenAt       time.Time `json:"last_seen_at"`
}

type playerRecordJSON struct {
	PlayerID         string    `json:"player_id"`
	SteamID          string    `json:"steam_id"`
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

func EnsureRecord(players map[string]*PlayerRecord, playerID string, now time.Time) *PlayerRecord {
	playerID = utils.NormalizePlayerID(playerID)
	record, exists := players[playerID]
	if exists && record != nil {
		if record.PlayerID == "" {
			record.PlayerID = playerID
		}
		return record
	}

	record = &PlayerRecord{
		PlayerID:     playerID,
		LastEarnedAt: now,
		LastSeenAt:   now,
	}
	players[playerID] = record
	return record
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
