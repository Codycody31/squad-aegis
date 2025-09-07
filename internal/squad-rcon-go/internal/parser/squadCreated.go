package parser

import (
	"regexp"
	"strings"

	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/rconEvents"
	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/rconTypes"
)

func squadCreated(line string) (event string, data interface{}) {
	re := regexp.MustCompile(`(.+) \(Online IDs: EOS: ([0-9a-f]{32}) steam: (\d{17})\) has created Squad (\d+) \(Squad Name: (.+)\) on (.+)`)
	matches := re.FindStringSubmatch(line)

	if matches != nil {
		return rconEvents.SQUAD_CREATED, rconTypes.SquadCreated{
			Raw:        line,
			PlayerName: strings.TrimSpace(matches[1]),
			EosID:      matches[2],
			SteamID:    matches[3],
			SquadID:    matches[4],
			SquadName:  matches[5],
			TeamName:   matches[6],
		}
	}

	return rconEvents.SQUAD_CREATED, nil
}
