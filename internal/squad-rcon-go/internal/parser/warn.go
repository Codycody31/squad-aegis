package parser

import (
	"regexp"
	"strings"

	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/rconEvents"
	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/rconTypes"
)

func warn(line string) (event string, data interface{}) {
	re := regexp.MustCompile(`Remote admin has warned player (.*)\. Message was "([\s\S]*?)"`)
	matches := re.FindStringSubmatch(line)

	if matches != nil {
		return rconEvents.PLAYER_WARNED, rconTypes.Warn{
			Raw:        line,
			PlayerName: strings.TrimSpace(matches[1]),
			Message:    matches[2],
		}
	}

	return rconEvents.PLAYER_WARNED, nil
}
