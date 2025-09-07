package parser

import (
	"regexp"
	"strings"

	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/rconEvents"
	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/rconTypes"
)

func kick(line string) (event string, data interface{}) {
	re := regexp.MustCompile(`Kicked player ([0-9]+)\. \[Online IDs= EOS: ([0-9a-f]{32}) steam: (\d{17})] (.*)`)
	matches := re.FindStringSubmatch(line)

	if matches != nil {
		return rconEvents.PLAYER_KICKED, rconTypes.Kick{
			Raw:        line,
			PlayerID:   matches[1],
			EosID:      matches[2],
			SteamID:    matches[3],
			PlayerName: strings.TrimSpace(matches[4]),
		}
	}

	return rconEvents.PLAYER_KICKED, nil
}
