package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// PlayerProfile represents a player's comprehensive profile
type PlayerProfile struct {
	SteamID        string             `json:"steam_id"`
	EOSID          string             `json:"eos_id"`
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
	TotalWarns  int64      `json:"total_warns"`
	TotalKicks  int64      `json:"total_kicks"`
	TotalBans   int64      `json:"total_bans"`
	LastAction  *time.Time `json:"last_action,omitempty"`
}

// TeamkillMetrics represents detailed teamkill statistics
type TeamkillMetrics struct {
	TotalTeamkills      int64   `json:"total_teamkills"`
	TeamkillsPerSession float64 `json:"teamkills_per_session"`
	TeamkillRatio       float64 `json:"teamkill_ratio"`       // TKs / total kills
	RecentTeamkills     int64   `json:"recent_teamkills"`     // Last 7 days
	TotalTeamWounds     int64   `json:"total_team_wounds"`    // Times downed a teammate
	TotalTeamDamage     int64   `json:"total_team_damage"`    // Times damaged a teammate
	RecentTeamWounds    int64   `json:"recent_team_wounds"`   // Team wounds in last 7 days
	RecentTeamDamage    int64   `json:"recent_team_damage"`   // Team damage in last 7 days
}

// RiskIndicator represents a risk flag for admin attention
type RiskIndicator struct {
	Type        string `json:"type"`        // "high_tk_rate", "recent_ban", "multiple_names", "cbl_flagged", "ip_shared"
	Severity    string `json:"severity"`    // "critical", "high", "medium", "low"
	Description string `json:"description"`
}

// ActiveBan represents a currently active ban on the player
type ActiveBan struct {
	BanID      string     `json:"ban_id"`
	ServerID   string     `json:"server_id"`
	ServerName string     `json:"server_name"`
	Reason     string     `json:"reason"`
	Duration   int        `json:"duration"`
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
	VictimName   string    `json:"victim_name"`
	VictimSteam  string    `json:"victim_steam"`
	VictimEOS    string    `json:"victim_eos"`
	TKCount      int64     `json:"tk_count"`
	WeaponsUsed  []string  `json:"weapons_used"`
	FirstTK      time.Time `json:"first_tk"`
	LastTK       time.Time `json:"last_tk"`
}

// SessionHistoryEntry represents a paired connection session
type SessionHistoryEntry struct {
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
	Messages   []ChatMessage `json:"messages"`
	Total      int64         `json:"total"`
	Page       int           `json:"page"`
	Limit      int           `json:"limit"`
	TotalPages int           `json:"total_pages"`
}

