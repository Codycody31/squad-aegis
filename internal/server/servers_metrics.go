package server

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// MetricPoint represents a time-series data point
type MetricPoint struct {
	Timestamp time.Time   `json:"timestamp"`
	Value     interface{} `json:"value"`
}

// ServerMetricsData represents the metrics response
type ServerMetricsData struct {
	PlayerCount            []MetricPoint  `json:"player_count"`
	QueueCount             []MetricPoint  `json:"queue_count"`
	TickRate               []MetricPoint  `json:"tick_rate"`
	Rounds                 []MetricPoint  `json:"rounds"`
	Maps                   []MetricPoint  `json:"maps"`
	ChatActivity           []MetricPoint  `json:"chat_activity"`
	ConnectionStats        []MetricPoint  `json:"connection_stats"` // Connection activity (connects + disconnects)
	TeamkillStats          []MetricPoint  `json:"teamkill_stats"`
	PlayerWoundedStats     []MetricPoint  `json:"player_wounded_stats"`
	PlayerRevivedStats     []MetricPoint  `json:"player_revived_stats"`
	PlayerPossessStats     []MetricPoint  `json:"player_possess_stats"`
	PlayerDiedStats        []MetricPoint  `json:"player_died_stats"`
	PlayerDamagedStats     []MetricPoint  `json:"player_damaged_stats"`
	DeployableDamagedStats []MetricPoint  `json:"deployable_damaged_stats"`
	AdminBroadcastStats    []MetricPoint  `json:"admin_broadcast_stats"`
	Period                 string         `json:"period"`
	Summary                MetricsSummary `json:"summary"`
}

// MetricsSummary provides aggregate statistics
type MetricsSummary struct {
	TotalPlayers           int     `json:"total_players"`
	AvgTickRate            float64 `json:"avg_tick_rate"`
	TotalRounds            int     `json:"total_rounds"`
	UniquePlayersCount     int     `json:"unique_players_count"`
	TotalChatMessages      int     `json:"total_chat_messages"`
	TotalConnections       int     `json:"total_connections"` // Total connection activity (connects + disconnects)
	TotalTeamkills         int     `json:"total_teamkills"`
	TotalPlayerWounded     int     `json:"total_player_wounded"`
	TotalPlayerRevived     int     `json:"total_player_revived"`
	TotalPlayerPossess     int     `json:"total_player_possess"`
	TotalPlayerDied        int     `json:"total_player_died"`
	TotalPlayerDamaged     int     `json:"total_player_damaged"`
	TotalDeployableDamaged int     `json:"total_deployable_damaged"`
	TotalAdminBroadcasts   int     `json:"total_admin_broadcasts"`
	TotalPluginLogs        int     `json:"total_plugin_logs"`
	MostPlayedMap          string  `json:"most_played_map"`
	PeakPlayerCount        int     `json:"peak_player_count"`
}

// ServerMetricsHistory provides detailed metrics history from ClickHouse
func (s *Server) ServerMetricsHistory(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Get parameters
	period := c.DefaultQuery("period", "24h")       // 24h, 7d, 30d
	intervalStr := c.DefaultQuery("interval", "60") // minutes
	interval, err := strconv.Atoi(intervalStr)
	if err != nil || interval <= 0 {
		interval = 60
	}

	// Get real metrics from ClickHouse
	metricsData, err := s.getMetricsFromClickHouse(c.Request.Context(), serverId, period, interval)
	if err != nil {
		responses.BadRequest(c, "Failed to fetch metrics", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Server metrics retrieved successfully", &gin.H{
		"metrics": metricsData,
	})
}

