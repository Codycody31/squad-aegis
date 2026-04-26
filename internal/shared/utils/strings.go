package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func ReturnOldIfEmpty(oldValue, newValue string) string {
	if newValue == "" {
		return oldValue
	}
	return newValue
}

// IsHex returns true if s consists entirely of hexadecimal characters.
func IsHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return len(s) > 0
}

// IsEOSID returns true if the string looks like an EOS ID (32-char hex).
func IsEOSID(s string) bool {
	return len(s) == 32 && IsHex(s)
}

// NormalizeEOSID trims surrounding whitespace and lowercases the identifier so
// comparisons are consistent across UI input, imports, DB storage, and logs.
// Returns an empty string if the result is not a valid 32-char hex EOS ID.
func NormalizeEOSID(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if !IsEOSID(s) {
		return ""
	}
	return s
}

// NormalizePlayerID trims whitespace and normalizes EOS identifiers to their
// canonical lowercase representation. Steam IDs are returned trimmed as-is.
func NormalizePlayerID(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	normalizedEOSID := NormalizeEOSID(s)
	if IsEOSID(normalizedEOSID) {
		return normalizedEOSID
	}

	return s
}

// MatchPlayerID reports whether playerID matches either the provided Steam or
// EOS identifier after normalization.
func MatchPlayerID(playerID, steamID, eosID string) bool {
	playerID = NormalizePlayerID(playerID)
	if playerID == "" {
		return false
	}

	if steamID != "" && playerID == NormalizePlayerID(steamID) {
		return true
	}
	if eosID != "" && playerID == NormalizePlayerID(eosID) {
		return true
	}

	return false
}

// ParsePlayerID validates a generic player identifier and returns either a
// Steam ID or EOS ID ready for storage.
func ParsePlayerID(playerID string) (steamID *int64, eosID *string, normalized string, err error) {
	normalized = NormalizePlayerID(playerID)
	if normalized == "" {
		return nil, nil, "", fmt.Errorf("player ID is required")
	}

	if IsSteamID(normalized) {
		value, parseErr := strconv.ParseInt(normalized, 10, 64)
		if parseErr != nil {
			return nil, nil, "", fmt.Errorf("invalid steam ID format: %w", parseErr)
		}
		return &value, nil, normalized, nil
	}

	if IsEOSID(normalized) {
		return nil, &normalized, normalized, nil
	}

	return nil, nil, "", fmt.Errorf("player ID must be a valid Steam ID or EOS ID")
}

// IsSteamID returns true if the string looks like a valid Steam ID 64.
// Valid Steam IDs are unsigned 64-bit integers starting at 76561197960265728.
func IsSteamID(s string) bool {
	val, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return false
	}
	return val >= 76561197960265728
}

// IsSquadID returns true if the string looks like an in-match Squad player ID.
// Squad assigns small unsigned integer IDs (typically 1..hundreds) to players
// in a match; the RCON command AdminRemovePlayerFromSquadById accepts these.
// We accept 1-4 digit positive integers below the Steam ID floor to avoid
// overlap with Steam IDs.
func IsSquadID(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if len(s) > 4 {
		return false
	}
	val, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return false
	}
	return val > 0
}

// IsAnyPlayerID returns true if s is a recognized Steam, EOS, or Squad ID.
// Squad IDs are only meaningful for in-match RCON commands; do not accept them
// for ban/kick/warn flows that need persistent identifiers.
func IsAnyPlayerID(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if IsSteamID(s) {
		return true
	}
	normalizedEOS := NormalizeEOSID(s)
	if IsEOSID(normalizedEOS) {
		return true
	}
	return IsSquadID(s)
}

// SanitizeBanReason replaces newlines with spaces so ban reasons are safe for
// single-line config formats like Bans.cfg.
func SanitizeBanReason(reason string) string {
	reason = strings.ReplaceAll(reason, "\n", " ")
	reason = strings.ReplaceAll(reason, "\r", " ")
	return reason
}
