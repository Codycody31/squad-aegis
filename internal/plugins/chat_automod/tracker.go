package chat_automod

import (
	"encoding/json"
	"fmt"
	"time"

	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/utils"
)

// Violation represents a single violation record
type Violation struct {
	Timestamp   time.Time      `json:"timestamp"`
	EventID     string         `json:"event_id"`
	Category    FilterCategory `json:"category"`
	ActionTaken string         `json:"action_taken"`
	Message     string         `json:"message,omitempty"`
}

// ViolationRecord stores all violations for a player.
// PlayerID may be a SteamID or EOS ID depending on what was available.
// The JSON field is kept as "steam_id" for backward compatibility with existing stored data.
type ViolationRecord struct {
	PlayerID   string      `json:"player_id,omitempty"`
	SteamID    string      `json:"steam_id,omitempty"`
	EOSID      string      `json:"eos_id,omitempty"`
	Violations []Violation `json:"violations"`
}

// ViolationTracker handles violation tracking and persistence
type ViolationTracker struct {
	dbAPI      plugin_manager.DatabaseAPI
	expiryDays int
}

// NewViolationTracker creates a new violation tracker
func NewViolationTracker(dbAPI plugin_manager.DatabaseAPI, expiryDays int) *ViolationTracker {
	return &ViolationTracker{
		dbAPI:      dbAPI,
		expiryDays: expiryDays,
	}
}

// getStorageKey returns the storage key for a player's violations
func (t *ViolationTracker) getStorageKey(playerID string) string {
	return fmt.Sprintf("violations:%s", playerID)
}

func normalizeViolationIdentifiers(playerID string, steamID string, eosID string) (string, string, string) {
	playerID = utils.NormalizePlayerID(playerID)
	steamID = utils.NormalizePlayerID(steamID)
	eosID = utils.NormalizePlayerID(eosID)

	if !utils.IsSteamID(steamID) {
		steamID = ""
	}
	if !utils.IsEOSID(eosID) {
		eosID = ""
	}

	if playerID == "" {
		if steamID != "" {
			playerID = steamID
		} else {
			playerID = eosID
		}
	}

	if steamID == "" && utils.IsSteamID(playerID) {
		steamID = playerID
	}
	if eosID == "" && utils.IsEOSID(playerID) {
		eosID = playerID
	}

	if steamID != "" {
		playerID = steamID
	} else if eosID != "" {
		playerID = eosID
	}

	return playerID, steamID, eosID
}

func violationStorageIDs(playerID string, steamID string, eosID string) []string {
	playerID, steamID, eosID = normalizeViolationIdentifiers(playerID, steamID, eosID)
	candidates := []string{playerID, steamID, eosID}
	seen := make(map[string]bool, len(candidates))
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		result = append(result, candidate)
	}
	return result
}

func newViolationRecord(playerID string, steamID string, eosID string) *ViolationRecord {
	playerID, steamID, eosID = normalizeViolationIdentifiers(playerID, steamID, eosID)
	return &ViolationRecord{
		PlayerID:   playerID,
		SteamID:    steamID,
		EOSID:      eosID,
		Violations: []Violation{},
	}
}

func normalizeViolationRecord(record *ViolationRecord) {
	if record == nil {
		return
	}
	record.PlayerID, record.SteamID, record.EOSID = normalizeViolationIdentifiers(record.PlayerID, record.SteamID, record.EOSID)
	if record.Violations == nil {
		record.Violations = []Violation{}
	}
}

// GetViolationRecord retrieves the violation record for a player
func (t *ViolationTracker) GetViolationRecord(playerID string, steamID string, eosID string) (*ViolationRecord, error) {
	for _, storageID := range violationStorageIDs(playerID, steamID, eosID) {
		data, err := t.dbAPI.GetPluginData(t.getStorageKey(storageID))
		if err != nil {
			continue
		}

		var record ViolationRecord
		if err := json.Unmarshal([]byte(data), &record); err != nil {
			continue
		}

		normalizeViolationRecord(&record)
		if record.PlayerID == "" {
			record.PlayerID, record.SteamID, record.EOSID = normalizeViolationIdentifiers(storageID, steamID, eosID)
		}

		_, normalizedSteamID, normalizedEOSID := normalizeViolationIdentifiers(record.PlayerID, steamID, eosID)
		if record.SteamID == "" {
			record.SteamID = normalizedSteamID
		}
		if record.EOSID == "" {
			record.EOSID = normalizedEOSID
		}
		normalizeViolationRecord(&record)
		return &record, nil
	}

	return newViolationRecord(playerID, steamID, eosID), nil
}

