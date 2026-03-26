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

// SanitizeRCONParam strips characters that could be used to inject additional
// RCON commands or break out of quoted parameters. This includes double quotes
// (which delimit RCON arguments) and newline characters (which delimit RCON
// commands at the protocol level).
func SanitizeRCONParam(s string) string {
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\x00", "")
	return s
}

// SanitizeAndQuoteRCONParam sanitizes a parameter and wraps it in double
// quotes so multi-word values are treated as a single RCON argument.
func SanitizeAndQuoteRCONParam(s string) string {
	s = SanitizeRCONParam(s)
	return `"` + s + `"`
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
