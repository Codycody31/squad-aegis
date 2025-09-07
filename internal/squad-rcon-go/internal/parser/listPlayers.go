package parser

import (
	"regexp"
	"strings"

	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/rconEvents"
	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/rconTypes"
)

func listPlayers(line string) (event string, data interface{}) {
	re := regexp.MustCompile(`ID: ([0-9]+) \| Online IDs: EOS: ([0-9a-f]{32}) steam: (\d{17}) \| Name: (.+) \| Team ID: ([0-9]+) \| Squad ID: ([0-9]+|N\/A) \| Is Leader: (True|False) \| Role: ([A-Za-z0-9_]*)\b`)
	strs := strings.Split(line, "\n")
	players := make(rconTypes.Players, 0)

	for _, v := range strs {
		matches := re.FindStringSubmatch(v)
		if matches == nil {
			continue
		}

		players = append(players, rconTypes.Player{
			PlayerID:   matches[1],
			EosID:      matches[2],
			SteamID:    matches[3],
			PlayerName: strings.TrimSpace(matches[4]),
			TeamID:     matches[5],
			SquadID:    matches[6],
			IsInSquad:  matches[6] != "N/A",
			IsLeader:   matches[7] == "True",
			Role:       matches[8],
		})
	}

	if len(players) == 0 {
		return rconEvents.LIST_PLAYERS, nil
	}

	return rconEvents.LIST_PLAYERS, players
}
