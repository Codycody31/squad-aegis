package parser

import (
	"github.com/SquadGO/squad-rcon-go/v2/rconEvents"
	"github.com/iamalone98/eventEmitter"
)

var parsers = []func(string) (event string, data interface{}){
	ban,
	kick,
	message,
	posAdminCam,
	unposAdminCam,
	squadCreated,
	warn,
	/* COMMANDS */
	listPlayers,
	listSquads,
	showCurrentMap,
	showNextMap,
	showServerInfo,
}

func RconParser(line string, emitter eventEmitter.EventEmitter) {
	for _, fn := range parsers {
		event, data := fn(line)

		if data != nil {
			emitter.Emit(rconEvents.DATA, data)
			emitter.Emit(event, data)
			break
		}
	}
}
