package event_manager

// EventData is the base interface that all event data types must implement
type EventData interface {
	GetEventType() EventType
}

// RCON Event Data Types

// RconChatMessageData represents RCON chat message event data
type RconChatMessageData struct {
	ChatType   string `json:"chat_type"`
	EosID      string `json:"eos_id"`
	SteamID    string `json:"steam_id"`
	PlayerName string `json:"player_name"`
	Message    string `json:"message"`
}

func (d RconChatMessageData) GetEventType() EventType { return EventTypeRconChatMessage }

// RconPlayerWarnedData represents RCON player warned event data
type RconPlayerWarnedData struct {
	PlayerName string `json:"player_name"`
	Message    string `json:"message"`
}

func (d RconPlayerWarnedData) GetEventType() EventType { return EventTypeRconPlayerWarned }

// RconPlayerKickedData represents RCON player kicked event data
type RconPlayerKickedData struct {
	PlayerID   string `json:"player_id,omitempty"`
	EosID      string `json:"eos_id,omitempty"`
	SteamID    string `json:"steam_id,omitempty"`
	PlayerName string `json:"player_name"`
}

func (d RconPlayerKickedData) GetEventType() EventType { return EventTypeRconPlayerKicked }

// RconPlayerBannedData represents RCON player banned event data
type RconPlayerBannedData struct {
	PlayerID   string `json:"player_id,omitempty"`
	SteamID    string `json:"steam_id,omitempty"`
	PlayerName string `json:"player_name"`
	Interval   int    `json:"interval"`
}

func (d RconPlayerBannedData) GetEventType() EventType { return EventTypeRconPlayerBanned }

// RconAdminCameraData represents RCON admin camera possession events
type RconAdminCameraData struct {
	EosID     string `json:"eos_id"`
	SteamID   string `json:"steam_id"`
	AdminName string `json:"admin_name"`
	Action    string `json:"action"` // "possessed" or "unpossessed"
}

func (d RconAdminCameraData) GetEventType() EventType {
	if d.Action == "possessed" {
		return EventTypeRconPossessedAdminCamera
	}
	return EventTypeRconUnpossessedAdminCamera
}

// RconSquadCreatedData represents RCON squad creation event data
type RconSquadCreatedData struct {
	PlayerName string `json:"player_name"`
	EosID      string `json:"eos_id"`
	SteamID    string `json:"steam_id"`
	SquadID    string `json:"squad_id"`
	SquadName  string `json:"squad_name"`
	TeamName   string `json:"team_name"`
}

func (d RconSquadCreatedData) GetEventType() EventType { return EventTypeRconSquadCreated }

// RconServerInfoData represents RCON server info event data
type RconServerInfoData struct {
	PlayerCount     int `json:"player_count"`
	PublicQueue     int `json:"public_queue"`
	ReservedQueue   int `json:"reserved_queue"`
	TotalQueueCount int `json:"total_queue_count"` // PublicQueue + ReservedQueue
}

func (d RconServerInfoData) GetEventType() EventType { return EventTypeRconServerInfo }

// Log Event Data Types

// LogAdminBroadcastData represents log admin broadcast event data
type LogAdminBroadcastData struct {
	Time    string `json:"time"`
	ChainID string `json:"chain_id"`
	Message string `json:"message"`
	From    string `json:"from"`
}

func (d LogAdminBroadcastData) GetEventType() EventType { return EventTypeLogAdminBroadcast }

// LogDeployableDamagedData represents log deployable damaged event data
type LogDeployableDamagedData struct {
	Time            string `json:"time"`
	ChainID         string `json:"chain_id"`
	Deployable      string `json:"deployable"`
	Damage          string `json:"damage"`
	Weapon          string `json:"weapon"`
	PlayerSuffix    string `json:"player_suffix"`
	DamageType      string `json:"damage_type"`
	HealthRemaining string `json:"health_remaining"`
}

func (d LogDeployableDamagedData) GetEventType() EventType { return EventTypeLogDeployableDamaged }

// LogPlayerConnectedData represents log player connected event data
type LogPlayerConnectedData struct {
	Time             string `json:"time"`
	ChainID          string `json:"chain_id"`
	PlayerController string `json:"player_controller"`
	IPAddress        string `json:"ip_address"`
	SteamID          string `json:"steam_id,omitempty"`
	EOSID            string `json:"eos_id,omitempty"`
}

func (d LogPlayerConnectedData) GetEventType() EventType { return EventTypeLogPlayerConnected }

// LogPlayerDamagedData represents log player damaged event data
type LogPlayerDamagedData struct {
	Time               string      `json:"time"`
	ChainID            string      `json:"chain_id"`
	VictimName         string      `json:"victim_name,omitempty"`
	Damage             string      `json:"damage"`
	AttackerName       string      `json:"attacker_name,omitempty"`
	AttackerController string      `json:"attacker_controller"`
	Weapon             string      `json:"weapon"`
	AttackerEOS        string      `json:"attacker_eos,omitempty"`
	AttackerSteam      string      `json:"attacker_steam,omitempty"`
	Victim             *PlayerInfo `json:"victim,omitempty"`
	Attacker           *PlayerInfo `json:"attacker,omitempty"`
	Teamkill           bool        `json:"teamkill,omitempty"`
}

func (d LogPlayerDamagedData) GetEventType() EventType { return EventTypeLogPlayerDamaged }

