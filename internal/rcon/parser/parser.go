package parser

import (
	"regexp"
	"strings"
)

const (
	warn          = `Remote admin has warned player (.*)\. Message was "([\s\S]*?)"`
	kick          = `Kicked player ([0-9]+)\. \[Online IDs= EOS: ([0-9a-f]{32}) steam: (\d{17})] (.*)`
	message       = `\[(ChatAll|ChatTeam|ChatSquad|ChatAdmin)] \[Online IDs:EOS: ([0-9a-f]{32}) steam: (\d{17})\] (.+?) : (.*)`
	posAdminCam   = `\[Online Ids:EOS: ([0-9a-f]{32}) steam: (\d{17})\] (.+) has possessed admin camera\.`
	unposAdminCam = `\[Online IDs:EOS: ([0-9a-f]{32}) steam: (\d{17})\] (.+) has unpossessed admin camera\.`
	squadCreated  = `(.+) \(Online IDs: EOS: ([0-9a-f]{32}) steam: (\d{17})\) has created Squad (\d+) \(Squad Name: (.+)\) on (.+)`

	listPlayers = `ID: ([0-9]+) \| Online IDs: EOS: ([0-9a-f]{32}) steam: (\d{17}) \| Name: (.+) \| Team ID: ([0-9]+) \| Squad ID: ([0-9]+|N\/A) \| Is Leader: (True|False) \| Role: ([A-Za-z0-9_]*)\b`
	listSquads  = `ID: ([0-9]+) \| Name: (.+) \| Size: ([0-9]+) \| Locked: (True|False) \| Creator Name: (.+) \| Creator Online IDs: EOS: ([0-9a-f]{32}) steam: (\d{17})`
)

type Warn struct {
	PlayerName string
	Message    string
}

type Kick struct {
	PlayerID   string
	EosID      string
	SteamID    string
	PlayerName string
}

type Message struct {
	ChatType   string
	EosID      string
	SteamID    string
	PlayerName string
	Message    string
}

type PosAdminCam struct {
	EosID     string
	SteamID   string
	AdminName string
}

type UnposAdminCam struct {
	EosID     string
	SteamID   string
	AdminName string
}

type SquadCreated struct {
	PlayerName string
	EosID      string
	SteamID    string
	SquadID    string
	SquadName  string
	TeamName   string
}

func ChatParser(str string) interface{} {
	var re *regexp.Regexp
	var matches []string

	// WARN
	re = regexp.MustCompile(warn)
	matches = re.FindStringSubmatch(str)

	if matches != nil {
		return Warn{
			PlayerName: matches[1],
			Message:    matches[2],
		}
	}

	// KICK
	re = regexp.MustCompile(kick)
	matches = re.FindStringSubmatch(str)

	if matches != nil {
		return Kick{
			PlayerID:   matches[1],
			EosID:      matches[2],
			SteamID:    matches[3],
			PlayerName: matches[4],
		}
	}

	// MESSAGE
	re = regexp.MustCompile(message)
	matches = re.FindStringSubmatch(str)

	if matches != nil {
		return Message{
			ChatType:   matches[1],
			EosID:      matches[2],
			SteamID:    matches[3],
			PlayerName: matches[4],
			Message:    matches[5],
		}
	}

	// POSADMINCAM
	re = regexp.MustCompile(posAdminCam)
	matches = re.FindStringSubmatch(str)

	if matches != nil {
		return PosAdminCam{
			EosID:     matches[1],
			SteamID:   matches[2],
			AdminName: matches[3],
		}
	}

	// UNPOSADMINCAM
	re = regexp.MustCompile(unposAdminCam)
	matches = re.FindStringSubmatch(str)

	if matches != nil {
		return UnposAdminCam{
			EosID:     matches[1],
			SteamID:   matches[2],
			AdminName: matches[3],
		}
	}

	// SQUADCREATED
	re = regexp.MustCompile(squadCreated)
	matches = re.FindStringSubmatch(str)

	if matches != nil {
		return SquadCreated{
			PlayerName: matches[1],
			EosID:      matches[2],
			SteamID:    matches[3],
			SquadID:    matches[4],
			SquadName:  matches[5],
			TeamName:   matches[6],
		}
	}

	return nil
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

type Squad struct {
	SquadID        string
	SquadName      string
	Size           string
	CreatorName    string
	CreatorEOSID   string
	CreatorSteamID string
	TeamID         string
	TeamName       string
	Locked         bool
}

type Players []Player
type Squads []Squad

func CommandParser(str string, command string) interface{} {
	strs := strings.Split(str, "\n")
	players := make(Players, 0)
	squads := make(Squads, 0)
	teamID := ""
	teamName := ""

	switch command {
	case "ListPlayers":
		{
			for _, v := range strs {
				re := regexp.MustCompile(listPlayers)
				matches := re.FindStringSubmatch(v)

				if matches == nil {
					continue
				}

				players = append(players, Player{
					PlayerID:   matches[1],
					EosID:      matches[2],
					SteamID:    matches[3],
					PlayerName: matches[4],
					TeamID:     matches[5],
					SquadID:    matches[6],
					IsInSquad:  matches[6] != "N/A",
					IsLeader:   matches[7] == "True",
					Role:       matches[8],
				})
			}

			return players
		}

	case "ListSquads":
		{
			for _, v := range strs {
				re := regexp.MustCompile(listSquads)
				matches := re.FindStringSubmatch(v)

				teamMatches := regexp.MustCompile(`Team ID: (1|2) \((.+)\)/`).FindStringSubmatch(v)

				if teamMatches != nil {
					teamID = teamMatches[1]
					teamName = teamMatches[2]
				}

				if matches == nil {
					continue
				}

				squads = append(squads, Squad{
					SquadID:        matches[1],
					SquadName:      matches[2],
					Size:           matches[3],
					Locked:         matches[4] == "True",
					CreatorName:    matches[5],
					CreatorEOSID:   matches[6],
					CreatorSteamID: matches[7],
					TeamID:         teamID,
					TeamName:       teamName,
				})
			}

			return squads
		}
	}

	return nil
}
