package rconTypes

type Warn struct {
	Raw        string
	PlayerName string
	Message    string
}

type Ban struct {
	Raw        string
	PlayerID   string
	SteamID    string
	PlayerName string
	Interval   int
}

type Kick struct {
	Raw        string
	PlayerID   string
	EosID      string
	SteamID    string
	PlayerName string
}

type Message struct {
	Raw        string
	ChatType   string
	EosID      string
	SteamID    string
	PlayerName string
	Message    string
}

type PosAdminCam struct {
	Raw       string
	EosID     string
	SteamID   string
	AdminName string
}

type CurrentMap struct {
	Raw      string
	Level    string
	Layer    string
	Factions []string
}

type NextMap struct {
	Raw      string
	Level    string
	Layer    string
	Factions []string
}

type SquadCreated struct {
	Raw        string
	PlayerName string
	EosID      string
	SteamID    string
	SquadID    string
	SquadName  string
	TeamName   string
}

type UnposAdminCam struct {
	Raw       string
	EosID     string
	SteamID   string
	AdminName string
}

type ServerInfo struct {
	Raw              string
	ServerName       string `json:"ServerName_s"`
	MaxPlayers       int
	PublicQueueLimit int    `json:"PublicQueueLimit_I,string"`
	ReserveSlots     int    `json:"PlayerReserveCount_I,string"`
	PlayerCount      int    `json:"PlayerCount_I,string"`
	PublicQueue      int    `json:"PublicQueue_I,string"`
	ReserveQueue     int    `json:"ReservedQueue_I,string"`
	MatchTimeout     int    `json:"MatchTimeout_d"`
	MatchStartTime   int    `json:"PLAYTIME_I,string"`
	CurrentLayer     string `json:"MapName_s"`
	NextLayer        string `json:"NextLayer_s"`
	TeamOne          string `json:"TeamOne_s"`
	TeamTwo          string `json:"TeamTwo_s"`
	GameMode         string `json:"GameMode_s"`
	GameVersion      string `json:"GameVersion_s"`
}

type Player struct {
	PlayerID   string
	EosID      string
	SteamID    string
	PlayerName string
	TeamID     string
	SquadID    string
	Role       string
	IsLeader   bool
	IsInSquad  bool
}

type Players []Player

type Squad struct {
	SquadID        string
	SquadName      string
	Size           string
	CreatorName    string
	CreatorEOSID   string
	CreatorSteamID string
	TeamID         int
	TeamName       string
	Locked         bool
}

type Squads []Squad
