package server

import (
	"strings"

	"go.codycody31.dev/squad-aegis/internal/shared/utils"
)

// normalizeOptionalEOSID distinguishes between an absent EOS ID and a
// malformed one so handlers can reject invalid input before broader lookups.
func normalizeOptionalEOSID(rawEOSID string) (normalizedEOSID string, provided bool, valid bool) {
	trimmedEOSID := strings.TrimSpace(rawEOSID)
	if trimmedEOSID == "" {
		return "", false, true
	}

	normalizedEOSID = utils.NormalizeEOSID(trimmedEOSID)
	return normalizedEOSID, true, normalizedEOSID != ""
}
