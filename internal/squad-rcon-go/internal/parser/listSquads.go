package parser

import (
	"regexp"
	"strconv"
	"strings"

	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/rconEvents"
	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/rconTypes"
)

func listSquads(line string) (event string, data interface{}) {
	re := regexp.MustCompile(`ID: ([0-9]+) \| Name: (.+) \| Size: ([0-9]+) \| Locked: (True|False) \| Creator Name: (.+) \| Creator Online IDs: EOS: ([0-9a-f]{32}) steam: (\d{17})`)
	strs := strings.Split(line, "\n")
	squads := make(rconTypes.Squads, 0)
	teamID := 0
	teamName := ""

	for _, v := range strs {
		matches := re.FindStringSubmatch(v)
		teamMatches := regexp.MustCompile(`Team ID: (1|2) \((.+)\)/`).FindStringSubmatch(v)
		if teamMatches != nil {
			if id, err := strconv.Atoi(teamMatches[1]); err == nil {
				teamID = id
			}

			teamName = teamMatches[2]
		}

		if matches == nil {
			continue
		}

		squads = append(squads, rconTypes.Squad{
			SquadID:        matches[1],
			SquadName:      matches[2],
			Size:           matches[3],
			Locked:         matches[4] == "True",
			CreatorName:    matches[5],
			CreatorEOSID:   matches[6],
			CreatorSteamID: matches[7],
			TeamID:         teamID,
			TeamName:       teamName,
		})
	}

	if len(squads) == 0 {
		return rconEvents.LIST_SQUADS, nil
	}

	return rconEvents.LIST_SQUADS, squads

}
