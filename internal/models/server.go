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
	ID          string    `json:"id"`
	ServerID    uuid.UUID `json:"server_id"`
	AdminID     uuid.UUID `json:"admin_id"`
	AdminName   string    `json:"admin_name"`
	SteamID     string    `json:"steam_id,string"`
	Name        string    `json:"name"`
	Reason      string    `json:"reason"`
	Duration    int       `json:"duration"`
	RuleID      *string   `json:"rule_id,omitempty"`
	BanListID   *string   `json:"ban_list_id,omitempty"`
	BanListName *string   `json:"ban_list_name,omitempty"`
	Permanent   bool      `json:"permanent"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type BanList struct {
	ID                uuid.UUID  `json:"id"`
	Name              string     `json:"name"`
	Description       *string    `json:"description,omitempty"`
	IsRemote          bool       `json:"is_remote"`
	RemoteURL         *string    `json:"remote_url,omitempty"`
	RemoteSyncEnabled bool       `json:"remote_sync_enabled"`
	LastSyncedAt      *time.Time `json:"last_synced_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type ServerBanListSubscription struct {
	ID          uuid.UUID `json:"id"`
	ServerID    uuid.UUID `json:"server_id"`
	BanListID   uuid.UUID `json:"ban_list_id"`
	BanListName string    `json:"ban_list_name"`
	CreatedAt   time.Time `json:"created_at"`
}

type RemoteBanSource struct {
	ID                  uuid.UUID  `json:"id"`
	Name                string     `json:"name"`
	URL                 string     `json:"url"`
	SyncEnabled         bool       `json:"sync_enabled"`
	SyncIntervalMinutes int        `json:"sync_interval_minutes"`
	LastSyncedAt        *time.Time `json:"last_synced_at,omitempty"`
	LastSyncStatus      *string    `json:"last_sync_status,omitempty"`
	LastSyncError       *string    `json:"last_sync_error,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
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
	SteamID   string  `json:"steam_id"`
	Reason    string  `json:"reason"`
	Duration  int     `json:"duration"`
	RuleID    *string `json:"rule_id,omitempty"`
	BanListID *string `json:"ban_list_id,omitempty"`
}

type BanListCreateRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type BanListUpdateRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type ServerBanListSubscriptionRequest struct {
	BanListID string `json:"ban_list_id"`
}

type RemoteBanSourceCreateRequest struct {
	Name                string `json:"name"`
	URL                 string `json:"url"`
	SyncEnabled         bool   `json:"sync_enabled"`
	SyncIntervalMinutes int    `json:"sync_interval_minutes"`
}

type RemoteBanSourceUpdateRequest struct {
	Name                string `json:"name"`
	URL                 string `json:"url"`
	SyncEnabled         bool   `json:"sync_enabled"`
	SyncIntervalMinutes int    `json:"sync_interval_minutes"`
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

// IgnoredSteamID represents a Steam ID that should be ignored from remote ban sources
type IgnoredSteamID struct {
	ID        string    `json:"id" db:"id"`
	SteamID   string    `json:"steam_id" db:"steam_id"`
	Reason    *string   `json:"reason" db:"reason"`
	CreatedBy *string   `json:"created_by" db:"created_by"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// IgnoredSteamIDCreateRequest represents a request to add a Steam ID to the ignore list
type IgnoredSteamIDCreateRequest struct {
	SteamID   string  `json:"steam_id" binding:"required"`
	Reason    *string `json:"reason"`
	CreatedBy *string `json:"created_by"`
}

// IgnoredSteamIDUpdateRequest represents a request to update an ignored Steam ID
type IgnoredSteamIDUpdateRequest struct {
	Reason *string `json:"reason"`
}
