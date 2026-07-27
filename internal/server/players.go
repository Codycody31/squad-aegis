package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/pagination"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	"go.codycody31.dev/squad-aegis/internal/shared/utils"
	"go.codycody31.dev/squad-aegis/internal/squadlayers"
)

// PlayerProfile represents a player's comprehensive profile
type PlayerProfile struct {
	SteamID        string             `json:"steam_id"`
	EOSID          string             `json:"eos_id"`
	EpicID         string             `json:"epic_id,omitempty"`
	PlayerName     string             `json:"player_name"`
	LastSeen       *time.Time         `json:"last_seen"`
	FirstSeen      *time.Time         `json:"first_seen"`
	TotalPlayTime  int64              `json:"total_play_time"` // in seconds
	TotalSessions  int64              `json:"total_sessions"`
	Statistics     PlayerStatistics   `json:"statistics"`
	RecentActivity []PlayerActivity   `json:"recent_activity"`
	ChatHistory    []ChatMessage      `json:"chat_history,omitempty"`
	Violations     []RuleViolation    `json:"violations,omitempty"`
	RecentServers  []RecentServerInfo `json:"recent_servers"`

	// Admin-focused fields
	ActiveBans       []ActiveBan        `json:"active_bans"`
	ViolationSummary ViolationSummary   `json:"violation_summary"`
	TeamkillMetrics  TeamkillMetrics    `json:"teamkill_metrics"`
	RiskIndicators   []RiskIndicator    `json:"risk_indicators"`
	NameHistory      []NameHistoryEntry `json:"name_history"`
	WeaponStats      []WeaponStat       `json:"weapon_stats"`

	// Consolidated identity fields
	CanonicalID    string   `json:"canonical_id,omitempty"`
	AllSteamIDs    []string `json:"all_steam_ids,omitempty"`
	AllEOSIDs      []string `json:"all_eos_ids,omitempty"`
	AllEpicIDs     []string `json:"all_epic_ids,omitempty"`
	AllNames       []string `json:"all_names,omitempty"`
	IdentityStatus string   `json:"identity_status,omitempty"` // "resolved", "pending"
}

// PlayerStatistics holds combat and gameplay statistics
type PlayerStatistics struct {
	Kills        int64   `json:"kills"`
	Deaths       int64   `json:"deaths"`
	Teamkills    int64   `json:"teamkills"`
	Revives      int64   `json:"revives"`
	TimesRevived int64   `json:"times_revived"`
	DamageDealt  float64 `json:"damage_dealt"`
	DamageTaken  float64 `json:"damage_taken"`
	KDRatio      float64 `json:"kd_ratio"`
}

// PlayerActivity represents a player action or event
type PlayerActivity struct {
	EventTime   time.Time `json:"event_time"`
	EventType   string    `json:"event_type"`
	Description string    `json:"description"`
	ServerID    string    `json:"server_id"`
	ServerName  string    `json:"server_name,omitempty"`
}

// ChatMessage represents a chat message from the player
type ChatMessage struct {
	MessageID  string    `json:"message_id"`
	SentAt     time.Time `json:"sent_at"`
	Message    string    `json:"message"`
	ChatType   string    `json:"chat_type"`
	ServerID   string    `json:"server_id"`
	ServerName string    `json:"server_name,omitempty"`
}

// RuleViolation represents a rule violation by the player
type RuleViolation struct {
	ViolationID string    `json:"violation_id"`
	ServerID    string    `json:"server_id"`
	ServerName  string    `json:"server_name,omitempty"`
	RuleID      *string   `json:"rule_id"`
	RuleName    *string   `json:"rule_name,omitempty"`
	ActionType  string    `json:"action_type"`
	AdminUserID *string   `json:"admin_user_id"`
	AdminName   *string   `json:"admin_name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// RecentServerInfo represents a server the player has recently played on
type RecentServerInfo struct {
	ServerID   string    `json:"server_id"`
	ServerName string    `json:"server_name"`
	LastSeen   time.Time `json:"last_seen"`
	Sessions   int64     `json:"sessions"`
}

// ViolationSummary represents a summary of player violations
type ViolationSummary struct {
	TotalWarns int64      `json:"total_warns"`
	TotalKicks int64      `json:"total_kicks"`
	TotalBans  int64      `json:"total_bans"`
	LastAction *time.Time `json:"last_action,omitempty"`
}

// TeamkillMetrics represents detailed teamkill statistics.
//
// The headline fields (TotalTeamkills, TeamkillRatio, RecentTeamkills and the
// team wound/damage counts) cover COMPETITIVE (real-match) play only. Teamkills
// on seeding or training layers (e.g. Jensen's Range) carry no admin signal, so
// they are split out into NonCompetitive and excluded from these figures and
// from the risk flags. AllTeamkills/AllTeamkillRatio keep the raw all-context
// totals for transparency.
type TeamkillMetrics struct {
	TotalTeamkills      int64   `json:"total_teamkills"` // Competitive TKs
	TeamkillsPerSession float64 `json:"teamkills_per_session"`
	TeamkillRatio       float64 `json:"teamkill_ratio"`     // Competitive TKs / competitive kills
	RecentTeamkills     int64   `json:"recent_teamkills"`   // Competitive TKs, last 7 days
	TotalTeamWounds     int64   `json:"total_team_wounds"`  // Competitive team wounds
	TotalTeamDamage     int64   `json:"total_team_damage"`  // Competitive team damage
	RecentTeamWounds    int64   `json:"recent_team_wounds"` // Competitive team wounds, last 7 days
	RecentTeamDamage    int64   `json:"recent_team_damage"` // Competitive team damage, last 7 days

	NonCompetitive   NonCompetitiveTeamkills `json:"non_competitive"`    // Excluded seeding/training friendly fire
	AllTeamkills     int64                   `json:"all_teamkills"`      // Raw TKs across every layer
	AllTeamkillRatio float64                 `json:"all_teamkill_ratio"` // Raw TKs / all kills
}

// NonCompetitiveTeamkills breaks the excluded friendly fire into seeding vs
// training so the UI can show where it happened.
type NonCompetitiveTeamkills struct {
	SeedingTeamkills   int64 `json:"seeding_teamkills"`
	TrainingTeamkills  int64 `json:"training_teamkills"`
	SeedingTeamWounds  int64 `json:"seeding_team_wounds"`
	TrainingTeamWounds int64 `json:"training_team_wounds"`
	SeedingTeamDamage  int64 `json:"seeding_team_damage"`
	TrainingTeamDamage int64 `json:"training_team_damage"`
}

// RiskIndicator represents a risk flag for admin attention
type RiskIndicator struct {
	Type        string `json:"type"`     // "high_tk_rate", "recent_ban", "multiple_names", "cbl_flagged", "ip_shared"
	Severity    string `json:"severity"` // "critical", "high", "medium", "low"
	Description string `json:"description"`
}

// ActiveBan represents a currently active ban on the player
type ActiveBan struct {
	BanID      string     `json:"ban_id"`
	ServerID   string     `json:"server_id"`
	ServerName string     `json:"server_name"`
	Reason     string     `json:"reason"`
	Permanent  bool       `json:"permanent"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	AdminName  string     `json:"admin_name"`
}

// NameHistoryEntry represents a name used by the player
type NameHistoryEntry struct {
	Name         string    `json:"name"`
	FirstUsed    time.Time `json:"first_used"`
	LastUsed     time.Time `json:"last_used"`
	SessionCount int64     `json:"session_count"`
}

// WeaponStat represents weapon usage statistics
type WeaponStat struct {
	Weapon    string `json:"weapon"`
	Kills     int64  `json:"kills"`
	Teamkills int64  `json:"teamkills"`
}

// TeamkillVictim represents a player who was teamkilled
type TeamkillVictim struct {
	VictimName  string    `json:"victim_name"`
	VictimSteam string    `json:"victim_steam"`
	VictimEOS   string    `json:"victim_eos"`
	TKCount     int64     `json:"tk_count"`
	WeaponsUsed []string  `json:"weapons_used"`
	FirstTK     time.Time `json:"first_tk"`
	LastTK      time.Time `json:"last_tk"`
}

// SessionHistoryEntry represents a paired connection session
type SessionHistoryEntry struct {
	CursorKey         string     `json:"-"`
	ConnectTime       time.Time  `json:"connect_time"`
	DisconnectTime    *time.Time `json:"disconnect_time,omitempty"`
	DurationSeconds   *int64     `json:"duration_seconds,omitempty"`
	ServerID          string     `json:"server_id"`
	ServerName        string     `json:"server_name,omitempty"`
	IP                string     `json:"ip,omitempty"` // Only visible with permission
	MissingDisconnect bool       `json:"missing_disconnect"`
	Ongoing           bool       `json:"ongoing"`
}

// CombatHistoryEntry represents a kill or death event
type CombatHistoryEntry struct {
	CursorKey    string    `json:"-"`
	EventID      string    `json:"event_id,omitempty"`
	ChainID      string    `json:"chain_id,omitempty"`
	EventTime    time.Time `json:"event_time"`
	EventType    string    `json:"event_type"` // "kill", "death", "wounded", "damaged", "wounded_by", "damaged_by"
	ServerID     string    `json:"server_id"`
	ServerName   string    `json:"server_name,omitempty"`
	Weapon       string    `json:"weapon"`
	Damage       float32   `json:"damage"`
	Teamkill     bool      `json:"teamkill"`
	OtherName    string    `json:"other_name"`
	OtherSteamID string    `json:"other_steam_id,omitempty"`
	OtherEOSID   string    `json:"other_eos_id,omitempty"`
	OtherTeam    string    `json:"other_team"`
	OtherSquad   string    `json:"other_squad"`
	PlayerTeam   string    `json:"player_team"`
	PlayerSquad  string    `json:"player_squad"`
}

// RelatedPlayer represents a player potentially related (same IP)
type RelatedPlayer struct {
	SteamID        string `json:"steam_id"`
	EOSID          string `json:"eos_id"`
	PlayerName     string `json:"player_name"`
	RelationType   string `json:"relation_type"` // "same_ip"
	SharedSessions int64  `json:"shared_sessions"`
	IsBanned       bool   `json:"is_banned"`
}

// PaginatedChatHistory represents paginated chat messages
type PaginatedChatHistory struct {
	Messages []ChatMessage `json:"messages"`
}

// PlayerSearchResult represents a simplified player profile for search results
type PlayerSearchResult struct {
	SteamID    string     `json:"steam_id"`
	EOSID      string     `json:"eos_id"`
	EpicID     string     `json:"epic_id,omitempty"`
	PlayerName string     `json:"player_name"`
	LastSeen   *time.Time `json:"last_seen"`
	FirstSeen  *time.Time `json:"first_seen"`
}

// TopPlayerStats represents a player in top statistics
type TopPlayerStats struct {
	SteamID    string  `json:"steam_id"`
	EOSID      string  `json:"eos_id"`
	PlayerName string  `json:"player_name"`
	Kills      int64   `json:"kills"`
	Deaths     int64   `json:"deaths"`
	KDRatio    float64 `json:"kd_ratio"`
	Teamkills  int64   `json:"teamkills"`
	Revives    int64   `json:"revives"`
}

// PlayerStatsSummary represents overall player statistics
type PlayerStatsSummary struct {
	TopPlayers        []TopPlayerStats     `json:"top_players"`
	TopTeamkillers    []TopPlayerStats     `json:"top_teamkillers"`
	TopMedics         []TopPlayerStats     `json:"top_medics"`
	MostRecentPlayers []PlayerSearchResult `json:"most_recent_players"`
	TotalPlayers      int64                `json:"total_players"`
	TotalKills        int64                `json:"total_kills"`
	TotalDeaths       int64                `json:"total_deaths"`
	TotalTeamkills    int64                `json:"total_teamkills"`
}

// AltAccountPlayer represents a player in an alt account group
type AltAccountPlayer struct {
	SteamID        string     `json:"steam_id"`
	EOSID          string     `json:"eos_id"`
	PlayerName     string     `json:"player_name"`
	IsBanned       bool       `json:"is_banned"`
	SharedSessions int64      `json:"shared_sessions"`
	LastSeen       *time.Time `json:"last_seen"`
}

// AltAccountGroup represents a group of players sharing IPs (potential alts)
type AltAccountGroup struct {
	GroupID       string             `json:"group_id"`
	Players       []AltAccountPlayer `json:"players"`
	SharedIPCount int                `json:"shared_ip_count"`
	LastActivity  *time.Time         `json:"last_activity"`
}

// PlayersList handles GET /api/players - search and list players
func (s *Server) PlayersList(c *gin.Context) {
	// Get search query parameter
	searchQuery := c.Query("search")
	limit := 50 // Default limit

	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	if searchQuery == "" {
		responses.BadRequest(c, "Search query is required", nil)
		return
	}

	// Normalize the search query
	searchQuery = strings.TrimSpace(searchQuery)
	searchPattern := "%" + searchQuery + "%"

	// Search the published snapshot first. Raw events are a rolling-deploy
	// fallback only when the snapshot objects or pointer are unavailable.
	players, err := s.searchPlayersFromIdentityTable(c.Request.Context(), searchQuery, searchPattern, limit)
	if err != nil {
		log.Debug().Err(err).Msg("Identity snapshot unavailable, falling back to raw events")
		players, err = s.searchPlayersFromRawEvents(c.Request.Context(), searchPattern, limit)
		if err != nil {
			responses.InternalServerError(c, err, nil)
			return
		}
	}

	responses.Success(c, "Players fetched successfully", &gin.H{
		"players": players,
		"count":   len(players),
	})
}

// searchPlayersFromIdentityTable searches the pre-computed player_identities table
func (s *Server) searchPlayersFromIdentityTable(ctx context.Context, searchQuery, searchPattern string, limit int) ([]PlayerSearchResult, error) {
	predicate := "pil.identifier_type = 'name' AND lowerUTF8(pil.identifier_value) LIKE ?"
	lookupValue := strings.ToLower(searchPattern)
	if utils.IsSteamID(searchQuery) {
		predicate = "pil.identifier_type = 'steam' AND pil.identifier_value = ?"
		lookupValue = searchQuery
	} else if normalized := utils.NormalizeEOSID(searchQuery); normalized != "" {
		predicate = "pil.identifier_type IN ('eos', 'epic') AND pil.identifier_value = ?"
		lookupValue = normalized
	}
	query := fmt.Sprintf(`
		/* sa.player.identity_search */
		WITH current AS (
			SELECT snapshot_id
			FROM squad_aegis.player_identity_current
			WHERE pointer_key = 1
			ORDER BY published_at DESC
			LIMIT 1
		), matches AS (
			SELECT pil.canonical_id
			FROM squad_aegis.player_identity_lookup_snapshots pil
			INNER JOIN current c ON pil.snapshot_id = c.snapshot_id
			WHERE %s
			GROUP BY pil.canonical_id
		), results AS (
			SELECT pi.canonical_id, pi.primary_steam_id, pi.primary_eos_id,
			       pi.primary_epic_id, pi.primary_name, pi.last_seen, pi.first_seen
			FROM squad_aegis.player_identity_snapshots pi
			INNER JOIN current c ON pi.snapshot_id = c.snapshot_id
			INNER JOIN matches m ON pi.canonical_id = m.canonical_id
			ORDER BY pi.last_seen DESC, pi.canonical_id ASC
			LIMIT ?
		)
		SELECT *
		FROM
		(
			SELECT toUInt8(1) AS snapshot_status, primary_steam_id, primary_eos_id,
			       primary_epic_id, primary_name, last_seen, first_seen
			FROM results
			UNION ALL
			SELECT
				if((SELECT count() FROM current) = 0, toUInt8(2), toUInt8(0)),
				'', '', '', '', toDateTime64(0, 3, 'UTC'), toDateTime64(0, 3, 'UTC')
			WHERE NOT EXISTS (SELECT 1 FROM results)
		)
		ORDER BY snapshot_status DESC, last_seen DESC
	`, predicate)
	rows, err := s.Dependencies.Clickhouse.Query(ctx, query, lookupValue, limit)
	if err != nil {
		return nil, fmt.Errorf("search current identity snapshot: %w", err)
	}
	defer rows.Close()

	players := []PlayerSearchResult{}
	for rows.Next() {
		var player PlayerSearchResult
		var steamID, eosID, epicID string
		var snapshotStatus uint8

		if err := rows.Scan(&snapshotStatus, &steamID, &eosID, &epicID, &player.PlayerName, &player.LastSeen, &player.FirstSeen); err != nil {
			return nil, fmt.Errorf("scan identity search result: %w", err)
		}
		if snapshotStatus == 2 {
			return nil, errPlayerSnapshotUnavailable
		}
		if snapshotStatus == 0 {
			continue
		}

		player.SteamID = steamID
		player.EOSID = eosID
		player.EpicID = epicID
		players = append(players, player)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read identity search results: %w", err)
	}

	return players, nil
}

