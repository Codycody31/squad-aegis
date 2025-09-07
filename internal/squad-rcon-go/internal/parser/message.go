package parser

import (
	"regexp"
	"strings"

	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/rconEvents"
	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/rconTypes"
)

func message(line string) (event string, data interface{}) {
	re := regexp.MustCompile(`\[(ChatAll|ChatTeam|ChatSquad|ChatAdmin)] \[Online IDs:EOS: ([0-9a-f]{32}) steam: (\d{17})\] (.+?) : (.*)`)
	matches := re.FindStringSubmatch(line)

	if matches != nil {
		return rconEvents.CHAT_MESSAGE, rconTypes.Message{
			Raw:        line,
			ChatType:   matches[1],
			EosID:      matches[2],
			SteamID:    matches[3],
			PlayerName: strings.TrimSpace(matches[4]),
			Message:    matches[5],
		}
	}

	return rconEvents.CHAT_MESSAGE, nil
}
