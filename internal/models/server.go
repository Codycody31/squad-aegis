package models

import (
	"time"

	"github.com/google/uuid"
)

type Server struct {
	Id           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	IpAddress    string    `json:"ip_address"`
	GamePort     int       `json:"game_port"`
	RconPort     int       `json:"rcon_port"`
	RconPassword string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ServerBan struct {
	ID        string    `json:"id"`
	ServerID  uuid.UUID `json:"server_id"`
	AdminID   uuid.UUID `json:"admin_id"`
	AdminName string    `json:"admin_name"`
	PlayerID  uuid.UUID `json:"player_id"`
	SteamID   string    `json:"steam_id"`
	Name      string    `json:"name"`
	Reason    string    `json:"reason"`
	Duration  int       `json:"duration"`
	RuleID    *string   `json:"rule_id,omitempty"`
	Permanent bool      `json:"permanent"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ServerAdmin struct {
	Id           uuid.UUID  `json:"id"`
	ServerId     uuid.UUID  `json:"server_id"`
	UserId       *uuid.UUID `json:"user_id,omitempty"`
	SteamId      *int64     `json:"steam_id,omitempty"`
	ServerRoleId uuid.UUID  `json:"server_role_id"`
	CreatedAt    time.Time  `json:"created_at"`
}

type ServerRole struct {
	Id          uuid.UUID `json:"id"`
	ServerId    uuid.UUID `json:"server_id"`
	Name        string    `json:"name"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
}

// ------------------------------------------
// Requests
// ------------------------------------------

type ServerBanCreateRequest struct {
	SteamID  string  `json:"steam_dd"`
	Reason   string  `json:"reason"`
	Duration int     `json:"duration"`
	RuleID   *string `json:"ruleId,omitempty"`
}

type ServerAdminCreateRequest struct {
	UserID       *string `json:"user_id,omitempty"`  // Optional: existing user ID
	SteamID      *int64  `json:"steam_id,omitempty"` // Optional: Steam ID for new admin
	ServerRoleID string  `json:"server_role_id"`     // Required: role to assign
}

type ServerCreateRequest struct {
	Name         string `json:"name"`
	IpAddress    string `json:"ip_address"`
	GamePort     int    `json:"game_port"`
	RconPort     int    `json:"rcon_port"`
	RconPassword string `json:"rcon_password"`
}

type ServerRconExecuteRequest struct {
	Command string `json:"command"`
}

// BanPlayerRequest represents a request to ban a player
type BanPlayerRequest struct {
	SteamID  string `json:"steam_id"`
	Name     string `json:"name"`
	Reason   string `json:"reason"`
	Duration string `json:"duration"` // "permanent" or a duration like "24h"
}

// UnbanPlayerRequest represents a request to unban a player
type UnbanPlayerRequest struct {
	SteamID string `json:"steam_id"`
}

// ServerRoleCreateRequest represents a request to create a role
type ServerRoleCreateRequest struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

// ServerUpdateRequest represents a request to update a server
type ServerUpdateRequest struct {
	Name         string `json:"name" binding:"required"`
	IpAddress    string `json:"ip_address" binding:"required"`
	GamePort     int    `json:"game_port" binding:"required"`
	RconPort     int    `json:"rcon_port" binding:"required"`
	RconPassword string `json:"rcon_password"`
}