// searchPlayersFromRawEvents searches raw event tables for players (fallback)
// Searches across multiple event tables to find players even without join events
func (s *Server) searchPlayersFromRawEvents(ctx context.Context, searchPattern string, limit int) ([]PlayerSearchResult, error) {
	query := `
		WITH all_player_records AS (
			-- Join succeeded events
			SELECT steam, eos, epic, player_suffix as name, event_time
			FROM squad_aegis.server_join_succeeded_events
			WHERE player_suffix ILIKE ? OR steam ILIKE ? OR eos ILIKE ? OR epic ILIKE ?
			UNION ALL
			-- Disconnected events
			SELECT steam, eos, epic, player_suffix as name, event_time
			FROM squad_aegis.server_player_disconnected_events
			WHERE player_suffix ILIKE ? OR steam ILIKE ? OR eos ILIKE ? OR epic ILIKE ?
			UNION ALL
			-- Possess events
			SELECT player_steam as steam, player_eos as eos, player_epic as epic, player_suffix as name, event_time
			FROM squad_aegis.server_player_possess_events
			WHERE player_suffix ILIKE ? OR player_steam ILIKE ? OR player_eos ILIKE ? OR player_epic ILIKE ?
			UNION ALL
			-- Damage events (attacker)
			SELECT attacker_steam as steam, attacker_eos as eos, '' as epic, attacker_name as name, event_time
			FROM squad_aegis.server_player_damaged_events
			WHERE attacker_name ILIKE ? OR attacker_steam ILIKE ? OR attacker_eos ILIKE ?
			UNION ALL
			-- Damage events (victim)
			SELECT victim_steam as steam, victim_eos as eos, '' as epic, victim_name as name, event_time
			FROM squad_aegis.server_player_damaged_events
			WHERE victim_name ILIKE ? OR victim_steam ILIKE ? OR victim_eos ILIKE ?
			UNION ALL
			-- Died events (victim)
			SELECT victim_steam as steam, victim_eos as eos, '' as epic, victim_name as name, event_time
			FROM squad_aegis.server_player_died_events
			WHERE victim_name ILIKE ? OR victim_steam ILIKE ? OR victim_eos ILIKE ?
			UNION ALL
			-- Died events (attacker)
			SELECT attacker_steam as steam, attacker_eos as eos, '' as epic, attacker_name as name, event_time
			FROM squad_aegis.server_player_died_events
			WHERE attacker_name ILIKE ? OR attacker_steam ILIKE ? OR attacker_eos ILIKE ?
			UNION ALL
			-- Wounded events (victim)
			SELECT victim_steam as steam, victim_eos as eos, '' as epic, victim_name as name, event_time
			FROM squad_aegis.server_player_wounded_events
			WHERE victim_name ILIKE ? OR victim_steam ILIKE ? OR victim_eos ILIKE ?
			UNION ALL
			-- Wounded events (attacker)
			SELECT attacker_steam as steam, attacker_eos as eos, '' as epic, attacker_name as name, event_time
			FROM squad_aegis.server_player_wounded_events
			WHERE attacker_name ILIKE ? OR attacker_steam ILIKE ? OR attacker_eos ILIKE ?
			UNION ALL
			-- Revived events (reviver)
			SELECT reviver_steam as steam, reviver_eos as eos, '' as epic, reviver_name as name, event_time
			FROM squad_aegis.server_player_revived_events
			WHERE reviver_name ILIKE ? OR reviver_steam ILIKE ? OR reviver_eos ILIKE ?
			UNION ALL
			-- Revived events (victim)
			SELECT victim_steam as steam, victim_eos as eos, '' as epic, victim_name as name, event_time
			FROM squad_aegis.server_player_revived_events
			WHERE victim_name ILIKE ? OR victim_steam ILIKE ? OR victim_eos ILIKE ?
		),
		player_identifiers AS (
			SELECT
				steam,
				eos,
				epic,
				argMax(name, if(name != '', event_time, toDateTime64('1970-01-01', 3, 'UTC'))) as best_name,
				max(event_time) as last_seen,
				min(event_time) as first_seen
			FROM all_player_records
			WHERE steam != '' OR eos != '' OR epic != ''
			GROUP BY steam, eos, epic
		)
		SELECT
			anyIf(steam, steam != '') as steam_id,
			anyIf(eos, eos != '') as eos_id,
			anyIf(epic, epic != '') as epic_id,
			anyIf(best_name, best_name != '') as player_name,
			max(last_seen) as last_seen,
			min(first_seen) as first_seen
		FROM player_identifiers
		GROUP BY
			multiIf(
				steam != '', steam,
				eos != '', eos,
				epic != '', epic,
				''
			)
		ORDER BY last_seen DESC
		LIMIT ?
	`

	// Join/disconnected/possess subqueries search name, steam, eos, and epic.
	// Remaining subqueries search name, steam, and eos.
	rows, err := s.Dependencies.Clickhouse.Query(
		ctx, query,
		searchPattern, searchPattern, searchPattern, searchPattern, // join_succeeded
		searchPattern, searchPattern, searchPattern, searchPattern, // disconnected
		searchPattern, searchPattern, searchPattern, searchPattern, // possess
		searchPattern, searchPattern, searchPattern, // damaged (attacker)
		searchPattern, searchPattern, searchPattern, // damaged (victim)
		searchPattern, searchPattern, searchPattern, // died (victim)
		searchPattern, searchPattern, searchPattern, // died (attacker)
		searchPattern, searchPattern, searchPattern, // wounded (victim)
		searchPattern, searchPattern, searchPattern, // wounded (attacker)
		searchPattern, searchPattern, searchPattern, // revived (reviver)
		searchPattern, searchPattern, searchPattern, // revived (victim)
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	players := []PlayerSearchResult{}
	for rows.Next() {
		var player PlayerSearchResult
		if err := rows.Scan(&player.SteamID, &player.EOSID, &player.EpicID, &player.PlayerName, &player.LastSeen, &player.FirstSeen); err != nil {
			return nil, err
		}
		players = append(players, player)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return players, nil
}

func normalizePlayerIdentifier(playerID string) (string, bool) {
	if _, err := strconv.ParseUint(playerID, 10, 64); err == nil {
		return playerID, true
	}

	return utils.NormalizeEOSID(playerID), false
}

func selectProfileLookupIdentifier(profile *PlayerProfile, fallbackID string, fallbackIsSteamID bool) (string, bool) {
	if profile != nil {
		if profile.SteamID != "" {
			return profile.SteamID, true
		}
		if profile.EOSID != "" {
			return profile.EOSID, false
		}
	}

	return fallbackID, fallbackIsSteamID
}

// ResolvedPlayerIdentity is the single identity resolution shared by a player
// history request. Helpers receive these arrays instead of resolving the same
// player independently.
type ResolvedPlayerIdentity struct {
	CanonicalID    string
	SteamIDs       []string
	EOSIDs         []string
	EpicIDs        []string
	Found          bool
	SnapshotBacked bool
}

func resolvedPlayerIdentityFromProfile(profile *PlayerProfile, fallbackID string, fallbackIsSteam bool) ResolvedPlayerIdentity {
	resolved := ResolvedPlayerIdentity{
		CanonicalID:    profile.CanonicalID,
		SteamIDs:       append([]string(nil), profile.AllSteamIDs...),
		EOSIDs:         append([]string(nil), profile.AllEOSIDs...),
		EpicIDs:        append([]string(nil), profile.AllEpicIDs...),
		Found:          true,
		SnapshotBacked: profile.CanonicalID != "",
	}
	resolved.SteamIDs = appendUniqueSteamIdentifier(resolved.SteamIDs, profile.SteamID)
	resolved.EOSIDs = appendUniqueEOSIdentifier(resolved.EOSIDs, profile.EOSID)
	resolved.EpicIDs = appendUniquePlayerIdentifier(resolved.EpicIDs, profile.EpicID)
	if len(resolved.SteamIDs) == 0 && len(resolved.EOSIDs) == 0 && len(resolved.EpicIDs) == 0 {
		if fallbackIsSteam {
			resolved.SteamIDs = appendUniqueSteamIdentifier(resolved.SteamIDs, fallbackID)
		} else {
			resolved.EOSIDs = appendUniqueEOSIdentifier(resolved.EOSIDs, fallbackID)
			resolved.EpicIDs = appendUniquePlayerIdentifier(resolved.EpicIDs, fallbackID)
		}
	}
	return resolved
}

func (s *Server) resolvePlayerIdentity(ctx context.Context, rawPlayerID string) (ResolvedPlayerIdentity, error) {
	playerID, isSteamID := normalizePlayerIdentifier(strings.TrimSpace(rawPlayerID))
	if playerID == "" {
		return ResolvedPlayerIdentity{}, fmt.Errorf("invalid player identifier")
	}
	profile, err := s.getPlayerFromIdentityTable(ctx, playerID, isSteamID)
	if err == nil {
		resolved := ResolvedPlayerIdentity{
			CanonicalID:    profile.CanonicalID,
			SteamIDs:       append([]string(nil), profile.AllSteamIDs...),
			EOSIDs:         append([]string(nil), profile.AllEOSIDs...),
			EpicIDs:        append([]string(nil), profile.AllEpicIDs...),
			Found:          true,
			SnapshotBacked: true,
		}
		resolved.SteamIDs = appendUniqueSteamIdentifier(resolved.SteamIDs, profile.SteamID)
		resolved.EOSIDs = appendUniqueEOSIdentifier(resolved.EOSIDs, profile.EOSID)
		resolved.EpicIDs = appendUniquePlayerIdentifier(resolved.EpicIDs, profile.EpicID)
		return resolved, nil
	}
	if errors.Is(err, errPlayerNotFoundInSnapshot) {
		return ResolvedPlayerIdentity{SnapshotBacked: true}, nil
	}

	// Compatibility fallback when the snapshot objects or pointer are not yet
	// available during a rolling deployment.
	resolved := ResolvedPlayerIdentity{Found: true}
	if isSteamID {
		resolved.SteamIDs = []string{playerID}
	} else {
		resolved.EOSIDs = []string{playerID}
		resolved.EpicIDs = []string{playerID}
	}
	return resolved, nil
}

func steamIDsAsUint64(values []string) ([]uint64, error) {
	result := make([]uint64, 0, len(values))
	for _, value := range values {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid Steam ID %q: %w", value, err)
		}
		result = append(result, parsed)
	}
	return result, nil
}

func (s *Server) loadServerNamesBatch(ctx context.Context, serverIDs []string) (map[string]string, error) {
	unique := make(map[string]struct{}, len(serverIDs))
	ids := make([]string, 0, len(serverIDs))
	for _, id := range serverIDs {
		if id == "" {
			continue
		}
		if _, exists := unique[id]; exists {
			continue
		}
		if _, err := uuid.Parse(id); err != nil {
			continue
		}
		unique[id] = struct{}{}
		ids = append(ids, id)
	}
	result := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := s.Dependencies.DB.QueryContext(ctx, `
		/* sa.player.server_names */
		SELECT id::text, name
		FROM servers
		WHERE id = ANY($1::uuid[])
	`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		result[id] = name
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func appendUniquePlayerIdentifier(values []string, value string) []string {
	if value == "" {
		return values
	}

	for _, existing := range values {
		if existing == value {
			return values
		}
	}

	return append(values, value)
}

func appendUniqueSteamIdentifier(values []string, steamID string) []string {
	if !utils.IsSteamID(steamID) {
		return values
	}

	return appendUniquePlayerIdentifier(values, steamID)
}

func appendUniqueEOSIdentifier(values []string, eosID string) []string {
	normalizedEOSID := utils.NormalizeEOSID(eosID)
	if normalizedEOSID == "" {
		return values
	}

	return appendUniquePlayerIdentifier(values, normalizedEOSID)
}

// resolveLinkedPlayerIdentifiers returns all known Steam/EOS identifiers linked
// to the supplied player ID, falling back to the original identifier when no
// identity graph data is available.
func (s *Server) resolveLinkedPlayerIdentifiers(c *gin.Context, playerID string, isSteamID bool) ([]string, []string) {
	lookupPlayerID := playerID
	if !isSteamID {
		lookupPlayerID = utils.NormalizeEOSID(playerID)
	}

	var steamIDs []string
	var eosIDs []string

	if isSteamID {
		steamIDs = appendUniqueSteamIdentifier(steamIDs, lookupPlayerID)
	} else {
		eosIDs = appendUniqueEOSIdentifier(eosIDs, lookupPlayerID)
	}

	if profile, err := s.getPlayerBasicInfo(c, lookupPlayerID, isSteamID); err == nil && profile != nil {
		steamIDs = appendUniqueSteamIdentifier(steamIDs, profile.SteamID)
		for _, steamID := range profile.AllSteamIDs {
			steamIDs = appendUniqueSteamIdentifier(steamIDs, steamID)
		}

		eosIDs = appendUniqueEOSIdentifier(eosIDs, profile.EOSID)
		for _, eosID := range profile.AllEOSIDs {
			eosIDs = appendUniqueEOSIdentifier(eosIDs, eosID)
		}
	}

	if linkedSteamIDs, linkedEOSIDs, err := s.getLinkedPlayerIdentifiers(c, lookupPlayerID, isSteamID); err == nil {
		for _, steamID := range linkedSteamIDs {
			steamIDs = appendUniqueSteamIdentifier(steamIDs, steamID)
		}
		for _, eosID := range linkedEOSIDs {
			eosIDs = appendUniqueEOSIdentifier(eosIDs, eosID)
		}
	}

	return steamIDs, eosIDs
}

func buildServerBanIdentifierWhereClause(steamIDs, eosIDs []string, alias string, startArg int) (string, []interface{}, int) {
	var conditions []string
	var args []interface{}
	argIdx := startArg

	for _, steamID := range steamIDs {
		steamIDInt, err := strconv.ParseInt(steamID, 10, 64)
		if err != nil {
			continue
		}

		conditions = append(conditions, fmt.Sprintf("%ssteam_id = $%d", alias, argIdx))
		args = append(args, steamIDInt)
		argIdx++
	}

	for _, eosID := range eosIDs {
		normalizedEOSID := utils.NormalizeEOSID(eosID)
		if normalizedEOSID == "" {
			continue
		}

		conditions = append(conditions, fmt.Sprintf("%seos_id = $%d", alias, argIdx))
		args = append(args, normalizedEOSID)
		argIdx++
	}

	if len(conditions) == 0 {
		return "1 = 0", nil, argIdx
	}
	return strings.Join(conditions, " OR "), args, argIdx
}

// PlayerGet handles GET /api/players/:playerId - get player profile by steam or eos id
func (s *Server) PlayerGet(c *gin.Context) {
	ctx := c.Request.Context()
	playerID := c.Param("playerId")

	if playerID == "" {
		responses.BadRequest(c, "Player ID is required", nil)
		return
	}

	playerID, isSteamID := normalizePlayerIdentifier(playerID)

	// Get basic player info
	profile, err := s.getPlayerBasicInfo(c, playerID, isSteamID)
	if err != nil {
		responses.NotFound(c, "Player not found", &gin.H{"error": err.Error()})
		return
	}

	identity := resolvedPlayerIdentityFromProfile(profile, playerID, isSteamID)

	// Get player statistics
	statistics, err := s.getPlayerStatistics(ctx, identity)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("load player statistics: %w", err), nil)
		return
	}
	profile.Statistics = *statistics

	// Get recent activity
	recentActivity, err := s.getPlayerRecentActivity(ctx, identity, 20)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("load player activity: %w", err), nil)
		return
	}
	profile.RecentActivity = recentActivity

	// Get chat history (last 50 messages)
	chatHistory, err := s.getPlayerChatHistory(ctx, identity, 50)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("load player chat history: %w", err), nil)
		return
	}
	profile.ChatHistory = chatHistory

	// Get violations
	violations, err := s.getPlayerViolations(ctx, identity)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("load player violations: %w", err), nil)
		return
	}
	profile.Violations = violations

	// Get recent servers
	recentServers, err := s.getPlayerRecentServers(ctx, identity)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("load recent servers: %w", err), nil)
		return
	}
	profile.RecentServers = recentServers

	// Get admin-focused data
	// Active bans — pass both IDs so legacy steam-only bans are found for EOS lookups
	activeBans, err := s.getPlayerActiveBans(ctx, identity)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("load active bans: %w", err), nil)
		return
	}
	profile.ActiveBans = activeBans

	// Violation summary
	violationSummary, err := s.getPlayerViolationSummary(ctx, identity)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("load violation summary: %w", err), nil)
		return
	}
	profile.ViolationSummary = *violationSummary

	// Teamkill metrics
	tkMetrics, err := s.getPlayerTeamkillMetrics(ctx, identity, profile.TotalSessions)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("load teamkill metrics: %w", err), nil)
		return
	}
	profile.TeamkillMetrics = *tkMetrics

	// Name history
	nameHistory, err := s.getPlayerNameHistory(ctx, identity)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("load player name history: %w", err), nil)
		return
	}
	profile.NameHistory = nameHistory

	// Weapon stats
	weaponStats, err := s.getPlayerWeaponStats(ctx, identity)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("load player weapon statistics: %w", err), nil)
		return
	}
	profile.WeaponStats = weaponStats

	// Calculate risk indicators
	profile.RiskIndicators = s.calculateRiskIndicators(profile)

	responses.Success(c, "Player profile fetched successfully", &gin.H{"player": profile})
}

// PlayerBanHistory handles GET /api/players/:playerId/ban-history - historical
// bans for a player by Steam or EOS identifier.
func (s *Server) PlayerBanHistory(c *gin.Context) {
	playerID := c.Param("playerId")
	if playerID == "" {
		responses.BadRequest(c, "Player ID is required", nil)
		return
	}

	playerID, isSteamID := normalizePlayerIdentifier(playerID)

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	history, err := s.getPlayerBanHistory(c, playerID, isSteamID, limit)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.Success(c, "Player ban history fetched successfully", &gin.H{
		"history": history,
		"count":   len(history),
	})
}

