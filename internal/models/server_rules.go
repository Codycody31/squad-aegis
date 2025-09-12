package models

import (
	"time"

	"github.com/google/uuid"
)

type ServerRule struct {
	ID           uuid.UUID  `json:"id"`
	ServerID     uuid.UUID  `json:"server_id"`
	ParentID     *uuid.UUID `json:"parent_id"`
	DisplayOrder int        `json:"display_order"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	Actions  []ServerRuleAction `json:"actions,omitempty"`
	SubRules []ServerRule       `json:"sub_rules,omitempty"`
}

type ServerRuleAction struct {
	ID              uuid.UUID `json:"id"`
	RuleID          uuid.UUID `json:"rule_id"`
	ViolationCount  int       `json:"violation_count"`
	ActionType      string    `json:"action_type"` // WARN, KICK, BAN
	DurationMinutes *int      `json:"duration_minutes,omitempty"`
	Message         string    `json:"message"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type PlayerRuleViolation struct {
	ID            uuid.UUID  `json:"id"`
	ServerID      uuid.UUID  `json:"server_id"`
	PlayerSteamID int64      `json:"player_steam_id,string"`
	RuleID        uuid.UUID  `json:"rule_id"`
	AdminUserID   *uuid.UUID `json:"admin_user_id,omitempty"` // Can be empty if violation was automatically triggered
	CreatedAt     time.Time  `json:"created_at"`
}