// getMetricsFromClickHouse retrieves real metrics data from ClickHouse
func (s *Server) getMetricsFromClickHouse(ctx context.Context, serverId uuid.UUID, period string, interval int) (ServerMetricsData, error) {
	// Access ClickHouse client through PluginManager
	if s.Dependencies.PluginManager == nil {
		return s.generateSampleMetrics(period, interval), nil
	}

	clickhouseClient := s.Dependencies.PluginManager.GetClickHouseClient()
	if clickhouseClient == nil {
		return s.generateSampleMetrics(period, interval), nil
	}

	// Calculate time range based on period
	now := time.Now()
	var startTime time.Time
	switch period {
	case "1h":
		startTime = now.Add(-1 * time.Hour)
	case "6h":
		startTime = now.Add(-6 * time.Hour)
	case "24h":
		startTime = now.Add(-24 * time.Hour)
	case "7d":
		startTime = now.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = now.Add(-30 * 24 * time.Hour)
	default:
		startTime = now.Add(-24 * time.Hour)
	}

	// Query ClickHouse for real metrics data
	metricsData := ServerMetricsData{
		Period: period,
		Summary: MetricsSummary{
			TotalPlayers:           0,
			AvgTickRate:            0,
			TotalChatMessages:      0,
			TotalConnections:       0,
			TotalTeamkills:         0,
			TotalPlayerWounded:     0,
			TotalPlayerRevived:     0,
			TotalPlayerPossess:     0,
			TotalPlayerDied:        0,
			TotalPlayerDamaged:     0,
			TotalDeployableDamaged: 0,
			TotalAdminBroadcasts:   0,
			TotalPluginLogs:        0,
		},
		PlayerCount:            []MetricPoint{},
		QueueCount:             []MetricPoint{},
		TickRate:               []MetricPoint{},
		ChatActivity:           []MetricPoint{},
		ConnectionStats:        []MetricPoint{},
		TeamkillStats:          []MetricPoint{},
		PlayerWoundedStats:     []MetricPoint{},
		PlayerRevivedStats:     []MetricPoint{},
		PlayerPossessStats:     []MetricPoint{},
		PlayerDiedStats:        []MetricPoint{},
		PlayerDamagedStats:     []MetricPoint{},
		DeployableDamagedStats: []MetricPoint{},
		AdminBroadcastStats:    []MetricPoint{},
	}

	// Query player count metrics from server info data (including public/reserved queue)
	playerCountQuery := `
		SELECT 
			toUnixTimestamp(toStartOfInterval(event_time, INTERVAL ? minute)) * 1000 as timestamp,
			avg(player_count) as avg_player_count,
			avg(public_queue) as avg_public_queue,
			avg(reserved_queue) as avg_reserved_queue
		FROM squad_aegis.server_info_metrics 
		WHERE server_id = ? 
		AND event_time >= ? 
		AND event_time <= ?
		GROUP BY toStartOfInterval(event_time, INTERVAL ? minute)
		ORDER BY timestamp ASC
	`

	// Calculate interval in minutes based on period for higher fidelity
	intervalMinutes := interval
	if interval <= 0 {
		switch period {
		case "1h":
			intervalMinutes = 5 // 5 minute intervals for 1 hour
		case "6h":
			intervalMinutes = 15 // 15 minute intervals for 6 hours
		case "24h":
			intervalMinutes = 60 // 1 hour intervals for 24 hours
		case "7d":
			intervalMinutes = 360 // 6 hour intervals for 7 days
		case "30d":
			intervalMinutes = 1440 // 24 hour intervals for 30 days
		default:
			intervalMinutes = 60
		}
	}

	rows, err := clickhouseClient.Query(ctx, playerCountQuery, intervalMinutes, serverId, startTime, now, intervalMinutes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query player count metrics from ClickHouse")
		return s.generateSampleMetrics(period, interval), nil
	}
	defer rows.Close()

	for rows.Next() {
		var timestamp int64
		var avgPlayerCount, avgPublicQueue, avgReservedQueue float64
		if err := rows.Scan(&timestamp, &avgPlayerCount, &avgPublicQueue, &avgReservedQueue); err != nil {
			log.Error().Err(err).Msg("Failed to scan player count metric")
			continue
		}
		metricsData.PlayerCount = append(metricsData.PlayerCount, MetricPoint{
			Timestamp: time.Unix(timestamp/1000, 0),
			Value: map[string]interface{}{
				"player_count":   int(avgPlayerCount),
				"public_queue":   int(avgPublicQueue),
				"reserved_queue": int(avgReservedQueue),
				"total_queue":    int(avgPublicQueue + avgReservedQueue),
			},
		})
	}

	// Query queue count metrics from server info data
	queueCountQuery := `
		SELECT 
			toUnixTimestamp(toStartOfInterval(event_time, INTERVAL ? minute)) * 1000 as timestamp,
			avg(public_queue + reserved_queue) as avg_queue_count
		FROM squad_aegis.server_info_metrics 
		WHERE server_id = ? 
		AND event_time >= ? 
		AND event_time <= ?
		GROUP BY toStartOfInterval(event_time, INTERVAL ? minute)
		ORDER BY timestamp ASC
	`

	rows, err = clickhouseClient.Query(ctx, queueCountQuery, intervalMinutes, serverId, startTime, now, intervalMinutes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query queue count metrics from ClickHouse")
	} else {
		defer rows.Close()
		for rows.Next() {
			var timestamp int64
			var avgQueueCount float64
			if err := rows.Scan(&timestamp, &avgQueueCount); err != nil {
				log.Error().Err(err).Msg("Failed to scan queue count metric")
				continue
			}
			metricsData.QueueCount = append(metricsData.QueueCount, MetricPoint{
				Timestamp: time.Unix(timestamp/1000, 0),
				Value:     int(avgQueueCount), // Convert to int for consistency
			})
		}
	}

	// Query tick rate metrics
	tickRateQuery := `
		SELECT 
			toUnixTimestamp(toStartOfInterval(event_time, INTERVAL ? minute)) * 1000 as timestamp,
			avg(tick_rate) as value
		FROM squad_aegis.server_tick_rate_events 
		WHERE server_id = ? 
		AND event_time >= ? 
		AND event_time <= ?
		GROUP BY toStartOfInterval(event_time, INTERVAL ? minute)
		ORDER BY timestamp ASC
	`

	rows, err = clickhouseClient.Query(ctx, tickRateQuery, intervalMinutes, serverId, startTime, now, intervalMinutes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query tick rate metrics from ClickHouse")
	} else {
		defer rows.Close()
		for rows.Next() {
			var timestamp int64
			var value float64
			if err := rows.Scan(&timestamp, &value); err != nil {
				log.Error().Err(err).Msg("Failed to scan tick rate metric")
				continue
			}
			metricsData.TickRate = append(metricsData.TickRate, MetricPoint{
				Timestamp: time.Unix(timestamp/1000, 0),
				Value:     value, // Keep as float64 for precise tick rate
			})
		}
	}

	// Query chat activity metrics
	chatQuery := `
		SELECT 
			toUnixTimestamp(toStartOfInterval(sent_at, INTERVAL ? minute)) * 1000 as timestamp,
			count(*) as value
		FROM squad_aegis.server_player_chat_messages 
		WHERE server_id = ? 
		AND sent_at >= ? 
		AND sent_at <= ?
		GROUP BY toStartOfInterval(sent_at, INTERVAL ? minute)
		ORDER BY timestamp ASC
	`

	// Create a map to store chat activity data
	chatDataMap := make(map[int64]int)
	rows, err = clickhouseClient.Query(ctx, chatQuery, intervalMinutes, serverId, startTime, now, intervalMinutes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query chat activity metrics from ClickHouse")
	} else {
		defer rows.Close()
		for rows.Next() {
			var timestamp int64
			var value int64
			if err := rows.Scan(&timestamp, &value); err != nil {
				log.Error().Err(err).Msg("Failed to scan chat activity metric")
				continue
			}
			chatDataMap[timestamp] = int(value)
		}
	}

	// Fill in the chat activity data with 0s for missing intervals
	metricsData.ChatActivity = fillTimeSeriesGaps(startTime, now, intervalMinutes, chatDataMap)

	// Query connection metrics - combine both connected and disconnected events
	connectionQuery := `
		WITH connection_events AS (
			SELECT 
				toUnixTimestamp(toStartOfInterval(event_time, INTERVAL ? minute)) * 1000 as timestamp,
				count(*) as connected_count,
				0 as disconnected_count
			FROM squad_aegis.server_player_connected_events 
			WHERE server_id = ? 
			AND event_time >= ? 
			AND event_time <= ?
			GROUP BY toStartOfInterval(event_time, INTERVAL ? minute)
			
			UNION ALL
			
			SELECT 
				toUnixTimestamp(toStartOfInterval(event_time, INTERVAL ? minute)) * 1000 as timestamp,
				0 as connected_count,
				count(*) as disconnected_count
			FROM squad_aegis.server_player_disconnected_events 
			WHERE server_id = ? 
			AND event_time >= ? 
			AND event_time <= ?
			GROUP BY toStartOfInterval(event_time, INTERVAL ? minute)
		)
		SELECT 
			timestamp,
			sum(connected_count) as total_connections,
			sum(disconnected_count) as total_disconnections,
			sum(connected_count) + sum(disconnected_count) as total_activity
		FROM connection_events
		GROUP BY timestamp
		ORDER BY timestamp ASC
	`

	// Create a map to store connection data
	connectionDataMap := make(map[int64]int)
	rows, err = clickhouseClient.Query(ctx, connectionQuery,
		intervalMinutes, serverId, startTime, now, intervalMinutes,
		intervalMinutes, serverId, startTime, now, intervalMinutes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query connection metrics from ClickHouse")
	} else {
		defer rows.Close()
		for rows.Next() {
			var timestamp int64
			var totalConnections, totalDisconnections, totalActivity int64
			if err := rows.Scan(&timestamp, &totalConnections, &totalDisconnections, &totalActivity); err != nil {
				log.Error().Err(err).Msg("Failed to scan connection metric")
				continue
			}
			// Store total activity (connections + disconnections) as the main metric
			connectionDataMap[timestamp] = int(totalActivity)
		}
	}

	// Fill in the connection data with 0s for missing intervals
	metricsData.ConnectionStats = fillTimeSeriesGaps(startTime, now, intervalMinutes, connectionDataMap)

	// Query teamkill metrics
	teamkillQuery := `
		SELECT 
			toUnixTimestamp(toStartOfInterval(event_time, INTERVAL ? minute)) * 1000 as timestamp,
			count(*) as value
		FROM squad_aegis.server_player_died_events 
		WHERE server_id = ? 
		AND event_time >= ? 
		AND event_time <= ?
		AND teamkill = 1
		GROUP BY toStartOfInterval(event_time, INTERVAL ? minute)
		ORDER BY timestamp ASC
	`

	// Create a map to store teamkill data
	teamkillDataMap := make(map[int64]int)
	rows, err = clickhouseClient.Query(ctx, teamkillQuery, intervalMinutes, serverId, startTime, now, intervalMinutes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query teamkill metrics from ClickHouse")
	} else {
		defer rows.Close()
		for rows.Next() {
			var timestamp int64
			var value int64
			if err := rows.Scan(&timestamp, &value); err != nil {
				log.Error().Err(err).Msg("Failed to scan teamkill metric")
				continue
			}
			teamkillDataMap[timestamp] = int(value)
		}
	}

	// Fill in the teamkill data with 0s for missing intervals
	metricsData.TeamkillStats = fillTimeSeriesGaps(startTime, now, intervalMinutes, teamkillDataMap)

	// Query player wounded metrics
	playerWoundedQuery := `
		SELECT 
			toUnixTimestamp(toStartOfInterval(event_time, INTERVAL ? minute)) * 1000 as timestamp,
			count(*) as value
		FROM squad_aegis.server_player_wounded_events 
		WHERE server_id = ? 
		AND event_time >= ? 
		AND event_time <= ?
		GROUP BY toStartOfInterval(event_time, INTERVAL ? minute)
		ORDER BY timestamp ASC
	`

	// Create a map to store player wounded data
	playerWoundedDataMap := make(map[int64]int)
	rows, err = clickhouseClient.Query(ctx, playerWoundedQuery, intervalMinutes, serverId, startTime, now, intervalMinutes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query player wounded metrics from ClickHouse")
	} else {
		defer rows.Close()
		for rows.Next() {
			var timestamp int64
			var value int64
			if err := rows.Scan(&timestamp, &value); err != nil {
				log.Error().Err(err).Msg("Failed to scan player wounded metric")
				continue
			}
			playerWoundedDataMap[timestamp] = int(value)
		}
	}

	// Fill in the player wounded data with 0s for missing intervals
	metricsData.PlayerWoundedStats = fillTimeSeriesGaps(startTime, now, intervalMinutes, playerWoundedDataMap)

	// Query player revived metrics
	playerRevivedQuery := `
		SELECT 
			toUnixTimestamp(toStartOfInterval(event_time, INTERVAL ? minute)) * 1000 as timestamp,
			count(*) as value
		FROM squad_aegis.server_player_revived_events 
		WHERE server_id = ? 
		AND event_time >= ? 
		AND event_time <= ?
		GROUP BY toStartOfInterval(event_time, INTERVAL ? minute)
		ORDER BY timestamp ASC
	`

	// Create a map to store player revived data
	playerRevivedDataMap := make(map[int64]int)
	rows, err = clickhouseClient.Query(ctx, playerRevivedQuery, intervalMinutes, serverId, startTime, now, intervalMinutes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query player revived metrics from ClickHouse")
	} else {
		defer rows.Close()
		for rows.Next() {
			var timestamp int64
			var value int64
			if err := rows.Scan(&timestamp, &value); err != nil {
				log.Error().Err(err).Msg("Failed to scan player revived metric")
				continue
			}
			playerRevivedDataMap[timestamp] = int(value)
		}
	}

	// Fill in the player revived data with 0s for missing intervals
	metricsData.PlayerRevivedStats = fillTimeSeriesGaps(startTime, now, intervalMinutes, playerRevivedDataMap)

	// Query player possess metrics
	playerPossessQuery := `
		SELECT 
			toUnixTimestamp(toStartOfInterval(event_time, INTERVAL ? minute)) * 1000 as timestamp,
			count(*) as value
		FROM squad_aegis.server_player_possess_events 
		WHERE server_id = ? 
		AND event_time >= ? 
		AND event_time <= ?
		GROUP BY toStartOfInterval(event_time, INTERVAL ? minute)
		ORDER BY timestamp ASC
	`

	// Create a map to store player possess data
	playerPossessDataMap := make(map[int64]int)
	rows, err = clickhouseClient.Query(ctx, playerPossessQuery, intervalMinutes, serverId, startTime, now, intervalMinutes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query player possess metrics from ClickHouse")
	} else {
		defer rows.Close()
		for rows.Next() {
			var timestamp int64
			var value int64
			if err := rows.Scan(&timestamp, &value); err != nil {
				log.Error().Err(err).Msg("Failed to scan player possess metric")
				continue
			}
			playerPossessDataMap[timestamp] = int(value)
		}
	}

	// Fill in the player possess data with 0s for missing intervals
	metricsData.PlayerPossessStats = fillTimeSeriesGaps(startTime, now, intervalMinutes, playerPossessDataMap)

	// Query player died metrics (non-teamkill deaths)
	playerDiedQuery := `
		SELECT 
			toUnixTimestamp(toStartOfInterval(event_time, INTERVAL ? minute)) * 1000 as timestamp,
			count(*) as value
		FROM squad_aegis.server_player_died_events 
		WHERE server_id = ? 
		AND event_time >= ? 
		AND event_time <= ?
		GROUP BY toStartOfInterval(event_time, INTERVAL ? minute)
		ORDER BY timestamp ASC
	`

	// Create a map to store player died data
	playerDiedDataMap := make(map[int64]int)
	rows, err = clickhouseClient.Query(ctx, playerDiedQuery, intervalMinutes, serverId, startTime, now, intervalMinutes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query player died metrics from ClickHouse")
	} else {
		defer rows.Close()
		for rows.Next() {
			var timestamp int64
			var value int64
			if err := rows.Scan(&timestamp, &value); err != nil {
				log.Error().Err(err).Msg("Failed to scan player died metric")
				continue
			}
			playerDiedDataMap[timestamp] = int(value)
		}
	}

	// Fill in the player died data with 0s for missing intervals
	metricsData.PlayerDiedStats = fillTimeSeriesGaps(startTime, now, intervalMinutes, playerDiedDataMap)

	// Query player damaged metrics
	playerDamagedQuery := `
		SELECT 
			toUnixTimestamp(toStartOfInterval(event_time, INTERVAL ? minute)) * 1000 as timestamp,
			count(*) as value
		FROM squad_aegis.server_player_damaged_events 
		WHERE server_id = ? 
		AND event_time >= ? 
		AND event_time <= ?
		GROUP BY toStartOfInterval(event_time, INTERVAL ? minute)
		ORDER BY timestamp ASC
	`

	// Create a map to store player damaged data
	playerDamagedDataMap := make(map[int64]int)
	rows, err = clickhouseClient.Query(ctx, playerDamagedQuery, intervalMinutes, serverId, startTime, now, intervalMinutes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query player damaged metrics from ClickHouse")
	} else {
		defer rows.Close()
		for rows.Next() {
			var timestamp int64
			var value int64
			if err := rows.Scan(&timestamp, &value); err != nil {
				log.Error().Err(err).Msg("Failed to scan player damaged metric")
				continue
			}
			playerDamagedDataMap[timestamp] = int(value)
		}
	}

	// Fill in the player damaged data with 0s for missing intervals
	metricsData.PlayerDamagedStats = fillTimeSeriesGaps(startTime, now, intervalMinutes, playerDamagedDataMap)

	// Query deployable damaged metrics
	deployableDamagedQuery := `
		SELECT 
			toUnixTimestamp(toStartOfInterval(event_time, INTERVAL ? minute)) * 1000 as timestamp,
			count(*) as value
		FROM squad_aegis.server_deployable_damaged_events 
		WHERE server_id = ? 
		AND event_time >= ? 
		AND event_time <= ?
		GROUP BY toStartOfInterval(event_time, INTERVAL ? minute)
		ORDER BY timestamp ASC
	`

	// Create a map to store deployable damaged data
	deployableDamagedDataMap := make(map[int64]int)
	rows, err = clickhouseClient.Query(ctx, deployableDamagedQuery, intervalMinutes, serverId, startTime, now, intervalMinutes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query deployable damaged metrics from ClickHouse")
	} else {
		defer rows.Close()
		for rows.Next() {
			var timestamp int64
			var value int64
			if err := rows.Scan(&timestamp, &value); err != nil {
				log.Error().Err(err).Msg("Failed to scan deployable damaged metric")
				continue
			}
			deployableDamagedDataMap[timestamp] = int(value)
		}
	}

	// Fill in the deployable damaged data with 0s for missing intervals
	metricsData.DeployableDamagedStats = fillTimeSeriesGaps(startTime, now, intervalMinutes, deployableDamagedDataMap)

	// Query admin broadcast metrics
	adminBroadcastQuery := `
		SELECT 
			toUnixTimestamp(toStartOfInterval(event_time, INTERVAL ? minute)) * 1000 as timestamp,
			count(*) as value
		FROM squad_aegis.server_admin_broadcast_events 
		WHERE server_id = ? 
		AND event_time >= ? 
		AND event_time <= ?
		GROUP BY toStartOfInterval(event_time, INTERVAL ? minute)
		ORDER BY timestamp ASC
	`

	// Create a map to store admin broadcast data
	adminBroadcastDataMap := make(map[int64]int)
	rows, err = clickhouseClient.Query(ctx, adminBroadcastQuery, intervalMinutes, serverId, startTime, now, intervalMinutes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query admin broadcast metrics from ClickHouse")
	} else {
		defer rows.Close()
		for rows.Next() {
			var timestamp int64
			var value int64
			if err := rows.Scan(&timestamp, &value); err != nil {
				log.Error().Err(err).Msg("Failed to scan admin broadcast metric")
				continue
			}
			adminBroadcastDataMap[timestamp] = int(value)
		}
	}

	// Fill in the admin broadcast data with 0s for missing intervals
	metricsData.AdminBroadcastStats = fillTimeSeriesGaps(startTime, now, intervalMinutes, adminBroadcastDataMap)

	// Query rounds data from unified game events (ROUND_ENDED events)
	roundsQuery := `
		SELECT 
			toUnixTimestamp(toStartOfInterval(event_time, INTERVAL ? minute)) * 1000 as timestamp,
			count(*) as value
		FROM squad_aegis.server_game_events_unified 
		WHERE server_id = ? 
		AND event_time >= ? 
		AND event_time <= ?
		AND event_type = 'ROUND_ENDED'
		GROUP BY toStartOfInterval(event_time, INTERVAL ? minute)
		ORDER BY timestamp ASC
	`

	// Create a map to store rounds data
	roundsDataMap := make(map[int64]int)
	rows, err = clickhouseClient.Query(ctx, roundsQuery, intervalMinutes, serverId, startTime, now, intervalMinutes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query rounds metrics from ClickHouse")
	} else {
		defer rows.Close()
		for rows.Next() {
			var timestamp int64
			var value int64
			if err := rows.Scan(&timestamp, &value); err != nil {
				log.Error().Err(err).Msg("Failed to scan rounds metric")
				continue
			}
			roundsDataMap[timestamp] = int(value)
		}
	}

	// Fill in the rounds data with 0s for missing intervals
	metricsData.Rounds = fillTimeSeriesGaps(startTime, now, intervalMinutes, roundsDataMap)

	// Query map data from ROUND_ENDED events to get completed games
	mapsQuery := `
		SELECT 
			event_time,
			layer,
			winner
		FROM squad_aegis.server_game_events_unified 
		WHERE server_id = ? 
		AND event_time >= ? 
		AND event_time <= ?
		AND event_type = 'ROUND_ENDED'
		AND layer IS NOT NULL
		AND layer != ''
		ORDER BY event_time DESC
		LIMIT 10
	`

	rows, err = clickhouseClient.Query(ctx, mapsQuery, serverId, startTime, now)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query maps data from ClickHouse")
	} else {
		defer rows.Close()
		for rows.Next() {
			var eventTime time.Time
			var layer, winner string
			if err := rows.Scan(&eventTime, &layer, &winner); err != nil {
				log.Error().Err(err).Msg("Failed to scan map data")
				continue
			}
			metricsData.Maps = append(metricsData.Maps, MetricPoint{
				Timestamp: eventTime,
				Value: map[string]interface{}{
					"layer":  layer,
					"winner": winner,
				},
			})
		}
	}

	// Calculate summary metrics
	if len(metricsData.PlayerCount) > 0 {
		// Current players is the last data point
		if lastPoint := metricsData.PlayerCount[len(metricsData.PlayerCount)-1]; lastPoint.Value != nil {
			if playerData, ok := lastPoint.Value.(map[string]interface{}); ok {
				if playerCount, ok := playerData["player_count"].(int); ok {
					metricsData.Summary.TotalPlayers = playerCount
				}
			}
		}
	}

	if len(metricsData.TickRate) > 0 {
		// Average tick rate
		var total float64
		var count int
		for _, point := range metricsData.TickRate {
			if point.Value != nil {
				if tickRate, ok := point.Value.(float64); ok {
					total += tickRate
					count++
				}
			}
		}
		if count > 0 {
			metricsData.Summary.AvgTickRate = total / float64(count)
		}
	}

	if len(metricsData.ChatActivity) > 0 {
		// Total chat messages
		for _, point := range metricsData.ChatActivity {
			if point.Value != nil {
				if chatCount, ok := point.Value.(int); ok {
					metricsData.Summary.TotalChatMessages += chatCount
				}
			}
		}
	}

	if len(metricsData.ConnectionStats) > 0 {
		// Total connection activity (connections + disconnections)
		for _, point := range metricsData.ConnectionStats {
			if point.Value != nil {
				if connCount, ok := point.Value.(int); ok {
					metricsData.Summary.TotalConnections += connCount
				}
			}
		}
	}

	if len(metricsData.TeamkillStats) > 0 {
		// Total teamkills
		for _, point := range metricsData.TeamkillStats {
			if point.Value != nil {
				if tkCount, ok := point.Value.(int); ok {
					metricsData.Summary.TotalTeamkills += tkCount
				}
			}
		}
	}

	if len(metricsData.PlayerWoundedStats) > 0 {
		// Total player wounded events
		for _, point := range metricsData.PlayerWoundedStats {
			if point.Value != nil {
				if woundedCount, ok := point.Value.(int); ok {
					metricsData.Summary.TotalPlayerWounded += woundedCount
				}
			}
		}
	}

	if len(metricsData.PlayerRevivedStats) > 0 {
		// Total player revived events
		for _, point := range metricsData.PlayerRevivedStats {
			if point.Value != nil {
				if revivedCount, ok := point.Value.(int); ok {
					metricsData.Summary.TotalPlayerRevived += revivedCount
				}
			}
		}
	}

	if len(metricsData.PlayerPossessStats) > 0 {
		// Total player possess events
		for _, point := range metricsData.PlayerPossessStats {
			if point.Value != nil {
				if possessCount, ok := point.Value.(int); ok {
					metricsData.Summary.TotalPlayerPossess += possessCount
				}
			}
		}
	}

	if len(metricsData.PlayerDiedStats) > 0 {
		// Total player died events
		for _, point := range metricsData.PlayerDiedStats {
			if point.Value != nil {
				if diedCount, ok := point.Value.(int); ok {
					metricsData.Summary.TotalPlayerDied += diedCount
				}
			}
		}
	}

	if len(metricsData.PlayerDamagedStats) > 0 {
		// Total player damaged events
		for _, point := range metricsData.PlayerDamagedStats {
			if point.Value != nil {
				if damagedCount, ok := point.Value.(int); ok {
					metricsData.Summary.TotalPlayerDamaged += damagedCount
				}
			}
		}
	}

	if len(metricsData.DeployableDamagedStats) > 0 {
		// Total deployable damaged events
		for _, point := range metricsData.DeployableDamagedStats {
			if point.Value != nil {
				if deployableDamagedCount, ok := point.Value.(int); ok {
					metricsData.Summary.TotalDeployableDamaged += deployableDamagedCount
				}
			}
		}
	}

	if len(metricsData.AdminBroadcastStats) > 0 {
		// Total admin broadcast events
		for _, point := range metricsData.AdminBroadcastStats {
			if point.Value != nil {
				if broadcastCount, ok := point.Value.(int); ok {
					metricsData.Summary.TotalAdminBroadcasts += broadcastCount
				}
			}
		}
	}

	if len(metricsData.Rounds) > 0 {
		// Total rounds
		for _, point := range metricsData.Rounds {
			if point.Value != nil {
				if roundCount, ok := point.Value.(int); ok {
					metricsData.Summary.TotalRounds += roundCount
				}
			}
		}
	}

	// Calculate peak player count and most played map
	var peakPlayerCount int
	mapCounts := make(map[string]int)

	if len(metricsData.PlayerCount) > 0 {
		for _, point := range metricsData.PlayerCount {
			if point.Value != nil {
				if playerData, ok := point.Value.(map[string]interface{}); ok {
					if playerCount, ok := playerData["player_count"].(int); ok {
						if playerCount > peakPlayerCount {
							peakPlayerCount = playerCount
						}
					}
				}
			}
		}
	}

	if len(metricsData.Maps) > 0 {
		for _, point := range metricsData.Maps {
			if point.Value != nil {
				if mapData, ok := point.Value.(map[string]interface{}); ok {
					if mapName, ok := mapData["layer"].(string); ok && mapName != "" {
						mapCounts[mapName]++
					}
				} else if mapName, ok := point.Value.(string); ok && mapName != "" {
					// Backward compatibility for old format
					mapCounts[mapName]++
				}
			}
		}
	}

	// Find most played map
	var mostPlayedMap string
	var maxCount int
	for mapName, count := range mapCounts {
		if count > maxCount {
			maxCount = count
			mostPlayedMap = mapName
		}
	}

	metricsData.Summary.PeakPlayerCount = peakPlayerCount
	metricsData.Summary.MostPlayedMap = mostPlayedMap

	// If no real data was found, fall back to sample data
	if len(metricsData.PlayerCount) == 0 && len(metricsData.TickRate) == 0 && len(metricsData.ChatActivity) == 0 {
		log.Warn().Msg("No metrics data found in ClickHouse, using sample data")
		return s.generateSampleMetrics(period, interval), nil
	}

	return metricsData, nil
}