// PlayersStats handles GET /api/players/stats. Actor identifiers are reduced
// to one exact type/value pair before joining the current identity snapshot,
// so every aggregate is grouped by canonical ID without OR joins.
func (s *Server) PlayersStats(c *gin.Context) {
	ctx := c.Request.Context()
	summary := PlayerStatsSummary{
		TopPlayers:        []TopPlayerStats{},
		TopTeamkillers:    []TopPlayerStats{},
		TopMedics:         []TopPlayerStats{},
		MostRecentPlayers: []PlayerSearchResult{},
	}

	query := fmt.Sprintf(`
		/* sa.player.statistics */
		WITH current AS (
			SELECT snapshot_id
			FROM squad_aegis.player_identity_current
			WHERE pointer_key = 1
			ORDER BY published_at DESC
			LIMIT 1
		), identity_lookup AS (
			SELECT identifier_type, identifier_value, min(canonical_id) AS canonical_id
			FROM squad_aegis.player_identity_lookup_snapshots
			WHERE snapshot_id = (SELECT snapshot_id FROM current)
			  AND identifier_type IN ('steam', 'eos')
			GROUP BY identifier_type, identifier_value
		), identities AS (
			SELECT canonical_id, primary_steam_id, primary_eos_id, primary_name,
			       first_seen, last_seen
			FROM squad_aegis.player_identity_snapshots
			WHERE snapshot_id = (SELECT snapshot_id FROM current)
		), combat_rows AS (
			SELECT
				d.event_time,
				tupleElement(actor, 1) AS actor_role,
				if(tupleElement(actor, 2) != '', 'steam', 'eos') AS identifier_type,
				if(tupleElement(actor, 2) != '', tupleElement(actor, 2), tupleElement(actor, 3)) AS identifier_value,
				tupleElement(actor, 4) AS observed_name,
				toUInt64(actor_role = 'attacker') AS kills,
				toUInt64(actor_role = 'victim') AS deaths,
				toUInt64(actor_role = 'attacker' AND d.teamkill = 1 AND %s = '%s') AS teamkills
			FROM squad_aegis.server_player_died_events AS d
			ASOF LEFT JOIN squad_aegis.server_new_game_events AS g
				ON d.server_id = g.server_id AND d.event_time >= g.event_time
			ARRAY JOIN [
				tuple('attacker', ifNull(d.attacker_steam, ''), ifNull(d.attacker_eos, ''), d.attacker_name),
				tuple('victim', ifNull(d.victim_steam, ''), ifNull(d.victim_eos, ''), d.victim_name)
			] AS actor
			WHERE identifier_value != ''
		), normalized_combat AS (
			SELECT
				if(ifNull(l.canonical_id, '') != '', l.canonical_id, concat(r.identifier_type, ':', r.identifier_value)) AS canonical_id,
				ifNull(i.primary_steam_id, if(r.identifier_type = 'steam', r.identifier_value, '')) AS steam_id,
				ifNull(i.primary_eos_id, if(r.identifier_type = 'eos', r.identifier_value, '')) AS eos_id,
				if(ifNull(i.primary_name, '') != '', i.primary_name, argMax(r.observed_name, r.event_time)) AS player_name,
				sum(r.kills) AS kills,
				sum(r.deaths) AS deaths,
				sum(r.teamkills) AS teamkills
			FROM combat_rows AS r
			LEFT JOIN identity_lookup AS l
				ON l.identifier_type = r.identifier_type AND l.identifier_value = r.identifier_value
			LEFT JOIN identities AS i ON i.canonical_id = l.canonical_id
			GROUP BY canonical_id, steam_id, eos_id, i.primary_name
		), revive_rows AS (
			SELECT
				event_time,
				if(ifNull(reviver_steam, '') != '', 'steam', 'eos') AS identifier_type,
				if(ifNull(reviver_steam, '') != '', assumeNotNull(reviver_steam), assumeNotNull(reviver_eos)) AS identifier_value,
				reviver_name AS observed_name
			FROM squad_aegis.server_player_revived_events
			WHERE ifNull(reviver_steam, '') != '' OR ifNull(reviver_eos, '') != ''
		), normalized_revives AS (
			SELECT
				if(ifNull(l.canonical_id, '') != '', l.canonical_id, concat(r.identifier_type, ':', r.identifier_value)) AS canonical_id,
				ifNull(i.primary_steam_id, if(r.identifier_type = 'steam', r.identifier_value, '')) AS steam_id,
				ifNull(i.primary_eos_id, if(r.identifier_type = 'eos', r.identifier_value, '')) AS eos_id,
				if(ifNull(i.primary_name, '') != '', i.primary_name, argMax(r.observed_name, r.event_time)) AS player_name,
				count() AS revives
			FROM revive_rows AS r
			LEFT JOIN identity_lookup AS l
				ON l.identifier_type = r.identifier_type AND l.identifier_value = r.identifier_value
			LEFT JOIN identities AS i ON i.canonical_id = l.canonical_id
			GROUP BY canonical_id, steam_id, eos_id, i.primary_name
		), top_players AS (
			SELECT * FROM normalized_combat
			WHERE kills >= 10
			ORDER BY if(deaths > 0, kills / deaths, toFloat64(kills)) DESC, canonical_id
			LIMIT 10
		), top_teamkillers AS (
			SELECT * FROM normalized_combat
			WHERE teamkills > 0
			ORDER BY teamkills DESC, canonical_id
			LIMIT 10
		), top_medics AS (
			SELECT * FROM normalized_revives
			WHERE revives > 0
			ORDER BY revives DESC, canonical_id
			LIMIT 10
		), recent_players AS (
			SELECT * FROM identities
			ORDER BY last_seen DESC, canonical_id
			LIMIT 10
		)
		SELECT category, canonical_id, steam_id, eos_id, player_name,
		       kills, deaths, kd_ratio, teamkills, revives,
		       last_seen, first_seen, total_players
		FROM
		(
			SELECT 'top_player' AS category, canonical_id, steam_id, eos_id, player_name,
			       kills, deaths, if(deaths > 0, kills / deaths, toFloat64(kills)) AS kd_ratio,
			       teamkills, toUInt64(0) AS revives,
			       CAST(NULL AS Nullable(DateTime64(3, 'UTC'))) AS last_seen,
			       CAST(NULL AS Nullable(DateTime64(3, 'UTC'))) AS first_seen,
			       toUInt64(0) AS total_players
			FROM top_players

			UNION ALL

			SELECT 'top_teamkiller', canonical_id, steam_id, eos_id, player_name,
			       kills, deaths, if(deaths > 0, kills / deaths, toFloat64(kills)),
			       teamkills, toUInt64(0), NULL, NULL, toUInt64(0)
			FROM top_teamkillers

			UNION ALL

			SELECT 'top_medic', canonical_id, steam_id, eos_id, player_name,
			       toUInt64(0), toUInt64(0), toFloat64(0), toUInt64(0), revives,
			       NULL, NULL, toUInt64(0)
			FROM top_medics

			UNION ALL

			SELECT 'recent', canonical_id, primary_steam_id, primary_eos_id, primary_name,
			       toUInt64(0), toUInt64(0), toFloat64(0), toUInt64(0), toUInt64(0),
			       toNullable(last_seen), toNullable(first_seen), toUInt64(0)
			FROM recent_players

			UNION ALL

			SELECT 'summary', '', '', '', '',
			       ifNull(sum(kills), toUInt64(0)), ifNull(sum(deaths), toUInt64(0)), toFloat64(0),
			       ifNull(sum(teamkills), toUInt64(0)), toUInt64(0), NULL, NULL,
			       uniqExact(canonical_id)
			FROM normalized_combat
		)
		ORDER BY category, canonical_id
	`, squadlayers.LayerCategorySQL("g.layer", "g.map_classname", "g.layer_classname"), squadlayers.CategoryCompetitive)

	rows, err := s.Dependencies.Clickhouse.Query(ctx, query)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("query player statistics: %w", err), nil)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var (
			category, canonicalID, steamID, eosID, playerName string
			kills, deaths, teamkills, revives, totalPlayers   uint64
			kdRatio                                           float64
			lastSeen, firstSeen                               *time.Time
		)
		if err := rows.Scan(
			&category, &canonicalID, &steamID, &eosID, &playerName,
			&kills, &deaths, &kdRatio, &teamkills, &revives,
			&lastSeen, &firstSeen, &totalPlayers,
		); err != nil {
			responses.InternalServerError(c, fmt.Errorf("scan player statistics: %w", err), nil)
			return
		}

		switch category {
		case "top_player":
			summary.TopPlayers = append(summary.TopPlayers, TopPlayerStats{
				SteamID: steamID, EOSID: eosID, PlayerName: playerName,
				Kills: int64(kills), Deaths: int64(deaths), KDRatio: kdRatio, Teamkills: int64(teamkills),
			})
		case "top_teamkiller":
			summary.TopTeamkillers = append(summary.TopTeamkillers, TopPlayerStats{
				SteamID: steamID, EOSID: eosID, PlayerName: playerName,
				Kills: int64(kills), Deaths: int64(deaths), KDRatio: kdRatio, Teamkills: int64(teamkills),
			})
		case "top_medic":
			summary.TopMedics = append(summary.TopMedics, TopPlayerStats{
				SteamID: steamID, EOSID: eosID, PlayerName: playerName, Revives: int64(revives),
			})
		case "recent":
			summary.MostRecentPlayers = append(summary.MostRecentPlayers, PlayerSearchResult{
				SteamID: steamID, EOSID: eosID, PlayerName: playerName,
				LastSeen: lastSeen, FirstSeen: firstSeen,
			})
		case "summary":
			summary.TotalPlayers = int64(totalPlayers)
			summary.TotalKills = int64(kills)
			summary.TotalDeaths = int64(deaths)
			summary.TotalTeamkills = int64(teamkills)
		default:
			responses.InternalServerError(c, fmt.Errorf("unknown player statistics category %q", category), nil)
			return
		}
	}
	if err := rows.Err(); err != nil {
		responses.InternalServerError(c, fmt.Errorf("read player statistics: %w", err), nil)
		return
	}

	responses.Success(c, "Player statistics fetched successfully", &gin.H{"stats": summary})
}

// getLinkedPlayerIdentifiers retrieves all Steam and EOS IDs linked to a given player ID
// Returns arrays of all linked steam IDs and eos IDs, seeding on Steam IDs
// directly and on either EOS or Epic IDs for 32-character platform IDs.
func (s *Server) getLinkedPlayerIdentifiers(c *gin.Context, playerID string, isSteamID bool) (steamIDs []string, eosIDs []string, err error) {
	whereClause := "steam = ?"
	playerWhereClause := "player_steam = ?"
	queryArgs := []interface{}{playerID, playerID, playerID, playerID}
	if !isSteamID {
		whereClause = "(eos = ? OR epic = ?)"
		playerWhereClause = "(player_eos = ? OR player_epic = ?)"
		queryArgs = []interface{}{
			playerID, playerID, // join_succeeded
			playerID, playerID, // connected
			playerID, playerID, // disconnected
			playerID, playerID, // possess
		}
	}

	query := fmt.Sprintf(`
		WITH initial_records AS (
			SELECT steam, eos
			FROM squad_aegis.server_join_succeeded_events
			WHERE %[1]s
			UNION ALL
			SELECT steam, eos
			FROM squad_aegis.server_player_connected_events
			WHERE %[1]s
			UNION ALL
			SELECT steam, eos
			FROM squad_aegis.server_player_disconnected_events
			WHERE %[1]s
			UNION ALL
			SELECT player_steam as steam, player_eos as eos
			FROM squad_aegis.server_player_possess_events
			WHERE %[2]s
		)
		SELECT
			groupUniqArrayIf(steam, steam != '') as steam_ids,
			groupUniqArrayIf(eos, eos != '') as eos_ids
		FROM initial_records
	`, whereClause, playerWhereClause)

	row := s.Dependencies.Clickhouse.QueryRow(c.Request.Context(), query, queryArgs...)

	var steamIDsArr, eosIDsArr []string
	err = row.Scan(&steamIDsArr, &eosIDsArr)
	if err != nil {
		return nil, nil, err
	}

	// Filter out empty strings
	steamIDs = []string{}
	for _, id := range steamIDsArr {
		if id != "" {
			steamIDs = append(steamIDs, id)
		}
	}

	eosIDs = []string{}
	for _, id := range eosIDsArr {
		if id != "" {
			eosIDs = append(eosIDs, id)
		}
	}

	return steamIDs, eosIDs, nil
}

// getPlayerBasicInfo retrieves basic player information
// Aggregates data across all records that share the same Steam ID or EOS ID
// Handles transitive linking: if records share ANY identifier, they're the same player
func (s *Server) getPlayerBasicInfo(c *gin.Context, playerID string, isSteamID bool) (*PlayerProfile, error) {
	profile, err := s.getPlayerFromIdentityTable(c.Request.Context(), playerID, isSteamID)
	if err == nil && profile != nil {
		return profile, nil
	}
	if errors.Is(err, errPlayerNotFoundInSnapshot) {
		return nil, err
	}

	// Raw events are a compatibility fallback only when there is no published
	// snapshot or the snapshot tables are unavailable.
	return s.getPlayerFromRawEvents(c.Request.Context(), playerID, isSteamID)
}

var (
	errPlayerNotFoundInSnapshot  = errors.New("player not found in current identity snapshot")
	errPlayerSnapshotUnavailable = errors.New("no published player identity snapshot")
)

// getPlayerFromIdentityTable fetches player profile from pre-computed identity table
func (s *Server) getPlayerFromIdentityTable(ctx context.Context, playerID string, isSteamID bool) (*PlayerProfile, error) {
	lookupTypeClause := "pil.identifier_type IN ('eos', 'epic')"
	if isSteamID {
		lookupTypeClause = "pil.identifier_type = 'steam'"
	}
	identityQuery := fmt.Sprintf(`
		/* sa.player.identity_resolve */
		WITH current AS (
			SELECT snapshot_id
			FROM squad_aegis.player_identity_current
			WHERE pointer_key = 1
			ORDER BY published_at DESC
			LIMIT 1
		), matched AS (
			SELECT pil.canonical_id
			FROM squad_aegis.player_identity_lookup_snapshots pil
			INNER JOIN current c ON pil.snapshot_id = c.snapshot_id
			WHERE %s AND pil.identifier_value = ?
			ORDER BY pil.canonical_id
			LIMIT 1
		), identity AS (
			SELECT pi.canonical_id, pi.primary_steam_id, pi.primary_eos_id,
			       pi.primary_epic_id, pi.primary_name, pi.all_steam_ids,
			       pi.all_eos_ids, pi.all_epic_ids, pi.all_names,
			       pi.total_sessions, pi.first_seen, pi.last_seen
			FROM squad_aegis.player_identity_snapshots pi
			INNER JOIN current c ON pi.snapshot_id = c.snapshot_id
			INNER JOIN matched m ON pi.canonical_id = m.canonical_id
		)
		SELECT toUInt8(1), * FROM identity
		UNION ALL
		SELECT if((SELECT count() FROM current) = 0, toUInt8(2), toUInt8(0)),
		       '', '', '', '', '', [], [], [], [], toUInt64(0),
		       toDateTime64(0, 3, 'UTC'), toDateTime64(0, 3, 'UTC')
		WHERE NOT EXISTS (SELECT 1 FROM identity)
		LIMIT 1
	`, lookupTypeClause)

	var profile PlayerProfile
	var allSteamIDs, allEOSIDs, allEpicIDs, allNames []string
	var totalSessions uint64
	var found uint8
	var firstSeen, lastSeen time.Time

	row := s.Dependencies.Clickhouse.QueryRow(ctx, identityQuery, playerID)
	err := row.Scan(
		&found,
		&profile.CanonicalID,
		&profile.SteamID,
		&profile.EOSID,
		&profile.EpicID,
		&profile.PlayerName,
		&allSteamIDs,
		&allEOSIDs,
		&allEpicIDs,
		&allNames,
		&totalSessions,
		&firstSeen,
		&lastSeen,
	)
	if err != nil {
		return nil, fmt.Errorf("resolve current identity snapshot: %w", err)
	}
	if found == 2 {
		return nil, errPlayerSnapshotUnavailable
	}
	if found == 0 {
		return nil, errPlayerNotFoundInSnapshot
	}

	profile.FirstSeen = &firstSeen
	profile.LastSeen = &lastSeen
	profile.AllSteamIDs = allSteamIDs
	profile.AllEOSIDs = allEOSIDs
	profile.AllEpicIDs = allEpicIDs
	profile.AllNames = allNames
	profile.TotalSessions = int64(totalSessions)
	profile.IdentityStatus = "resolved"

	// Calculate total play time
	if profile.FirstSeen != nil && profile.LastSeen != nil {
		profile.TotalPlayTime = int64(profile.LastSeen.Sub(*profile.FirstSeen).Seconds())
	}

	return &profile, nil
}

