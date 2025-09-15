package logwatcher_manager

// PlayerData represents persistent player information stored across game sessions
type PlayerData struct {
	PlayerController string `json:"playercontroller"`
	IP               string `json:"ip"`
	SteamID          string `json:"steam"`
	EOSID            string `json:"eos"`
	PlayerSuffix     string `json:"playerSuffix,omitempty"`
	Controller       string `json:"controller,omitempty"`
	TeamID           string `json:"teamID,omitempty"`
}

// SessionData represents temporary session data tied to player names
type SessionData struct {
	ChainID            string `json:"chainID,omitempty"`
	Time               string `json:"time,omitempty"`
	WoundTime          string `json:"woundTime,omitempty"`
	VictimName         string `json:"victimName,omitempty"`
	Damage             string `json:"damage,omitempty"`
	AttackerName       string `json:"attackerName,omitempty"`
	AttackerEOS        string `json:"attackerEOS,omitempty"`
	AttackerSteam      string `json:"attackerSteam,omitempty"`
	AttackerController string `json:"attackerController,omitempty"`
	Weapon             string `json:"weapon,omitempty"`
	TeamID             string `json:"teamID,omitempty"`
	EOSID              string `json:"eosID,omitempty"`
}

// JoinRequestData represents join request information stored by chainID
type JoinRequestData struct {
	PlayerController string `json:"playercontroller"`
	IP               string `json:"ip"`
	SteamID          string `json:"steam"`
	EOSID            string `json:"eos"`
}

// ToPlayerData converts JoinRequestData to PlayerData
func (j *JoinRequestData) ToPlayerData() *PlayerData {
	return &PlayerData{
		PlayerController: j.PlayerController,
		IP:               j.IP,
		SteamID:          j.SteamID,
		EOSID:            j.EOSID,
	}
}

// DisconnectedPlayerData represents data for players who have disconnected
type DisconnectedPlayerData struct {
	PlayerID         string `json:"playerID"`
	DisconnectTime   string `json:"disconnectTime,omitempty"`
	LastKnownTeamID  string `json:"lastKnownTeamID,omitempty"`
	PlayerController string `json:"playerController,omitempty"`
}

// RoundWinnerData represents round winner information
type RoundWinnerData struct {
	Time       string `json:"time"`
	ChainID    string `json:"chainID"`
	Team       string `json:"team"`
	Subfaction string `json:"subfaction"`
	Faction    string `json:"faction"`
	Action     string `json:"action"`
	Tickets    string `json:"tickets"`
	Layer      string `json:"layer"`
	Level      string `json:"level"`
}

// RoundLoserData represents round loser information
type RoundLoserData struct {
	Time       string `json:"time"`
	ChainID    string `json:"chainID"`
	Team       string `json:"team"`
	Subfaction string `json:"subfaction"`
	Faction    string `json:"faction"`
	Action     string `json:"action"`
	Tickets    string `json:"tickets"`
	Layer      string `json:"layer"`
	Level      string `json:"level"`
}

// WonData represents match winner information for new game correlation
type WonData struct {
	Time       string  `json:"time"`
	ChainID    string  `json:"chainID"`
	Winner     *string `json:"winner"` // Pointer to allow nil values
	Layer      string  `json:"layer"`
	Team       string  `json:"team,omitempty"`
	Subfaction string  `json:"subfaction,omitempty"`
	Faction    string  `json:"faction,omitempty"`
	Action     string  `json:"action,omitempty"`
	Tickets    string  `json:"tickets,omitempty"`
	Level      string  `json:"level,omitempty"`
}