// LogPlayerDiedData represents log player died event data
type LogPlayerDiedData struct {
	Time                     string      `json:"time"`
	WoundTime                string      `json:"wound_time,omitempty"`
	ChainID                  string      `json:"chain_id"`
	VictimName               string      `json:"victim_name,omitempty"`
	Damage                   string      `json:"damage"`
	AttackerPlayerController string      `json:"attacker_player_controller"`
	Weapon                   string      `json:"weapon"`
	AttackerEOS              string      `json:"attacker_eos,omitempty"`
	AttackerSteam            string      `json:"attacker_steam,omitempty"`
	Victim                   *PlayerInfo `json:"victim,omitempty"`
	Attacker                 *PlayerInfo `json:"attacker,omitempty"`
	Teamkill                 bool        `json:"teamkill,omitempty"`
}

func (d LogPlayerDiedData) GetEventType() EventType { return EventTypeLogPlayerDied }

// LogPlayerWoundedData represents log player wounded event data
type LogPlayerWoundedData struct {
	Time                     string      `json:"time"`
	ChainID                  string      `json:"chain_id"`
	VictimName               string      `json:"victim_name,omitempty"`
	Damage                   string      `json:"damage"`
	AttackerPlayerController string      `json:"attacker_player_controller"`
	Weapon                   string      `json:"weapon"`
	AttackerEOS              string      `json:"attacker_eos,omitempty"`
	AttackerSteam            string      `json:"attacker_steam,omitempty"`
	Victim                   *PlayerInfo `json:"victim,omitempty"`
	Attacker                 *PlayerInfo `json:"attacker,omitempty"`
	Teamkill                 bool        `json:"teamkill,omitempty"`
}

func (d LogPlayerWoundedData) GetEventType() EventType { return EventTypeLogPlayerWounded }

// LogPlayerRevivedData represents log player revived event data
type LogPlayerRevivedData struct {
	Time         string `json:"time"`
	ChainID      string `json:"chain_id"`
	ReviverName  string `json:"reviver_name"`
	VictimName   string `json:"victim_name"`
	ReviverEOS   string `json:"reviver_eos,omitempty"`
	ReviverSteam string `json:"reviver_steam,omitempty"`
	VictimEOS    string `json:"victim_eos,omitempty"`
	VictimSteam  string `json:"victim_steam,omitempty"`
}

func (d LogPlayerRevivedData) GetEventType() EventType { return EventTypeLogPlayerRevived }

// LogPlayerPossessData represents log player possess event data
type LogPlayerPossessData struct {
	Time             string `json:"time"`
	ChainID          string `json:"chain_id"`
	PlayerSuffix     string `json:"player_suffix"`
	PossessClassname string `json:"possess_classname"`
	PlayerEOS        string `json:"player_eos,omitempty"`
	PlayerSteam      string `json:"player_steam,omitempty"`
}

func (d LogPlayerPossessData) GetEventType() EventType { return EventTypeLogPlayerPossess }

// LogJoinSucceededData represents log join succeeded event data
type LogJoinSucceededData struct {
	Time         string `json:"time"`
	ChainID      string `json:"chain_id"`
	PlayerSuffix string `json:"player_suffix"`
	IPAddress    string `json:"ip_address,omitempty"`
	SteamID      string `json:"steam_id,omitempty"`
	EOSID        string `json:"eos_id,omitempty"`
}

func (d LogJoinSucceededData) GetEventType() EventType { return EventTypeLogJoinSucceeded }

// LogTickRateData represents log tick rate event data
type LogTickRateData struct {
	Time     string `json:"time"`
	ChainID  string `json:"chain_id"`
	TickRate string `json:"tick_rate"`
}

func (d LogTickRateData) GetEventType() EventType { return EventTypeLogTickRate }

// LogGameEventUnifiedData represents a unified game event that can handle multiple event types
type LogGameEventUnifiedData struct {
	Time      string `json:"time"`
	ChainID   string `json:"chain_id,omitempty"`
	EventType string `json:"event_type"` // "ROUND_ENDED", "NEW_GAME", "MATCH_WINNER", "TICKET_UPDATE"

	// Round/Match data
	Winner     string `json:"winner,omitempty"`
	Layer      string `json:"layer,omitempty"`
	Team       string `json:"team,omitempty"`
	Subfaction string `json:"subfaction,omitempty"`
	Faction    string `json:"faction,omitempty"`
	Action     string `json:"action,omitempty"` // "won" or "lost"
	Tickets    string `json:"tickets,omitempty"`
	Level      string `json:"level,omitempty"`

	// New Game data
	DLC            string `json:"dlc,omitempty"`
	MapClassname   string `json:"map_classname,omitempty"`
	LayerClassname string `json:"layer_classname,omitempty"`

	// Game state data
	FromState string `json:"from_state,omitempty"`
	ToState   string `json:"to_state,omitempty"`

	// Complex data as JSON strings
	WinnerData string `json:"winner_data,omitempty"`
	LoserData  string `json:"loser_data,omitempty"`
	Metadata   string `json:"metadata,omitempty"`

	// Raw log line
	RawLog string `json:"raw_log"`
}

func (d LogGameEventUnifiedData) GetEventType() EventType { return EventTypeLogGameEventUnified }

// LogPlayerDisconnectedData represents log player disconnected event data
type LogPlayerDisconnectedData struct {
	Time             string `json:"time"`
	ChainID          string `json:"chain_id"`
	IP               string `json:"ip,omitempty"`
	PlayerController string `json:"player_controller"`
	PlayerSuffix     string `json:"player_suffix,omitempty"`
	TeamID           string `json:"team_id,omitempty"`
	SteamID          string `json:"steam_id,omitempty"`
	EOSID            string `json:"eos_id,omitempty"`
}

func (d LogPlayerDisconnectedData) GetEventType() EventType { return EventTypeLogPlayerDisconnected }
