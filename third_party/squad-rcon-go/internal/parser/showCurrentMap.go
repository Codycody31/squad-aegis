package parser

import (
	"regexp"
	"strings"

	"github.com/SquadGO/squad-rcon-go/v2/rconEvents"
	"github.com/SquadGO/squad-rcon-go/v2/rconTypes"
)

func showCurrentMap(line string) (event string, data interface{}) {
	re := regexp.MustCompile(`^Current level is (.*), layer is (.*), factions (.*)`)
	matches := re.FindStringSubmatch(line)

	if matches != nil {
		return rconEvents.SHOW_CURRENT_MAP, rconTypes.CurrentMap{
			Raw:      line,
			Level:    matches[1],
			Layer:    matches[2],
			Factions: strings.Split(matches[3], " "),
		}
	}

	return rconEvents.SHOW_CURRENT_MAP, nil
}
