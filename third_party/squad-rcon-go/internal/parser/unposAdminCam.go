package parser

import (
	"regexp"
	"strings"

	"github.com/SquadGO/squad-rcon-go/v2/rconEvents"
	"github.com/SquadGO/squad-rcon-go/v2/rconTypes"
)

func unposAdminCam(line string) (event string, data interface{}) {
	re := regexp.MustCompile(`\[Online IDs:EOS: ([0-9a-f]{32}) steam: (\d{17})\] (.+) has unpossessed admin camera\.`)
	matches := re.FindStringSubmatch(line)

	if matches != nil {
		return rconEvents.UNPOSSESSED_ADMIN_CAMERA, rconTypes.UnposAdminCam{
			Raw:       line,
			EosID:     matches[1],
			SteamID:   matches[2],
			AdminName: strings.TrimSpace(matches[3]),
		}
	}

	return rconEvents.POSSESSED_ADMIN_CAMERA, nil
}
