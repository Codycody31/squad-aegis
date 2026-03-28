package parser

import (
	"regexp"

	"github.com/SquadGO/squad-rcon-go/v2/rconEvents"
	"github.com/SquadGO/squad-rcon-go/v2/rconTypes"
)

func showNextMap(line string) (event string, data interface{}) {
	re := regexp.MustCompile(`^Next level is (.*), layer is (.*)`)
	matches := re.FindStringSubmatch(line)

	if matches != nil {
		return rconEvents.SHOW_NEXT_MAP, rconTypes.NextMap{
			Raw:   line,
			Level: matches[1],
			Layer: matches[2],
		}
	}

	return rconEvents.SHOW_NEXT_MAP, nil
}
