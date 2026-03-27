package server

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.codycody31.dev/squad-aegis/internal/shared/utils"
)

func mergeResolvedPlayerIdentifiers(steamIDs []string, eosIDs []string) utils.PlayerIdentifiers {
	var steamID string
	var eosID string
	if len(steamIDs) > 0 {
		steamID = steamIDs[0]
	}
	if len(eosIDs) > 0 {
		eosID = eosIDs[0]
	}
	return utils.NormalizePlayerIdentifiers("", steamID, eosID)
}

func mergeVerifiedPlayerIdentifiers(primary utils.PlayerIdentifiers, steamIDs []string, eosIDs []string) utils.PlayerIdentifiers {
	var verifiedSteamIDs []string
	var verifiedEOSIDs []string

	if utils.IsSteamID(primary.PlayerID) {
		verifiedSteamIDs = appendUniqueSteamIdentifier(verifiedSteamIDs, primary.PlayerID)
	}
	if utils.IsEOSID(primary.PlayerID) {
		verifiedEOSIDs = appendUniqueEOSIdentifier(verifiedEOSIDs, primary.PlayerID)
	}

	for _, steamID := range steamIDs {
		verifiedSteamIDs = appendUniqueSteamIdentifier(verifiedSteamIDs, steamID)
	}
	for _, eosID := range eosIDs {
		verifiedEOSIDs = appendUniqueEOSIdentifier(verifiedEOSIDs, eosID)
	}

	return mergeResolvedPlayerIdentifiers(verifiedSteamIDs, verifiedEOSIDs)
}

func (s *Server) resolveCanonicalPlayerIdentifiers(c *gin.Context, playerID string, steamID string, eosID string) utils.PlayerIdentifiers {
	identifiers := utils.NormalizePlayerIdentifiers(playerID, steamID, eosID)
	if identifiers.PlayerID == "" {
		return identifiers
	}

	lookupPlayerID := identifiers.PlayerID
	lookupIsSteam := utils.IsSteamID(lookupPlayerID)
	linkedSteamIDs, linkedEOSIDs := s.resolveLinkedPlayerIdentifiers(c, lookupPlayerID, lookupIsSteam)

	resolved := mergeVerifiedPlayerIdentifiers(identifiers, linkedSteamIDs, linkedEOSIDs)
	if resolved.PlayerID == "" {
		return identifiers
	}

	return resolved
}

var validPRVAliases = map[string]bool{"": true, "pv.": true}

func buildPlayerRuleViolationWhereClause(steamIDs []string, eosIDs []string, alias string) (string, []interface{}) {
	if !validPRVAliases[alias] {
		return "1 = 0", nil
	}

	var conditions []string
	var args []interface{}

	playerIDs := make([]string, 0, len(steamIDs)+len(eosIDs))
	for _, steamID := range steamIDs {
		playerIDs = appendUniquePlayerIdentifier(playerIDs, steamID)
	}
	for _, eosID := range eosIDs {
		normalizedEOSID := utils.NormalizeEOSID(eosID)
		if normalizedEOSID == "" {
			continue
		}
		playerIDs = appendUniquePlayerIdentifier(playerIDs, normalizedEOSID)
	}

	for _, playerID := range playerIDs {
		conditions = append(conditions, fmt.Sprintf("%splayer_id = ?", alias))
		args = append(args, playerID)
	}

	for _, steamID := range steamIDs {
		steamIDInt, err := strconv.ParseInt(steamID, 10, 64)
		if err != nil {
			continue
		}
		conditions = append(conditions, fmt.Sprintf("%splayer_steam_id = ?", alias))
		args = append(args, steamIDInt)
	}

	for _, eosID := range eosIDs {
		normalizedEOSID := utils.NormalizeEOSID(eosID)
		if normalizedEOSID == "" {
			continue
		}
		conditions = append(conditions, fmt.Sprintf("%splayer_eos_id = ?", alias))
		args = append(args, normalizedEOSID)
	}

	if len(conditions) == 0 {
		return "1 = 0", nil
	}
	return strings.Join(conditions, " OR "), args
}

func parseSteamIdentifierList(steamIDs []string) []uint64 {
	result := make([]uint64, 0, len(steamIDs))
	seen := make(map[uint64]struct{}, len(steamIDs))
	for _, steamID := range steamIDs {
		parsedSteamID, err := strconv.ParseUint(steamID, 10, 64)
		if err != nil {
			continue
		}
		if _, exists := seen[parsedSteamID]; exists {
			continue
		}
		seen[parsedSteamID] = struct{}{}
		result = append(result, parsedSteamID)
	}
	return result
}

var validCHColumns = map[string]bool{
	"steam": true, "eos": true,
	"victim_steam": true, "victim_eos": true,
	"steam_id": true, "eos_id": true,
	"pc.steam": true, "pc.eos": true,
	"player_steam_id": true, "player_eos_id": true,
}

func buildClickHouseIdentifierWhereClause(steamColumn string, eosColumn string, steamIDs []string, eosIDs []string, steamAsUInt64 bool) (string, []interface{}) {
	var conditions []string
	var args []interface{}

	if steamColumn != "" && !validCHColumns[steamColumn] {
		return "1 = 0", nil
	}
	if eosColumn != "" && !validCHColumns[eosColumn] {
		return "1 = 0", nil
	}

	if steamColumn != "" && len(steamIDs) > 0 {
		if steamAsUInt64 {
			parsedSteamIDs := parseSteamIdentifierList(steamIDs)
			if len(parsedSteamIDs) > 0 {
				conditions = append(conditions, fmt.Sprintf("%s IN (?)", steamColumn))
				args = append(args, parsedSteamIDs)
			}
		} else {
			conditions = append(conditions, fmt.Sprintf("%s IN (?)", steamColumn))
			args = append(args, steamIDs)
		}
	}

	if eosColumn != "" && len(eosIDs) > 0 {
		conditions = append(conditions, fmt.Sprintf("%s IN (?)", eosColumn))
		args = append(args, eosIDs)
	}

	if len(conditions) == 0 {
		return "1 = 0", nil
	}
	return strings.Join(conditions, " OR "), args
}
