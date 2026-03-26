package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseBanDuration parses a ban duration string and returns the computed expiry time.
// Returns nil for permanent bans.
//
// Supported formats:
//   - "0" or "permanent" → nil (permanent)
//   - "7d" → now + 7 days
//   - "2h" → now + 2 hours
//   - "30m" → now + 30 minutes
//   - "7" (bare number) → now + 7 days (backward compat)
func ParseBanDuration(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" || strings.EqualFold(s, "permanent") {
		return nil, nil
	}

	now := time.Now()

	// Try bare number (backward compat: treat as days)
	if days, err := strconv.Atoi(s); err == nil {
		if days <= 0 {
			return nil, fmt.Errorf("invalid duration: %q must be positive", s)
		}
		t := now.AddDate(0, 0, days)
		return &t, nil
	}

	if len(s) < 2 {
		return nil, fmt.Errorf("invalid duration format: %q", s)
	}

	unit := s[len(s)-1]
	valueStr := s[:len(s)-1]
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return nil, fmt.Errorf("invalid duration format: %q", s)
	}
	if value <= 0 {
		return nil, fmt.Errorf("invalid duration: %q must be positive", s)
	}

	var t time.Time
	switch unit {
	case 'm':
		t = now.Add(time.Duration(value) * time.Minute)
	case 'h':
		t = now.Add(time.Duration(value) * time.Hour)
	case 'd':
		t = now.AddDate(0, 0, value)
	case 'M':
		t = now.AddDate(0, value, 0)
	default:
		return nil, fmt.Errorf("unsupported duration unit %q in %q (use m, h, d, or M)", string(unit), s)
	}

	return &t, nil
}
