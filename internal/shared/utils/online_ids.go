package utils

import (
	"regexp"
	"strings"
)

var onlineIDTokenRegexp = regexp.MustCompile(`\b(EOS|epic|steam):\s*([^\s|)]+)`)

// OnlineIDs represents the identifiers surfaced inside Squad "Online IDs"
// log/RCON segments.
type OnlineIDs struct {
	EOSID   string
	EpicID  string
	SteamID string
}

// ParseOnlineIDs extracts EOS, Epic, and Steam identifiers from a raw
// "Online IDs:" segment. Invalid identifiers are ignored.
func ParseOnlineIDs(segment string) OnlineIDs {
	ids := OnlineIDs{}

	for _, match := range onlineIDTokenRegexp.FindAllStringSubmatch(segment, -1) {
		if len(match) < 3 {
			continue
		}

		key := strings.ToLower(match[1])
		value := strings.TrimSpace(match[2])

		switch key {
		case "eos":
			ids.EOSID = NormalizeEOSID(value)
		case "epic":
			ids.EpicID = NormalizeEOSID(value)
		case "steam":
			ids.SteamID = NormalizePlayerID(value)
			if !IsSteamID(ids.SteamID) {
				ids.SteamID = ""
			}
		}
	}

	return ids
}