// getPlayerFromRawEvents fetches player profile from raw events (fallback)
// Aggregates timestamps from ALL player activity tables for accurate first/last seen
// Uses linked_identifiers CTE to capture all events for players with multiple Steam/EOS IDs
func (s *Server) getPlayerFromRawEvents(ctx context.Context, playerID string, isSteamID bool) (*PlayerProfile, error) {
	// Build where clauses for different table column naming conventions
	whereClause := "steam = ?"
	playerWhereClause := "player_steam = ?"
	attackerWhereClause := "attacker_steam = ?"
	victimWhereClause := "victim_steam = ?"
	reviverWhereClause := "reviver_steam = ?"
	queryArgs := []interface{}{
		playerID, // join
		playerID, // connected
		playerID, // disconnected
		playerID, // possess
		playerID, // damaged attacker
		playerID, // damaged victim
		playerID, // died attacker
		playerID, // died victim
		playerID, // wounded attacker
		playerID, // wounded victim
		playerID, // revived reviver
		playerID, // revived victim
	}
	if !isSteamID {
		whereClause = "(eos = ? OR epic = ?)"
		playerWhereClause = "(player_eos = ? OR player_epic = ?)"
		attackerWhereClause = "attacker_eos = ?"
		victimWhereClause = "victim_eos = ?"
		reviverWhereClause = "reviver_eos = ?"
		queryArgs = []interface{}{
			playerID, playerID, // join
			playerID, playerID, // connected
			playerID, playerID, // disconnected
			playerID, playerID, // possess
			playerID, // damaged attacker
			playerID, // damaged victim
			playerID, // died attacker
			playerID, // died victim
			playerID, // wounded attacker
			playerID, // wounded victim
			playerID, // revived reviver
			playerID, // revived victim
		}
	}

	// Query across ALL player activity tables using linked_identifiers for transitive identity linking
	// The linked_identifiers CTE seeds from ALL event tables to find players even without join_succeeded events
	query := fmt.Sprintf(`
		WITH seed_identifiers AS (
			-- Seed from ALL event tables to find any matching steam/eos IDs
			SELECT steam, eos, epic FROM squad_aegis.server_join_succeeded_events WHERE %[1]s
			UNION ALL
			SELECT steam, eos, epic FROM squad_aegis.server_player_connected_events WHERE %[1]s
			UNION ALL
			SELECT steam, eos, epic FROM squad_aegis.server_player_disconnected_events WHERE %[1]s
			UNION ALL
			SELECT player_steam as steam, player_eos as eos, player_epic as epic FROM squad_aegis.server_player_possess_events WHERE %[2]s
			UNION ALL
			SELECT attacker_steam as steam, attacker_eos as eos, '' as epic FROM squad_aegis.server_player_damaged_events WHERE %[3]s
			UNION ALL
			SELECT victim_steam as steam, victim_eos as eos, '' as epic FROM squad_aegis.server_player_damaged_events WHERE %[4]s
			UNION ALL
			SELECT attacker_steam as steam, attacker_eos as eos, '' as epic FROM squad_aegis.server_player_died_events WHERE %[3]s
			UNION ALL
			SELECT victim_steam as steam, victim_eos as eos, '' as epic FROM squad_aegis.server_player_died_events WHERE %[4]s
			UNION ALL
			SELECT attacker_steam as steam, attacker_eos as eos, '' as epic FROM squad_aegis.server_player_wounded_events WHERE %[3]s
			UNION ALL
			SELECT victim_steam as steam, victim_eos as eos, '' as epic FROM squad_aegis.server_player_wounded_events WHERE %[4]s
			UNION ALL
			SELECT reviver_steam as steam, reviver_eos as eos, '' as epic FROM squad_aegis.server_player_revived_events WHERE %[5]s
			UNION ALL
			SELECT victim_steam as steam, victim_eos as eos, '' as epic FROM squad_aegis.server_player_revived_events WHERE %[4]s
		),
		linked_identifiers AS (
			-- Collect all unique identifiers from the seed
			SELECT
				groupUniqArrayIf(steam, steam != '') as steam_ids,
				groupUniqArrayIf(eos, eos != '') as eos_ids,
				groupUniqArrayIf(epic, epic != '') as epic_ids
			FROM seed_identifiers
		),
		all_player_events AS (
			-- Join succeeded events (primary source for identity)
			SELECT steam, eos, epic, player_suffix as name, event_time
			FROM squad_aegis.server_join_succeeded_events
			WHERE (steam != '' AND steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers WHERE length(steam_ids) > 0))
			   OR (eos != '' AND eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers WHERE length(eos_ids) > 0))
			   OR (epic != '' AND epic IN (SELECT arrayJoin(epic_ids) FROM linked_identifiers WHERE length(epic_ids) > 0))
			UNION ALL
			-- Connected events
			SELECT steam, eos, epic, '' as name, event_time
			FROM squad_aegis.server_player_connected_events
			WHERE (steam != '' AND steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers WHERE length(steam_ids) > 0))
			   OR (eos != '' AND eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers WHERE length(eos_ids) > 0))
			   OR (epic != '' AND epic IN (SELECT arrayJoin(epic_ids) FROM linked_identifiers WHERE length(epic_ids) > 0))
			UNION ALL
			-- Disconnected events
			SELECT steam, eos, epic, player_suffix as name, event_time
			FROM squad_aegis.server_player_disconnected_events
			WHERE (steam != '' AND steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers WHERE length(steam_ids) > 0))
			   OR (eos != '' AND eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers WHERE length(eos_ids) > 0))
			   OR (epic != '' AND epic IN (SELECT arrayJoin(epic_ids) FROM linked_identifiers WHERE length(epic_ids) > 0))
			UNION ALL
			-- Possess events (different column names)
			SELECT player_steam as steam, player_eos as eos, player_epic as epic, player_suffix as name, event_time
			FROM squad_aegis.server_player_possess_events
			WHERE (player_steam != '' AND player_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers WHERE length(steam_ids) > 0))
			   OR (player_eos != '' AND player_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers WHERE length(eos_ids) > 0))
			   OR (player_epic != '' AND player_epic IN (SELECT arrayJoin(epic_ids) FROM linked_identifiers WHERE length(epic_ids) > 0))
			UNION ALL
			-- Damage dealt (as attacker)
			SELECT attacker_steam as steam, attacker_eos as eos, '' as epic, attacker_name as name, event_time
			FROM squad_aegis.server_player_damaged_events
			WHERE (attacker_steam != '' AND attacker_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers WHERE length(steam_ids) > 0))
			   OR (attacker_eos != '' AND attacker_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers WHERE length(eos_ids) > 0))
			UNION ALL
			-- Damage taken (as victim)
			SELECT victim_steam as steam, victim_eos as eos, '' as epic, victim_name as name, event_time
			FROM squad_aegis.server_player_damaged_events
			WHERE (victim_steam != '' AND victim_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers WHERE length(steam_ids) > 0))
			   OR (victim_eos != '' AND victim_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers WHERE length(eos_ids) > 0))
			UNION ALL
			-- Deaths (as victim)
			SELECT victim_steam as steam, victim_eos as eos, '' as epic, victim_name as name, event_time
			FROM squad_aegis.server_player_died_events
			WHERE (victim_steam != '' AND victim_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers WHERE length(steam_ids) > 0))
			   OR (victim_eos != '' AND victim_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers WHERE length(eos_ids) > 0))
			UNION ALL
			-- Kills (as attacker)
			SELECT attacker_steam as steam, attacker_eos as eos, '' as epic, attacker_name as name, event_time
			FROM squad_aegis.server_player_died_events
			WHERE (attacker_steam != '' AND attacker_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers WHERE length(steam_ids) > 0))
			   OR (attacker_eos != '' AND attacker_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers WHERE length(eos_ids) > 0))
			UNION ALL
			-- Wounded (as victim)
			SELECT victim_steam as steam, victim_eos as eos, '' as epic, victim_name as name, event_time
			FROM squad_aegis.server_player_wounded_events
			WHERE (victim_steam != '' AND victim_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers WHERE length(steam_ids) > 0))
			   OR (victim_eos != '' AND victim_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers WHERE length(eos_ids) > 0))
			UNION ALL
			-- Wounded someone (as attacker)
			SELECT attacker_steam as steam, attacker_eos as eos, '' as epic, attacker_name as name, event_time
			FROM squad_aegis.server_player_wounded_events
			WHERE (attacker_steam != '' AND attacker_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers WHERE length(steam_ids) > 0))
			   OR (attacker_eos != '' AND attacker_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers WHERE length(eos_ids) > 0))
			UNION ALL
			-- Revived someone (as reviver)
			SELECT reviver_steam as steam, reviver_eos as eos, '' as epic, reviver_name as name, event_time
			FROM squad_aegis.server_player_revived_events
			WHERE (reviver_steam != '' AND reviver_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers WHERE length(steam_ids) > 0))
			   OR (reviver_eos != '' AND reviver_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers WHERE length(eos_ids) > 0))
			UNION ALL
			-- Got revived (as victim)
			SELECT victim_steam as steam, victim_eos as eos, '' as epic, victim_name as name, event_time
			FROM squad_aegis.server_player_revived_events
			WHERE (victim_steam != '' AND victim_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers WHERE length(steam_ids) > 0))
			   OR (victim_eos != '' AND victim_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers WHERE length(eos_ids) > 0))
		)
		SELECT
			anyIf(steam, steam != '') as steam_id,
			anyIf(eos, eos != '') as eos_id,
			anyIf(epic, epic != '') as epic_id,
			anyIf(name, name != '') as player_name,
			max(event_time) as last_seen,
			min(event_time) as first_seen,
			count(DISTINCT toDate(event_time)) as total_sessions
		FROM all_player_events
		WHERE steam != '' OR eos != '' OR epic != ''
	`, whereClause, playerWhereClause, attackerWhereClause, victimWhereClause, reviverWhereClause)

	// Pass the playerID for each subquery in the seed_identifiers UNION ALL
	row := s.Dependencies.Clickhouse.QueryRow(ctx, query, queryArgs...)

	var profile PlayerProfile
	var steamID, eosID, epicID *string
	var totalSessions int64

	err := row.Scan(
		&steamID,
		&eosID,
		&epicID,
		&profile.PlayerName,
		&profile.LastSeen,
		&profile.FirstSeen,
		&totalSessions,
	)
	if err != nil {
		return nil, err
	}

	if steamID != nil {
		profile.SteamID = *steamID
	}
	if eosID != nil {
		profile.EOSID = *eosID
	}
	if epicID != nil {
		profile.EpicID = *epicID
	}
	profile.TotalSessions = totalSessions
	profile.IdentityStatus = "pending" // Not yet in identity table

	// Calculate total play time (approximate based on session days)
	if profile.FirstSeen != nil && profile.LastSeen != nil {
		profile.TotalPlayTime = int64(profile.LastSeen.Sub(*profile.FirstSeen).Seconds())
	}

	return &profile, nil
}

// getPlayerStatistics retrieves player combat statistics
// Includes all linked identities (transitive linking by Steam ID or EOS ID)
func (s *Server) getPlayerStatistics(ctx context.Context, identity ResolvedPlayerIdentity) (*PlayerStatistics, error) {
	if len(identity.SteamIDs) == 0 && len(identity.EOSIDs) == 0 {
		return &PlayerStatistics{}, nil
	}
	const identityArrays = `
		WITH
			CAST(? AS Array(String)) AS steam_ids,
			CAST(? AS Array(String)) AS eos_ids`
	const attackerMatch = `(ifNull(attacker_steam, '') IN steam_ids OR ifNull(attacker_eos, '') IN eos_ids)`
	const victimMatch = `(ifNull(victim_steam, '') IN steam_ids OR ifNull(victim_eos, '') IN eos_ids)`

	query := identityArrays + `
		/* sa.player.profile_combat */
		SELECT
			countIf(` + attackerMatch + `),
			countIf(` + victimMatch + `),
			countIf(teamkill = 1 AND ` + attackerMatch + `),
			sumIf(toFloat64(damage), ` + attackerMatch + `),
			sumIf(toFloat64(damage), ` + victimMatch + `)
		FROM squad_aegis.server_player_died_events`

	var stats PlayerStatistics
	if err := s.Dependencies.Clickhouse.QueryRow(ctx, query, identity.SteamIDs, identity.EOSIDs).Scan(
		&stats.Kills,
		&stats.Deaths,
		&stats.Teamkills,
		&stats.DamageDealt,
		&stats.DamageTaken,
	); err != nil {
		return nil, err
	}
	if stats.Deaths > 0 {
		stats.KDRatio = float64(stats.Kills) / float64(stats.Deaths)
	} else {
		stats.KDRatio = float64(stats.Kills)
	}

	reviveQuery := identityArrays + `
		/* sa.player.profile_revives */
		SELECT
			countIf(ifNull(reviver_steam, '') IN steam_ids OR ifNull(reviver_eos, '') IN eos_ids),
			countIf(ifNull(victim_steam, '') IN steam_ids OR ifNull(victim_eos, '') IN eos_ids)
		FROM squad_aegis.server_player_revived_events`
	if err := s.Dependencies.Clickhouse.QueryRow(ctx, reviveQuery, identity.SteamIDs, identity.EOSIDs).Scan(
		&stats.Revives,
		&stats.TimesRevived,
	); err != nil {
		return nil, err
	}

	return &stats, nil
}