// generateSampleMetrics generates sample metrics data for demonstration
func (s *Server) generateSampleMetrics(period string, interval int) ServerMetricsData {
	now := time.Now()
	var startTime time.Time
	var points int

	// Use same high-fidelity interval logic as real data
	actualInterval := interval
	if interval <= 0 {
		switch period {
		case "1h":
			actualInterval = 5 // 5 minute intervals for 1 hour
		case "6h":
			actualInterval = 15 // 15 minute intervals for 6 hours
		case "24h":
			actualInterval = 60 // 1 hour intervals for 24 hours
		case "7d":
			actualInterval = 360 // 6 hour intervals for 7 days
		case "30d":
			actualInterval = 1440 // 24 hour intervals for 30 days
		default:
			actualInterval = 60
		}
	}

	switch period {
	case "1h":
		startTime = now.Add(-1 * time.Hour)
		points = 60 / actualInterval // Every actualInterval minutes for 1 hour
	case "6h":
		startTime = now.Add(-6 * time.Hour)
		points = 6 * 60 / actualInterval // Every actualInterval minutes for 6 hours
	case "7d":
		startTime = now.AddDate(0, 0, -7)
		points = 7 * 24 * 60 / actualInterval // Every actualInterval minutes for 7 days
	case "30d":
		startTime = now.AddDate(0, 0, -30)
		points = 30 * 24 * 60 / actualInterval // Every actualInterval minutes for 30 days
	default: // 24h
		startTime = now.Add(-24 * time.Hour)
		points = 24 * 60 / actualInterval // Every actualInterval minutes for 24 hours
	}

	if points > 1000 {
		points = 1000 // Limit to prevent too much data
	}

	playerCount := make([]MetricPoint, 0, points)
	tickRate := make([]MetricPoint, 0, points)
	rounds := make([]MetricPoint, 0, points)
	maps := make([]MetricPoint, 0, points)
	chatActivity := make([]MetricPoint, 0, points)
	connectionStats := make([]MetricPoint, 0, points)
	teamkillStats := make([]MetricPoint, 0, points)
	playerWoundedStats := make([]MetricPoint, 0, points)
	playerRevivedStats := make([]MetricPoint, 0, points)
	playerPossessStats := make([]MetricPoint, 0, points)
	playerDiedStats := make([]MetricPoint, 0, points)
	playerDamagedStats := make([]MetricPoint, 0, points)
	deployableDamagedStats := make([]MetricPoint, 0, points)
	adminBroadcastStats := make([]MetricPoint, 0, points)

	for i := 0; i < points; i++ {
		timestamp := startTime.Add(time.Duration(i) * time.Duration(actualInterval) * time.Minute)

		// Generate realistic sample data
		basePlayerCount := 40 + int(20*sin(float64(i)*0.1)) // Simulate player fluctuation
		playerCount = append(playerCount, MetricPoint{
			Timestamp: timestamp,
			Value:     basePlayerCount + randomInt(-5, 5),
		})

		tickRate = append(tickRate, MetricPoint{
			Timestamp: timestamp,
			Value:     45.0 + randomFloat(-10, 10), // TPS around 45
		})

		// Rounds (fewer data points)
		if i%120 == 0 { // Every 2 hours
			rounds = append(rounds, MetricPoint{
				Timestamp: timestamp,
				Value:     1, // One round completed
			})
		}

		// Maps (when rounds change)
		if i%120 == 0 {
			mapNames := []string{"Tallil", "Yehorivka", "Gorodok", "Kohat", "Sumari", "Logar"}
			maps = append(maps, MetricPoint{
				Timestamp: timestamp,
				Value:     mapNames[i/120%len(mapNames)],
			})
		}

		// Chat activity (messages per interval) - more realistic
		chatValue := 0
		if randomInt(0, 100) < 70 { // 70% chance of having chat messages
			chatValue = randomInt(1, 8)
		}
		chatActivity = append(chatActivity, MetricPoint{
			Timestamp: timestamp,
			Value:     chatValue,
		})

		// Connection stats (connection activity: connects + disconnects per interval)
		connectionValue := 0
		if randomInt(0, 100) < 40 { // 40% chance of connection activity in any given interval
			connectionValue = randomInt(1, 4) // Include both connects and disconnects
		}
		connectionStats = append(connectionStats, MetricPoint{
			Timestamp: timestamp,
			Value:     connectionValue,
		})

		// Teamkill stats (teamkills per interval) - should be rare
		teamkillValue := 0
		if randomInt(0, 100) < 10 { // Only 10% chance of teamkills
			teamkillValue = 1
		}
		teamkillStats = append(teamkillStats, MetricPoint{
			Timestamp: timestamp,
			Value:     teamkillValue,
		})

		// Player wounded stats (wounded per interval)
		woundedValue := 0
		if randomInt(0, 100) < 60 { // 60% chance of wounded events
			woundedValue = randomInt(1, 8)
		}
		playerWoundedStats = append(playerWoundedStats, MetricPoint{
			Timestamp: timestamp,
			Value:     woundedValue,
		})

		// Player revived stats (revived per interval)
		revivedValue := 0
		if randomInt(0, 100) < 50 { // 50% chance of revived events
			revivedValue = randomInt(1, 6)
		}
		playerRevivedStats = append(playerRevivedStats, MetricPoint{
			Timestamp: timestamp,
			Value:     revivedValue,
		})

		// Player possess stats (possessions per interval)
		possessValue := 0
		if randomInt(0, 100) < 30 { // 30% chance of possess events
			possessValue = randomInt(1, 4)
		}
		playerPossessStats = append(playerPossessStats, MetricPoint{
			Timestamp: timestamp,
			Value:     possessValue,
		})

		// Player died stats (deaths per interval)
		diedValue := 0
		if randomInt(0, 100) < 80 { // 80% chance of death events
			diedValue = randomInt(1, 10)
		}
		playerDiedStats = append(playerDiedStats, MetricPoint{
			Timestamp: timestamp,
			Value:     diedValue,
		})

		// Player damaged stats (damage events per interval)
		damagedValue := 0
		if randomInt(0, 100) < 90 { // 90% chance of damage events
			damagedValue = randomInt(1, 25)
		}
		playerDamagedStats = append(playerDamagedStats, MetricPoint{
			Timestamp: timestamp,
			Value:     damagedValue,
		})

		// Deployable damaged stats (deployable damage per interval)
		deployableDamagedValue := 0
		if randomInt(0, 100) < 20 { // Only 20% chance of deployable damage
			deployableDamagedValue = randomInt(1, 2)
		}
		deployableDamagedStats = append(deployableDamagedStats, MetricPoint{
			Timestamp: timestamp,
			Value:     deployableDamagedValue,
		})

		// Admin broadcast stats (broadcasts per interval) - should be rare
		broadcastValue := 0
		if randomInt(0, 100) < 5 { // Only 5% chance of admin broadcasts
			broadcastValue = 1
		}
		adminBroadcastStats = append(adminBroadcastStats, MetricPoint{
			Timestamp: timestamp,
			Value:     broadcastValue,
		})
	}

	summary := MetricsSummary{
		TotalPlayers:           80,
		AvgTickRate:            45.2,
		TotalRounds:            12,
		UniquePlayersCount:     156,
		TotalChatMessages:      1240,
		TotalConnections:       89,
		TotalTeamkills:         23,
		TotalPlayerWounded:     145,
		TotalPlayerRevived:     98,
		TotalPlayerPossess:     67,
		TotalPlayerDied:        189,
		TotalPlayerDamaged:     456,
		TotalDeployableDamaged: 34,
		TotalAdminBroadcasts:   12,
		TotalPluginLogs:        2340,
		MostPlayedMap:          "Tallil Outskirts",
		PeakPlayerCount:        78,
	}

	return ServerMetricsData{
		PlayerCount:            playerCount,
		TickRate:               tickRate,
		Rounds:                 rounds,
		Maps:                   maps,
		ChatActivity:           chatActivity,
		ConnectionStats:        connectionStats,
		TeamkillStats:          teamkillStats,
		PlayerWoundedStats:     playerWoundedStats,
		PlayerRevivedStats:     playerRevivedStats,
		PlayerPossessStats:     playerPossessStats,
		PlayerDiedStats:        playerDiedStats,
		PlayerDamagedStats:     playerDamagedStats,
		DeployableDamagedStats: deployableDamagedStats,
		AdminBroadcastStats:    adminBroadcastStats,
		Period:                 period,
		Summary:                summary,
	}
}

