package utils

import "strconv"

// PlayerIdentifiers stores a canonical player identifier alongside any known
// Steam and EOS aliases for the same player.
type PlayerIdentifiers struct {
	PlayerID string
	SteamID  string
	EOSID    string
}

// NormalizePlayerIdentifiers normalizes and canonicalizes the supplied player
// identifiers. When both Steam and EOS are known, PlayerID prefers Steam.
func NormalizePlayerIdentifiers(playerID, steamID, eosID string) PlayerIdentifiers {
	playerID = NormalizePlayerID(playerID)
	steamID = NormalizePlayerID(steamID)
	eosID = NormalizePlayerID(eosID)

	if !IsSteamID(steamID) {
		steamID = ""
	}
	if !IsEOSID(eosID) {
		eosID = ""
	}

	if playerID == "" {
		if steamID != "" {
			playerID = steamID
		} else {
			playerID = eosID
		}
	}

	if steamID == "" && IsSteamID(playerID) {
		steamID = playerID
	}
	if eosID == "" && IsEOSID(playerID) {
		eosID = playerID
	}

	if steamID != "" {
		playerID = steamID
	} else if eosID != "" {
		playerID = eosID
	}

	return PlayerIdentifiers{
		PlayerID: playerID,
		SteamID:  steamID,
		EOSID:    eosID,
	}
}

// MergePlayerIdentifiers combines an existing identifier set with a newer one,
// preserving already-known aliases when the update is partial.
func MergePlayerIdentifiers(existing, incoming PlayerIdentifiers) PlayerIdentifiers {
	playerID := incoming.PlayerID
	if playerID == "" {
		playerID = existing.PlayerID
	}

	steamID := incoming.SteamID
	if steamID == "" {
		steamID = existing.SteamID
	}

	eosID := incoming.EOSID
	if eosID == "" {
		eosID = existing.EOSID
	}

	return NormalizePlayerIdentifiers(playerID, steamID, eosID)
}

// StorageIDs returns every non-empty identifier that should resolve to the
// same player, with the canonical PlayerID first.
func (ids PlayerIdentifiers) StorageIDs() []string {
	candidates := []string{ids.PlayerID, ids.SteamID, ids.EOSID}
	seen := make(map[string]bool, len(candidates))
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		result = append(result, candidate)
	}
	return result
}

// Match reports whether candidate matches any known identifier.
func (ids PlayerIdentifiers) Match(candidate string) bool {
	candidate = NormalizePlayerID(candidate)
	if candidate == "" {
		return false
	}

	for _, value := range ids.StorageIDs() {
		if candidate == value {
			return true
		}
	}

	return false
}

// ContainsIdentifier reports whether values contains target.
func ContainsIdentifier(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// DatabaseArgs converts the identifiers into values suitable for database
// queries or inserts.
func (ids PlayerIdentifiers) DatabaseArgs() (steamID interface{}, eosID interface{}, err error) {
	if ids.SteamID != "" {
		parsedSteamID, parseErr := strconv.ParseInt(ids.SteamID, 10, 64)
		if parseErr != nil {
			return nil, nil, parseErr
		}
		steamID = parsedSteamID
	}

	if ids.EOSID != "" {
		eosID = ids.EOSID
	}

	return steamID, eosID, nil
}
