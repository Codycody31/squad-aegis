package event_manager

// PlayerInfo represents player information used in events
type PlayerInfo struct {
	PlayerController string `json:"playercontroller,omitempty"`
	IP               string `json:"ip,omitempty"`
	SteamID          string `json:"steam,omitempty"`
	EOSID            string `json:"eos,omitempty"`
	PlayerSuffix     string `json:"playerSuffix,omitempty"`
	Controller       string `json:"controller,omitempty"`
	TeamID           string `json:"teamID,omitempty"`
	SquadID          string `json:"squadID,omitempty"`
}

// RoundWinnerInfo represents round winner information in events
type RoundWinnerInfo struct {
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

// RoundLoserInfo represents round loser information in events
type RoundLoserInfo struct {
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
