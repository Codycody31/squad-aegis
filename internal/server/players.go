package server

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

	// Build the ClickHouse query to search for players
	// Search by player name (suffix), steam ID, or EOS ID
	query := `
		SELECT 
			steam as steam_id,
			eos as eos_id,
			player_suffix as player_name,
			max(event_time) as last_seen,
			min(event_time) as first_seen
		FROM squad_aegis.server_join_succeeded_events
		WHERE 
			player_suffix ILIKE ? OR
			steam ILIKE ? OR
			eos ILIKE ?
		GROUP BY steam_id, eos_id, player_suffix
		ORDER BY last_seen DESC
		LIMIT ?
	`

	searchPattern := "%" + searchQuery + "%"

	rows, err := s.Dependencies.Clickhouse.Query(
		c.Request.Context(),
		query,
		searchPattern,
		searchPattern,
		searchPattern,
		limit,
	)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	defer rows.Close()

	players := []PlayerSearchResult{}
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
		if err != nil {
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

	responses.Success(c, "Players fetched successfully", &gin.H{
		"players": players,
		"count":   len(players),
	})
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

	responses.Success(c, "Player profile fetched successfully", &gin.H{"player": profile})
}

// PlayersStats handles GET /api/players/stats - get player statistics summary
func (s *Server) PlayersStats(c *gin.Context) {
	ctx := c.Request.Context()

	summary := PlayerStatsSummary{}

	// Get top players by K/D ratio (min 10 kills to qualify)
	topPlayersQuery := `
		WITH player_stats AS (
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
		)
		SELECT 
			attacker_steam,
			attacker_eos,
			any(attacker_name) as player_name,
			sum(kills) as total_kills,
			sum(deaths) as total_deaths,
			if(sum(deaths) > 0, sum(kills) / sum(deaths), sum(kills)) as kd_ratio
		FROM player_stats
		WHERE attacker_steam != '' OR attacker_eos != ''
		GROUP BY attacker_steam, attacker_eos
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
				summary.TopPlayers = append(summary.TopPlayers, player)
			}
		}
	}

	// Get top teamkillers
	topTeamkillersQuery := `
		SELECT 
			attacker_steam,
			attacker_eos,
			attacker_name,
			count(*) as teamkills,
			0 as kills,
			0 as deaths
		FROM squad_aegis.server_player_died_events
		WHERE teamkill = 1 AND (attacker_steam != '' OR attacker_eos != '')
		GROUP BY attacker_steam, attacker_eos, attacker_name
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
				summary.TopTeamkillers = append(summary.TopTeamkillers, player)
			}
		}
	}

	// Get top medics (by revives)
	topMedicsQuery := `
		SELECT 
			reviver_steam,
			reviver_eos,
			reviver_name,
			count(*) as revives,
			0 as kills,
			0 as deaths
		FROM squad_aegis.server_player_revived_events
		WHERE (reviver_steam != '' OR reviver_eos != '')
		GROUP BY reviver_steam, reviver_eos, reviver_name
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
				summary.TopMedics = append(summary.TopMedics, player)
			}
		}
	}

	// Get most recent players
	recentPlayersQuery := `
		SELECT 
			steam,
			eos,
			player_suffix,
			max(event_time) as last_seen,
			min(event_time) as first_seen
		FROM squad_aegis.server_join_succeeded_events
		WHERE (steam != '' OR eos != '')
		GROUP BY steam, eos, player_suffix
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
				summary.MostRecentPlayers = append(summary.MostRecentPlayers, player)
			}
		}
	}

	// Get overall statistics
	overallStatsQuery := `
		SELECT 
			uniq(attacker_steam, attacker_eos) + uniq(victim_steam, victim_eos) as total_players,
			count(*) as total_kills,
			count(*) as total_deaths,
			countIf(teamkill = 1) as total_teamkills
		FROM squad_aegis.server_player_died_events
	`

	row := s.Dependencies.Clickhouse.QueryRow(ctx, overallStatsQuery)
	err = row.Scan(
		&summary.TotalPlayers,
		&summary.TotalKills,
		&summary.TotalDeaths,
		&summary.TotalTeamkills,
	)

	responses.Success(c, "Player statistics fetched successfully", &gin.H{"stats": summary})
}

// getPlayerBasicInfo retrieves basic player information
func (s *Server) getPlayerBasicInfo(c *gin.Context, playerID string, isSteamID bool) (*PlayerProfile, error) {
	whereClause := "steam = ?"
	if !isSteamID {
		whereClause = "eos = ?"
	}

	query := fmt.Sprintf(`
		SELECT 
			steam,
			eos,
			player_suffix,
			max(event_time) as last_seen,
			min(event_time) as first_seen,
			count(DISTINCT toDate(event_time)) as total_sessions
		FROM squad_aegis.server_join_succeeded_events
		WHERE %s
		GROUP BY steam, eos, player_suffix
		LIMIT 1
	`, whereClause)

	row := s.Dependencies.Clickhouse.QueryRow(c.Request.Context(), query, playerID)

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

	// Calculate total play time (approximate based on session days)
	if profile.FirstSeen != nil && profile.LastSeen != nil {
		profile.TotalPlayTime = int64(profile.LastSeen.Sub(*profile.FirstSeen).Seconds())
	}

	return &profile, nil
}

// getPlayerStatistics retrieves player combat statistics
func (s *Server) getPlayerStatistics(c *gin.Context, playerID string, isSteamID bool) (*PlayerStatistics, error) {
	whereClause := "attacker_steam = ?"
	victimWhereClause := "victim_steam = ?"
	if !isSteamID {
		whereClause = "attacker_eos = ?"
		victimWhereClause = "victim_eos = ?"
	}

	// Get kills, deaths, teamkills
	query := `
		SELECT 
			countIf(attacker_steam = ? OR attacker_eos = ?) as kills,
			countIf(victim_steam = ? OR victim_eos = ?) as deaths,
			countIf((attacker_steam = ? OR attacker_eos = ?) AND teamkill = 1) as teamkills,
			sumIf(damage, attacker_steam = ? OR attacker_eos = ?) as damage_dealt,
			sumIf(damage, victim_steam = ? OR victim_eos = ?) as damage_taken
		FROM squad_aegis.server_player_died_events
		WHERE attacker_steam = ? OR attacker_eos = ? OR victim_steam = ? OR victim_eos = ?
	`

	row := s.Dependencies.Clickhouse.QueryRow(c.Request.Context(), query,
		playerID, playerID, // kills
		playerID, playerID, // deaths
		playerID, playerID, // teamkills
		playerID, playerID, // damage_dealt
		playerID, playerID, // damage_taken
		playerID, playerID, playerID, playerID, // where clause
	)

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

	// Get revives
	reviveQuery := fmt.Sprintf(`
		SELECT 
			countIf(%s) as revives,
			countIf(%s) as times_revived
		FROM squad_aegis.server_player_revived_events
		WHERE %s OR %s
	`, whereClause, victimWhereClause, whereClause, victimWhereClause)

	row = s.Dependencies.Clickhouse.QueryRow(c.Request.Context(), reviveQuery, playerID, playerID, playerID, playerID)
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
