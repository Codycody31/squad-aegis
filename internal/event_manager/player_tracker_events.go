package event_manager

import (
	"time"
)

// PlayerListUpdatedData is published when the player list is refreshed
type PlayerListUpdatedData struct {
	PlayerCount int       `json:"player_count"`
	TeamCount   int       `json:"team_count"`
	SquadCount  int       `json:"squad_count"`
	Timestamp   time.Time `json:"timestamp"`
}

func (d PlayerListUpdatedData) GetEventType() EventType { return EventTypePlayerListUpdated }

// PlayerTeamChangedData is published when a player changes team
type PlayerTeamChangedData struct {
	EOSID       string `json:"eos_id"`
	SteamID     string `json:"steam_id"`
	PlayerName  string `json:"player_name"`
	OldTeamID   string `json:"old_team_id"`
	NewTeamID   string `json:"new_team_id"`
	OldTeamName string `json:"old_team_name"`
	NewTeamName string `json:"new_team_name"`
}

func (d PlayerTeamChangedData) GetEventType() EventType { return EventTypePlayerTeamChanged }

// PlayerSquadChangedData is published when a player changes squad
type PlayerSquadChangedData struct {
	EOSID        string `json:"eos_id"`
	SteamID      string `json:"steam_id"`
	PlayerName   string `json:"player_name"`
	OldSquadID   string `json:"old_squad_id"`
	NewSquadID   string `json:"new_squad_id"`
	OldSquadName string `json:"old_squad_name"`
	NewSquadName string `json:"new_squad_name"`
}

func (d PlayerSquadChangedData) GetEventType() EventType { return EventTypePlayerSquadChanged }

// SquadCreatedData is published when a new squad is created
type SquadCreatedData struct {
	SquadID     string `json:"squad_id"`
	SquadName   string `json:"squad_name"`
	TeamID      string `json:"team_id"`
	TeamName    string `json:"team_name"`
	CreatorEOS  string `json:"creator_eos_id"`
	CreatorName string `json:"creator_name"`
}

func (d SquadCreatedData) GetEventType() EventType { return EventTypeSquadCreated }

// SquadDisbandedData is published when a squad is disbanded
type SquadDisbandedData struct {
	SquadID   string `json:"squad_id"`
	SquadName string `json:"squad_name"`
	TeamID    string `json:"team_id"`
	TeamName  string `json:"team_name"`
}

func (d SquadDisbandedData) GetEventType() EventType { return EventTypeSquadDisbanded }

// PlayerConnectedData is published when a player connects
type PlayerConnectedData struct {
	EOSID            string    `json:"eos_id"`
	SteamID          string    `json:"steam_id"`
	PlayerName       string    `json:"player_name"`
	IPAddress        string    `json:"ip_address"`
	PlayerController string    `json:"player_controller"`
	PlayerSuffix     string    `json:"player_suffix"`
	Timestamp        time.Time `json:"timestamp"`
}

func (d PlayerConnectedData) GetEventType() EventType { return EventTypePlayerConnected }

// PlayerDisconnectedData is published when a player disconnects
type PlayerDisconnectedData struct {
	EOSID            string        `json:"eos_id"`
	SteamID          string        `json:"steam_id"`
	PlayerName       string        `json:"player_name"`
	IPAddress        string        `json:"ip_address"`
	PlayerController string        `json:"player_controller"`
	PlayerSuffix     string        `json:"player_suffix"`
	TeamID           string        `json:"team_id"`
	SquadID          string        `json:"squad_id"`
	SessionDuration  time.Duration `json:"session_duration"`
	Timestamp        time.Time     `json:"timestamp"`
}

func (d PlayerDisconnectedData) GetEventType() EventType { return EventTypePlayerDisconnected }

// PlayerStatsUpdatedData is published when player statistics are updated
type PlayerStatsUpdatedData struct {
	EOSID      string    `json:"eos_id"`
	SteamID    string    `json:"steam_id"`
	PlayerName string    `json:"player_name"`
	TeamID     string    `json:"team_id"`
	SquadID    string    `json:"squad_id"`
	Kills      int       `json:"kills"`
	Deaths     int       `json:"deaths"`
	Score      int       `json:"score"`
	Role       string    `json:"role"`
	Kit        string    `json:"kit"`
	Timestamp  time.Time `json:"timestamp"`
}

func (d PlayerStatsUpdatedData) GetEventType() EventType { return EventTypePlayerStatsUpdated }