// Helper functions for sample data generation
func sin(x float64) float64 {
	// Simple sine approximation
	return x - (x*x*x)/6 + (x*x*x*x*x)/120
}

func randomInt(min, max int) int {
	// Better random number generation with more realistic distribution
	if min >= max {
		return min
	}
	now := time.Now().UnixNano()
	// Use a combination of time and index to get better distribution
	return min + int((now/1000000)%int64(max-min+1))
}

func randomFloat(min, max float64) float64 {
	// Better random float generation
	if min >= max {
		return min
	}
	now := time.Now().UnixNano()
	ratio := float64((now/1000)%1000) / 1000.0
	return min + ratio*(max-min)
}

// fillTimeSeriesGaps fills in missing time intervals with zero values
func fillTimeSeriesGaps(startTime, endTime time.Time, intervalMinutes int, dataMap map[int64]int) []MetricPoint {
	var points []MetricPoint

	// Calculate the time step for intervals
	interval := time.Duration(intervalMinutes) * time.Minute

	// Round start time to the nearest interval boundary using the same logic as ClickHouse
	// ClickHouse's toStartOfInterval truncates to the interval boundary
	startTimestamp := startTime.Truncate(interval)

	// Ensure we don't go beyond the end time
	endTimestamp := endTime.Truncate(interval)
	if endTime.After(endTimestamp) {
		endTimestamp = endTimestamp.Add(interval)
	}

	// Iterate through all possible time intervals
	for current := startTimestamp; current.Before(endTimestamp) || current.Equal(endTimestamp); current = current.Add(interval) {
		// Convert to milliseconds timestamp (same as ClickHouse query)
		timestamp := current.Unix() * 1000

		// Check if we have data for this timestamp, try a few variations due to potential rounding differences
		value := 0

		// Try exact match first
		if val, exists := dataMap[timestamp]; exists {
			value = val
		} else {
			// Try with small tolerance (±1 second) for potential rounding differences
			for tolerance := int64(-1000); tolerance <= 1000; tolerance += 1000 {
				if val, exists := dataMap[timestamp+tolerance]; exists {
					value = val
					break
				}
			}
		}

		points = append(points, MetricPoint{
			Timestamp: current,
			Value:     value,
		})
	}

	return points
}
