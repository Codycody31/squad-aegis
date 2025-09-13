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

// UpdateFromMap updates PlayerData from map[string]interface{} (for backwards compatibility during migration)
func (p *PlayerData) UpdateFromMap(data map[string]interface{}) {
	if val, ok := data["playercontroller"].(string); ok && val != "" {
		p.PlayerController = val
	}
	if val, ok := data["ip"].(string); ok && val != "" {
		p.IP = val
	}
	if val, ok := data["steam"].(string); ok && val != "" {
		p.SteamID = val
	}
	if val, ok := data["eos"].(string); ok && val != "" {
		p.EOSID = val
	}
	if val, ok := data["playerSuffix"].(string); ok && val != "" {
		p.PlayerSuffix = val
	}
	if val, ok := data["controller"].(string); ok && val != "" {
		p.Controller = val
	}
	if val, ok := data["teamID"].(string); ok && val != "" {
		p.TeamID = val
	}
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

// UpdateFromMap updates SessionData from map[string]interface{} (for backwards compatibility during migration)
func (s *SessionData) UpdateFromMap(data map[string]interface{}) {
	if val, ok := data["chainID"].(string); ok && val != "" {
		s.ChainID = val
	}
	if val, ok := data["time"].(string); ok && val != "" {
		s.Time = val
	}
	if val, ok := data["woundTime"].(string); ok && val != "" {
		s.WoundTime = val
	}
	if val, ok := data["victimName"].(string); ok && val != "" {
		s.VictimName = val
	}
	if val, ok := data["damage"].(string); ok && val != "" {
		s.Damage = val
	}
	if val, ok := data["attackerName"].(string); ok && val != "" {
		s.AttackerName = val
	}
	if val, ok := data["attackerEOS"].(string); ok && val != "" {
		s.AttackerEOS = val
	}
	if val, ok := data["attackerSteam"].(string); ok && val != "" {
		s.AttackerSteam = val
	}
	if val, ok := data["attackerController"].(string); ok && val != "" {
		s.AttackerController = val
	}
	if val, ok := data["weapon"].(string); ok && val != "" {
		s.Weapon = val
	}
	if val, ok := data["teamID"].(string); ok && val != "" {
		s.TeamID = val
	}
	if val, ok := data["eosID"].(string); ok && val != "" {
		s.EOSID = val
	}
}

// NewSessionDataFromMap creates SessionData from map[string]interface{} (for backwards compatibility)
func NewSessionDataFromMap(data map[string]interface{}) *SessionData {
	s := &SessionData{}
	s.UpdateFromMap(data)
	return s
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

// NewWonDataFromMap creates WonData from map[string]interface{} (for backwards compatibility)
func NewWonDataFromMap(data map[string]interface{}) *WonData {
	w := &WonData{}
	if val, ok := data["time"].(string); ok {
		w.Time = val
	}
	if val, ok := data["chainID"].(string); ok {
		w.ChainID = val
	}
	if val, ok := data["layer"].(string); ok {
		w.Layer = val
	}
	if val, ok := data["winner"].(string); ok {
		w.Winner = &val
	} else if data["winner"] == nil {
		w.Winner = nil
	}
	if val, ok := data["team"].(string); ok {
		w.Team = val
	}
	if val, ok := data["subfaction"].(string); ok {
		w.Subfaction = val
	}
	if val, ok := data["faction"].(string); ok {
		w.Faction = val
	}
	if val, ok := data["action"].(string); ok {
		w.Action = val
	}
	if val, ok := data["tickets"].(string); ok {
		w.Tickets = val
	}
	if val, ok := data["level"].(string); ok {
		w.Level = val
	}
	return w
}