// getPlayerRecentActivity retrieves recent player activity
func (s *Server) getPlayerRecentActivity(ctx context.Context, identity ResolvedPlayerIdentity, limit int) ([]PlayerActivity, error) {
	// Combine multiple event types into a single activity feed
	// We'll query died events, wounded events, and chat messages
	steamIDs, eosIDs := identity.SteamIDs, identity.EOSIDs
	connectionWhereClause, connectionArgs := buildClickHouseIdentifierWhereClause("steam", "eos", steamIDs, eosIDs, false)
	deathWhereClause, deathArgs := buildClickHouseIdentifierWhereClause("victim_steam", "victim_eos", steamIDs, eosIDs, false)
	chatWhereClause, chatArgs := buildClickHouseIdentifierWhereClause("steam_id", "eos_id", steamIDs, eosIDs, true)
	if len(steamIDs) == 0 && len(eosIDs) == 0 {
		return []PlayerActivity{}, nil
	}

	query := fmt.Sprintf(`/* sa.player.recent_activity */
		SELECT
			event_time,
			'connection' as event_type,
			concat('Connected to server') as description,
			server_id
		FROM squad_aegis.server_join_succeeded_events
		WHERE %s

		UNION ALL

		SELECT
			event_time,
			'death' as event_type,
			concat('Killed by ', attacker_name, ' with ', weapon) as description,
			server_id
		FROM squad_aegis.server_player_died_events
		WHERE %s

		UNION ALL

		SELECT
			sent_at as event_time,
			'chat' as event_type,
			concat('[', chat_type, '] ', message) as description,
			server_id
		FROM squad_aegis.server_player_chat_messages
		WHERE %s

		ORDER BY event_time DESC
		LIMIT ?
	`, connectionWhereClause, deathWhereClause, chatWhereClause)

	queryArgs := make([]interface{}, 0, len(connectionArgs)+len(deathArgs)+len(chatArgs)+1)
	queryArgs = append(queryArgs, connectionArgs...)
	queryArgs = append(queryArgs, deathArgs...)
	queryArgs = append(queryArgs, chatArgs...)
	queryArgs = append(queryArgs, limit)

	rows, err := s.Dependencies.Clickhouse.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	activities := []PlayerActivity{}
	for rows.Next() {
		var activity PlayerActivity
		if err := rows.Scan(
			&activity.EventTime,
			&activity.EventType,
			&activity.Description,
			&activity.ServerID,
		); err != nil {
			return nil, err
		}
		activities = append(activities, activity)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return activities, nil
}

// getPlayerChatHistory retrieves player chat history
func (s *Server) getPlayerChatHistory(ctx context.Context, identity ResolvedPlayerIdentity, limit int) ([]ChatMessage, error) {
	steamIDs, eosIDs := identity.SteamIDs, identity.EOSIDs
	whereClause, queryArgs := buildClickHouseIdentifierWhereClause("steam_id", "eos_id", steamIDs, eosIDs, true)
	if whereClause == "1 = 0" {
		return []ChatMessage{}, nil
	}

	query := fmt.Sprintf(`/* sa.player.profile_chat */
		SELECT
			sent_at,
			message,
			chat_type,
			server_id
		FROM squad_aegis.server_player_chat_messages
		WHERE %s
		ORDER BY sent_at DESC
		LIMIT ?
	`, whereClause)

	queryArgs = append(queryArgs, limit)

	rows, err := s.Dependencies.Clickhouse.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []ChatMessage{}
	for rows.Next() {
		var msg ChatMessage
		if err := rows.Scan(
			&msg.SentAt,
			&msg.Message,
			&msg.ChatType,
			&msg.ServerID,
		); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}

func (s *Server) getPlayerBanHistory(c *gin.Context, playerID string, isSteamID bool, limit int) ([]ActiveBan, error) {
	steamIDs, eosIDs := s.resolveLinkedPlayerIdentifiers(c, playerID, isSteamID)
	whereClause, args, nextArg := buildServerBanIdentifierWhereClause(steamIDs, eosIDs, "b.", 1)
	if whereClause == "1 = 0" {
		return []ActiveBan{}, nil
	}

	query := fmt.Sprintf(`
		SELECT
			b.id,
			b.server_id,
			s.name,
			b.reason,
			b.expires_at,
			b.created_at,
			COALESCE(u.name, u.username, 'System') as admin_name
		FROM server_bans b
		JOIN servers s ON b.server_id = s.id
		LEFT JOIN users u ON b.admin_id = u.id
		WHERE (%s)
		ORDER BY b.created_at DESC
		LIMIT $%d
	`, whereClause, nextArg)

	args = append(args, limit)

	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	history := []ActiveBan{}
	for rows.Next() {
		var entry ActiveBan
		var banID uuid.UUID
		var serverID uuid.UUID
		var expiresAt sql.NullTime
		if err := rows.Scan(
			&banID,
			&serverID,
			&entry.ServerName,
			&entry.Reason,
			&expiresAt,
			&entry.CreatedAt,
			&entry.AdminName,
		); err != nil {
			return nil, err
		}

		entry.BanID = banID.String()
		entry.ServerID = serverID.String()
		if expiresAt.Valid {
			entry.ExpiresAt = &expiresAt.Time
		}
		entry.Permanent = entry.ExpiresAt == nil

		history = append(history, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return history, nil
}

// getPlayerViolations retrieves player rule violations
func (s *Server) getPlayerViolations(ctx context.Context, identity ResolvedPlayerIdentity) ([]RuleViolation, error) {
	steamIDs, eosIDs := identity.SteamIDs, identity.EOSIDs
	whereClause, args := buildPlayerRuleViolationWhereClause(steamIDs, eosIDs, "")
	if whereClause == "1 = 0" {
		return []RuleViolation{}, nil
	}

	query := fmt.Sprintf(`/* sa.player.profile_violations */
		SELECT
			violation_id,
			server_id,
			rule_id,
			admin_user_id,
			action_type,
			created_at
		FROM squad_aegis.player_rule_violations
		WHERE %s
		ORDER BY created_at DESC
	`, whereClause)

	rows, err := s.Dependencies.Clickhouse.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	violations := []RuleViolation{}
	for rows.Next() {
		var violation RuleViolation
		if err := rows.Scan(
			&violation.ViolationID,
			&violation.ServerID,
			&violation.RuleID,
			&violation.AdminUserID,
			&violation.ActionType,
			&violation.CreatedAt,
		); err != nil {
			return nil, err
		}
		violations = append(violations, violation)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.enrichPlayerViolationsBatch(ctx, violations); err != nil {
		return nil, err
	}

	return violations, nil
}

func (s *Server) enrichPlayerViolationsBatch(ctx context.Context, violations []RuleViolation) error {
	serverIDs := make([]string, 0, len(violations))
	ruleIDs := make([]string, 0, len(violations))
	adminIDs := make([]string, 0, len(violations))
	appendUUID := func(values []string, value string) []string {
		if _, err := uuid.Parse(value); err == nil {
			return append(values, value)
		}
		return values
	}
	for i := range violations {
		serverIDs = appendUUID(serverIDs, violations[i].ServerID)
		if violations[i].RuleID != nil {
			ruleIDs = appendUUID(ruleIDs, *violations[i].RuleID)
		}
		if violations[i].AdminUserID != nil {
			adminIDs = appendUUID(adminIDs, *violations[i].AdminUserID)
		}
	}
	if len(serverIDs) == 0 && len(ruleIDs) == 0 && len(adminIDs) == 0 {
		return nil
	}

	rows, err := s.Dependencies.DB.QueryContext(ctx, `
		/* sa.player.violation_enrichment */
		SELECT 'server' AS entity_type, id::text, name
		FROM servers
		WHERE id = ANY($1::uuid[])
		UNION ALL
		SELECT 'rule', id::text, title
		FROM server_rules
		WHERE id = ANY($2::uuid[])
		UNION ALL
		SELECT 'admin', id::text, COALESCE(name, username)
		FROM users
		WHERE id = ANY($3::uuid[])
	`, pq.Array(serverIDs), pq.Array(ruleIDs), pq.Array(adminIDs))
	if err != nil {
		return err
	}
	defer rows.Close()

	names := make(map[string]string, len(serverIDs)+len(ruleIDs)+len(adminIDs))
	for rows.Next() {
		var entityType, id, name string
		if err := rows.Scan(&entityType, &id, &name); err != nil {
			return err
		}
		names[entityType+":"+id] = name
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range violations {
		violations[i].ServerName = names["server:"+violations[i].ServerID]
		if violations[i].RuleID != nil {
			if name, ok := names["rule:"+*violations[i].RuleID]; ok {
				violations[i].RuleName = &name
			}
		}
		if violations[i].AdminUserID != nil {
			if name, ok := names["admin:"+*violations[i].AdminUserID]; ok {
				violations[i].AdminName = &name
			}
		}
	}
	return nil
}

// getPlayerRecentServers retrieves servers the player has recently played on
func (s *Server) getPlayerRecentServers(ctx context.Context, identity ResolvedPlayerIdentity) ([]RecentServerInfo, error) {
	steamIDs, eosIDs := identity.SteamIDs, identity.EOSIDs
	whereClause, args := buildClickHouseIdentifierWhereClause("steam", "eos", steamIDs, eosIDs, false)
	if whereClause == "1 = 0" {
		return []RecentServerInfo{}, nil
	}

	query := fmt.Sprintf(`/* sa.player.recent_servers */
		SELECT
			server_id,
			max(event_time) as last_seen,
			count(*) as sessions
		FROM squad_aegis.server_join_succeeded_events
		WHERE %s
		GROUP BY server_id
		ORDER BY last_seen DESC
		LIMIT 10
	`, whereClause)

	rows, err := s.Dependencies.Clickhouse.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	servers := []RecentServerInfo{}
	serverIDs := []string{}
	for rows.Next() {
		var server RecentServerInfo
		if err := rows.Scan(
			&server.ServerID,
			&server.LastSeen,
			&server.Sessions,
		); err != nil {
			return nil, err
		}
		serverIDs = append(serverIDs, server.ServerID)
		servers = append(servers, server)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	serverNames, err := s.loadServerNamesBatch(ctx, serverIDs)
	if err != nil {
		return nil, err
	}
	for i := range servers {
		servers[i].ServerName = serverNames[servers[i].ServerID]
	}

	return servers, nil
}

// getPlayerActiveBans retrieves active bans for the player, matching on both
// steam_id and eos_id so that legacy steam-only bans are found for EOS lookups.
func (s *Server) getPlayerActiveBans(ctx context.Context, identity ResolvedPlayerIdentity) ([]ActiveBan, error) {
	whereClause, args, _ := buildServerBanIdentifierWhereClause(identity.SteamIDs, identity.EOSIDs, "b.", 1)
	if whereClause == "1 = 0" {
		return []ActiveBan{}, nil
	}

	// Query PostgreSQL for active bans
	query := fmt.Sprintf(`
			SELECT
				b.id, b.server_id, b.reason, b.expires_at,
				b.created_at,
				COALESCE(s.name, 'Unknown Server') as server_name,
				COALESCE(u.name, u.username, 'System') as admin_name
			FROM server_bans b
			LEFT JOIN servers s ON b.server_id = s.id
			LEFT JOIN users u ON b.admin_id = u.id
			WHERE (%s)
			AND (b.expires_at IS NULL OR b.expires_at > NOW())
			ORDER BY b.created_at DESC
	`, whereClause)

	rows, err := s.Dependencies.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bans := []ActiveBan{}
	for rows.Next() {
		var ban ActiveBan
		var expiresAt sql.NullTime
		if err := rows.Scan(
			&ban.BanID,
			&ban.ServerID,
			&ban.Reason,
			&expiresAt,
			&ban.CreatedAt,
			&ban.ServerName,
			&ban.AdminName,
		); err != nil {
			return nil, err
		}
		if expiresAt.Valid {
			ban.ExpiresAt = &expiresAt.Time
		}
		ban.Permanent = ban.ExpiresAt == nil
		bans = append(bans, ban)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return bans, nil
}

// getPlayerViolationSummary retrieves a summary of player violations
func (s *Server) getPlayerViolationSummary(ctx context.Context, identity ResolvedPlayerIdentity) (*ViolationSummary, error) {
	steamIDs, eosIDs := identity.SteamIDs, identity.EOSIDs
	whereClause, args := buildPlayerRuleViolationWhereClause(steamIDs, eosIDs, "")
	if whereClause == "1 = 0" {
		return &ViolationSummary{}, nil
	}

	query := fmt.Sprintf(`/* sa.player.violation_summary */
		SELECT
			countIf(action_type = 'WARN') as total_warns,
			countIf(action_type = 'KICK') as total_kicks,
			countIf(action_type = 'BAN') as total_bans,
			max(created_at) as last_action
		FROM squad_aegis.player_rule_violations
		WHERE %s
	`, whereClause)

	row := s.Dependencies.Clickhouse.QueryRow(ctx, query, args...)

	var summary ViolationSummary
	err := row.Scan(
		&summary.TotalWarns,
		&summary.TotalKicks,
		&summary.TotalBans,
		&summary.LastAction,
	)
	if err != nil {
		return nil, err
	}

	return &summary, nil
}

// getPlayerTeamkillMetrics retrieves detailed teamkill statistics, split by the
// layer each teamkill occurred on. Each friendly-fire row is matched to the
// layer that was live when it happened via an ASOF join against the most recent
// NEW_GAME event for that server, then classified competitive/seeding/training.
// The headline metrics count competitive play only; seeding and training
// (Jensen's Range) friendly fire is reported separately and excluded from the
// TK rate and risk flags.
func (s *Server) getPlayerTeamkillMetrics(ctx context.Context, identity ResolvedPlayerIdentity, totalSessions int64) (*TeamkillMetrics, error) {
	if len(identity.SteamIDs) == 0 && len(identity.EOSIDs) == 0 {
		return &TeamkillMetrics{}, nil
	}
	categoryExpr := squadlayers.LayerCategorySQL("g.layer", "g.map_classname", "g.layer_classname")

	var metrics TeamkillMetrics
	var competitiveKills, allKills int64
	query := fmt.Sprintf(`/* sa.player.teamkill_metrics */
		WITH
			CAST(? AS Array(String)) AS steam_ids,
			CAST(? AS Array(String)) AS eos_ids,
		classified AS (
			SELECT
				d.kind,
				d.event_time,
				d.teamkill,
				%s AS category
			FROM (
				SELECT server_id, event_time, 'kill' AS kind, toUInt8(teamkill) AS teamkill
				FROM squad_aegis.server_player_died_events
				WHERE ifNull(attacker_steam, '') IN steam_ids OR ifNull(attacker_eos, '') IN eos_ids
				UNION ALL
				SELECT server_id, event_time, 'wound' AS kind, toUInt8(teamkill) AS teamkill
				FROM squad_aegis.server_player_wounded_events
				WHERE teamkill = 1 AND (ifNull(attacker_steam, '') IN steam_ids OR ifNull(attacker_eos, '') IN eos_ids)
				UNION ALL
				SELECT server_id, event_time, 'damage' AS kind, toUInt8(teamkill) AS teamkill
				FROM squad_aegis.server_player_damaged_events
				WHERE teamkill = 1 AND (ifNull(attacker_steam, '') IN steam_ids OR ifNull(attacker_eos, '') IN eos_ids)
			) AS d
			ASOF LEFT JOIN squad_aegis.server_new_game_events AS g
				ON d.server_id = g.server_id AND d.event_time >= g.event_time
		)
		SELECT
			countIf(kind = 'kill' AND teamkill = 1),
			countIf(kind = 'kill' AND teamkill = 1 AND category = '%s'),
			countIf(kind = 'kill' AND teamkill = 1 AND category = '%s' AND event_time >= now() - INTERVAL 7 DAY),
			countIf(kind = 'kill' AND teamkill = 1 AND category = '%s'),
			countIf(kind = 'kill' AND teamkill = 1 AND category = '%s'),
			countIf(kind = 'wound' AND teamkill = 1 AND category = '%s'),
			countIf(kind = 'wound' AND teamkill = 1 AND category = '%s' AND event_time >= now() - INTERVAL 7 DAY),
			countIf(kind = 'wound' AND teamkill = 1 AND category = '%s'),
			countIf(kind = 'wound' AND teamkill = 1 AND category = '%s'),
			countIf(kind = 'damage' AND teamkill = 1 AND category = '%s'),
			countIf(kind = 'damage' AND teamkill = 1 AND category = '%s' AND event_time >= now() - INTERVAL 7 DAY),
			countIf(kind = 'damage' AND teamkill = 1 AND category = '%s'),
			countIf(kind = 'damage' AND teamkill = 1 AND category = '%s'),
			countIf(kind = 'kill' AND category = '%s'),
			countIf(kind = 'kill')
		FROM classified
	`,
		categoryExpr,
		squadlayers.CategoryCompetitive,
		squadlayers.CategoryCompetitive,
		squadlayers.CategorySeeding,
		squadlayers.CategoryTraining,
		squadlayers.CategoryCompetitive,
		squadlayers.CategoryCompetitive,
		squadlayers.CategorySeeding,
		squadlayers.CategoryTraining,
		squadlayers.CategoryCompetitive,
		squadlayers.CategoryCompetitive,
		squadlayers.CategorySeeding,
		squadlayers.CategoryTraining,
		squadlayers.CategoryCompetitive,
	)

	err := s.Dependencies.Clickhouse.QueryRow(
		ctx,
		query,
		identity.SteamIDs,
		identity.EOSIDs,
	).Scan(
		&metrics.AllTeamkills,
		&metrics.TotalTeamkills,
		&metrics.RecentTeamkills,
		&metrics.NonCompetitive.SeedingTeamkills,
		&metrics.NonCompetitive.TrainingTeamkills,
		&metrics.TotalTeamWounds,
		&metrics.RecentTeamWounds,
		&metrics.NonCompetitive.SeedingTeamWounds,
		&metrics.NonCompetitive.TrainingTeamWounds,
		&metrics.TotalTeamDamage,
		&metrics.RecentTeamDamage,
		&metrics.NonCompetitive.SeedingTeamDamage,
		&metrics.NonCompetitive.TrainingTeamDamage,
		&competitiveKills,
		&allKills,
	)
	if err != nil {
		return nil, fmt.Errorf("query player teamkill metrics: %w", err)
	}

	// Per-session rate (competitive TKs across all sessions)
	if totalSessions > 0 {
		metrics.TeamkillsPerSession = float64(metrics.TotalTeamkills) / float64(totalSessions)
	}

	// Competitive TK ratio (competitive TKs / competitive kills)
	if competitiveKills > 0 {
		metrics.TeamkillRatio = float64(metrics.TotalTeamkills) / float64(competitiveKills)
	}

	// Raw all-context ratio (for transparency / tooltips)
	if allKills > 0 {
		metrics.AllTeamkillRatio = float64(metrics.AllTeamkills) / float64(allKills)
	}

	return &metrics, nil
}

// getPlayerNameHistory retrieves all names used by the player
// Aggregates names from multiple event tables for comprehensive history
func (s *Server) getPlayerNameHistory(ctx context.Context, identity ResolvedPlayerIdentity) ([]NameHistoryEntry, error) {
	if len(identity.SteamIDs) == 0 && len(identity.EOSIDs) == 0 {
		return []NameHistoryEntry{}, nil
	}
	type identityColumns struct{ steam, eos string }
	columns := []identityColumns{
		{"steam", "eos"},
		{"steam", "eos"},
		{"player_steam", "player_eos"},
		{"attacker_steam", "attacker_eos"},
		{"victim_steam", "victim_eos"},
		{"attacker_steam", "attacker_eos"},
		{"victim_steam", "victim_eos"},
		{"reviver_steam", "reviver_eos"},
		{"victim_steam", "victim_eos"},
	}
	where := make([]string, 0, len(columns))
	args := make([]interface{}, 0, len(columns)*2)
	for _, pair := range columns {
		clause, clauseArgs := buildClickHouseIdentifierWhereClause(pair.steam, pair.eos, identity.SteamIDs, identity.EOSIDs, false)
		where = append(where, clause)
		args = append(args, clauseArgs...)
	}
	query := fmt.Sprintf(`/* sa.player.name_history */
		WITH all_names AS (
			SELECT player_suffix AS name, event_time FROM squad_aegis.server_join_succeeded_events WHERE player_suffix != '' AND (%s)
			UNION ALL
			SELECT player_suffix, event_time FROM squad_aegis.server_player_disconnected_events WHERE player_suffix != '' AND (%s)
			UNION ALL
			SELECT player_suffix, event_time FROM squad_aegis.server_player_possess_events WHERE player_suffix != '' AND (%s)
			UNION ALL
			SELECT attacker_name, event_time FROM squad_aegis.server_player_died_events WHERE attacker_name != '' AND (%s)
			UNION ALL
			SELECT victim_name, event_time FROM squad_aegis.server_player_died_events WHERE victim_name != '' AND (%s)
			UNION ALL
			SELECT attacker_name, event_time FROM squad_aegis.server_player_wounded_events WHERE attacker_name != '' AND (%s)
			UNION ALL
			SELECT victim_name, event_time FROM squad_aegis.server_player_wounded_events WHERE victim_name != '' AND (%s)
			UNION ALL
			SELECT reviver_name, event_time FROM squad_aegis.server_player_revived_events WHERE reviver_name != '' AND (%s)
			UNION ALL
			SELECT victim_name, event_time FROM squad_aegis.server_player_revived_events WHERE victim_name != '' AND (%s)
		)
		SELECT name, min(event_time), max(event_time), count(DISTINCT toDate(event_time))
		FROM all_names
		GROUP BY name
		ORDER BY max(event_time) DESC, name
	`, where[0], where[1], where[2], where[3], where[4], where[5], where[6], where[7], where[8])
	rows, err := s.Dependencies.Clickhouse.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := []NameHistoryEntry{}
	for rows.Next() {
		var entry NameHistoryEntry
		if err := rows.Scan(&entry.Name, &entry.FirstUsed, &entry.LastUsed, &entry.SessionCount); err != nil {
			return nil, err
		}
		names = append(names, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return names, nil
}

// getPlayerNameHistoryLegacy is retained only for rollback compatibility with
// the pre-snapshot query shape and is not used by request paths.
func (s *Server) getPlayerNameHistoryLegacy(c *gin.Context, playerID string, isSteamID bool) ([]NameHistoryEntry, error) {
	// Build seed where clause - matches either steam or eos depending on input type
	seedWhereClause := "steam = ?"
	seedPlayerWhereClause := "player_steam = ?"
	seedAttackerWhereClause := "attacker_steam = ?"
	seedVictimWhereClause := "victim_steam = ?"
	if !isSteamID {
		seedWhereClause = "eos = ?"
		seedPlayerWhereClause = "player_eos = ?"
		seedAttackerWhereClause = "attacker_eos = ?"
		seedVictimWhereClause = "victim_eos = ?"
	}

	// Query names from all event tables using linked identifiers
	// Handles cases where player has: steam-only, eos-only, or both
	query := fmt.Sprintf(`
		WITH seed_identifiers AS (
			-- Seed from multiple event tables to find ALL linked steam/eos IDs
			SELECT steam, eos FROM squad_aegis.server_join_succeeded_events WHERE %[1]s
			UNION ALL
			SELECT steam, eos FROM squad_aegis.server_player_connected_events WHERE %[1]s
			UNION ALL
			SELECT steam, eos FROM squad_aegis.server_player_disconnected_events WHERE %[1]s
			UNION ALL
			SELECT player_steam as steam, player_eos as eos FROM squad_aegis.server_player_possess_events WHERE %[2]s
			UNION ALL
			SELECT attacker_steam as steam, attacker_eos as eos FROM squad_aegis.server_player_died_events WHERE %[3]s
			UNION ALL
			SELECT victim_steam as steam, victim_eos as eos FROM squad_aegis.server_player_died_events WHERE %[4]s
		),
		linked_identifiers AS (
			SELECT
				groupUniqArrayIf(steam, steam != '') as steam_ids,
				groupUniqArrayIf(eos, eos != '') as eos_ids
			FROM seed_identifiers
		),
		all_names AS (
			-- Join succeeded events
			SELECT player_suffix as name, event_time
			FROM squad_aegis.server_join_succeeded_events
			WHERE player_suffix != '' AND (
				(length((SELECT steam_ids FROM linked_identifiers)) > 0 AND steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers))
				OR (length((SELECT eos_ids FROM linked_identifiers)) > 0 AND eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers))
			)
			UNION ALL
			-- Disconnected events
			SELECT player_suffix as name, event_time
			FROM squad_aegis.server_player_disconnected_events
			WHERE player_suffix != '' AND (
				(length((SELECT steam_ids FROM linked_identifiers)) > 0 AND steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers))
				OR (length((SELECT eos_ids FROM linked_identifiers)) > 0 AND eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers))
			)
			UNION ALL
			-- Possess events
			SELECT player_suffix as name, event_time
			FROM squad_aegis.server_player_possess_events
			WHERE player_suffix != '' AND (
				(length((SELECT steam_ids FROM linked_identifiers)) > 0 AND player_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers))
				OR (length((SELECT eos_ids FROM linked_identifiers)) > 0 AND player_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers))
			)
			UNION ALL
			-- Deaths (as attacker - kills)
			SELECT attacker_name as name, event_time
			FROM squad_aegis.server_player_died_events
			WHERE attacker_name != '' AND (
				(length((SELECT steam_ids FROM linked_identifiers)) > 0 AND attacker_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers))
				OR (length((SELECT eos_ids FROM linked_identifiers)) > 0 AND attacker_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers))
			)
			UNION ALL
			-- Deaths (as victim)
			SELECT victim_name as name, event_time
			FROM squad_aegis.server_player_died_events
			WHERE victim_name != '' AND (
				(length((SELECT steam_ids FROM linked_identifiers)) > 0 AND victim_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers))
				OR (length((SELECT eos_ids FROM linked_identifiers)) > 0 AND victim_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers))
			)
			UNION ALL
			-- Wounded (as attacker)
			SELECT attacker_name as name, event_time
			FROM squad_aegis.server_player_wounded_events
			WHERE attacker_name != '' AND (
				(length((SELECT steam_ids FROM linked_identifiers)) > 0 AND attacker_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers))
				OR (length((SELECT eos_ids FROM linked_identifiers)) > 0 AND attacker_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers))
			)
			UNION ALL
			-- Wounded (as victim)
			SELECT victim_name as name, event_time
			FROM squad_aegis.server_player_wounded_events
			WHERE victim_name != '' AND (
				(length((SELECT steam_ids FROM linked_identifiers)) > 0 AND victim_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers))
				OR (length((SELECT eos_ids FROM linked_identifiers)) > 0 AND victim_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers))
			)
			UNION ALL
			-- Revived (as reviver)
			SELECT reviver_name as name, event_time
			FROM squad_aegis.server_player_revived_events
			WHERE reviver_name != '' AND (
				(length((SELECT steam_ids FROM linked_identifiers)) > 0 AND reviver_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers))
				OR (length((SELECT eos_ids FROM linked_identifiers)) > 0 AND reviver_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers))
			)
			UNION ALL
			-- Revived (as victim)
			SELECT victim_name as name, event_time
			FROM squad_aegis.server_player_revived_events
			WHERE victim_name != '' AND (
				(length((SELECT steam_ids FROM linked_identifiers)) > 0 AND victim_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers))
				OR (length((SELECT eos_ids FROM linked_identifiers)) > 0 AND victim_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers))
			)
		)
		SELECT
			name,
			min(event_time) as first_used,
			max(event_time) as last_used,
			count(DISTINCT toDate(event_time)) as session_count
		FROM all_names
		WHERE name != ''
		GROUP BY name
		ORDER BY last_used DESC
	`, seedWhereClause, seedPlayerWhereClause, seedAttackerWhereClause, seedVictimWhereClause)

	// Pass playerID for each seed source (6 sources)
	rows, err := s.Dependencies.Clickhouse.Query(
		c.Request.Context(), query,
		playerID, playerID, playerID, // join, connected, disconnected
		playerID,           // possess
		playerID, playerID, // died (attacker, victim)
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := []NameHistoryEntry{}
	for rows.Next() {
		var entry NameHistoryEntry
		if err := rows.Scan(&entry.Name, &entry.FirstUsed, &entry.LastUsed, &entry.SessionCount); err != nil {
			return nil, err
		}
		names = append(names, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return names, nil
}

// getPlayerWeaponStats retrieves weapon usage statistics
func (s *Server) getPlayerWeaponStats(ctx context.Context, identity ResolvedPlayerIdentity) ([]WeaponStat, error) {
	whereClause, args := buildClickHouseIdentifierWhereClause(
		"attacker_steam", "attacker_eos", identity.SteamIDs, identity.EOSIDs, false,
	)
	if whereClause == "1 = 0" {
		return []WeaponStat{}, nil
	}
	query := fmt.Sprintf(`/* sa.player.weapon_stats */
		SELECT weapon, count(), countIf(teamkill = 1)
		FROM squad_aegis.server_player_died_events
		WHERE (%s) AND weapon != ''
		GROUP BY weapon
		ORDER BY count() DESC, weapon
		LIMIT 20
	`, whereClause)
	rows, err := s.Dependencies.Clickhouse.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	weapons := []WeaponStat{}
	for rows.Next() {
		var stat WeaponStat
		if err := rows.Scan(&stat.Weapon, &stat.Kills, &stat.Teamkills); err != nil {
			return nil, err
		}
		weapons = append(weapons, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return weapons, nil
}

// getPlayerWeaponStatsLegacy is retained for one rollback-compatible release.
func (s *Server) getPlayerWeaponStatsLegacy(c *gin.Context, playerID string, isSteamID bool) ([]WeaponStat, error) {
	whereClause := "attacker_steam = ?"
	if !isSteamID {
		whereClause = "attacker_eos = ?"
	}

	query := fmt.Sprintf(`
		SELECT
			weapon,
			count(*) as kills,
			countIf(teamkill = 1) as teamkills
		FROM squad_aegis.server_player_died_events
		WHERE (%s) AND weapon != ''
		GROUP BY weapon
		ORDER BY kills DESC
		LIMIT 20
	`, whereClause)

	rows, err := s.Dependencies.Clickhouse.Query(c.Request.Context(), query, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	weapons := []WeaponStat{}
	for rows.Next() {
		var stat WeaponStat
		if err := rows.Scan(&stat.Weapon, &stat.Kills, &stat.Teamkills); err != nil {
			return nil, err
		}
		weapons = append(weapons, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return weapons, nil
}

// PlayerIdentifier represents a player identifier for batch lookups
type PlayerIdentifier struct {
	Value   string
	IsSteam bool
}

// lookupPlayerNamesBatchByIdentifiers looks up Steam and EOS names in one
// ClickHouse round trip. Result keys use "steam:12345" or "eos:abc123".
func (s *Server) lookupPlayerNamesBatchByIdentifiers(ctx context.Context, identifiers []PlayerIdentifier) (map[string]string, error) {
	result := make(map[string]string)
	if len(identifiers) == 0 || s.Dependencies.Clickhouse == nil {
		return result, nil
	}

	steamIDs := make([]string, 0, len(identifiers))
	eosIDs := make([]string, 0, len(identifiers))
	seen := make(map[string]struct{}, len(identifiers))
	for _, identifier := range identifiers {
		if identifier.Value == "" {
			continue
		}
		prefix := "eos:"
		if identifier.IsSteam {
			prefix = "steam:"
		}
		if _, exists := seen[prefix+identifier.Value]; exists {
			continue
		}
		seen[prefix+identifier.Value] = struct{}{}
		if identifier.IsSteam {
			steamIDs = append(steamIDs, identifier.Value)
			continue
		}
		eosIDs = append(eosIDs, identifier.Value)
	}
	if len(steamIDs) == 0 && len(eosIDs) == 0 {
		return result, nil
	}

	rows, err := s.Dependencies.Clickhouse.Query(ctx, `
		/* sa.player.names_batch */
		WITH
			CAST(? AS Array(String)) AS steam_ids,
			CAST(? AS Array(String)) AS eos_ids,
			observed_names AS (
				SELECT
					'steam' AS identifier_type,
					assumeNotNull(steam) AS identifier_value,
					player_suffix,
					event_time
				FROM squad_aegis.server_join_succeeded_events
				WHERE ifNull(steam, '') IN steam_ids AND player_suffix != ''
				UNION ALL
				SELECT
					'eos' AS identifier_type,
					assumeNotNull(eos) AS identifier_value,
					player_suffix,
					event_time
				FROM squad_aegis.server_join_succeeded_events
				WHERE ifNull(eos, '') IN eos_ids AND player_suffix != ''
			)
		SELECT identifier_type, identifier_value, argMax(player_suffix, event_time)
		FROM observed_names
		GROUP BY identifier_type, identifier_value
	`, steamIDs, eosIDs)
	if err != nil {
		return nil, fmt.Errorf("query player names batch: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var identifierType, identifierValue, playerName string
		if err := rows.Scan(&identifierType, &identifierValue, &playerName); err != nil {
			return nil, fmt.Errorf("scan player names batch: %w", err)
		}
		if playerName != "" {
			result[identifierType+":"+identifierValue] = playerName
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read player names batch: %w", err)
	}

	return result, nil
}

// calculateRiskIndicators generates risk flags based on player data
func (s *Server) calculateRiskIndicators(profile *PlayerProfile) []RiskIndicator {
	indicators := []RiskIndicator{}

	// Check for active bans
	if len(profile.ActiveBans) > 0 {
		indicators = append(indicators, RiskIndicator{
			Type:        "active_ban",
			Severity:    "critical",
			Description: fmt.Sprintf("Player has %d active ban(s)", len(profile.ActiveBans)),
		})
	}

	// Check teamkill rate
	if profile.TeamkillMetrics.TeamkillRatio > 0.1 { // More than 10% of kills are TKs
		indicators = append(indicators, RiskIndicator{
			Type:        "high_tk_rate",
			Severity:    "high",
			Description: fmt.Sprintf("High teamkill ratio: %.1f%% of kills are teamkills", profile.TeamkillMetrics.TeamkillRatio*100),
		})
	} else if profile.TeamkillMetrics.TeamkillRatio > 0.05 { // More than 5%
		indicators = append(indicators, RiskIndicator{
			Type:        "elevated_tk_rate",
			Severity:    "medium",
			Description: fmt.Sprintf("Elevated teamkill ratio: %.1f%% of kills are teamkills", profile.TeamkillMetrics.TeamkillRatio*100),
		})
	}

	// Check for recent teamkills
	if profile.TeamkillMetrics.RecentTeamkills >= 5 {
		indicators = append(indicators, RiskIndicator{
			Type:        "recent_teamkills",
			Severity:    "high",
			Description: fmt.Sprintf("%d teamkills in the last 7 days", profile.TeamkillMetrics.RecentTeamkills),
		})
	}

	// Note teamkills excluded from the rate because they happened on
	// seeding/training layers (informational, so admins know the rate is scoped)
	excludedTeamkills := profile.TeamkillMetrics.NonCompetitive.SeedingTeamkills +
		profile.TeamkillMetrics.NonCompetitive.TrainingTeamkills
	if excludedTeamkills > 0 {
		indicators = append(indicators, RiskIndicator{
			Type:        "non_competitive_tks",
			Severity:    "low",
			Description: fmt.Sprintf("%d teamkill(s) on seeding/training layers excluded from TK rate", excludedTeamkills),
		})
	}

	// Check for multiple names
	if len(profile.NameHistory) > 3 {
		indicators = append(indicators, RiskIndicator{
			Type:        "multiple_names",
			Severity:    "low",
			Description: fmt.Sprintf("Player has used %d different names", len(profile.NameHistory)),
		})
	}

	// Check for recent bans in violation history
	if profile.ViolationSummary.TotalBans > 0 {
		indicators = append(indicators, RiskIndicator{
			Type:        "prior_bans",
			Severity:    "medium",
			Description: fmt.Sprintf("Player has %d prior ban(s) on record", profile.ViolationSummary.TotalBans),
		})
	}

	// Check for high violation count
	totalViolations := profile.ViolationSummary.TotalWarns + profile.ViolationSummary.TotalKicks + profile.ViolationSummary.TotalBans
	if totalViolations >= 10 {
		indicators = append(indicators, RiskIndicator{
			Type:        "high_violations",
			Severity:    "high",
			Description: fmt.Sprintf("Player has %d total violations", totalViolations),
		})
	} else if totalViolations >= 5 {
		indicators = append(indicators, RiskIndicator{
			Type:        "multiple_violations",
			Severity:    "medium",
			Description: fmt.Sprintf("Player has %d violations", totalViolations),
		})
	}

	return indicators
}

const (
	playerChatCursorKind     = "player.chat"
	playerSessionsCursorKind = "player.sessions"
	playerCombatCursorKind   = "player.combat"
	altGroupsCursorKind      = "players.alt_groups"
)

type playerChatCursorValues struct {
	SentAt    time.Time `json:"sent_at"`
	MessageID uuid.UUID `json:"message_id"`
}

type playerSessionCursorValues struct {
	ConnectTime time.Time `json:"connect_time"`
	CursorKey   uuid.UUID `json:"cursor_key"`
}

type playerCombatCursorValues struct {
	EventTime time.Time `json:"event_time"`
	EventType string    `json:"event_type"`
	CursorKey uuid.UUID `json:"cursor_key"`
}

type altGroupCursorValues struct {
	LastActivity time.Time `json:"last_activity"`
	IP           string    `json:"ip"`
}

// PlayerChatHistoryPaginated handles GET /api/players/:playerId/chat.
func (s *Server) PlayerChatHistoryPaginated(c *gin.Context) {
	ctx := c.Request.Context()
	if c.Param("playerId") == "" {
		responses.BadRequest(c, "Player ID is required", nil)
		return
	}
	limit, ok := parseCursorLimit(c, 50, 100)
	if !ok || rejectLegacyPagination(c, "page", "offset") {
		return
	}
	identity, err := s.resolvePlayerIdentity(ctx, c.Param("playerId"))
	if err != nil {
		responses.BadRequest(c, "Invalid player ID", &gin.H{"error": err.Error()})
		return
	}
	steamIDs, err := steamIDsAsUint64(identity.SteamIDs)
	if err != nil {
		responses.BadRequest(c, "Invalid Steam ID", &gin.H{"error": err.Error()})
		return
	}
	chatType := strings.ToLower(normalizedAllFilter(c.Query("type")))
	search := strings.ToLower(strings.TrimSpace(c.Query("search")))
	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	filterHash := pagination.MustFilterHash(struct {
		CanonicalID string   `json:"canonical_id"`
		SteamIDs    []string `json:"steam_ids"`
		EOSIDs      []string `json:"eos_ids"`
		ChatType    string   `json:"chat_type"`
		Search      string   `json:"search"`
		StartDate   string   `json:"start_date"`
		EndDate     string   `json:"end_date"`
	}{identity.CanonicalID, identity.SteamIDs, identity.EOSIDs, chatType, search, startDate, endDate})
	var cursorValues *playerChatCursorValues
	if encodedCursor := c.Query("cursor"); encodedCursor != "" {
		cursorValues = &playerChatCursorValues{}
		if err := pagination.Decode(encodedCursor, playerChatCursorKind, filterHash, cursorValues); err != nil {
			responses.BadRequest(c, "Invalid cursor", &gin.H{"error": err.Error()})
			return
		}
	}
	if !identity.Found {
		responses.Success(c, "Chat history fetched successfully", &gin.H{"chat": PaginatedChatHistory{Messages: []ChatMessage{}}, "page_info": pagination.PageInfo{}})
		return
	}

	where := make([]string, 0, 6)
	args := make([]interface{}, 0, 10)
	identityPredicates := make([]string, 0, 2)
	if len(steamIDs) > 0 {
		identityPredicates = append(identityPredicates, "steam_id IN (?)")
		args = append(args, steamIDs)
	}
	if len(identity.EOSIDs) > 0 {
		identityPredicates = append(identityPredicates, "eos_id IN (?)")
		args = append(args, identity.EOSIDs)
	}
	if len(identityPredicates) == 0 {
		responses.Success(c, "Chat history fetched successfully", &gin.H{"chat": PaginatedChatHistory{Messages: []ChatMessage{}}, "page_info": pagination.PageInfo{}})
		return
	}
	where = append(where, "("+strings.Join(identityPredicates, " OR ")+")")
	if chatType != "" {
		where = append(where, "chat_type = ?")
		args = append(args, chatType)
	}
	if search != "" {
		where = append(where, "lowerUTF8(message) LIKE ?")
		args = append(args, "%"+search+"%")
	}
	if startDate != "" {
		start, err := parsePlayerHistoryDate(startDate, false)
		if err != nil {
			responses.BadRequest(c, "Invalid start date", &gin.H{"error": err.Error()})
			return
		}
		where = append(where, "sent_at >= ?")
		args = append(args, start)
	}
	if endDate != "" {
		end, err := parsePlayerHistoryDate(endDate, true)
		if err != nil {
			responses.BadRequest(c, "Invalid end date", &gin.H{"error": err.Error()})
			return
		}
		where = append(where, "sent_at < ?")
		args = append(args, end)
	}
	if cursorValues != nil {
		where = append(where, "(sent_at, message_id) < (?, ?)")
		args = append(args, cursorValues.SentAt, cursorValues.MessageID)
	}
	args = append(args, limit+1)
	query := fmt.Sprintf(`
		/* sa.player.chat */
		SELECT
			toString(message_id),
			sent_at,
			message,
			chat_type,
			server_id
		FROM squad_aegis.server_player_chat_messages
		WHERE %s
		ORDER BY sent_at DESC, message_id DESC
		LIMIT ?
	`, strings.Join(where, " AND "))

	rows, err := s.Dependencies.Clickhouse.Query(ctx, query, args...)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	defer rows.Close()

	messages := make([]ChatMessage, 0, limit+1)
	for rows.Next() {
		var msg ChatMessage
		if err := rows.Scan(&msg.MessageID, &msg.SentAt, &msg.Message, &msg.ChatType, &msg.ServerID); err != nil {
			responses.InternalServerError(c, err, nil)
			return
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	messages, hasMore := pagination.TrimPage(messages, limit)
	serverIDs := make([]string, 0, len(messages))
	for i := range messages {
		serverIDs = append(serverIDs, messages[i].ServerID)
	}
	serverNames, err := s.loadServerNamesBatch(ctx, serverIDs)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	for i := range messages {
		messages[i].ServerName = serverNames[messages[i].ServerID]
	}
	pageInfo := pagination.PageInfo{HasMore: hasMore}
	if hasMore && len(messages) > 0 {
		last := messages[len(messages)-1]
		lastID, err := uuid.Parse(last.MessageID)
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("invalid chat message ID: %w", err), nil)
			return
		}
		encoded, err := pagination.Encode(playerChatCursorKind, filterHash, playerChatCursorValues{SentAt: last.SentAt, MessageID: lastID})
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("encode chat cursor: %w", err), nil)
			return
		}
		pageInfo.NextCursor = &encoded
	}

	responses.Success(c, "Chat history fetched successfully", &gin.H{
		"chat":      PaginatedChatHistory{Messages: messages},
		"page_info": pageInfo,
	})
}

func parsePlayerHistoryDate(raw string, endExclusive bool) (time.Time, error) {
	if value, err := time.Parse(time.RFC3339, raw); err == nil {
		return value, nil
	}
	value, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, err
	}
	if endExclusive {
		value = value.AddDate(0, 0, 1)
	}
	return value, nil
}

// PlayerTeamkillsAnalysis handles GET /api/players/:playerId/teamkills - teamkill analysis
func (s *Server) PlayerTeamkillsAnalysis(c *gin.Context) {
	playerID := c.Param("playerId")
	if playerID == "" {
		responses.BadRequest(c, "Player ID is required", nil)
		return
	}

	playerID, isSteamID := normalizePlayerIdentifier(playerID)

	whereClause := "attacker_steam = ?"
	if !isSteamID {
		whereClause = "attacker_eos = ?"
	}

	// Get teamkill victims
	victimsQuery := fmt.Sprintf(`
		SELECT
			victim_name,
			victim_steam,
			victim_eos,
			count(*) as tk_count,
			groupArray(weapon) as weapons_used,
			min(event_time) as first_tk,
			max(event_time) as last_tk
		FROM squad_aegis.server_player_died_events
		WHERE teamkill = 1 AND (%s)
		GROUP BY victim_name, victim_steam, victim_eos
		ORDER BY tk_count DESC
		LIMIT 20
	`, whereClause)

	rows, err := s.Dependencies.Clickhouse.Query(c.Request.Context(), victimsQuery, playerID)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	defer rows.Close()

	victims := []TeamkillVictim{}
	for rows.Next() {
		var victim TeamkillVictim
		err := rows.Scan(
			&victim.VictimName,
			&victim.VictimSteam,
			&victim.VictimEOS,
			&victim.TKCount,
			&victim.WeaponsUsed,
			&victim.FirstTK,
			&victim.LastTK,
		)
		if err != nil {
			continue
		}
		victims = append(victims, victim)
	}

	// Get TK weapon breakdown
	weaponsQuery := fmt.Sprintf(`
		SELECT
			weapon,
			count(*) as tk_count
		FROM squad_aegis.server_player_died_events
		WHERE teamkill = 1 AND (%s) AND weapon != ''
		GROUP BY weapon
		ORDER BY tk_count DESC
		LIMIT 10
	`, whereClause)

	weaponRows, err := s.Dependencies.Clickhouse.Query(c.Request.Context(), weaponsQuery, playerID)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	defer weaponRows.Close()

	tkWeapons := []struct {
		Weapon  string `json:"weapon"`
		TKCount int64  `json:"tk_count"`
	}{}
	for weaponRows.Next() {
		var w struct {
			Weapon  string `json:"weapon"`
			TKCount int64  `json:"tk_count"`
		}
		if err := weaponRows.Scan(&w.Weapon, &w.TKCount); err == nil {
			tkWeapons = append(tkWeapons, w)
		}
	}

	responses.Success(c, "Teamkill analysis fetched successfully", &gin.H{
		"victims":    victims,
		"tk_weapons": tkWeapons,
	})
}

func playerSessionsBaseQuery(identityWhere string) string {
	return fmt.Sprintf(`
		/* sa.player.sessions */
		WITH events AS (
			SELECT event_time, server_id, ip, 'connected' AS event_type, cursor_key
			FROM squad_aegis.server_player_connected_events
			WHERE %s
			UNION ALL
			SELECT event_time, server_id, ip, 'disconnected' AS event_type, cursor_key
			FROM squad_aegis.server_player_disconnected_events
			WHERE %s
		), sequenced AS (
			SELECT
				event_time,
				server_id,
				ip,
				event_type,
				cursor_key,
				leadInFrame(event_type, 1, '') OVER session_window AS next_event_type,
				leadInFrame(event_time, 1, toDateTime64(0, 3, 'UTC')) OVER session_window AS next_event_time
			FROM events
			WINDOW session_window AS (
				PARTITION BY server_id
				ORDER BY event_time ASC, cursor_key ASC
				ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING
			)
		), sessions AS (
			SELECT
				event_time AS connect_time,
				if(next_event_type = 'disconnected', toNullable(next_event_time), CAST(NULL, 'Nullable(DateTime64(3, \'UTC\'))')) AS disconnect_time,
				server_id,
				ip,
				cursor_key
			FROM sequenced
			WHERE event_type = 'connected'
		)
		SELECT connect_time, disconnect_time, toString(server_id), ip, toString(cursor_key)
		FROM sessions
	`, identityWhere, identityWhere)
}

func classifyPlayerSession(session *SessionHistoryEntry, disconnectTime *time.Time, now time.Time) {
	if disconnectTime != nil {
		session.DisconnectTime = disconnectTime
		duration := int64(disconnectTime.Sub(session.ConnectTime).Seconds())
		session.DurationSeconds = &duration
		return
	}
	if now.Sub(session.ConnectTime) < time.Hour {
		session.Ongoing = true
	} else {
		session.MissingDisconnect = true
	}
}

// PlayerSessionHistory handles GET /api/players/:playerId/sessions - session history
func (s *Server) PlayerSessionHistory(c *gin.Context) {
	ctx := c.Request.Context()
	if c.Param("playerId") == "" {
		responses.BadRequest(c, "Player ID is required", nil)
		return
	}

	// Check if user has permission to view IPs
	canViewIPs := false
	if user := s.getUserFromSession(c); user != nil && user.SuperAdmin {
		canViewIPs = true
	}

	limit, ok := parseCursorLimit(c, 50, 100)
	if !ok || rejectLegacyPagination(c, "page", "offset") {
		return
	}
	identity, err := s.resolvePlayerIdentity(ctx, c.Param("playerId"))
	if err != nil {
		responses.BadRequest(c, "Invalid player ID", &gin.H{"error": err.Error()})
		return
	}
	filterHash := pagination.MustFilterHash(struct {
		CanonicalID string   `json:"canonical_id"`
		SteamIDs    []string `json:"steam_ids"`
		EOSIDs      []string `json:"eos_ids"`
		EpicIDs     []string `json:"epic_ids"`
	}{identity.CanonicalID, identity.SteamIDs, identity.EOSIDs, identity.EpicIDs})
	var cursorValues *playerSessionCursorValues
	if encodedCursor := c.Query("cursor"); encodedCursor != "" {
		cursorValues = &playerSessionCursorValues{}
		if err := pagination.Decode(encodedCursor, playerSessionsCursorKind, filterHash, cursorValues); err != nil {
			responses.BadRequest(c, "Invalid cursor", &gin.H{"error": err.Error()})
			return
		}
	}
	if !identity.Found {
		responses.Success(c, "Session history fetched successfully", &gin.H{"sessions": []SessionHistoryEntry{}, "can_view_ip": canViewIPs, "page_info": pagination.PageInfo{}})
		return
	}
	identityPredicates := make([]string, 0, 3)
	identityArgs := make([]interface{}, 0, 3)
	if len(identity.SteamIDs) > 0 {
		identityPredicates = append(identityPredicates, "steam IN (?)")
		identityArgs = append(identityArgs, identity.SteamIDs)
	}
	if len(identity.EOSIDs) > 0 {
		identityPredicates = append(identityPredicates, "eos IN (?)")
		identityArgs = append(identityArgs, identity.EOSIDs)
	}
	if len(identity.EpicIDs) > 0 {
		identityPredicates = append(identityPredicates, "epic IN (?)")
		identityArgs = append(identityArgs, identity.EpicIDs)
	}
	if len(identityPredicates) == 0 {
		responses.Success(c, "Session history fetched successfully", &gin.H{"sessions": []SessionHistoryEntry{}, "can_view_ip": canViewIPs, "page_info": pagination.PageInfo{}})
		return
	}
	identityWhere := "(" + strings.Join(identityPredicates, " OR ") + ")"
	query := playerSessionsBaseQuery(identityWhere)
	args := append([]interface{}{}, identityArgs...)
	args = append(args, identityArgs...)
	if cursorValues != nil {
		query += " WHERE (connect_time, cursor_key) < (?, ?)"
		args = append(args, cursorValues.ConnectTime, cursorValues.CursorKey)
	}
	query += " ORDER BY connect_time DESC, cursor_key DESC LIMIT ?"
	args = append(args, limit+1)

	rows, err := s.Dependencies.Clickhouse.Query(ctx, query, args...)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	defer rows.Close()

	sessions := make([]SessionHistoryEntry, 0, limit+1)
	for rows.Next() {
		var session SessionHistoryEntry
		var disconnectTime *time.Time
		var ip string
		if err := rows.Scan(&session.ConnectTime, &disconnectTime, &session.ServerID, &ip, &session.CursorKey); err != nil {
			responses.InternalServerError(c, err, nil)
			return
		}
		if canViewIPs {
			session.IP = ip
		}
		classifyPlayerSession(&session, disconnectTime, time.Now())

		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	sessions, hasMore := pagination.TrimPage(sessions, limit)
	serverIDs := make([]string, 0, len(sessions))
	for i := range sessions {
		serverIDs = append(serverIDs, sessions[i].ServerID)
	}
	serverNames, err := s.loadServerNamesBatch(ctx, serverIDs)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	for i := range sessions {
		sessions[i].ServerName = serverNames[sessions[i].ServerID]
	}
	pageInfo := pagination.PageInfo{HasMore: hasMore}
	if hasMore && len(sessions) > 0 {
		last := sessions[len(sessions)-1]
		lastKey, err := uuid.Parse(last.CursorKey)
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("invalid session cursor key: %w", err), nil)
			return
		}
		encoded, err := pagination.Encode(playerSessionsCursorKind, filterHash, playerSessionCursorValues{ConnectTime: last.ConnectTime, CursorKey: lastKey})
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("encode session cursor: %w", err), nil)
			return
		}
		pageInfo.NextCursor = &encoded
	}

	responses.Success(c, "Session history fetched successfully", &gin.H{
		"sessions":    sessions,
		"can_view_ip": canViewIPs,
		"page_info":   pageInfo,
	})
}

// PlayerRelatedPlayers handles GET /api/players/:playerId/related - related players (same IP)
func (s *Server) PlayerRelatedPlayers(c *gin.Context) {
	ctx := c.Request.Context()
	playerID := c.Param("playerId")
	if playerID == "" {
		responses.BadRequest(c, "Player ID is required", nil)
		return
	}

	// Only super admins can view related players (IP-based)
	user := s.getUserFromSession(c)
	if user == nil || !user.SuperAdmin {
		responses.Forbidden(c, "Permission denied", nil)
		return
	}

	identity, err := s.resolvePlayerIdentity(ctx, playerID)
	if err != nil {
		responses.BadRequest(c, "Invalid player ID", &gin.H{"error": err.Error()})
		return
	}
	if !identity.Found {
		responses.Success(c, "Related players fetched successfully", &gin.H{"related_players": []RelatedPlayer{}})
		return
	}

	query := `/* sa.player.related */
		WITH
			CAST(? AS Array(String)) AS target_steam_ids,
			CAST(? AS Array(String)) AS target_eos_ids,
			CAST(? AS Array(String)) AS target_epic_ids,
		player_ips AS (
			SELECT DISTINCT ip
			FROM squad_aegis.server_player_connected_events
			WHERE ip != '' AND (
				ifNull(steam, '') IN target_steam_ids
				OR ifNull(eos, '') IN target_eos_ids
				OR ifNull(epic, '') IN target_epic_ids
			)
		),
		related_connections AS (
			SELECT
				steam,
				eos,
				if(ifNull(steam, '') != '', 'steam', 'eos') AS identifier_type,
				if(ifNull(steam, '') != '', assumeNotNull(steam), assumeNotNull(eos)) AS identifier_value,
				count(*) as shared_sessions
			FROM squad_aegis.server_player_connected_events
			WHERE ip IN (SELECT ip FROM player_ips)
				AND NOT (
					ifNull(steam, '') IN target_steam_ids
					OR ifNull(eos, '') IN target_eos_ids
					OR ifNull(epic, '') IN target_epic_ids
				)
				AND (steam != '' OR eos != '')
			GROUP BY steam, eos
		),
		player_names AS (
			SELECT
				identifier_type,
				identifier_value,
				argMax(player_suffix, event_time) AS player_name
			FROM (
				SELECT 'steam' AS identifier_type, assumeNotNull(steam) AS identifier_value, player_suffix, event_time
				FROM squad_aegis.server_join_succeeded_events
				WHERE ifNull(steam, '') != ''
				UNION ALL
				SELECT 'eos' AS identifier_type, assumeNotNull(eos) AS identifier_value, player_suffix, event_time
				FROM squad_aegis.server_join_succeeded_events
				WHERE ifNull(eos, '') != ''
			)
			GROUP BY identifier_type, identifier_value
		)
		SELECT
			rc.steam,
			rc.eos,
			COALESCE(pn.player_name, '') as player_name,
			rc.shared_sessions
		FROM related_connections rc
		LEFT JOIN player_names pn
		  ON rc.identifier_type = pn.identifier_type
		 AND rc.identifier_value = pn.identifier_value
		ORDER BY rc.shared_sessions DESC
		LIMIT 20
	`

	rows, err := s.Dependencies.Clickhouse.Query(ctx, query, identity.SteamIDs, identity.EOSIDs, identity.EpicIDs)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	defer rows.Close()

	related := []RelatedPlayer{}
	identifiers := []PlayerIdentifier{}
	for rows.Next() {
		var player RelatedPlayer
		if err := rows.Scan(&player.SteamID, &player.EOSID, &player.PlayerName, &player.SharedSessions); err != nil {
			responses.InternalServerError(c, err, nil)
			return
		}
		player.RelationType = "same_ip"
		if player.SteamID != "" {
			identifiers = append(identifiers, PlayerIdentifier{Value: player.SteamID, IsSteam: true})
		}
		if player.EOSID != "" {
			identifiers = append(identifiers, PlayerIdentifier{Value: player.EOSID})
		}
		related = append(related, player)
	}
	if err := rows.Err(); err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	banned, err := s.loadActiveBanIdentifiers(ctx, identifiers)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	for idx := range related {
		related[idx].IsBanned = banned["steam:"+related[idx].SteamID] || banned["eos:"+utils.NormalizeEOSID(related[idx].EOSID)]
	}

	responses.Success(c, "Related players fetched successfully", &gin.H{
		"related_players": related,
	})
}

// PlayerCombatHistory handles GET /api/players/:playerId/combat - combat history (kills and deaths)
func (s *Server) PlayerCombatHistory(c *gin.Context) {
	ctx := c.Request.Context()
	if c.Param("playerId") == "" {
		responses.BadRequest(c, "Player ID is required", nil)
		return
	}
	limit, ok := parseCursorLimit(c, 50, 100)
	if !ok || rejectLegacyPagination(c, "page", "offset") {
		return
	}
	identity, err := s.resolvePlayerIdentity(ctx, c.Param("playerId"))
	if err != nil {
		responses.BadRequest(c, "Invalid player ID", &gin.H{"error": err.Error()})
		return
	}
	eventTypeFilter := strings.ToLower(normalizedAllFilter(c.Query("type")))
	validTypes := map[string]bool{"": true, "kill": true, "death": true, "wounded": true, "wounded_by": true, "damaged": true, "damaged_by": true}
	if !validTypes[eventTypeFilter] {
		responses.BadRequest(c, "Invalid combat event type", nil)
		return
	}
	filterHash := pagination.MustFilterHash(struct {
		CanonicalID string   `json:"canonical_id"`
		SteamIDs    []string `json:"steam_ids"`
		EOSIDs      []string `json:"eos_ids"`
		EventType   string   `json:"event_type"`
	}{identity.CanonicalID, identity.SteamIDs, identity.EOSIDs, eventTypeFilter})
	var cursorValues *playerCombatCursorValues
	if encodedCursor := c.Query("cursor"); encodedCursor != "" {
		cursorValues = &playerCombatCursorValues{}
		if err := pagination.Decode(encodedCursor, playerCombatCursorKind, filterHash, cursorValues); err != nil {
			responses.BadRequest(c, "Invalid cursor", &gin.H{"error": err.Error()})
			return
		}
	}
	if !identity.Found || (len(identity.SteamIDs) == 0 && len(identity.EOSIDs) == 0) {
		responses.Success(c, "Combat history fetched successfully", &gin.H{"events": []CombatHistoryEntry{}, "page_info": pagination.PageInfo{}})
		return
	}

	type combatBranch struct {
		eventType string
		table     string
		actor     string
		other     string
	}
	branches := []combatBranch{
		{"kill", "server_player_died_events", "attacker", "victim"},
		{"death", "server_player_died_events", "victim", "attacker"},
		{"wounded", "server_player_wounded_events", "attacker", "victim"},
		{"wounded_by", "server_player_wounded_events", "victim", "attacker"},
		{"damaged", "server_player_damaged_events", "attacker", "victim"},
		{"damaged_by", "server_player_damaged_events", "victim", "attacker"},
	}
	queryParts := make([]string, 0, len(branches))
	args := make([]interface{}, 0, len(branches)*3+4)
	for _, branch := range branches {
		if eventTypeFilter != "" && branch.eventType != eventTypeFilter {
			continue
		}
		identityParts := make([]string, 0, 2)
		if len(identity.SteamIDs) > 0 {
			identityParts = append(identityParts, branch.actor+"_steam IN (?)")
			args = append(args, identity.SteamIDs)
		}
		if len(identity.EOSIDs) > 0 {
			identityParts = append(identityParts, branch.actor+"_eos IN (?)")
			args = append(args, identity.EOSIDs)
		}
		where := "(" + strings.Join(identityParts, " OR ") + ")"
		if cursorValues != nil {
			where += " AND event_time <= ?"
			args = append(args, cursorValues.EventTime)
		}
		queryParts = append(queryParts, fmt.Sprintf(`
			SELECT
				toString(cursor_key) AS cursor_key,
				chain_id,
				event_time,
				'%s' AS event_type,
				toString(server_id) AS server_id,
				toString(weapon) AS weapon,
				damage,
				teamkill,
				%s_name AS other_name,
				ifNull(%s_steam, '') AS other_steam_id,
				ifNull(%s_eos, '') AS other_eos_id,
				ifNull(%s_team, '') AS other_team,
				ifNull(%s_squad, '') AS other_squad,
				ifNull(%s_team, '') AS player_team,
				ifNull(%s_squad, '') AS player_squad
			FROM squad_aegis.%s
			WHERE %s`, branch.eventType, branch.other, branch.other, branch.other, branch.other, branch.other, branch.actor, branch.actor, branch.table, where))
	}
	query := "/* sa.player.combat */ SELECT * FROM (" + strings.Join(queryParts, " UNION ALL ") + ")"
	if cursorValues != nil {
		query += " WHERE (event_time, event_type, cursor_key) < (?, ?, ?)"
		args = append(args, cursorValues.EventTime, cursorValues.EventType, cursorValues.CursorKey)
	}
	query += " ORDER BY event_time DESC, event_type DESC, cursor_key DESC LIMIT ?"
	args = append(args, limit+1)

	rows, err := s.Dependencies.Clickhouse.Query(ctx, query, args...)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	defer rows.Close()

	events := make([]CombatHistoryEntry, 0, limit+1)
	missingIdentifiers := []PlayerIdentifier{}
	identifierSet := make(map[string]bool)

	// First pass: scan all events and collect missing player identifiers
	for rows.Next() {
		var entry CombatHistoryEntry
		var teamkillInt uint8
		err := rows.Scan(
			&entry.CursorKey,
			&entry.ChainID,
			&entry.EventTime,
			&entry.EventType,
			&entry.ServerID,
			&entry.Weapon,
			&entry.Damage,
			&teamkillInt,
			&entry.OtherName,
			&entry.OtherSteamID,
			&entry.OtherEOSID,
			&entry.OtherTeam,
			&entry.OtherSquad,
			&entry.PlayerTeam,
			&entry.PlayerSquad,
		)
		if err != nil {
			responses.InternalServerError(c, err, nil)
			return
		}
		entry.EventID = entry.CursorKey
		entry.Teamkill = teamkillInt == 1

		// Collect missing player identifiers for batch lookup
		if entry.OtherName == "" && (entry.OtherSteamID != "" || entry.OtherEOSID != "") {
			if entry.OtherSteamID != "" {
				key := "steam:" + entry.OtherSteamID
				if !identifierSet[key] {
					identifierSet[key] = true
					missingIdentifiers = append(missingIdentifiers, PlayerIdentifier{Value: entry.OtherSteamID, IsSteam: true})
				}
			} else {
				key := "eos:" + entry.OtherEOSID
				if !identifierSet[key] {
					identifierSet[key] = true
					missingIdentifiers = append(missingIdentifiers, PlayerIdentifier{Value: entry.OtherEOSID, IsSteam: false})
				}
			}
		}

		events = append(events, entry)
	}
	if err := rows.Err(); err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	events, hasMore := pagination.TrimPage(events, limit)

	// Batch lookup missing player names
	playerNameMap, err := s.lookupPlayerNamesBatchByIdentifiers(ctx, missingIdentifiers)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	serverIDs := make([]string, 0, len(events))
	for i := range events {
		serverIDs = append(serverIDs, events[i].ServerID)
	}
	serverNames, err := s.loadServerNamesBatch(ctx, serverIDs)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	// Second pass: fill in missing player names from batch lookup
	for i := range events {
		events[i].ServerName = serverNames[events[i].ServerID]
		if events[i].OtherName == "" && (events[i].OtherSteamID != "" || events[i].OtherEOSID != "") {
			var key string
			if events[i].OtherSteamID != "" {
				key = "steam:" + events[i].OtherSteamID
			} else {
				key = "eos:" + events[i].OtherEOSID
			}
			if name, ok := playerNameMap[key]; ok {
				events[i].OtherName = name
			}
		}
	}
	pageInfo := pagination.PageInfo{HasMore: hasMore}
	if hasMore && len(events) > 0 {
		last := events[len(events)-1]
		lastKey, err := uuid.Parse(last.CursorKey)
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("invalid combat cursor key: %w", err), nil)
			return
		}
		encoded, err := pagination.Encode(playerCombatCursorKind, filterHash, playerCombatCursorValues{EventTime: last.EventTime, EventType: last.EventType, CursorKey: lastKey})
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("encode combat cursor: %w", err), nil)
			return
		}
		pageInfo.NextCursor = &encoded
	}

	responses.Success(c, "Combat history fetched successfully", &gin.H{
		"events":    events,
		"page_info": pageInfo,
	})
}

// PlayersAltGroups handles GET /api/players/alt-groups - list all suspected alt account groups
func (s *Server) PlayersAltGroups(c *gin.Context) {
	// Only super admins can view alt account groups (IP-based)
	user := s.getUserFromSession(c)
	if user == nil || !user.SuperAdmin {
		responses.Forbidden(c, "Permission denied", nil)
		return
	}

	ctx := c.Request.Context()
	limit, ok := parseCursorLimit(c, 20, 50)
	if !ok || rejectLegacyPagination(c, "page", "offset") {
		return
	}
	filterHash := pagination.MustFilterHash(struct {
		Scope string `json:"scope"`
	}{Scope: "all"})
	var cursorValues *altGroupCursorValues
	if encodedCursor := c.Query("cursor"); encodedCursor != "" {
		cursorValues = &altGroupCursorValues{}
		if err := pagination.Decode(encodedCursor, altGroupsCursorKind, filterHash, cursorValues); err != nil {
			responses.BadRequest(c, "Invalid cursor", &gin.H{"error": err.Error()})
			return
		}
	}

	query := `
		/* sa.player.alt_groups */
		WITH
			(
				SELECT ifNull(argMax(backfill_complete, updated_at), 0)
				FROM squad_aegis.sql_rollup_state
				WHERE rollup_name = 'player_ip_activity'
			) = 1 AS use_rollup,
			current_identity AS (
				SELECT argMax(snapshot_id, published_at) AS snapshot_id
				FROM squad_aegis.player_identity_current
				WHERE pointer_key = 1
			),
			activity AS (
				SELECT
					ip,
					multiIf(ifNull(steam, '') != '', 'steam', ifNull(eos, '') != '', 'eos', 'epic') AS identifier_type,
					multiIf(ifNull(steam, '') != '', assumeNotNull(steam), ifNull(eos, '') != '', assumeNotNull(eos), assumeNotNull(epic)) AS identifier_value,
					count() AS sessions,
					max(event_time) AS last_seen
				FROM squad_aegis.server_player_connected_events
				WHERE NOT use_rollup AND ip != ''
				  AND (ifNull(steam, '') != '' OR ifNull(eos, '') != '' OR ifNull(epic, '') != '')
				GROUP BY ip, identifier_type, identifier_value
				UNION ALL
				SELECT ip, identifier_type, identifier_value, sum(sessions), max(last_seen)
				FROM squad_aegis.player_ip_activity
				WHERE use_rollup
				GROUP BY ip, identifier_type, identifier_value
			),
			eligible_ips AS (
				SELECT ip, max(last_seen) AS last_activity
				FROM activity
				GROUP BY ip
				HAVING uniqExact(tuple(identifier_type, identifier_value)) > 1
			),
			page_ips AS (
				SELECT ip, last_activity
				FROM eligible_ips`
	args := make([]interface{}, 0, 3)
	if cursorValues != nil {
		query += " WHERE (last_activity, ip) < (?, ?)"
		args = append(args, cursorValues.LastActivity, cursorValues.IP)
	}
	query += `
				ORDER BY last_activity DESC, ip DESC
				LIMIT ?
			)
		SELECT
			p.ip,
			p.last_activity,
			a.identifier_type,
			a.identifier_value,
			a.sessions,
			a.last_seen,
			ifNull(any(pi.primary_name), '') AS player_name
		FROM page_ips p
		INNER JOIN activity a ON a.ip = p.ip
		LEFT JOIN current_identity ci ON 1
		LEFT JOIN squad_aegis.player_identity_lookup_snapshots pil
		  ON pil.snapshot_id = ci.snapshot_id
		 AND pil.identifier_type = a.identifier_type
		 AND pil.identifier_value = a.identifier_value
		LEFT JOIN squad_aegis.player_identity_snapshots pi
		  ON pi.snapshot_id = ci.snapshot_id
		 AND pi.canonical_id = pil.canonical_id
		GROUP BY p.ip, p.last_activity, a.identifier_type, a.identifier_value, a.sessions, a.last_seen
		ORDER BY p.last_activity DESC, p.ip DESC, a.last_seen DESC, a.identifier_value ASC
	`
	args = append(args, limit+1)
	rows, err := s.Dependencies.Clickhouse.Query(ctx, query, args...)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	defer rows.Close()
	groups := make([]AltAccountGroup, 0, limit+1)
	groupIndex := make(map[string]int, limit+1)
	for rows.Next() {
		var ip, identifierType, identifierValue, playerName string
		var lastActivity, lastSeen time.Time
		var sessions uint64
		if err := rows.Scan(&ip, &lastActivity, &identifierType, &identifierValue, &sessions, &lastSeen, &playerName); err != nil {
			responses.InternalServerError(c, err, nil)
			return
		}
		idx, exists := groupIndex[ip]
		if !exists {
			idx = len(groups)
			groupIndex[ip] = idx
			activity := lastActivity
			groups = append(groups, AltAccountGroup{GroupID: ip, SharedIPCount: 1, LastActivity: &activity, Players: []AltAccountPlayer{}})
		}
		player := AltAccountPlayer{PlayerName: playerName, SharedSessions: int64(sessions), LastSeen: &lastSeen}
		if identifierType == "steam" {
			player.SteamID = identifierValue
		} else if identifierType == "eos" {
			player.EOSID = identifierValue
		}
		if player.PlayerName == "" {
			player.PlayerName = identifierValue
		}
		groups[idx].Players = append(groups[idx].Players, player)
	}
	if err := rows.Err(); err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	groups, hasMore := pagination.TrimPage(groups, limit)
	banned, err := s.loadActiveBanIdentifiersBatch(ctx, groups)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	for groupIdx := range groups {
		for playerIdx := range groups[groupIdx].Players {
			player := &groups[groupIdx].Players[playerIdx]
			player.IsBanned = banned["steam:"+player.SteamID] || banned["eos:"+player.EOSID]
		}
	}
	pageInfo := pagination.PageInfo{HasMore: hasMore}
	if hasMore && len(groups) > 0 {
		last := groups[len(groups)-1]
		encoded, err := pagination.Encode(altGroupsCursorKind, filterHash, altGroupCursorValues{LastActivity: *last.LastActivity, IP: last.GroupID})
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("encode alt group cursor: %w", err), nil)
			return
		}
		pageInfo.NextCursor = &encoded
	}
	responses.Success(c, "Alt account groups fetched successfully", &gin.H{"alt_groups": groups, "page_info": pageInfo})
}

func (s *Server) loadActiveBanIdentifiersBatch(ctx context.Context, groups []AltAccountGroup) (map[string]bool, error) {
	identifiers := make([]PlayerIdentifier, 0)
	for _, group := range groups {
		for _, player := range group.Players {
			if player.SteamID != "" {
				identifiers = append(identifiers, PlayerIdentifier{Value: player.SteamID, IsSteam: true})
			}
			if player.EOSID != "" {
				identifiers = append(identifiers, PlayerIdentifier{Value: player.EOSID})
			}
		}
	}
	return s.loadActiveBanIdentifiers(ctx, identifiers)
}

func (s *Server) loadActiveBanIdentifiers(ctx context.Context, identifiers []PlayerIdentifier) (map[string]bool, error) {
	steamIDs := make([]int64, 0, len(identifiers))
	eosIDs := make([]string, 0, len(identifiers))
	seen := make(map[string]struct{}, len(identifiers))
	for _, identifier := range identifiers {
		if identifier.IsSteam {
			value, err := strconv.ParseInt(identifier.Value, 10, 64)
			if err != nil {
				continue
			}
			key := "steam:" + strconv.FormatInt(value, 10)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			steamIDs = append(steamIDs, value)
			continue
		}
		normalized := utils.NormalizeEOSID(identifier.Value)
		if normalized == "" {
			continue
		}
		key := "eos:" + normalized
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		eosIDs = append(eosIDs, normalized)
	}
	result := make(map[string]bool)
	if len(steamIDs) == 0 && len(eosIDs) == 0 {
		return result, nil
	}
	rows, err := s.Dependencies.DB.QueryContext(ctx, `
		/* sa.player.active_bans */
		SELECT steam_id, eos_id
		FROM server_bans
		WHERE (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
		  AND (steam_id = ANY($1::bigint[]) OR eos_id = ANY($2::text[]))
	`, pq.Array(steamIDs), pq.Array(eosIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var steamID sql.NullInt64
		var eosID sql.NullString
		if err := rows.Scan(&steamID, &eosID); err != nil {
			return nil, err
		}
		if steamID.Valid {
			result["steam:"+strconv.FormatInt(steamID.Int64, 10)] = true
		}
		if eosID.Valid {
			result["eos:"+utils.NormalizeEOSID(eosID.String)] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