// GetActiveViolationCount returns the count of non-expired violations
func (t *ViolationTracker) GetActiveViolationCount(playerID string, steamID string, eosID string) (int, error) {
	record, err := t.GetViolationRecord(playerID, steamID, eosID)
	if err != nil {
		return 0, err
	}

	return t.countActiveViolations(record), nil
}

// countActiveViolations counts violations that haven't expired
func (t *ViolationTracker) countActiveViolations(record *ViolationRecord) int {
	if t.expiryDays <= 0 {
		// No expiry - all violations count
		return len(record.Violations)
	}

	expiryTime := time.Now().AddDate(0, 0, -t.expiryDays)
	count := 0

	for _, v := range record.Violations {
		if v.Timestamp.After(expiryTime) {
			count++
		}
	}

	return count
}

// RecordViolation adds a new violation to a player's record
func (t *ViolationTracker) RecordViolation(playerID string, steamID string, eosID string, eventID string, category FilterCategory, actionTaken string, message string) error {
	record, err := t.GetViolationRecord(playerID, steamID, eosID)
	if err != nil {
		return fmt.Errorf("failed to get violation record: %w", err)
	}
	record.PlayerID, record.SteamID, record.EOSID = normalizeViolationIdentifiers(playerID, steamID, eosID)

	// Add new violation
	violation := Violation{
		Timestamp:   time.Now(),
		EventID:     eventID,
		Category:    category,
		ActionTaken: actionTaken,
		Message:     message,
	}

	record.Violations = append(record.Violations, violation)

	// Clean up expired violations if expiry is enabled
	if t.expiryDays > 0 {
		record.Violations = t.filterActiveViolations(record.Violations)
	}

	// Save record
	return t.saveRecord(record)
}

// filterActiveViolations removes expired violations from the list
func (t *ViolationTracker) filterActiveViolations(violations []Violation) []Violation {
	if t.expiryDays <= 0 {
		return violations
	}

	expiryTime := time.Now().AddDate(0, 0, -t.expiryDays)
	active := make([]Violation, 0, len(violations))

	for _, v := range violations {
		if v.Timestamp.After(expiryTime) {
			active = append(active, v)
		}
	}

	return active
}

// saveRecord persists the violation record to storage
func (t *ViolationTracker) saveRecord(record *ViolationRecord) error {
	normalizeViolationRecord(record)

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal violation record: %w", err)
	}

	for _, storageID := range violationStorageIDs(record.PlayerID, record.SteamID, record.EOSID) {
		if err := t.dbAPI.SetPluginData(t.getStorageKey(storageID), string(data)); err != nil {
			return fmt.Errorf("failed to save violation record: %w", err)
		}
	}

	return nil
}

// ClearViolations removes all violations for a player
func (t *ViolationTracker) ClearViolations(playerID string, steamID string, eosID string) error {
	for _, storageID := range violationStorageIDs(playerID, steamID, eosID) {
		if err := t.dbAPI.DeletePluginData(t.getStorageKey(storageID)); err != nil {
			return err
		}
	}
	return nil
}

// GetRecentViolations returns violations from the last N days
func (t *ViolationTracker) GetRecentViolations(playerID string, steamID string, eosID string, days int) ([]Violation, error) {
	record, err := t.GetViolationRecord(playerID, steamID, eosID)
	if err != nil {
		return nil, err
	}

	if days <= 0 {
		return record.Violations, nil
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	recent := make([]Violation, 0)

	for _, v := range record.Violations {
		if v.Timestamp.After(cutoff) {
			recent = append(recent, v)
		}
	}

	return recent, nil
}

// EscalationAction represents an action to take based on violation count
type EscalationAction struct {
	ViolationCount  int
	Action          string // "WARN", "KICK", "BAN"
	BanDurationDays int
	Message         string
}

// DetermineAction determines the appropriate action based on violation count
func DetermineAction(violationCount int, actions []EscalationAction) *EscalationAction {
	if len(actions) == 0 {
		return nil
	}

	// Find the action that matches the violation count
	// If no exact match, use the highest action that's <= violation count
	var bestMatch *EscalationAction

	for i := range actions {
		action := &actions[i]
		if action.ViolationCount == violationCount {
			return action
		}
		if action.ViolationCount < violationCount {
			if bestMatch == nil || action.ViolationCount > bestMatch.ViolationCount {
				bestMatch = action
			}
		}
	}

	// If no match found and violation count exceeds all actions,
	// use the highest severity action
	if bestMatch == nil {
		maxCount := 0
		for i := range actions {
			if actions[i].ViolationCount > maxCount {
				maxCount = actions[i].ViolationCount
				bestMatch = &actions[i]
			}
		}
	}

	return bestMatch
}
