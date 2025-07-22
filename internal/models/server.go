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

type ServerBanCreateRequest struct {
	SteamID  string  `json:"steam_id" binding:"required"`
	Reason   string  `json:"reason" binding:"required"`
	Duration int     `json:"duration"`
	RuleID   *string `json:"rule_id,omitempty"`
}

type ServerAdmin struct {
	Id           uuid.UUID `json:"id"`
	ServerId     uuid.UUID `json:"server_id"`
	UserId       uuid.UUID `json:"user_id"`
	ServerRoleId uuid.UUID `json:"server_role_id"`
	CreatedAt    time.Time `json:"created_at"`
}

type ServerRole struct {
	Id          uuid.UUID `json:"id"`
	ServerId    uuid.UUID `json:"server_id"`
	Name        string    `json:"name"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
}
