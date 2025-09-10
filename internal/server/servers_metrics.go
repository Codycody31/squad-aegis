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
	PlayerCount            []MetricPoint `json:"player_count"`
	TickRate               []MetricPoint `json:"tick_rate"`
	Rounds                 []MetricPoint `json:"rounds"`
	Maps                   []MetricPoint `json:"maps"`
	ChatActivity           []MetricPoint `json:"chat_activity"`
	ConnectionStats        []MetricPoint `json:"connection_stats"`
	TeamkillStats          []MetricPoint `json:"teamkill_stats"`
	PlayerWoundedStats     []MetricPoint `json:"player_wounded_stats"`
	PlayerRevivedStats     []MetricPoint `json:"player_revived_stats"`
	PlayerPossessStats     []MetricPoint `json:"player_possess_stats"`
	PlayerDiedStats        []MetricPoint `json:"player_died_stats"`
	PlayerDamagedStats     []MetricPoint `json:"player_damaged_stats"`
	DeployableDamagedStats []MetricPoint `json:"deployable_damaged_stats"`
	AdminBroadcastStats    []MetricPoint `json:"admin_broadcast_stats"`
	Period                 string        `json:"period"`
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
	clickhouseClient := s.Dependencies.PluginManager.GetClickHouseClient()

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
		Period:                 period,
		PlayerCount:            []MetricPoint{},
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

	// Query player count metrics - using connection events to estimate player count
	playerCountQuery := `
		SELECT
			toUnixTimestamp(toStartOfInterval(event_time, INTERVAL ? minute)) * 1000 as timestamp,
			sum(if(chain_id LIKE '%Connected%', 1, -1)) as net_change
		FROM squad_aegis.server_player_connected_events
		WHERE server_id = ?
		AND event_time >= toDateTime64(?, 3, 'UTC')
		AND event_time <= toDateTime64(?, 3, 'UTC')
		GROUP BY timestamp, toStartOfInterval(event_time, INTERVAL ? minute)
		ORDER BY timestamp ASC
	`

	// Calculate interval in minutes based on period for higher fidelity
	intervalMinutes := interval
	if interval <= 0 {
		switch period {
		case "1h":
			intervalMinutes = 2
		case "6h":
			intervalMinutes = 5
		case "24h":
			intervalMinutes = 15
		case "7d":
			intervalMinutes = 120
		case "30d":
			intervalMinutes = 360
		default:
			intervalMinutes = 60
		}
	}

	rows, err := clickhouseClient.Query(ctx, playerCountQuery, intervalMinutes, serverId, startTime, now, intervalMinutes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query player count metrics from ClickHouse")
		return metricsData, err
	}
	defer rows.Close()

	playerCount := 0 // Running total
	for rows.Next() {
		var timestamp int64
		var netChange int
		if err := rows.Scan(&timestamp, &netChange); err != nil {
			log.Error().Err(err).Msg("Failed to scan player count metric")
			continue
		}
		playerCount += netChange
		if playerCount < 0 {
			playerCount = 0 // Prevent negative player counts
		}
		metricsData.PlayerCount = append(metricsData.PlayerCount, MetricPoint{
			Timestamp: time.UnixMilli(timestamp),
			Value:     playerCount,
		})
	}

	// Query tick rate metrics
	tickRateQuery := `
		SELECT
			toUnixTimestamp(toStartOfInterval(event_time, INTERVAL ? minute)) * 1000 as timestamp,
			avg(tick_rate) as value
		FROM squad_aegis.server_tick_rate_events
		WHERE server_id = ?
		AND event_time >= toDateTime64(?, 3, 'UTC')
		AND event_time <= toDateTime64(?, 3, 'UTC')
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
				Timestamp: time.UnixMilli(timestamp),
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
		AND sent_at >= toDateTime64(?, 3, 'UTC')
		AND sent_at <= toDateTime64(?, 3, 'UTC')
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

	// Query connection metrics
	connectionQuery := `
		SELECT
			toUnixTimestamp(toStartOfInterval(event_time, INTERVAL ? minute)) * 1000 as timestamp,
			count(*) as connections
		FROM squad_aegis.server_player_connected_events
		WHERE server_id = ?
		AND event_time >= toDateTime64(?, 3, 'UTC')
		AND event_time <= toDateTime64(?, 3, 'UTC')
		GROUP BY toStartOfInterval(event_time, INTERVAL ? minute)
		ORDER BY timestamp ASC
	`

	rows, err = clickhouseClient.Query(ctx, connectionQuery, intervalMinutes, serverId, startTime, now, intervalMinutes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query connection metrics from ClickHouse")
	} else {
		defer rows.Close()
		for rows.Next() {
			var timestamp int64
			var connections int64
			if err := rows.Scan(&timestamp, &connections); err != nil {
				log.Error().Err(err).Msg("Failed to scan connection metric")
				continue
			}
			// Use connections count as the main metric value
			metricsData.ConnectionStats = append(metricsData.ConnectionStats, MetricPoint{
				Timestamp: time.UnixMilli(timestamp),
				Value:     int(connections),
			})
		}
	}

	// Query teamkill metrics
	teamkillQuery := `
		SELECT
			toUnixTimestamp(toStartOfInterval(event_time, INTERVAL ? minute)) * 1000 as timestamp,
			count(*) as value
		FROM squad_aegis.server_player_died_events
		WHERE server_id = ?
		AND event_time >= toDateTime64(?, 3, 'UTC')
		AND event_time <= toDateTime64(?, 3, 'UTC')
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
		AND event_time >= toDateTime64(?, 3, 'UTC')
		AND event_time <= toDateTime64(?, 3, 'UTC')
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
		AND event_time >= toDateTime64(?, 3, 'UTC')
		AND event_time <= toDateTime64(?, 3, 'UTC')
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
		AND event_time >= toDateTime64(?, 3, 'UTC')
		AND event_time <= toDateTime64(?, 3, 'UTC')
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
		AND event_time >= toDateTime64(?, 3, 'UTC')
		AND event_time <= toDateTime64(?, 3, 'UTC')
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
		AND event_time >= toDateTime64(?, 3, 'UTC')
		AND event_time <= toDateTime64(?, 3, 'UTC')
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
		AND event_time >= toDateTime64(?, 3, 'UTC')
		AND event_time <= toDateTime64(?, 3, 'UTC')
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
		AND event_time >= toDateTime64(?, 3, 'UTC')
		AND event_time <= toDateTime64(?, 3, 'UTC')
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

	return metricsData, nil
}

