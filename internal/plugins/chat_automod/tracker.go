package chat_automod

import (
	"encoding/json"
	"fmt"
	"time"

	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
)

// Violation represents a single violation record
type Violation struct {
	Timestamp   time.Time      `json:"timestamp"`
	EventID     string         `json:"event_id"`
	Category    FilterCategory `json:"category"`
	ActionTaken string         `json:"action_taken"`
	Message     string         `json:"message,omitempty"`
}

// ViolationRecord stores all violations for a player
type ViolationRecord struct {
	SteamID    string      `json:"steam_id"`
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
func (t *ViolationTracker) getStorageKey(steamID string) string {
	return fmt.Sprintf("violations:%s", steamID)
}

// GetViolationRecord retrieves the violation record for a player
func (t *ViolationTracker) GetViolationRecord(steamID string) (*ViolationRecord, error) {
	key := t.getStorageKey(steamID)

	data, err := t.dbAPI.GetPluginData(key)
	if err != nil {
		// Key not found - return empty record
		return &ViolationRecord{
			SteamID:    steamID,
			Violations: []Violation{},
		}, nil
	}

	var record ViolationRecord
	if err := json.Unmarshal([]byte(data), &record); err != nil {
		// Corrupted data - return empty record
		return &ViolationRecord{
			SteamID:    steamID,
			Violations: []Violation{},
		}, nil
	}

	return &record, nil
}

// GetActiveViolationCount returns the count of non-expired violations
func (t *ViolationTracker) GetActiveViolationCount(steamID string) (int, error) {
	record, err := t.GetViolationRecord(steamID)
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
func (t *ViolationTracker) RecordViolation(steamID string, eventID string, category FilterCategory, actionTaken string, message string) error {
	record, err := t.GetViolationRecord(steamID)
	if err != nil {
		return fmt.Errorf("failed to get violation record: %w", err)
	}

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
	key := t.getStorageKey(record.SteamID)

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal violation record: %w", err)
	}

	if err := t.dbAPI.SetPluginData(key, string(data)); err != nil {
		return fmt.Errorf("failed to save violation record: %w", err)
	}

	return nil
}

// ClearViolations removes all violations for a player
func (t *ViolationTracker) ClearViolations(steamID string) error {
	key := t.getStorageKey(steamID)
	return t.dbAPI.DeletePluginData(key)
}

// GetRecentViolations returns violations from the last N days
func (t *ViolationTracker) GetRecentViolations(steamID string, days int) ([]Violation, error) {
	record, err := t.GetViolationRecord(steamID)
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
