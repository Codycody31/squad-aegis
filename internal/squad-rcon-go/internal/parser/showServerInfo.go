package parser

import (
	"encoding/json"

	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/rconEvents"
	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/rconTypes"
)

func showServerInfo(line string) (event string, data interface{}) {
	if len(line) > 10 {
		info := rconTypes.ServerInfo{}

		err := json.Unmarshal([]byte(line), &info)
		if err != nil {
			return rconEvents.SHOW_SERVER_INFO, nil
		}

		return rconEvents.SHOW_SERVER_INFO, info
	}

	return rconEvents.SHOW_SERVER_INFO, nil
}