// fillTimeSeriesGaps fills in missing time intervals with zero values
func fillTimeSeriesGaps(startTime, endTime time.Time, intervalMinutes int, dataMap map[int64]int) []MetricPoint {
	var points []MetricPoint

	// Calculate the time step for intervals
	interval := time.Duration(intervalMinutes) * time.Minute

	// Round start time DOWN to the nearest interval boundary to ensure we include all data
	// from the requested start time, not just from the next interval boundary
	startTimestamp := startTime.Truncate(interval)

	// But if the truncated start time is after the original start time,
	// go back one interval to ensure we include the full requested range
	if startTimestamp.After(startTime) {
		startTimestamp = startTimestamp.Add(-interval)
	}

	// Round end time UP to the nearest interval boundary to ensure we include all data
	// up to the requested end time
	endTimestamp := endTime.Truncate(interval)
	if !endTime.Equal(endTimestamp) {
		endTimestamp = endTimestamp.Add(interval)
	}

	// Iterate through all possible time intervals
	for current := startTimestamp; current.Before(endTimestamp) || current.Equal(endTimestamp); current = current.Add(interval) {
		// Skip this interval if it's before the requested start time
		if current.Before(startTime) {
			continue
		}

		// Skip this interval if it's after the requested end time
		if current.After(endTime) {
			break
		}

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
