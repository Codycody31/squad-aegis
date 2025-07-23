package utils

import (
	"errors"
	"strings"

	"github.com/SquadGO/squad-rcon-go/v2/rconTypes"
)

type CommandMessage struct {
	rconTypes.Message
	Command string
	Args    string
}

func ParseRconCommandMessage(message rconTypes.Message) (CommandMessage, error) {
	if !strings.HasPrefix(message.Message, "!") {
		return CommandMessage{}, errors.New("message does not start with '!'")
	}

	commandText := message.Message[1:]
	parts := strings.SplitN(commandText, " ", 2)
	command := parts[0]
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}

	return CommandMessage{
		Message: message,
		Command: command,
		Args:    args,
	}, nil
}
