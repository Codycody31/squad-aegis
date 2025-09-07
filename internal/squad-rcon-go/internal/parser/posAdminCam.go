package parser

import (
	"regexp"
	"strings"

	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/rconEvents"
	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/rconTypes"
)

func posAdminCam(line string) (event string, data interface{}) {
	re := regexp.MustCompile(`\[Online Ids:EOS: ([0-9a-f]{32}) steam: (\d{17})\] (.+) has possessed admin camera\.`)
	matches := re.FindStringSubmatch(line)

	if matches != nil {
		return rconEvents.POSSESSED_ADMIN_CAMERA, rconTypes.PosAdminCam{
			Raw:       line,
			EosID:     matches[1],
			SteamID:   matches[2],
			AdminName: strings.TrimSpace(matches[3]),
		}
	}

	return rconEvents.POSSESSED_ADMIN_CAMERA, nil
}
