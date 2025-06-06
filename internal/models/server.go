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
	Id        uuid.UUID `json:"id"`
	ServerId  uuid.UUID `json:"server_id"`
	AdminId   uuid.UUID `json:"admin_id"`
	SteamId   int64     `json:"steam_id"`
	Reason    string    `json:"reason"`
	Duration  int       `json:"duration"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
