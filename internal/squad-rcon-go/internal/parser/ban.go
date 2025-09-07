package parser

import (
	"regexp"
	"strconv"
	"strings"

	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/rconEvents"
	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/rconTypes"
)

func ban(line string) (event string, data interface{}) {
	re := regexp.MustCompile(`Banned player ([0-9]+)\. \[steamid=(.*?)\] (.*) for interval (.*)`)
	matches := re.FindStringSubmatch(line)

	if matches != nil {
		interval, err := strconv.Atoi(matches[4])
		if err != nil {
			return rconEvents.PLAYER_BANNED, nil
		}

		return rconEvents.PLAYER_BANNED, rconTypes.Ban{
			Raw:        line,
			PlayerID:   matches[1],
			SteamID:    matches[2],
			PlayerName: strings.TrimSpace(matches[3]),
			Interval:   interval,
		}
	}

	return rconEvents.PLAYER_BANNED, nil
}
