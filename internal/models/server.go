package models

import (
	"time"

	"github.com/google/uuid"
)

type Server struct {
	Id            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	IpAddress     string    `json:"ip_address"`
	GamePort      int       `json:"game_port"`
	RconIpAddress *string   `json:"rcon_ip_address"`
	RconPort      int       `json:"rcon_port"`
	RconPassword  string    `json:"-"`

	// Log configuration fields
	LogSourceType    *string `json:"log_source_type,omitempty"`     // "local", "sftp", "ftp"
	LogFilePath      *string `json:"log_file_path,omitempty"`       // Path to log file
	LogHost          *string `json:"log_host,omitempty"`            // Host for SFTP/FTP
	LogPort          *int    `json:"log_port,omitempty"`            // Port for SFTP/FTP
	LogUsername      *string `json:"log_username,omitempty"`        // Username for SFTP/FTP
	LogPassword      *string `json:"-"`                             // Password for SFTP/FTP (hidden in JSON)
	LogPollFrequency *int    `json:"log_poll_frequency,omitempty"`  // Poll frequency in seconds for SFTP/FTP
	LogReadFromStart *bool   `json:"log_read_from_start,omitempty"` // Whether to read from start of file

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ServerBan struct {
	ID        string    `json:"id"`
	ServerID  uuid.UUID `json:"server_id"`
	AdminID   uuid.UUID `json:"admin_id"`
	AdminName string    `json:"admin_name"`
	SteamID   string    `json:"steam_id,string"`
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
	SteamId      *int64     `json:"steam_id,string,omitempty"`
	ServerRoleId uuid.UUID  `json:"server_role_id"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	Notes        *string    `json:"notes,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// IsActive returns true if the admin role is active (not expired)
func (sa *ServerAdmin) IsActive() bool {
	return sa.ExpiresAt == nil || sa.ExpiresAt.After(time.Now())
}

// IsExpired returns true if the admin role has expired
func (sa *ServerAdmin) IsExpired() bool {
	return sa.ExpiresAt != nil && sa.ExpiresAt.Before(time.Now())
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
	SteamID  string  `json:"steam_id"`
	Reason   string  `json:"reason"`
	Duration int     `json:"duration"`
	RuleID   *string `json:"rule_id,omitempty"`
}

type ServerAdminCreateRequest struct {
	UserID       *string    `json:"user_id,omitempty"`    // Optional: existing user ID
	SteamID      *string    `json:"steam_id,omitempty"`   // Optional: Steam ID for new admin
	ServerRoleID string     `json:"server_role_id"`       // Required: role to assign
	ExpiresAt    *time.Time `json:"expires_at,omitempty"` // Optional: expiration date for temporary access
	Notes        *string    `json:"notes,omitempty"`      // Optional: notes about this admin assignment
}

type ServerCreateRequest struct {
	Name          string  `json:"name"`
	IpAddress     string  `json:"ip_address"`
	GamePort      int     `json:"game_port"`
	RconIpAddress *string `json:"rcon_ip_address"`
	RconPort      int     `json:"rcon_port"`
	RconPassword  string  `json:"rcon_password"`

	// Log configuration fields
	LogSourceType    *string `json:"log_source_type,omitempty"`
	LogFilePath      *string `json:"log_file_path,omitempty"`
	LogHost          *string `json:"log_host,omitempty"`
	LogPort          *int    `json:"log_port,omitempty"`
	LogUsername      *string `json:"log_username,omitempty"`
	LogPassword      *string `json:"log_password,omitempty"`
	LogPollFrequency *int    `json:"log_poll_frequency,omitempty"`
	LogReadFromStart *bool   `json:"log_read_from_start,omitempty"`
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
	Name          string  `json:"name" binding:"required"`
	IpAddress     string  `json:"ip_address" binding:"required"`
	GamePort      int     `json:"game_port" binding:"required"`
	RconIpAddress *string `json:"rcon_ip_address"`
	RconPort      int     `json:"rcon_port" binding:"required"`
	RconPassword  string  `json:"rcon_password"`

	// Log configuration fields
	LogSourceType    *string `json:"log_source_type,omitempty"`
	LogFilePath      *string `json:"log_file_path,omitempty"`
	LogHost          *string `json:"log_host,omitempty"`
	LogPort          *int    `json:"log_port,omitempty"`
	LogUsername      *string `json:"log_username,omitempty"`
	LogPassword      *string `json:"log_password,omitempty"`
	LogPollFrequency *int    `json:"log_poll_frequency,omitempty"`
	LogReadFromStart *bool   `json:"log_read_from_start,omitempty"`
}