// PlayerSearchResult represents a simplified player profile for search results
type PlayerSearchResult struct {
	SteamID    string     `json:"steam_id"`
	EOSID      string     `json:"eos_id"`
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

	// Try searching the pre-computed identity table first
	players, err := s.searchPlayersFromIdentityTable(c.Request.Context(), searchQuery, searchPattern, limit)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to search identity table, falling back to raw events")
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
	// Check if the identity table has data
	var count uint64
	countRow := s.Dependencies.Clickhouse.QueryRow(ctx, "SELECT count() FROM squad_aegis.player_identities")
	if err := countRow.Scan(&count); err != nil || count == 0 {
		return nil, fmt.Errorf("identity table empty or not accessible")
	}

	// Search by name (in all_names array), steam ID (in all_steam_ids), or EOS ID (in all_eos_ids)
	// Use arrayExists with ilike for case-insensitive partial matching on names
	query := `
		SELECT
			primary_steam_id,
			primary_eos_id,
			primary_name,
			last_seen,
			first_seen
		FROM squad_aegis.player_identities
		WHERE
			has(all_names, ?) OR
			primary_name ILIKE ? OR
			has(all_steam_ids, ?) OR
			has(all_eos_ids, ?) OR
			arrayExists(x -> x ILIKE ?, all_names)
		ORDER BY last_seen DESC
		LIMIT ?
	`

	// For exact matches, try the lookup table first for speed
	lookupQuery := `
		SELECT pi.primary_steam_id, pi.primary_eos_id, pi.primary_name, pi.last_seen, pi.first_seen
		FROM squad_aegis.player_identity_lookup pil
		JOIN squad_aegis.player_identities pi ON pil.canonical_id = pi.canonical_id
		WHERE pil.identifier_value = ? OR pil.identifier_value ILIKE ?
		ORDER BY pi.last_seen DESC
		LIMIT ?
	`

	// Try exact lookup first
	rows, err := s.Dependencies.Clickhouse.Query(ctx, lookupQuery, searchQuery, searchPattern, limit)
	if err == nil {
		defer rows.Close()
		players := []PlayerSearchResult{}
		seenCanonical := make(map[string]bool)

		for rows.Next() {
			var player PlayerSearchResult
			var steamID, eosID string

			if err := rows.Scan(&steamID, &eosID, &player.PlayerName, &player.LastSeen, &player.FirstSeen); err != nil {
				continue
			}

			// Deduplicate by canonical identity
			key := steamID + "|" + eosID
			if seenCanonical[key] {
				continue
			}
			seenCanonical[key] = true

			player.SteamID = steamID
			player.EOSID = eosID
			players = append(players, player)
		}

		if len(players) > 0 {
			return players, nil
		}
	}

	// Fallback to broader search on the identities table
	rows, err = s.Dependencies.Clickhouse.Query(ctx, query, searchQuery, searchPattern, searchQuery, searchQuery, searchPattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	players := []PlayerSearchResult{}
	for rows.Next() {
		var player PlayerSearchResult
		var steamID, eosID string

		if err := rows.Scan(&steamID, &eosID, &player.PlayerName, &player.LastSeen, &player.FirstSeen); err != nil {
			continue
		}

		player.SteamID = steamID
		player.EOSID = eosID
		players = append(players, player)
	}

	return players, nil
}

// searchPlayersFromRawEvents searches raw event tables for players (fallback)
// Searches across multiple event tables to find players even without join events
func (s *Server) searchPlayersFromRawEvents(ctx context.Context, searchPattern string, limit int) ([]PlayerSearchResult, error) {
	query := `
		WITH all_player_records AS (
			-- Join succeeded events
			SELECT steam, eos, player_suffix as name, event_time
			FROM squad_aegis.server_join_succeeded_events
			WHERE player_suffix ILIKE ? OR steam ILIKE ? OR eos ILIKE ?
			UNION ALL
			-- Disconnected events
			SELECT steam, eos, player_suffix as name, event_time
			FROM squad_aegis.server_player_disconnected_events
			WHERE player_suffix ILIKE ? OR steam ILIKE ? OR eos ILIKE ?
			UNION ALL
			-- Possess events
			SELECT player_steam as steam, player_eos as eos, player_suffix as name, event_time
			FROM squad_aegis.server_player_possess_events
			WHERE player_suffix ILIKE ? OR player_steam ILIKE ? OR player_eos ILIKE ?
			UNION ALL
			-- Damage events (attacker)
			SELECT attacker_steam as steam, attacker_eos as eos, attacker_name as name, event_time
			FROM squad_aegis.server_player_damaged_events
			WHERE attacker_name ILIKE ? OR attacker_steam ILIKE ? OR attacker_eos ILIKE ?
			UNION ALL
			-- Damage events (victim)
			SELECT victim_steam as steam, victim_eos as eos, victim_name as name, event_time
			FROM squad_aegis.server_player_damaged_events
			WHERE victim_name ILIKE ? OR victim_steam ILIKE ? OR victim_eos ILIKE ?
			UNION ALL
			-- Died events (victim)
			SELECT victim_steam as steam, victim_eos as eos, victim_name as name, event_time
			FROM squad_aegis.server_player_died_events
			WHERE victim_name ILIKE ? OR victim_steam ILIKE ? OR victim_eos ILIKE ?
			UNION ALL
			-- Died events (attacker)
			SELECT attacker_steam as steam, attacker_eos as eos, attacker_name as name, event_time
			FROM squad_aegis.server_player_died_events
			WHERE attacker_name ILIKE ? OR attacker_steam ILIKE ? OR attacker_eos ILIKE ?
			UNION ALL
			-- Wounded events (victim)
			SELECT victim_steam as steam, victim_eos as eos, victim_name as name, event_time
			FROM squad_aegis.server_player_wounded_events
			WHERE victim_name ILIKE ? OR victim_steam ILIKE ? OR victim_eos ILIKE ?
			UNION ALL
			-- Wounded events (attacker)
			SELECT attacker_steam as steam, attacker_eos as eos, attacker_name as name, event_time
			FROM squad_aegis.server_player_wounded_events
			WHERE attacker_name ILIKE ? OR attacker_steam ILIKE ? OR attacker_eos ILIKE ?
			UNION ALL
			-- Revived events (reviver)
			SELECT reviver_steam as steam, reviver_eos as eos, reviver_name as name, event_time
			FROM squad_aegis.server_player_revived_events
			WHERE reviver_name ILIKE ? OR reviver_steam ILIKE ? OR reviver_eos ILIKE ?
			UNION ALL
			-- Revived events (victim)
			SELECT victim_steam as steam, victim_eos as eos, victim_name as name, event_time
			FROM squad_aegis.server_player_revived_events
			WHERE victim_name ILIKE ? OR victim_steam ILIKE ? OR victim_eos ILIKE ?
		),
		player_identifiers AS (
			SELECT
				steam,
				eos,
				anyIf(name, name != '') as player_name,
				max(event_time) as last_seen,
				min(event_time) as first_seen
			FROM all_player_records
			WHERE steam != '' OR eos != ''
			GROUP BY steam, eos
		)
		SELECT
			anyIf(steam, steam != '') as steam_id,
			anyIf(eos, eos != '') as eos_id,
			any(player_name) as player_name,
			max(last_seen) as last_seen,
			min(first_seen) as first_seen
		FROM player_identifiers
		GROUP BY
			multiIf(
				steam != '', steam,
				eos != '', eos,
				''
			)
		HAVING steam_id != '' OR eos_id != ''
		ORDER BY last_seen DESC
		LIMIT ?
	`

	// Each subquery in the UNION ALL needs the search pattern 3 times (name, steam, eos)
	// 11 subqueries * 3 params = 33 params, plus 1 for limit
	rows, err := s.Dependencies.Clickhouse.Query(ctx, query,
		searchPattern, searchPattern, searchPattern, // join_succeeded
		searchPattern, searchPattern, searchPattern, // disconnected
		searchPattern, searchPattern, searchPattern, // possess
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
		var steamID, eosID *string

		if err := rows.Scan(&steamID, &eosID, &player.PlayerName, &player.LastSeen, &player.FirstSeen); err != nil {
			continue
		}

		if steamID != nil {
			player.SteamID = *steamID
		}
		if eosID != nil {
			player.EOSID = *eosID
		}

		players = append(players, player)
	}

	return players, nil
}

// PlayerGet handles GET /api/players/:playerId - get player profile by steam or eos id
func (s *Server) PlayerGet(c *gin.Context) {
	playerID := c.Param("playerId")

	if playerID == "" {
		responses.BadRequest(c, "Player ID is required", nil)
		return
	}

	// Determine if it's a Steam ID (numeric) or EOS ID (alphanumeric)
	isSteamID := false
	if _, err := strconv.ParseUint(playerID, 10, 64); err == nil {
		isSteamID = true
	}

	// Get basic player info
	profile, err := s.getPlayerBasicInfo(c, playerID, isSteamID)
	if err != nil {
		responses.NotFound(c, "Player not found", &gin.H{"error": err.Error()})
		return
	}

	// Get player statistics
	statistics, err := s.getPlayerStatistics(c, playerID, isSteamID)
	if err == nil {
		profile.Statistics = *statistics
	}

	// Get recent activity
	recentActivity, err := s.getPlayerRecentActivity(c, playerID, isSteamID, 20)
	if err == nil {
		profile.RecentActivity = recentActivity
	}

	// Get chat history (last 50 messages)
	chatHistory, err := s.getPlayerChatHistory(c, playerID, isSteamID, 50)
	if err == nil {
		profile.ChatHistory = chatHistory
	}

	// Get violations
	violations, err := s.getPlayerViolations(c, playerID, isSteamID)
	if err == nil {
		profile.Violations = violations
	}

	// Get recent servers
	recentServers, err := s.getPlayerRecentServers(c, playerID, isSteamID)
	if err == nil {
		profile.RecentServers = recentServers
	}

	// Get admin-focused data
	// Active bans
	activeBans, err := s.getPlayerActiveBans(c, playerID, isSteamID)
	if err == nil {
		profile.ActiveBans = activeBans
	}

	// Violation summary
	violationSummary, err := s.getPlayerViolationSummary(c, playerID, isSteamID)
	if err == nil {
		profile.ViolationSummary = *violationSummary
	}

	// Teamkill metrics
	tkMetrics, err := s.getPlayerTeamkillMetrics(c, playerID, isSteamID, profile.TotalSessions, profile.Statistics.Kills)
	if err == nil {
		profile.TeamkillMetrics = *tkMetrics
	}

	// Name history
	nameHistory, err := s.getPlayerNameHistory(c, playerID, isSteamID)
	if err == nil {
		profile.NameHistory = nameHistory
	}

	// Weapon stats
	weaponStats, err := s.getPlayerWeaponStats(c, playerID, isSteamID)
	if err == nil {
		profile.WeaponStats = weaponStats
	}

	// Calculate risk indicators
	profile.RiskIndicators = s.calculateRiskIndicators(profile)

	responses.Success(c, "Player profile fetched successfully", &gin.H{"player": profile})
}

// PlayersStats handles GET /api/players/stats - get player statistics summary
func (s *Server) PlayersStats(c *gin.Context) {
	ctx := c.Request.Context()

	summary := PlayerStatsSummary{}

	// Get top players by K/D ratio (min 10 kills to qualify)
	// Link players by Steam ID or EOS ID - if either matches, they're the same player
	topPlayersQuery := `
		WITH player_identity AS (
			SELECT
				if(steam != '', steam, '') as steam_id,
				if(eos != '', eos, '') as eos_id,
				any(player_suffix) as player_name
			FROM squad_aegis.server_join_succeeded_events
			WHERE steam != '' OR eos != ''
			GROUP BY steam, eos
		),
		player_stats AS (
			SELECT
				attacker_steam,
				attacker_eos,
				attacker_name,
				countIf(attacker_steam != '' OR attacker_eos != '') as kills,
				0 as deaths
			FROM squad_aegis.server_player_died_events
			WHERE (attacker_steam != '' OR attacker_eos != '')
			GROUP BY attacker_steam, attacker_eos, attacker_name

			UNION ALL

			SELECT
				victim_steam as attacker_steam,
				victim_eos as attacker_eos,
				victim_name as attacker_name,
				0 as kills,
				count(*) as deaths
			FROM squad_aegis.server_player_died_events
			WHERE (victim_steam != '' OR victim_eos != '')
			GROUP BY victim_steam, victim_eos, victim_name
		),
		normalized_stats AS (
			SELECT
				if(ps.attacker_steam != '', ps.attacker_steam, pi.steam_id) as norm_steam,
				if(ps.attacker_eos != '', ps.attacker_eos, pi.eos_id) as norm_eos,
				coalesce(pi.player_name, ps.attacker_name) as player_name,
				ps.kills,
				ps.deaths
			FROM player_stats ps
			LEFT JOIN player_identity pi ON
				(ps.attacker_steam != '' AND ps.attacker_steam = pi.steam_id) OR
				(ps.attacker_eos != '' AND ps.attacker_eos = pi.eos_id)
		)
		SELECT
			any(norm_steam) as steam_id,
			any(norm_eos) as eos_id,
			any(player_name) as player_name,
			sum(kills) as total_kills,
			sum(deaths) as total_deaths,
			if(sum(deaths) > 0, sum(kills) / sum(deaths), toFloat64(sum(kills))) as kd_ratio
		FROM normalized_stats
		WHERE norm_steam != '' OR norm_eos != ''
		GROUP BY if(norm_steam != '', norm_steam, norm_eos)
		HAVING total_kills >= 10
		ORDER BY kd_ratio DESC
		LIMIT 10
	`

	rows, err := s.Dependencies.Clickhouse.Query(ctx, topPlayersQuery)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var player TopPlayerStats
			var steamID, eosID *string
			err := rows.Scan(
				&steamID,
				&eosID,
				&player.PlayerName,
				&player.Kills,
				&player.Deaths,
				&player.KDRatio,
			)
			if err == nil {
				if steamID != nil {
					player.SteamID = *steamID
				}
				if eosID != nil {
					player.EOSID = *eosID
				}

				// If name is empty, try to fetch it using getPlayerBasicInfo
				if player.PlayerName == "" {
					var playerID string
					isSteamID := false
					if player.SteamID != "" {
						playerID = player.SteamID
						isSteamID = true
					} else if player.EOSID != "" {
						playerID = player.EOSID
						isSteamID = false
					}

					if playerID != "" {
						if profile, err := s.getPlayerBasicInfo(c, playerID, isSteamID); err == nil && profile != nil {
							player.PlayerName = profile.PlayerName
						}
					}
				}

				summary.TopPlayers = append(summary.TopPlayers, player)
			}
		}
	}

	// Get top teamkillers
	topTeamkillersQuery := `
		WITH player_identity AS (
			SELECT
				if(steam != '', steam, '') as steam_id,
				if(eos != '', eos, '') as eos_id,
				any(player_suffix) as player_name
			FROM squad_aegis.server_join_succeeded_events
			WHERE steam != '' OR eos != ''
			GROUP BY steam, eos
		),
		teamkill_stats AS (
			SELECT
				attacker_steam,
				attacker_eos,
				attacker_name,
				count(*) as teamkills
			FROM squad_aegis.server_player_died_events
			WHERE teamkill = 1 AND (attacker_steam != '' OR attacker_eos != '')
			GROUP BY attacker_steam, attacker_eos, attacker_name
		),
		normalized_stats AS (
			SELECT
				if(ts.attacker_steam != '', ts.attacker_steam, pi.steam_id) as norm_steam,
				if(ts.attacker_eos != '', ts.attacker_eos, pi.eos_id) as norm_eos,
				coalesce(pi.player_name, ts.attacker_name) as player_name,
				ts.teamkills
			FROM teamkill_stats ts
			LEFT JOIN player_identity pi ON
				(ts.attacker_steam != '' AND ts.attacker_steam = pi.steam_id) OR
				(ts.attacker_eos != '' AND ts.attacker_eos = pi.eos_id)
		)
		SELECT
			any(norm_steam) as steam_id,
			any(norm_eos) as eos_id,
			any(player_name) as player_name,
			sum(teamkills) as teamkills,
			0 as kills,
			0 as deaths
		FROM normalized_stats
		WHERE norm_steam != '' OR norm_eos != ''
		GROUP BY if(norm_steam != '', norm_steam, norm_eos)
		HAVING teamkills > 0
		ORDER BY teamkills DESC
		LIMIT 10
	`

	rows, err = s.Dependencies.Clickhouse.Query(ctx, topTeamkillersQuery)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var player TopPlayerStats
			var steamID, eosID *string
			err := rows.Scan(
				&steamID,
				&eosID,
				&player.PlayerName,
				&player.Teamkills,
				&player.Kills,
				&player.Deaths,
			)
			if err == nil {
				if steamID != nil {
					player.SteamID = *steamID
				}
				if eosID != nil {
					player.EOSID = *eosID
				}

				// If name is empty, try to fetch it using getPlayerBasicInfo
				if player.PlayerName == "" {
					var playerID string
					isSteamID := false
					if player.SteamID != "" {
						playerID = player.SteamID
						isSteamID = true
					} else if player.EOSID != "" {
						playerID = player.EOSID
						isSteamID = false
					}

					if playerID != "" {
						if profile, err := s.getPlayerBasicInfo(c, playerID, isSteamID); err == nil && profile != nil {
							player.PlayerName = profile.PlayerName
						}
					}
				}

				summary.TopTeamkillers = append(summary.TopTeamkillers, player)
			}
		}
	}

	// Get top medics (by revives)
	topMedicsQuery := `
		WITH player_identity AS (
			SELECT
				if(steam != '', steam, '') as steam_id,
				if(eos != '', eos, '') as eos_id,
				any(player_suffix) as player_name
			FROM squad_aegis.server_join_succeeded_events
			WHERE steam != '' OR eos != ''
			GROUP BY steam, eos
		),
		revive_stats AS (
			SELECT
				reviver_steam,
				reviver_eos,
				reviver_name,
				count(*) as revives
			FROM squad_aegis.server_player_revived_events
			WHERE (reviver_steam != '' OR reviver_eos != '')
			GROUP BY reviver_steam, reviver_eos, reviver_name
		),
		normalized_stats AS (
			SELECT
				if(rs.reviver_steam != '', rs.reviver_steam, pi.steam_id) as norm_steam,
				if(rs.reviver_eos != '', rs.reviver_eos, pi.eos_id) as norm_eos,
				coalesce(pi.player_name, rs.reviver_name) as player_name,
				rs.revives
			FROM revive_stats rs
			LEFT JOIN player_identity pi ON
				(rs.reviver_steam != '' AND rs.reviver_steam = pi.steam_id) OR
				(rs.reviver_eos != '' AND rs.reviver_eos = pi.eos_id)
		)
		SELECT
			any(norm_steam) as steam_id,
			any(norm_eos) as eos_id,
			any(player_name) as player_name,
			sum(revives) as revives,
			0 as kills,
			0 as deaths
		FROM normalized_stats
		WHERE norm_steam != '' OR norm_eos != ''
		GROUP BY if(norm_steam != '', norm_steam, norm_eos)
		HAVING revives > 0
		ORDER BY revives DESC
		LIMIT 10
	`

	rows, err = s.Dependencies.Clickhouse.Query(ctx, topMedicsQuery)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var player TopPlayerStats
			var steamID, eosID *string
			err := rows.Scan(
				&steamID,
				&eosID,
				&player.PlayerName,
				&player.Revives,
				&player.Kills,
				&player.Deaths,
			)
			if err == nil {
				if steamID != nil {
					player.SteamID = *steamID
				}
				if eosID != nil {
					player.EOSID = *eosID
				}

				// If name is empty, try to fetch it using getPlayerBasicInfo
				if player.PlayerName == "" {
					var playerID string
					isSteamID := false
					if player.SteamID != "" {
						playerID = player.SteamID
						isSteamID = true
					} else if player.EOSID != "" {
						playerID = player.EOSID
						isSteamID = false
					}

					if playerID != "" {
						if profile, err := s.getPlayerBasicInfo(c, playerID, isSteamID); err == nil && profile != nil {
							player.PlayerName = profile.PlayerName
						}
					}
				}

				summary.TopMedics = append(summary.TopMedics, player)
			}
		}
	}

	// Get most recent players
	recentPlayersQuery := `
		WITH player_records AS (
			SELECT
				steam,
				eos,
				any(player_suffix) as player_name,
				max(event_time) as last_seen,
				min(event_time) as first_seen
			FROM squad_aegis.server_join_succeeded_events
			WHERE (steam != '' OR eos != '')
			GROUP BY steam, eos
		)
		SELECT
			anyIf(steam, steam != '') as steam_id,
			anyIf(eos, eos != '') as eos_id,
			any(player_name) as player_name,
			max(last_seen) as last_seen,
			min(first_seen) as first_seen
		FROM player_records
		WHERE steam != '' OR eos != ''
		GROUP BY if(steam != '', steam, eos)
		ORDER BY last_seen DESC
		LIMIT 10
	`

	rows, err = s.Dependencies.Clickhouse.Query(ctx, recentPlayersQuery)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var player PlayerSearchResult
			var steamID, eosID *string
			err := rows.Scan(
				&steamID,
				&eosID,
				&player.PlayerName,
				&player.LastSeen,
				&player.FirstSeen,
			)
			if err == nil {
				if steamID != nil {
					player.SteamID = *steamID
				}
				if eosID != nil {
					player.EOSID = *eosID
				}

				// If name is empty, try to fetch it using getPlayerBasicInfo
				if player.PlayerName == "" {
					var playerID string
					isSteamID := false
					if player.SteamID != "" {
						playerID = player.SteamID
						isSteamID = true
					} else if player.EOSID != "" {
						playerID = player.EOSID
						isSteamID = false
					}

					if playerID != "" {
						if profile, err := s.getPlayerBasicInfo(c, playerID, isSteamID); err == nil && profile != nil {
							player.PlayerName = profile.PlayerName
						}
					}
				}

				summary.MostRecentPlayers = append(summary.MostRecentPlayers, player)
			}
		}
	}

	// Get overall statistics - do it in two steps for ClickHouse compatibility
	// First get event counts
	eventStatsQuery := `
		SELECT
			count(*) as total_kills,
			count(*) as total_deaths,
			countIf(teamkill = 1) as total_teamkills
		FROM squad_aegis.server_player_died_events
	`

	row := s.Dependencies.Clickhouse.QueryRow(ctx, eventStatsQuery)
	_ = row.Scan(
		&summary.TotalKills,
		&summary.TotalDeaths,
		&summary.TotalTeamkills,
	)

	// Then get unique player count
	playerCountQuery := `
		SELECT uniq(player_id) as total_players
		FROM (
			SELECT if(attacker_steam != '', attacker_steam, attacker_eos) as player_id
			FROM squad_aegis.server_player_died_events
			WHERE attacker_steam != '' OR attacker_eos != ''
			UNION ALL
			SELECT if(victim_steam != '', victim_steam, victim_eos) as player_id
			FROM squad_aegis.server_player_died_events
			WHERE victim_steam != '' OR victim_eos != ''
		)
	`

	row = s.Dependencies.Clickhouse.QueryRow(ctx, playerCountQuery)
	_ = row.Scan(&summary.TotalPlayers)

	responses.Success(c, "Player statistics fetched successfully", &gin.H{"stats": summary})
}

