package utils

import (
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

// IsSteamID returns true if the string looks like a Steam ID (numeric, parses to int64).
func IsSteamID(s string) bool {
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}

// SanitizeBanReason replaces newlines with spaces so ban reasons are safe for
// single-line config formats like Bans.cfg.
func SanitizeBanReason(reason string) string {
	reason = strings.ReplaceAll(reason, "\n", " ")
	reason = strings.ReplaceAll(reason, "\r", " ")
	return reason
}
