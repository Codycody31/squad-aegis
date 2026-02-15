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
	LogHost          *string `json:"log_host,omitempty"`            // Host for SFTP/FTP
	LogPort          *int    `json:"log_port,omitempty"`            // Port for SFTP/FTP
	LogUsername      *string `json:"log_username,omitempty"`        // Username for SFTP/FTP
	LogPassword      *string `json:"-"`                             // Password for SFTP/FTP (hidden in JSON)
	LogPollFrequency *int    `json:"log_poll_frequency,omitempty"`  // Poll frequency in seconds for SFTP/FTP
	LogReadFromStart *bool   `json:"log_read_from_start,omitempty"` // Whether to read from start of file
	SquadGamePath    *string `json:"squad_game_path,omitempty"`     // Base path to SquadGame folder


	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ServerBan struct {
	ID           string        `json:"id"`
	ServerID     uuid.UUID     `json:"server_id"`
	AdminID      uuid.UUID     `json:"admin_id"`
	AdminName    string        `json:"admin_name"`
	AdminSteamID string        `json:"admin_steam_id,omitempty"`
	SteamID      string        `json:"steam_id"`
	Name         string        `json:"name"`
	Reason       string        `json:"reason"`
	Duration     int           `json:"duration"`
	RuleID       *string       `json:"rule_id,omitempty"`
	RuleName     *string       `json:"rule_name,omitempty"`
	BanListID    *string       `json:"ban_list_id,omitempty"`
	BanListName  *string       `json:"ban_list_name,omitempty"`
	EvidenceText *string       `json:"evidence_text,omitempty"`
	Evidence     []BanEvidence `json:"evidence,omitempty"`
	Permanent    bool          `json:"permanent"`
	ExpiresAt    time.Time     `json:"expires_at,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

type BanEvidence struct {
	ID              string                 `json:"id"`
	BanID           string                 `json:"ban_id"`
	EvidenceType    string                 `json:"evidence_type"`
	ClickhouseTable *string                `json:"clickhouse_table,omitempty"` // Nullable for file/text evidence
	RecordID        *string                `json:"record_id,omitempty"`        // Nullable for file/text evidence
	ServerID        uuid.UUID              `json:"server_id"`
	EventTime       *time.Time             `json:"event_time,omitempty"` // Nullable for file uploads
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	FilePath        *string                `json:"file_path,omitempty"`    // For file uploads
	FileName        *string                `json:"file_name,omitempty"`    // For file uploads
	FileSize        *int64                 `json:"file_size,omitempty"`    // For file uploads (bytes)
	FileType        *string                `json:"file_type,omitempty"`    // MIME type for file uploads
	TextContent     *string                `json:"text_content,omitempty"` // For text paste evidence
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
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
	IsAdmin     bool      `json:"is_admin"`
	CreatedAt   time.Time `json:"created_at"`
}

// ------------------------------------------
// Requests
// ------------------------------------------

type ServerBanCreateRequest struct {
	SteamID      string                  `json:"steam_id"`
	Reason       string                  `json:"reason"`
	Duration     int                     `json:"duration"`
	RuleID       *string                 `json:"rule_id,omitempty"`
	BanListID    *string                 `json:"ban_list_id,omitempty"`
	EvidenceText *string                 `json:"evidence_text,omitempty"`
	Evidence     []BanEvidenceCreateItem `json:"evidence,omitempty"`
}

type BanEvidenceCreateItem struct {
	EvidenceType    string                 `json:"evidence_type"`              // 'player_died', 'player_wounded', 'player_damaged', 'chat_message', 'player_connected', 'file_upload', 'text_paste'
	ClickhouseTable *string                `json:"clickhouse_table,omitempty"` // Required for ClickHouse events, null for file/text
	RecordID        *string                `json:"record_id,omitempty"`        // Required for ClickHouse events, null for file/text
	EventTime       *time.Time             `json:"event_time,omitempty"`       // Required for ClickHouse events, optional for file/text
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	// File upload fields (for evidence_type = 'file_upload')
	FilePath *string `json:"file_path,omitempty"`
	FileName *string `json:"file_name,omitempty"`
	FileSize *int64  `json:"file_size,omitempty"`
	FileType *string `json:"file_type,omitempty"`
	// Text paste field (for evidence_type = 'text_paste')
	TextContent *string `json:"text_content,omitempty"`
}

type ServerBanUpdateRequest struct {
	Reason       *string                  `json:"reason,omitempty"`
	Duration     *int                     `json:"duration,omitempty"`
	BanListID    *string                  `json:"ban_list_id,omitempty"`
	RuleID       *uuid.UUID               `json:"rule_id,omitempty"`
	EvidenceText *string                  `json:"evidence_text,omitempty"`
	Evidence     *[]BanEvidenceCreateItem `json:"evidence,omitempty"`
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
	LogHost          *string `json:"log_host,omitempty"`
	LogPort          *int    `json:"log_port,omitempty"`
	LogUsername      *string `json:"log_username,omitempty"`
	LogPassword      *string `json:"log_password,omitempty"`
	LogPollFrequency *int    `json:"log_poll_frequency,omitempty"`
	LogReadFromStart *bool   `json:"log_read_from_start,omitempty"`
	SquadGamePath    *string `json:"squad_game_path,omitempty"`

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
	IsAdmin     *bool    `json:"is_admin,omitempty"` // Optional, defaults to true if not provided
}

// ServerRoleUpdateRequest represents a request to update a role
type ServerRoleUpdateRequest struct {
	Name        *string  `json:"name,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	IsAdmin     *bool    `json:"is_admin,omitempty"`
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
	LogHost          *string `json:"log_host,omitempty"`
	LogPort          *int    `json:"log_port,omitempty"`
	LogUsername      *string `json:"log_username,omitempty"`
	LogPassword      *string `json:"log_password,omitempty"`
	LogPollFrequency *int    `json:"log_poll_frequency,omitempty"`
	LogReadFromStart *bool   `json:"log_read_from_start,omitempty"`
	SquadGamePath    *string `json:"squad_game_path,omitempty"`

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