// getLinkedPlayerIdentifiers retrieves all Steam and EOS IDs linked to a given player ID
// Returns arrays of all linked steam IDs and eos IDs
func (s *Server) getLinkedPlayerIdentifiers(c *gin.Context, playerID string, isSteamID bool) (steamIDs []string, eosIDs []string, err error) {
	whereClause := "steam = ?"
	if !isSteamID {
		whereClause = "eos = ?"
	}

	query := fmt.Sprintf(`
		WITH initial_records AS (
			SELECT steam, eos
			FROM squad_aegis.server_join_succeeded_events
			WHERE %s
		)
		SELECT
			groupUniqArray(steam) as steam_ids,
			groupUniqArray(eos) as eos_ids
		FROM initial_records
		WHERE steam != '' OR eos != ''
	`, whereClause)

	row := s.Dependencies.Clickhouse.QueryRow(c.Request.Context(), query, playerID)

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
	// Try to get from pre-computed identity table first
	profile, err := s.getPlayerFromIdentityTable(c.Request.Context(), playerID, isSteamID)
	if err == nil && profile != nil {
		return profile, nil
	}

	// Fallback to raw events query
	return s.getPlayerFromRawEvents(c.Request.Context(), playerID, isSteamID)
}

// getPlayerFromIdentityTable fetches player profile from pre-computed identity table
func (s *Server) getPlayerFromIdentityTable(ctx context.Context, playerID string, isSteamID bool) (*PlayerProfile, error) {
	idType := "eos"
	if isSteamID {
		idType = "steam"
	}

	// First, look up the canonical ID
	lookupQuery := `
		SELECT canonical_id
		FROM squad_aegis.player_identity_lookup
		WHERE identifier_type = ? AND identifier_value = ?
		ORDER BY computed_at DESC
		LIMIT 1
	`

	var canonicalID string
	row := s.Dependencies.Clickhouse.QueryRow(ctx, lookupQuery, idType, playerID)
	if err := row.Scan(&canonicalID); err != nil {
		return nil, fmt.Errorf("player not found in identity table: %w", err)
	}

	// Fetch full identity data
	identityQuery := `
		SELECT
			canonical_id,
			primary_steam_id,
			primary_eos_id,
			primary_name,
			all_steam_ids,
			all_eos_ids,
			all_names,
			total_sessions,
			first_seen,
			last_seen
		FROM squad_aegis.player_identities
		WHERE canonical_id = ?
	`

	var profile PlayerProfile
	var allSteamIDs, allEOSIDs, allNames []string
	var totalSessions uint64

	row = s.Dependencies.Clickhouse.QueryRow(ctx, identityQuery, canonicalID)
	err := row.Scan(
		&profile.CanonicalID,
		&profile.SteamID,
		&profile.EOSID,
		&profile.PlayerName,
		&allSteamIDs,
		&allEOSIDs,
		&allNames,
		&totalSessions,
		&profile.FirstSeen,
		&profile.LastSeen,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch identity: %w", err)
	}

	profile.AllSteamIDs = allSteamIDs
	profile.AllEOSIDs = allEOSIDs
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
func (s *Server) getPlayerFromRawEvents(ctx context.Context, playerID string, isSteamID bool) (*PlayerProfile, error) {
	// Build where clauses for different table column naming conventions
	whereClause := "steam = ?"
	playerWhereClause := "player_steam = ?"
	attackerWhereClause := "attacker_steam = ?"
	victimWhereClause := "victim_steam = ?"
	reviverWhereClause := "reviver_steam = ?"
	if !isSteamID {
		whereClause = "eos = ?"
		playerWhereClause = "player_eos = ?"
		attackerWhereClause = "attacker_eos = ?"
		victimWhereClause = "victim_eos = ?"
		reviverWhereClause = "reviver_eos = ?"
	}

	// Query across ALL player activity tables to get accurate first/last seen timestamps
	// This ensures we capture activity even if no join_succeeded events exist
	query := fmt.Sprintf(`
		WITH all_player_events AS (
			-- Join succeeded events (primary source for identity)
			SELECT steam, eos, player_suffix as name, event_time
			FROM squad_aegis.server_join_succeeded_events
			WHERE %[1]s
			UNION ALL
			-- Connected events
			SELECT steam, eos, '' as name, event_time
			FROM squad_aegis.server_player_connected_events
			WHERE %[1]s
			UNION ALL
			-- Disconnected events
			SELECT steam, eos, player_suffix as name, event_time
			FROM squad_aegis.server_player_disconnected_events
			WHERE %[1]s
			UNION ALL
			-- Possess events (different column names)
			SELECT player_steam as steam, player_eos as eos, player_suffix as name, event_time
			FROM squad_aegis.server_player_possess_events
			WHERE %[2]s
			UNION ALL
			-- Damage dealt (as attacker)
			SELECT attacker_steam as steam, attacker_eos as eos, attacker_name as name, event_time
			FROM squad_aegis.server_player_damaged_events
			WHERE %[3]s
			UNION ALL
			-- Damage taken (as victim)
			SELECT victim_steam as steam, victim_eos as eos, victim_name as name, event_time
			FROM squad_aegis.server_player_damaged_events
			WHERE %[4]s
			UNION ALL
			-- Deaths (as victim)
			SELECT victim_steam as steam, victim_eos as eos, victim_name as name, event_time
			FROM squad_aegis.server_player_died_events
			WHERE %[4]s
			UNION ALL
			-- Kills (as attacker)
			SELECT attacker_steam as steam, attacker_eos as eos, attacker_name as name, event_time
			FROM squad_aegis.server_player_died_events
			WHERE %[3]s
			UNION ALL
			-- Wounded (as victim)
			SELECT victim_steam as steam, victim_eos as eos, victim_name as name, event_time
			FROM squad_aegis.server_player_wounded_events
			WHERE %[4]s
			UNION ALL
			-- Wounded someone (as attacker)
			SELECT attacker_steam as steam, attacker_eos as eos, attacker_name as name, event_time
			FROM squad_aegis.server_player_wounded_events
			WHERE %[3]s
			UNION ALL
			-- Revived someone (as reviver)
			SELECT reviver_steam as steam, reviver_eos as eos, reviver_name as name, event_time
			FROM squad_aegis.server_player_revived_events
			WHERE %[5]s
			UNION ALL
			-- Got revived (as victim)
			SELECT victim_steam as steam, victim_eos as eos, victim_name as name, event_time
			FROM squad_aegis.server_player_revived_events
			WHERE %[4]s
		)
		SELECT
			anyIf(steam, steam != '') as steam_id,
			anyIf(eos, eos != '') as eos_id,
			anyIf(name, name != '') as player_name,
			max(event_time) as last_seen,
			min(event_time) as first_seen,
			count(DISTINCT toDate(event_time)) as total_sessions
		FROM all_player_events
		WHERE steam != '' OR eos != ''
	`, whereClause, playerWhereClause, attackerWhereClause, victimWhereClause, reviverWhereClause)

	// Pass the playerID for each subquery in the UNION ALL
	row := s.Dependencies.Clickhouse.QueryRow(ctx, query,
		playerID, playerID, playerID, // join, connected, disconnected
		playerID,                      // possess
		playerID, playerID,            // damaged (attacker, victim)
		playerID, playerID,            // died (victim, attacker)
		playerID, playerID,            // wounded (victim, attacker)
		playerID, playerID,            // revived (reviver, victim)
	)

	var profile PlayerProfile
	var steamID, eosID *string
	var totalSessions int64

	err := row.Scan(
		&steamID,
		&eosID,
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
func (s *Server) getPlayerStatistics(c *gin.Context, playerID string, isSteamID bool) (*PlayerStatistics, error) {
	whereClause := "steam = ?"
	if !isSteamID {
		whereClause = "eos = ?"
	}

	// Get kills, deaths, teamkills across ALL linked identities
	query := fmt.Sprintf(`
		WITH initial_records AS (
			SELECT steam, eos
			FROM squad_aegis.server_join_succeeded_events
			WHERE %s
		),
		linked_identifiers AS (
			SELECT
				groupUniqArray(steam) as steam_ids,
				groupUniqArray(eos) as eos_ids
			FROM initial_records
			WHERE steam != '' OR eos != ''
		)
		SELECT
			countIf(
				(attacker_steam != '' AND attacker_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers WHERE steam_ids != [])) OR
				(attacker_eos != '' AND attacker_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers WHERE eos_ids != []))
			) as kills,
			countIf(
				(victim_steam != '' AND victim_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers WHERE steam_ids != [])) OR
				(victim_eos != '' AND victim_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers WHERE eos_ids != []))
			) as deaths,
			countIf(
				teamkill = 1 AND (
					(attacker_steam != '' AND attacker_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers WHERE steam_ids != [])) OR
					(attacker_eos != '' AND attacker_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers WHERE eos_ids != []))
				)
			) as teamkills,
			sumIf(
				damage,
				(attacker_steam != '' AND attacker_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers WHERE steam_ids != [])) OR
				(attacker_eos != '' AND attacker_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers WHERE eos_ids != []))
			) as damage_dealt,
			sumIf(
				damage,
				(victim_steam != '' AND victim_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers WHERE steam_ids != [])) OR
				(victim_eos != '' AND victim_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers WHERE eos_ids != []))
			) as damage_taken
		FROM squad_aegis.server_player_died_events
	`, whereClause)

	row := s.Dependencies.Clickhouse.QueryRow(c.Request.Context(), query, playerID)

	var stats PlayerStatistics
	var damageDealt, damageTaken *float64

	err := row.Scan(
		&stats.Kills,
		&stats.Deaths,
		&stats.Teamkills,
		&damageDealt,
		&damageTaken,
	)
	if err != nil {
		return nil, err
	}

	if damageDealt != nil {
		stats.DamageDealt = *damageDealt
	}
	if damageTaken != nil {
		stats.DamageTaken = *damageTaken
	}

	// Calculate K/D ratio
	if stats.Deaths > 0 {
		stats.KDRatio = float64(stats.Kills) / float64(stats.Deaths)
	} else {
		stats.KDRatio = float64(stats.Kills)
	}

	// Get revives across all linked identities
	reviveQuery := fmt.Sprintf(`
		WITH initial_records AS (
			SELECT steam, eos
			FROM squad_aegis.server_join_succeeded_events
			WHERE %s
		),
		linked_identifiers AS (
			SELECT
				groupUniqArray(steam) as steam_ids,
				groupUniqArray(eos) as eos_ids
			FROM initial_records
			WHERE steam != '' OR eos != ''
		)
		SELECT
			countIf(
				(reviver_steam != '' AND reviver_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers WHERE steam_ids != [])) OR
				(reviver_eos != '' AND reviver_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers WHERE eos_ids != []))
			) as revives,
			countIf(
				(victim_steam != '' AND victim_steam IN (SELECT arrayJoin(steam_ids) FROM linked_identifiers WHERE steam_ids != [])) OR
				(victim_eos != '' AND victim_eos IN (SELECT arrayJoin(eos_ids) FROM linked_identifiers WHERE eos_ids != []))
			) as times_revived
		FROM squad_aegis.server_player_revived_events
	`, whereClause)

	row = s.Dependencies.Clickhouse.QueryRow(c.Request.Context(), reviveQuery, playerID)
	err = row.Scan(&stats.Revives, &stats.TimesRevived)
	if err != nil {
		// Non-fatal, just log
		stats.Revives = 0
		stats.TimesRevived = 0
	}

	return &stats, nil
}

// getPlayerRecentActivity retrieves recent player activity
func (s *Server) getPlayerRecentActivity(c *gin.Context, playerID string, isSteamID bool, limit int) ([]PlayerActivity, error) {
	// Combine multiple event types into a single activity feed
	// We'll query died events, wounded events, and chat messages

	whereClause := "steam = ?"
	if !isSteamID {
		whereClause = "eos = ?"
	}

	query := fmt.Sprintf(`
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
		WHERE victim_steam = ? OR victim_eos = ?

		UNION ALL

		SELECT
			sent_at as event_time,
			'chat' as event_type,
			concat('[', chat_type, '] ', message) as description,
			server_id
		FROM squad_aegis.server_player_chat_messages
		WHERE steam_id = ? OR eos_id = ?

		ORDER BY event_time DESC
		LIMIT ?
	`, whereClause)

	rows, err := s.Dependencies.Clickhouse.Query(c.Request.Context(), query, playerID, playerID, playerID, playerID, playerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	activities := []PlayerActivity{}
	for rows.Next() {
		var activity PlayerActivity
		err := rows.Scan(
			&activity.EventTime,
			&activity.EventType,
			&activity.Description,
			&activity.ServerID,
		)
		if err != nil {
			continue
		}
		activities = append(activities, activity)
	}

	return activities, nil
}

// getPlayerChatHistory retrieves player chat history
func (s *Server) getPlayerChatHistory(c *gin.Context, playerID string, isSteamID bool, limit int) ([]ChatMessage, error) {
	whereClause := "steam_id = ?"
	if !isSteamID {
		whereClause = "eos_id = ?"
	}

	// Convert playerID to uint64 for steam_id
	var queryPlayerID interface{} = playerID
	if isSteamID {
		steamIDUint, err := strconv.ParseUint(playerID, 10, 64)
		if err != nil {
			return nil, err
		}
		queryPlayerID = steamIDUint
	}

	query := fmt.Sprintf(`
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

	rows, err := s.Dependencies.Clickhouse.Query(c.Request.Context(), query, queryPlayerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []ChatMessage{}
	for rows.Next() {
		var msg ChatMessage
		err := rows.Scan(
			&msg.SentAt,
			&msg.Message,
			&msg.ChatType,
			&msg.ServerID,
		)
		if err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// getPlayerViolations retrieves player rule violations
func (s *Server) getPlayerViolations(c *gin.Context, playerID string, isSteamID bool) ([]RuleViolation, error) {
	// For EOS ID, we need to first find the steam ID
	steamIDStr := playerID
	if !isSteamID {
		// Get steam ID from EOS ID
		query := `
			SELECT steam
			FROM squad_aegis.server_join_succeeded_events
			WHERE eos = ?
			LIMIT 1
		`
		row := s.Dependencies.Clickhouse.QueryRow(c.Request.Context(), query, playerID)
		var steamID *string
		if err := row.Scan(&steamID); err != nil || steamID == nil {
			return []RuleViolation{}, nil
		}
		steamIDStr = *steamID
	}

	// Convert steam ID to uint64
	steamIDUint, err := strconv.ParseUint(steamIDStr, 10, 64)
	if err != nil {
		return []RuleViolation{}, nil
	}

	query := `
		SELECT
			violation_id,
			server_id,
			rule_id,
			admin_user_id,
			action_type,
			created_at
		FROM squad_aegis.player_rule_violations
		WHERE player_steam_id = ?
		ORDER BY created_at DESC
	`

	rows, err := s.Dependencies.Clickhouse.Query(c.Request.Context(), query, steamIDUint)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	violations := []RuleViolation{}
	for rows.Next() {
		var violation RuleViolation
		err := rows.Scan(
			&violation.ViolationID,
			&violation.ServerID,
			&violation.RuleID,
			&violation.AdminUserID,
			&violation.ActionType,
			&violation.CreatedAt,
		)
		if err != nil {
			continue
		}

		// Enrich with server name from PostgreSQL
		serverNameQuery := `SELECT name FROM servers WHERE id = $1`
		row := s.Dependencies.DB.QueryRow(serverNameQuery, violation.ServerID)
		var serverName string
		if err := row.Scan(&serverName); err == nil {
			violation.ServerName = serverName
		}

		// Enrich with rule name from PostgreSQL if rule_id is present
		if violation.RuleID != nil && *violation.RuleID != "" {
			ruleNameQuery := `SELECT title FROM server_rules WHERE id = $1`
			row := s.Dependencies.DB.QueryRow(ruleNameQuery, *violation.RuleID)
			var ruleName string
			if err := row.Scan(&ruleName); err == nil {
				violation.RuleName = &ruleName
			}
		}

		// Enrich with admin name from PostgreSQL if admin_user_id is present
		if violation.AdminUserID != nil && *violation.AdminUserID != "" {
			adminNameQuery := `SELECT name FROM users WHERE id = $1`
			row := s.Dependencies.DB.QueryRow(adminNameQuery, *violation.AdminUserID)
			var adminName string
			if err := row.Scan(&adminName); err == nil {
				violation.AdminName = &adminName
			}
		}

		violations = append(violations, violation)
	}

	return violations, nil
}

// getPlayerRecentServers retrieves servers the player has recently played on
func (s *Server) getPlayerRecentServers(c *gin.Context, playerID string, isSteamID bool) ([]RecentServerInfo, error) {
	whereClause := "steam = ?"
	if !isSteamID {
		whereClause = "eos = ?"
	}

	query := fmt.Sprintf(`
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

	rows, err := s.Dependencies.Clickhouse.Query(c.Request.Context(), query, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	servers := []RecentServerInfo{}
	for rows.Next() {
		var server RecentServerInfo
		err := rows.Scan(
			&server.ServerID,
			&server.LastSeen,
			&server.Sessions,
		)
		if err != nil {
			continue
		}

		// Get server name from PostgreSQL
		serverNameQuery := `SELECT name FROM servers WHERE id = $1`
		row := s.Dependencies.DB.QueryRow(serverNameQuery, server.ServerID)
		var serverName string
		if err := row.Scan(&serverName); err == nil {
			server.ServerName = serverName
		}

		servers = append(servers, server)
	}

	return servers, nil
}

// getPlayerActiveBans retrieves active bans for the player
func (s *Server) getPlayerActiveBans(c *gin.Context, playerID string, isSteamID bool) ([]ActiveBan, error) {
	// Get steam ID
	steamIDStr := playerID
	if !isSteamID {
		query := `SELECT steam FROM squad_aegis.server_join_succeeded_events WHERE eos = ? LIMIT 1`
		row := s.Dependencies.Clickhouse.QueryRow(c.Request.Context(), query, playerID)
		var steamID *string
		if err := row.Scan(&steamID); err != nil || steamID == nil {
			return []ActiveBan{}, nil
		}
		steamIDStr = *steamID
	}

	// Query PostgreSQL for active bans
	// Duration is in days, 0 means permanent
	// Calculate expiration as created_at + duration days
	query := `
		SELECT
			b.id, b.server_id, b.reason, b.duration,
			b.created_at,
			COALESCE(s.name, 'Unknown Server') as server_name,
			COALESCE(u.name, 'System') as admin_name
		FROM server_bans b
		LEFT JOIN servers s ON b.server_id = s.id
		LEFT JOIN users u ON b.admin_id = u.id
		WHERE b.steam_id = $1
		AND (b.duration = 0 OR b.created_at + (b.duration || ' days')::interval > NOW())
		ORDER BY b.created_at DESC
	`

	rows, err := s.Dependencies.DB.Query(query, steamIDStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bans := []ActiveBan{}
	for rows.Next() {
		var ban ActiveBan
		err := rows.Scan(
			&ban.BanID,
			&ban.ServerID,
			&ban.Reason,
			&ban.Duration,
			&ban.CreatedAt,
			&ban.ServerName,
			&ban.AdminName,
		)
		if err != nil {
			continue
		}
		// Calculate expiration
		ban.Permanent = ban.Duration == 0
		if !ban.Permanent {
			expiresAt := ban.CreatedAt.AddDate(0, 0, ban.Duration)
			ban.ExpiresAt = &expiresAt
		}
		bans = append(bans, ban)
	}

	return bans, nil
}

// getPlayerViolationSummary retrieves a summary of player violations
func (s *Server) getPlayerViolationSummary(c *gin.Context, playerID string, isSteamID bool) (*ViolationSummary, error) {
	// Get steam ID
	steamIDStr := playerID
	if !isSteamID {
		query := `SELECT steam FROM squad_aegis.server_join_succeeded_events WHERE eos = ? LIMIT 1`
		row := s.Dependencies.Clickhouse.QueryRow(c.Request.Context(), query, playerID)
		var steamID *string
		if err := row.Scan(&steamID); err != nil || steamID == nil {
			return &ViolationSummary{}, nil
		}
		steamIDStr = *steamID
	}

	steamIDUint, err := strconv.ParseUint(steamIDStr, 10, 64)
	if err != nil {
		return &ViolationSummary{}, nil
	}

	query := `
		SELECT
			countIf(action_type = 'WARN') as total_warns,
			countIf(action_type = 'KICK') as total_kicks,
			countIf(action_type = 'BAN') as total_bans,
			max(created_at) as last_action
		FROM squad_aegis.player_rule_violations
		WHERE player_steam_id = ?
	`

	row := s.Dependencies.Clickhouse.QueryRow(c.Request.Context(), query, steamIDUint)

	var summary ViolationSummary
	err = row.Scan(
		&summary.TotalWarns,
		&summary.TotalKicks,
		&summary.TotalBans,
		&summary.LastAction,
	)
	if err != nil {
		return &ViolationSummary{}, nil
	}

	return &summary, nil
}

// getPlayerTeamkillMetrics retrieves detailed teamkill statistics
func (s *Server) getPlayerTeamkillMetrics(c *gin.Context, playerID string, isSteamID bool, totalSessions int64, totalKills int64) (*TeamkillMetrics, error) {
	whereClause := "attacker_steam = ?"
	if !isSteamID {
		whereClause = "attacker_eos = ?"
	}

	var metrics TeamkillMetrics

	// Total teamkills from died events
	tkQuery := fmt.Sprintf(`
		SELECT
			count(*) as total_teamkills,
			countIf(event_time >= now() - INTERVAL 7 DAY) as recent_teamkills
		FROM squad_aegis.server_player_died_events
		WHERE teamkill = 1 AND (%s)
	`, whereClause)

	row := s.Dependencies.Clickhouse.QueryRow(c.Request.Context(), tkQuery, playerID)
	if err := row.Scan(&metrics.TotalTeamkills, &metrics.RecentTeamkills); err != nil {
		// Continue with zeros if error
	}

	// Total team wounds from wounded events
	woundQuery := fmt.Sprintf(`
		SELECT
			count(*) as total_team_wounds,
			countIf(event_time >= now() - INTERVAL 7 DAY) as recent_team_wounds
		FROM squad_aegis.server_player_wounded_events
		WHERE teamkill = 1 AND (%s)
	`, whereClause)

	row = s.Dependencies.Clickhouse.QueryRow(c.Request.Context(), woundQuery, playerID)
	if err := row.Scan(&metrics.TotalTeamWounds, &metrics.RecentTeamWounds); err != nil {
		// Continue with zeros if error
	}

	// Total team damage from damaged events
	damageQuery := fmt.Sprintf(`
		SELECT
			count(*) as total_team_damage,
			countIf(event_time >= now() - INTERVAL 7 DAY) as recent_team_damage
		FROM squad_aegis.server_player_damaged_events
		WHERE teamkill = 1 AND (%s)
	`, whereClause)

	row = s.Dependencies.Clickhouse.QueryRow(c.Request.Context(), damageQuery, playerID)
	if err := row.Scan(&metrics.TotalTeamDamage, &metrics.RecentTeamDamage); err != nil {
		// Continue with zeros if error
	}

	// Calculate per-session rate
	if totalSessions > 0 {
		metrics.TeamkillsPerSession = float64(metrics.TotalTeamkills) / float64(totalSessions)
	}

	// Calculate TK ratio (TKs / total kills)
	if totalKills > 0 {
		metrics.TeamkillRatio = float64(metrics.TotalTeamkills) / float64(totalKills)
	}

	return &metrics, nil
}

// getPlayerNameHistory retrieves all names used by the player
// Aggregates names from multiple event tables for comprehensive history
func (s *Server) getPlayerNameHistory(c *gin.Context, playerID string, isSteamID bool) ([]NameHistoryEntry, error) {
	// Build where clauses for different table column naming conventions
	whereClause := "steam = ?"
	playerWhereClause := "player_steam = ?"
	attackerWhereClause := "attacker_steam = ?"
	victimWhereClause := "victim_steam = ?"
	reviverWhereClause := "reviver_steam = ?"
	if !isSteamID {
		whereClause = "eos = ?"
		playerWhereClause = "player_eos = ?"
		attackerWhereClause = "attacker_eos = ?"
		victimWhereClause = "victim_eos = ?"
		reviverWhereClause = "reviver_eos = ?"
	}

	// Query names from all event tables that have player names
	query := fmt.Sprintf(`
		WITH all_names AS (
			-- Join succeeded events
			SELECT player_suffix as name, event_time
			FROM squad_aegis.server_join_succeeded_events
			WHERE %[1]s AND player_suffix != ''
			UNION ALL
			-- Disconnected events
			SELECT player_suffix as name, event_time
			FROM squad_aegis.server_player_disconnected_events
			WHERE %[1]s AND player_suffix != ''
			UNION ALL
			-- Possess events
			SELECT player_suffix as name, event_time
			FROM squad_aegis.server_player_possess_events
			WHERE %[2]s AND player_suffix != ''
			UNION ALL
			-- Damage dealt (as attacker)
			SELECT attacker_name as name, event_time
			FROM squad_aegis.server_player_damaged_events
			WHERE %[3]s AND attacker_name != ''
			UNION ALL
			-- Damage taken (as victim)
			SELECT victim_name as name, event_time
			FROM squad_aegis.server_player_damaged_events
			WHERE %[4]s AND victim_name != ''
			UNION ALL
			-- Deaths (as victim)
			SELECT victim_name as name, event_time
			FROM squad_aegis.server_player_died_events
			WHERE %[4]s AND victim_name != ''
			UNION ALL
			-- Kills (as attacker)
			SELECT attacker_name as name, event_time
			FROM squad_aegis.server_player_died_events
			WHERE %[3]s AND attacker_name != ''
			UNION ALL
			-- Wounded (as victim)
			SELECT victim_name as name, event_time
			FROM squad_aegis.server_player_wounded_events
			WHERE %[4]s AND victim_name != ''
			UNION ALL
			-- Wounded someone (as attacker)
			SELECT attacker_name as name, event_time
			FROM squad_aegis.server_player_wounded_events
			WHERE %[3]s AND attacker_name != ''
			UNION ALL
			-- Revived someone (as reviver)
			SELECT reviver_name as name, event_time
			FROM squad_aegis.server_player_revived_events
			WHERE %[5]s AND reviver_name != ''
			UNION ALL
			-- Got revived (as victim)
			SELECT victim_name as name, event_time
			FROM squad_aegis.server_player_revived_events
			WHERE %[4]s AND victim_name != ''
		)
		SELECT
			name,
			min(event_time) as first_used,
			max(event_time) as last_used,
			count(*) as session_count
		FROM all_names
		WHERE name != ''
		GROUP BY name
		ORDER BY last_used DESC
	`, whereClause, playerWhereClause, attackerWhereClause, victimWhereClause, reviverWhereClause)

	rows, err := s.Dependencies.Clickhouse.Query(c.Request.Context(), query,
		playerID, playerID, // join, disconnected
		playerID,           // possess
		playerID, playerID, // damaged (attacker, victim)
		playerID, playerID, // died (victim, attacker)
		playerID, playerID, // wounded (victim, attacker)
		playerID, playerID, // revived (reviver, victim)
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := []NameHistoryEntry{}
	for rows.Next() {
		var entry NameHistoryEntry
		err := rows.Scan(&entry.Name, &entry.FirstUsed, &entry.LastUsed, &entry.SessionCount)
		if err != nil {
			continue
		}
		names = append(names, entry)
	}

	return names, nil
}

// getPlayerWeaponStats retrieves weapon usage statistics
func (s *Server) getPlayerWeaponStats(c *gin.Context, playerID string, isSteamID bool) ([]WeaponStat, error) {
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
		err := rows.Scan(&stat.Weapon, &stat.Kills, &stat.Teamkills)
		if err != nil {
			continue
		}
		weapons = append(weapons, stat)
	}

	return weapons, nil
}

// lookupPlayerName retrieves the most recent name for a player by their Steam ID or EOS ID
// Used to fill in missing names in combat events
func (s *Server) lookupPlayerName(ctx context.Context, playerID string, isSteamID bool) string {
	whereClause := "steam = ?"
	playerWhereClause := "player_steam = ?"
	attackerWhereClause := "attacker_steam = ?"
	victimWhereClause := "victim_steam = ?"
	if !isSteamID {
		whereClause = "eos = ?"
		playerWhereClause = "player_eos = ?"
		attackerWhereClause = "attacker_eos = ?"
		victimWhereClause = "victim_eos = ?"
	}

	// Query for the most recent non-empty name across all event tables
	query := fmt.Sprintf(`
		SELECT name FROM (
			SELECT player_suffix as name, event_time
			FROM squad_aegis.server_join_succeeded_events
			WHERE %[1]s AND player_suffix != ''
			UNION ALL
			SELECT player_suffix as name, event_time
			FROM squad_aegis.server_player_disconnected_events
			WHERE %[1]s AND player_suffix != ''
			UNION ALL
			SELECT player_suffix as name, event_time
			FROM squad_aegis.server_player_possess_events
			WHERE %[2]s AND player_suffix != ''
			UNION ALL
			SELECT attacker_name as name, event_time
			FROM squad_aegis.server_player_damaged_events
			WHERE %[3]s AND attacker_name != ''
			UNION ALL
			SELECT victim_name as name, event_time
			FROM squad_aegis.server_player_damaged_events
			WHERE %[4]s AND victim_name != ''
			UNION ALL
			SELECT attacker_name as name, event_time
			FROM squad_aegis.server_player_died_events
			WHERE %[3]s AND attacker_name != ''
			UNION ALL
			SELECT victim_name as name, event_time
			FROM squad_aegis.server_player_died_events
			WHERE %[4]s AND victim_name != ''
		)
		ORDER BY event_time DESC
		LIMIT 1
	`, whereClause, playerWhereClause, attackerWhereClause, victimWhereClause)

	var name string
	row := s.Dependencies.Clickhouse.QueryRow(ctx, query,
		playerID, playerID, // join, disconnected
		playerID,           // possess
		playerID, playerID, // damaged (attacker, victim)
		playerID, playerID, // died (attacker, victim)
	)
	if err := row.Scan(&name); err != nil {
		return ""
	}
	return name
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

// PlayerChatHistoryPaginated handles GET /api/players/:playerId/chat - paginated chat history
func (s *Server) PlayerChatHistoryPaginated(c *gin.Context) {
	playerID := c.Param("playerId")
	if playerID == "" {
		responses.BadRequest(c, "Player ID is required", nil)
		return
	}

	// Parse query parameters
	page := 1
	limit := 50
	chatType := c.Query("type")
	search := c.Query("search")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	isSteamID := false
	if _, err := strconv.ParseUint(playerID, 10, 64); err == nil {
		isSteamID = true
	}

	whereClause := "steam_id = ?"
	if !isSteamID {
		whereClause = "eos_id = ?"
	}

	var queryPlayerID interface{} = playerID
	if isSteamID {
		steamIDUint, err := strconv.ParseUint(playerID, 10, 64)
		if err != nil {
			responses.BadRequest(c, "Invalid Steam ID", nil)
			return
		}
		queryPlayerID = steamIDUint
	}

	// Build filters
	filters := []string{whereClause}
	args := []interface{}{queryPlayerID}

	if chatType != "" && chatType != "all" {
		filters = append(filters, "chat_type = ?")
		args = append(args, chatType)
	}
	if search != "" {
		filters = append(filters, "message ILIKE ?")
		args = append(args, "%"+search+"%")
	}
	if startDate != "" {
		filters = append(filters, "sent_at >= ?")
		args = append(args, startDate)
	}
	if endDate != "" {
		filters = append(filters, "sent_at <= ?")
		args = append(args, endDate)
	}

	whereSQL := strings.Join(filters, " AND ")

	// Get total count
	countQuery := fmt.Sprintf(`
		SELECT count(*) FROM squad_aegis.server_player_chat_messages WHERE %s
	`, whereSQL)

	var total int64
	row := s.Dependencies.Clickhouse.QueryRow(c.Request.Context(), countQuery, args...)
	if err := row.Scan(&total); err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	// Get paginated results
	offset := (page - 1) * limit
	args = append(args, limit, offset)

	query := fmt.Sprintf(`
		SELECT
			message_id,
			sent_at,
			message,
			chat_type,
			server_id
		FROM squad_aegis.server_player_chat_messages
		WHERE %s
		ORDER BY sent_at DESC
		LIMIT ? OFFSET ?
	`, whereSQL)

	rows, err := s.Dependencies.Clickhouse.Query(c.Request.Context(), query, args...)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	defer rows.Close()

	messages := []ChatMessage{}
	for rows.Next() {
		var msg ChatMessage
		var messageID string
		err := rows.Scan(&messageID, &msg.SentAt, &msg.Message, &msg.ChatType, &msg.ServerID)
		if err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	responses.Success(c, "Chat history fetched successfully", &gin.H{
		"chat": PaginatedChatHistory{
			Messages:   messages,
			Total:      total,
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages,
		},
	})
}

// PlayerTeamkillsAnalysis handles GET /api/players/:playerId/teamkills - teamkill analysis
func (s *Server) PlayerTeamkillsAnalysis(c *gin.Context) {
	playerID := c.Param("playerId")
	if playerID == "" {
		responses.BadRequest(c, "Player ID is required", nil)
		return
	}

	isSteamID := false
	if _, err := strconv.ParseUint(playerID, 10, 64); err == nil {
		isSteamID = true
	}

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

// PlayerSessionHistory handles GET /api/players/:playerId/sessions - session history
func (s *Server) PlayerSessionHistory(c *gin.Context) {
	playerID := c.Param("playerId")
	if playerID == "" {
		responses.BadRequest(c, "Player ID is required", nil)
		return
	}

	// Check if user has permission to view IPs
	canViewIPs := false
	if user := s.getUserFromSession(c); user != nil && user.SuperAdmin {
		canViewIPs = true
	}

	page := 1
	limit := 50
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	isSteamID := false
	if _, err := strconv.ParseUint(playerID, 10, 64); err == nil {
		isSteamID = true
	}

	whereClause := "steam = ?"
	if !isSteamID {
		whereClause = "eos = ?"
	}

	// Fetch all events (connects and disconnects) ordered by time DESC
	// We fetch more than needed to ensure proper pairing at page boundaries
	bufferMultiplier := 3
	fetchLimit := limit * bufferMultiplier

	query := fmt.Sprintf(`
		SELECT * FROM (
			SELECT
				event_time,
				server_id,
				ip,
				'connected' as event_type
			FROM squad_aegis.server_player_connected_events
			WHERE %s
			UNION ALL
			SELECT
				event_time,
				server_id,
				ip,
				'disconnected' as event_type
			FROM squad_aegis.server_player_disconnected_events
			WHERE %s
		) ORDER BY event_time DESC
		LIMIT ?
	`, whereClause, whereClause)

	rows, err := s.Dependencies.Clickhouse.Query(c.Request.Context(), query, playerID, playerID, fetchLimit)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	defer rows.Close()

	// Collect raw events
	type rawEvent struct {
		EventTime time.Time
		ServerID  string
		IP        string
		EventType string
	}
	var events []rawEvent
	for rows.Next() {
		var e rawEvent
		if err := rows.Scan(&e.EventTime, &e.ServerID, &e.IP, &e.EventType); err != nil {
			continue
		}
		events = append(events, e)
	}

	// Pair connect events with their corresponding disconnect events
	// Events are ordered newest first (DESC)
	sessions := []SessionHistoryEntry{}
	usedDisconnects := make(map[int]bool)

	for i, e := range events {
		if e.EventType != "connected" {
			continue
		}

		session := SessionHistoryEntry{
			ConnectTime: e.EventTime,
			ServerID:    e.ServerID,
		}
		if canViewIPs {
			session.IP = e.IP
		}

		// Look for the first disconnect after this connect (searching forward in time-descending list means looking backward)
		// In our DESC-ordered list, disconnects that happened AFTER connect are at indices BEFORE i
		foundDisconnect := false
		for j := i - 1; j >= 0; j-- {
			if events[j].EventType == "disconnected" && !usedDisconnects[j] {
				// Check if there's another connect between this disconnect and our connect
				hasIntermediateConnect := false
				for k := j + 1; k < i; k++ {
					if events[k].EventType == "connected" {
						hasIntermediateConnect = true
						break
					}
				}

				if !hasIntermediateConnect {
					// This disconnect belongs to our connect
					usedDisconnects[j] = true
					disconnectTime := events[j].EventTime
					session.DisconnectTime = &disconnectTime
					duration := int64(disconnectTime.Sub(e.EventTime).Seconds())
					session.DurationSeconds = &duration
					foundDisconnect = true
					break
				}
			}
		}

		if !foundDisconnect {
			// Check if this is an ongoing session (connected within the last hour)
			if time.Since(e.EventTime) < time.Hour {
				session.Ongoing = true
			} else {
				session.MissingDisconnect = true
			}
		}

		sessions = append(sessions, session)
	}

	// Apply pagination to paired sessions
	offset := (page - 1) * limit
	end := offset + limit
	if offset > len(sessions) {
		sessions = []SessionHistoryEntry{}
	} else {
		if end > len(sessions) {
			end = len(sessions)
		}
		sessions = sessions[offset:end]
	}

	// Cache server names to avoid repeated queries
	serverNameCache := make(map[string]string)
	for i := range sessions {
		serverID := sessions[i].ServerID
		if name, ok := serverNameCache[serverID]; ok {
			sessions[i].ServerName = name
		} else {
			var serverName string
			if err := s.Dependencies.DB.QueryRow(`SELECT name FROM servers WHERE id = $1`, serverID).Scan(&serverName); err == nil {
				serverNameCache[serverID] = serverName
				sessions[i].ServerName = serverName
			}
		}
	}

	responses.Success(c, "Session history fetched successfully", &gin.H{
		"sessions":    sessions,
		"can_view_ip": canViewIPs,
		"page":        page,
		"limit":       limit,
	})
}

// PlayerRelatedPlayers handles GET /api/players/:playerId/related - related players (same IP)
func (s *Server) PlayerRelatedPlayers(c *gin.Context) {
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

	isSteamID := false
	if _, err := strconv.ParseUint(playerID, 10, 64); err == nil {
		isSteamID = true
	}

	whereClause := "steam = ?"
	if !isSteamID {
		whereClause = "eos = ?"
	}

	// Find players sharing the same IPs
	// Need to join with server_join_succeeded_events to get player names
	query := fmt.Sprintf(`
		WITH player_ips AS (
			SELECT DISTINCT ip
			FROM squad_aegis.server_player_connected_events
			WHERE %s AND ip != ''
		),
		related_connections AS (
			SELECT
				steam,
				eos,
				count(*) as shared_sessions
			FROM squad_aegis.server_player_connected_events
			WHERE ip IN (SELECT ip FROM player_ips)
				AND NOT (%s)
				AND (steam != '' OR eos != '')
			GROUP BY steam, eos
		),
		player_names AS (
			SELECT
				steam,
				eos,
				any(player_suffix) as player_name
			FROM squad_aegis.server_join_succeeded_events
			WHERE steam != '' OR eos != ''
			GROUP BY steam, eos
		)
		SELECT
			rc.steam,
			rc.eos,
			COALESCE(pn.player_name, '') as player_name,
			rc.shared_sessions
		FROM related_connections rc
		LEFT JOIN player_names pn ON (rc.steam != '' AND rc.steam = pn.steam) OR (rc.eos != '' AND rc.eos = pn.eos)
		ORDER BY rc.shared_sessions DESC
		LIMIT 20
	`, whereClause, whereClause)

	rows, err := s.Dependencies.Clickhouse.Query(c.Request.Context(), query, playerID, playerID)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	defer rows.Close()

	related := []RelatedPlayer{}
	for rows.Next() {
		var player RelatedPlayer
		err := rows.Scan(&player.SteamID, &player.EOSID, &player.PlayerName, &player.SharedSessions)
		if err != nil {
			continue
		}
		player.RelationType = "same_ip"

		// Check if this player is banned (duration 0 = permanent, otherwise check if created_at + duration > now)
		if player.SteamID != "" {
			banQuery := `SELECT EXISTS(SELECT 1 FROM server_bans WHERE steam_id = $1 AND (duration = 0 OR created_at + (duration || ' days')::interval > NOW()))`
			var isBanned bool
			if err := s.Dependencies.DB.QueryRow(banQuery, player.SteamID).Scan(&isBanned); err == nil {
				player.IsBanned = isBanned
			}
		}

		related = append(related, player)
	}

	responses.Success(c, "Related players fetched successfully", &gin.H{
		"related_players": related,
	})
}

// PlayerCombatHistory handles GET /api/players/:playerId/combat - combat history (kills and deaths)
func (s *Server) PlayerCombatHistory(c *gin.Context) {
	playerID := c.Param("playerId")
	if playerID == "" {
		responses.BadRequest(c, "Player ID is required", nil)
		return
	}

	page := 1
	limit := 50
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	isSteamID := false
	if _, err := strconv.ParseUint(playerID, 10, 64); err == nil {
		isSteamID = true
	}

	// Build WHERE clauses for kills (player is attacker) and deaths (player is victim)
	var killWhereClause, deathWhereClause string
	if isSteamID {
		killWhereClause = "attacker_steam = ?"
		deathWhereClause = "victim_steam = ?"
	} else {
		killWhereClause = "attacker_eos = ?"
		deathWhereClause = "victim_eos = ?"
	}

	offset := (page - 1) * limit

	query := fmt.Sprintf(`
		SELECT * FROM (
			-- Kills (player killed someone)
			SELECT
				event_time,
				'kill' as event_type,
				server_id,
				weapon,
				damage,
				teamkill,
				victim_name as other_name,
				victim_steam as other_steam_id,
				victim_eos as other_eos_id,
				victim_team as other_team,
				victim_squad as other_squad,
				attacker_team as player_team,
				attacker_squad as player_squad
			FROM squad_aegis.server_player_died_events
			WHERE %s
			UNION ALL
			-- Deaths (player was killed)
			SELECT
				event_time,
				'death' as event_type,
				server_id,
				weapon,
				damage,
				teamkill,
				attacker_name as other_name,
				attacker_steam as other_steam_id,
				attacker_eos as other_eos_id,
				attacker_team as other_team,
				attacker_squad as other_squad,
				victim_team as player_team,
				victim_squad as player_squad
			FROM squad_aegis.server_player_died_events
			WHERE %s
			UNION ALL
			-- Wounded someone (player downed someone)
			SELECT
				event_time,
				'wounded' as event_type,
				server_id,
				weapon,
				damage,
				teamkill,
				victim_name as other_name,
				victim_steam as other_steam_id,
				victim_eos as other_eos_id,
				victim_team as other_team,
				victim_squad as other_squad,
				attacker_team as player_team,
				attacker_squad as player_squad
			FROM squad_aegis.server_player_wounded_events
			WHERE %s
			UNION ALL
			-- Wounded by (player was downed)
			SELECT
				event_time,
				'wounded_by' as event_type,
				server_id,
				weapon,
				damage,
				teamkill,
				attacker_name as other_name,
				attacker_steam as other_steam_id,
				attacker_eos as other_eos_id,
				attacker_team as other_team,
				attacker_squad as other_squad,
				victim_team as player_team,
				victim_squad as player_squad
			FROM squad_aegis.server_player_wounded_events
			WHERE %s
			UNION ALL
			-- Damaged someone (player dealt damage)
			SELECT
				event_time,
				'damaged' as event_type,
				server_id,
				weapon,
				damage,
				teamkill,
				victim_name as other_name,
				victim_steam as other_steam_id,
				victim_eos as other_eos_id,
				victim_team as other_team,
				victim_squad as other_squad,
				attacker_team as player_team,
				attacker_squad as player_squad
			FROM squad_aegis.server_player_damaged_events
			WHERE %s
			UNION ALL
			-- Damaged by (player took damage)
			SELECT
				event_time,
				'damaged_by' as event_type,
				server_id,
				weapon,
				damage,
				teamkill,
				attacker_name as other_name,
				attacker_steam as other_steam_id,
				attacker_eos as other_eos_id,
				attacker_team as other_team,
				attacker_squad as other_squad,
				victim_team as player_team,
				victim_squad as player_squad
			FROM squad_aegis.server_player_damaged_events
			WHERE %s
		) ORDER BY event_time DESC
		LIMIT ? OFFSET ?
	`, killWhereClause, deathWhereClause, killWhereClause, deathWhereClause, killWhereClause, deathWhereClause)

	rows, err := s.Dependencies.Clickhouse.Query(c.Request.Context(), query, playerID, playerID, playerID, playerID, playerID, playerID, limit, offset)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	defer rows.Close()

	events := []CombatHistoryEntry{}
	serverNameCache := make(map[string]string)

	for rows.Next() {
		var entry CombatHistoryEntry
		var teamkillInt uint8
		err := rows.Scan(
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
			continue
		}
		entry.Teamkill = teamkillInt == 1

		// Get server name from cache or database
		if name, ok := serverNameCache[entry.ServerID]; ok {
			entry.ServerName = name
		} else {
			var serverName string
			if err := s.Dependencies.DB.QueryRow(`SELECT name FROM servers WHERE id = $1`, entry.ServerID).Scan(&serverName); err == nil {
				serverNameCache[entry.ServerID] = serverName
				entry.ServerName = serverName
			}
		}

		events = append(events, entry)
	}

	responses.Success(c, "Combat history fetched successfully", &gin.H{
		"events": events,
		"page":   page,
		"limit":  limit,
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

	// Pagination
	page := 1
	limit := 20
	if pageStr := c.Query("page"); pageStr != "" {
		if parsedPage, err := strconv.Atoi(pageStr); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 50 {
			limit = parsedLimit
		}
	}
	offset := (page - 1) * limit

	// Step 1: Get IPs shared by multiple players with last activity
	ipsQuery := `
		SELECT
			ip,
			max(event_time) as last_activity,
			countDistinct(if(steam != '', steam, eos)) as player_count
		FROM squad_aegis.server_player_connected_events
		WHERE ip != '' AND (steam != '' OR eos != '')
		GROUP BY ip
		HAVING player_count > 1
		ORDER BY last_activity DESC
		LIMIT ? OFFSET ?
	`

	ipRows, err := s.Dependencies.Clickhouse.Query(c.Request.Context(), ipsQuery, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query shared IPs")
		responses.InternalServerError(c, err, nil)
		return
	}
	defer ipRows.Close()

	type ipInfo struct {
		IP           string
		LastActivity time.Time
		PlayerCount  uint64
	}
	sharedIPs := []ipInfo{}
	for ipRows.Next() {
		var info ipInfo
		if err := ipRows.Scan(&info.IP, &info.LastActivity, &info.PlayerCount); err != nil {
			log.Error().Err(err).Msg("Failed to scan IP row")
			continue
		}
		sharedIPs = append(sharedIPs, info)
	}

	groups := []AltAccountGroup{}

	// Step 2: For each IP, get the players
	for _, ipInfo := range sharedIPs {
		playersQuery := `
			WITH player_sessions AS (
				SELECT
					steam,
					eos,
					count() as sessions,
					max(event_time) as last_seen
				FROM squad_aegis.server_player_connected_events
				WHERE ip = ? AND (steam != '' OR eos != '')
				GROUP BY steam, eos
			),
			player_names AS (
				SELECT
					steam,
					eos,
					any(player_suffix) as player_name
				FROM squad_aegis.server_join_succeeded_events
				WHERE steam != '' OR eos != ''
				GROUP BY steam, eos
			)
			SELECT
				ps.steam,
				ps.eos,
				COALESCE(pn.player_name, '') as player_name,
				ps.sessions,
				ps.last_seen
			FROM player_sessions ps
			LEFT JOIN player_names pn ON (ps.steam != '' AND ps.steam = pn.steam) OR (ps.eos != '' AND ps.eos = pn.eos)
			ORDER BY ps.last_seen DESC
		`

		playerRows, err := s.Dependencies.Clickhouse.Query(c.Request.Context(), playersQuery, ipInfo.IP)
		if err != nil {
			log.Error().Err(err).Str("ip", ipInfo.IP).Msg("Failed to query players for IP")
			continue
		}

		lastActivity := ipInfo.LastActivity
		group := AltAccountGroup{
			GroupID:       ipInfo.IP,
			SharedIPCount: 1,
			LastActivity:  &lastActivity,
			Players:       []AltAccountPlayer{},
		}

		for playerRows.Next() {
			var steam, eos, name string
			var sessions uint64
			var lastSeen time.Time
			if err := playerRows.Scan(&steam, &eos, &name, &sessions, &lastSeen); err != nil {
				log.Error().Err(err).Msg("Failed to scan player row")
				continue
			}

			player := AltAccountPlayer{
				SteamID:        steam,
				EOSID:          eos,
				PlayerName:     name,
				SharedSessions: int64(sessions),
				LastSeen:       &lastSeen,
			}

			// Check if this player is banned
			if player.SteamID != "" {
				banQuery := `SELECT EXISTS(SELECT 1 FROM server_bans WHERE steam_id = $1 AND (duration = 0 OR created_at + (duration || ' days')::interval > NOW()))`
				var isBanned bool
				if err := s.Dependencies.DB.QueryRow(banQuery, player.SteamID).Scan(&isBanned); err == nil {
					player.IsBanned = isBanned
				}
			}

			group.Players = append(group.Players, player)
		}
		playerRows.Close()

		if len(group.Players) > 1 {
			groups = append(groups, group)
		}
	}

	// Get total count for pagination
	countQuery := `
		SELECT count() FROM (
			SELECT ip
			FROM squad_aegis.server_player_connected_events
			WHERE ip != '' AND (steam != '' OR eos != '')
			GROUP BY ip
			HAVING countDistinct(if(steam != '', steam, eos)) > 1
		)
	`
	var totalGroups int64
	if err := s.Dependencies.Clickhouse.QueryRow(c.Request.Context(), countQuery).Scan(&totalGroups); err != nil {
		log.Warn().Err(err).Msg("Failed to get alt groups count")
		totalGroups = int64(len(groups))
	}

	responses.Success(c, "Alt account groups fetched successfully", &gin.H{
		"alt_groups":   groups,
		"total_groups": totalGroups,
		"page":         page,
		"limit":        limit,
	})
}
