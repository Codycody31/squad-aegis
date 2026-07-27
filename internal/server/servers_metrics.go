package server

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// MetricPoint represents a time-series data point.
type MetricPoint struct {
	Timestamp time.Time   `json:"timestamp"`
	Value     interface{} `json:"value"`
}

type ConnectionMetricValue struct {
	Connections    int `json:"connections"`
	Disconnections int `json:"disconnections"`
	TotalActivity  int `json:"total_activity"`
}

// ServerMetricsData represents the metrics response.
type ServerMetricsData struct {
	PlayerCount            []MetricPoint  `json:"player_count"`
	QueueCount             []MetricPoint  `json:"queue_count"`
	TickRate               []MetricPoint  `json:"tick_rate"`
	Rounds                 []MetricPoint  `json:"rounds"`
	Maps                   []MetricPoint  `json:"maps"`
	ChatActivity           []MetricPoint  `json:"chat_activity"`
	ConnectionStats        []MetricPoint  `json:"connection_stats"`
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

// MetricsSummary provides aggregate statistics.
type MetricsSummary struct {
	TotalPlayers           int     `json:"total_players"`
	AvgTickRate            float64 `json:"avg_tick_rate"`
	TotalRounds            int     `json:"total_rounds"`
	UniquePlayersCount     int     `json:"unique_players_count"`
	TotalChatMessages      int     `json:"total_chat_messages"`
	TotalConnections       int     `json:"total_connections"`
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

// ServerMetricsHistory provides detailed metrics history from ClickHouse.
func (s *Server) ServerMetricsHistory(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	if _, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverID, user); err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	period := c.DefaultQuery("period", "24h")
	interval, err := strconv.Atoi(c.DefaultQuery("interval", "60"))
	if err != nil || interval <= 0 {
		interval = 60
	}

	var startTime, endTime time.Time
	startTimeText := c.Query("startTime")
	endTimeText := c.Query("endTime")
	if startTimeText != "" || endTimeText != "" {
		if startTimeText == "" || endTimeText == "" {
			responses.BadRequest(c, "Invalid time range", &gin.H{"error": "startTime and endTime must be provided together"})
			return
		}

		startTime, err = time.Parse(time.RFC3339, startTimeText)
		if err != nil {
			responses.BadRequest(c, "Invalid startTime format", &gin.H{"error": "startTime must be in RFC3339 format (e.g., 2024-01-01T00:00:00Z)"})
			return
		}
		endTime, err = time.Parse(time.RFC3339, endTimeText)
		if err != nil {
			responses.BadRequest(c, "Invalid endTime format", &gin.H{"error": "endTime must be in RFC3339 format (e.g., 2024-01-01T00:00:00Z)"})
			return
		}
		if endTime.Before(startTime) {
			responses.BadRequest(c, "Invalid time range", &gin.H{"error": "endTime must not be before startTime"})
			return
		}
		if endTime.Equal(startTime) {
			endTime = startTime.Add(24 * time.Hour)
		}
		if endTime.Sub(startTime) > 365*24*time.Hour {
			responses.BadRequest(c, "Time range too large", &gin.H{"error": "Time range cannot exceed 1 year"})
			return
		}
	}

	metrics, err := s.getMetricsFromClickHouse(c.Request.Context(), serverID, period, interval, startTime, endTime)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("fetch server metrics: %w", err), nil)
		return
	}

	responses.Success(c, "Server metrics retrieved successfully", &gin.H{"metrics": metrics})
}

// getMetricsFromClickHouse performs exactly four ClickHouse round trips: one
// combined player/queue series, one tick-rate series, one labeled event union,
// and one aggregate summary.
func (s *Server) getMetricsFromClickHouse(
	ctx context.Context,
	serverID uuid.UUID,
	period string,
	intervalMinutes int,
	customStartTime time.Time,
	customEndTime time.Time,
) (ServerMetricsData, error) {
	if s.Dependencies.Clickhouse == nil {
		return ServerMetricsData{}, fmt.Errorf("ClickHouse is unavailable")
	}

	startTime, endTime := metricsTimeRange(period, customStartTime, customEndTime)
	if intervalMinutes <= 0 {
		intervalMinutes = metricsInterval(period)
	}

	metrics := emptyServerMetrics(period)
	queryArgs := []any{intervalMinutes, serverID.String(), startTime, endTime}

	playerRows, err := s.Dependencies.Clickhouse.Query(ctx, `
		/* sa.metrics.player_queue */
		WITH
			toIntervalMinute(?) AS bucket_size,
			toUUID(?) AS target_server,
			? AS range_start,
			? AS range_end
		SELECT
			toInt64(toUnixTimestamp(toStartOfInterval(event_time, bucket_size))) * 1000 AS timestamp,
			avg(player_count) AS avg_player_count,
			avg(public_queue) AS avg_public_queue,
			avg(reserved_queue) AS avg_reserved_queue
		FROM squad_aegis.server_info_metrics
		WHERE server_id = target_server
		  AND event_time >= range_start
		  AND event_time <= range_end
		GROUP BY toStartOfInterval(event_time, bucket_size)
		ORDER BY timestamp ASC
	`, queryArgs...)
	if err != nil {
		return ServerMetricsData{}, fmt.Errorf("sa.metrics.player_queue: %w", err)
	}
	for playerRows.Next() {
		var timestamp int64
		var players, publicQueue, reservedQueue float64
		if err := playerRows.Scan(&timestamp, &players, &publicQueue, &reservedQueue); err != nil {
			_ = playerRows.Close()
			return ServerMetricsData{}, fmt.Errorf("sa.metrics.player_queue scan: %w", err)
		}
		pointTime := time.UnixMilli(timestamp)
		metrics.PlayerCount = append(metrics.PlayerCount, MetricPoint{
			Timestamp: pointTime,
			Value: map[string]interface{}{
				"player_count":   int(players),
				"public_queue":   int(publicQueue),
				"reserved_queue": int(reservedQueue),
				"total_queue":    int(publicQueue + reservedQueue),
			},
		})
		metrics.QueueCount = append(metrics.QueueCount, MetricPoint{
			Timestamp: pointTime,
			Value:     int(publicQueue + reservedQueue),
		})
	}
	if err := playerRows.Err(); err != nil {
		_ = playerRows.Close()
		return ServerMetricsData{}, fmt.Errorf("sa.metrics.player_queue iterate: %w", err)
	}
	if err := playerRows.Close(); err != nil {
		return ServerMetricsData{}, fmt.Errorf("sa.metrics.player_queue close: %w", err)
	}

	tickRows, err := s.Dependencies.Clickhouse.Query(ctx, `
		/* sa.metrics.tick_rate */
		WITH
			toIntervalMinute(?) AS bucket_size,
			toUUID(?) AS target_server,
			? AS range_start,
			? AS range_end
		SELECT
			toInt64(toUnixTimestamp(toStartOfInterval(event_time, bucket_size))) * 1000 AS timestamp,
			avg(tick_rate) AS value
		FROM squad_aegis.server_tick_rate_events
		WHERE server_id = target_server
		  AND event_time >= range_start
		  AND event_time <= range_end
		GROUP BY toStartOfInterval(event_time, bucket_size)
		ORDER BY timestamp ASC
	`, queryArgs...)
	if err != nil {
		return ServerMetricsData{}, fmt.Errorf("sa.metrics.tick_rate: %w", err)
	}
	for tickRows.Next() {
		var timestamp int64
		var value float64
		if err := tickRows.Scan(&timestamp, &value); err != nil {
			_ = tickRows.Close()
			return ServerMetricsData{}, fmt.Errorf("sa.metrics.tick_rate scan: %w", err)
		}
		metrics.TickRate = append(metrics.TickRate, MetricPoint{Timestamp: time.UnixMilli(timestamp), Value: value})
	}
	if err := tickRows.Err(); err != nil {
		_ = tickRows.Close()
		return ServerMetricsData{}, fmt.Errorf("sa.metrics.tick_rate iterate: %w", err)
	}
	if err := tickRows.Close(); err != nil {
		return ServerMetricsData{}, fmt.Errorf("sa.metrics.tick_rate close: %w", err)
	}

	eventCounts := map[string]map[int64]int{}
	connectionCounts := map[int64]ConnectionMetricValue{}
	eventRows, err := s.Dependencies.Clickhouse.Query(ctx, `
		/* sa.metrics.events */
		WITH
			toIntervalMinute(?) AS bucket_size,
			toUUID(?) AS target_server,
			? AS range_start,
			? AS range_end
		SELECT metric, timestamp, value, label, detail
		FROM
		(
			SELECT 'chat' AS metric, toInt64(toUnixTimestamp(toStartOfInterval(sent_at, bucket_size))) * 1000 AS timestamp, count() AS value, '' AS label, '' AS detail
			FROM squad_aegis.server_player_chat_messages
			WHERE server_id = target_server AND sent_at >= range_start AND sent_at <= range_end
			GROUP BY timestamp
			UNION ALL
			SELECT 'connected', toInt64(toUnixTimestamp(toStartOfInterval(event_time, bucket_size))) * 1000, count(), '', ''
			FROM squad_aegis.server_player_connected_events
			WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end
			GROUP BY toStartOfInterval(event_time, bucket_size)
			UNION ALL
			SELECT 'disconnected', toInt64(toUnixTimestamp(toStartOfInterval(event_time, bucket_size))) * 1000, count(), '', ''
			FROM squad_aegis.server_player_disconnected_events
			WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end
			GROUP BY toStartOfInterval(event_time, bucket_size)
			UNION ALL
			SELECT 'teamkill', toInt64(toUnixTimestamp(toStartOfInterval(event_time, bucket_size))) * 1000, count(), '', ''
			FROM squad_aegis.server_player_died_events
			WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end AND teamkill = 1
			GROUP BY toStartOfInterval(event_time, bucket_size)
			UNION ALL
			SELECT 'wounded', toInt64(toUnixTimestamp(toStartOfInterval(event_time, bucket_size))) * 1000, count(), '', ''
			FROM squad_aegis.server_player_wounded_events
			WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end
			GROUP BY toStartOfInterval(event_time, bucket_size)
			UNION ALL
			SELECT 'revived', toInt64(toUnixTimestamp(toStartOfInterval(event_time, bucket_size))) * 1000, count(), '', ''
			FROM squad_aegis.server_player_revived_events
			WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end
			GROUP BY toStartOfInterval(event_time, bucket_size)
			UNION ALL
			SELECT 'possess', toInt64(toUnixTimestamp(toStartOfInterval(event_time, bucket_size))) * 1000, count(), '', ''
			FROM squad_aegis.server_player_possess_events
			WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end
			GROUP BY toStartOfInterval(event_time, bucket_size)
			UNION ALL
			SELECT 'died', toInt64(toUnixTimestamp(toStartOfInterval(event_time, bucket_size))) * 1000, count(), '', ''
			FROM squad_aegis.server_player_died_events
			WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end
			GROUP BY toStartOfInterval(event_time, bucket_size)
			UNION ALL
			SELECT 'damaged', toInt64(toUnixTimestamp(toStartOfInterval(event_time, bucket_size))) * 1000, count(), '', ''
			FROM squad_aegis.server_player_damaged_events
			WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end
			GROUP BY toStartOfInterval(event_time, bucket_size)
			UNION ALL
			SELECT 'deployable_damaged', toInt64(toUnixTimestamp(toStartOfInterval(event_time, bucket_size))) * 1000, count(), '', ''
			FROM squad_aegis.server_deployable_damaged_events
			WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end
			GROUP BY toStartOfInterval(event_time, bucket_size)
			UNION ALL
			SELECT 'admin_broadcast', toInt64(toUnixTimestamp(toStartOfInterval(event_time, bucket_size))) * 1000, count(), '', ''
			FROM squad_aegis.server_admin_broadcast_events
			WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end
			GROUP BY toStartOfInterval(event_time, bucket_size)
			UNION ALL
			SELECT 'round', toInt64(toUnixTimestamp(toStartOfInterval(event_time, bucket_size))) * 1000, count(), '', ''
			FROM squad_aegis.server_game_events_unified
			WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end AND event_type = 'ROUND_ENDED'
			GROUP BY toStartOfInterval(event_time, bucket_size)
			UNION ALL
			SELECT 'map', toUnixTimestamp64Milli(event_time), toUInt64(0), ifNull(layer, ''), ifNull(winner, '')
			FROM
			(
				SELECT event_time, layer, winner
				FROM squad_aegis.server_game_events_unified
				WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end
				  AND event_type = 'ROUND_ENDED' AND ifNull(layer, '') != ''
				ORDER BY event_time DESC
				LIMIT 10
			)
		)
		ORDER BY metric ASC, timestamp ASC
	`, queryArgs...)
	if err != nil {
		return ServerMetricsData{}, fmt.Errorf("sa.metrics.events: %w", err)
	}
	for eventRows.Next() {
		var metric, label, detail string
		var timestamp int64
		var value uint64
		if err := eventRows.Scan(&metric, &timestamp, &value, &label, &detail); err != nil {
			_ = eventRows.Close()
			return ServerMetricsData{}, fmt.Errorf("sa.metrics.events scan: %w", err)
		}

		if metric == "map" {
			metrics.Maps = append(metrics.Maps, MetricPoint{
				Timestamp: time.UnixMilli(timestamp),
				Value:     map[string]interface{}{"layer": label, "winner": detail},
			})
			continue
		}
		if metric == "connected" || metric == "disconnected" {
			connection := connectionCounts[timestamp]
			if metric == "connected" {
				connection.Connections = int(value)
			} else {
				connection.Disconnections = int(value)
			}
			connection.TotalActivity = connection.Connections + connection.Disconnections
			connectionCounts[timestamp] = connection
			continue
		}
		if eventCounts[metric] == nil {
			eventCounts[metric] = map[int64]int{}
		}
		eventCounts[metric][timestamp] = int(value)
	}
	if err := eventRows.Err(); err != nil {
		_ = eventRows.Close()
		return ServerMetricsData{}, fmt.Errorf("sa.metrics.events iterate: %w", err)
	}
	if err := eventRows.Close(); err != nil {
		return ServerMetricsData{}, fmt.Errorf("sa.metrics.events close: %w", err)
	}

	metrics.ChatActivity = fillTimeSeriesGaps(startTime, endTime, intervalMinutes, eventCounts["chat"])
	metrics.ConnectionStats = fillConnectionSeriesGaps(startTime, endTime, intervalMinutes, connectionCounts)
	metrics.TeamkillStats = fillTimeSeriesGaps(startTime, endTime, intervalMinutes, eventCounts["teamkill"])
	metrics.PlayerWoundedStats = fillTimeSeriesGaps(startTime, endTime, intervalMinutes, eventCounts["wounded"])
	metrics.PlayerRevivedStats = fillTimeSeriesGaps(startTime, endTime, intervalMinutes, eventCounts["revived"])
	metrics.PlayerPossessStats = fillTimeSeriesGaps(startTime, endTime, intervalMinutes, eventCounts["possess"])
	metrics.PlayerDiedStats = fillTimeSeriesGaps(startTime, endTime, intervalMinutes, eventCounts["died"])
	metrics.PlayerDamagedStats = fillTimeSeriesGaps(startTime, endTime, intervalMinutes, eventCounts["damaged"])
	metrics.DeployableDamagedStats = fillTimeSeriesGaps(startTime, endTime, intervalMinutes, eventCounts["deployable_damaged"])
	metrics.AdminBroadcastStats = fillTimeSeriesGaps(startTime, endTime, intervalMinutes, eventCounts["admin_broadcast"])
	metrics.Rounds = fillTimeSeriesGaps(startTime, endTime, intervalMinutes, eventCounts["round"])

	var summaryValues [15]uint64
	var avgTickRate float64
	var mostPlayedMap string
	err = s.Dependencies.Clickhouse.QueryRow(ctx, `
		/* sa.metrics.summary */
		WITH
			toUUID(?) AS target_server,
			? AS range_start,
			? AS range_end
		SELECT
			(SELECT toUInt64(argMax(player_count, event_time)) FROM squad_aegis.server_info_metrics WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end),
			(SELECT ifNotFinite(avg(toFloat64(tick_rate)), 0.) FROM squad_aegis.server_tick_rate_events WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end),
			(SELECT count() FROM squad_aegis.server_game_events_unified WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end AND event_type = 'ROUND_ENDED'),
			(SELECT uniqExactIf(
				multiIf(ifNull(steam, '') != '', concat('steam:', assumeNotNull(steam)), ifNull(eos, '') != '', concat('eos:', assumeNotNull(eos)), concat('epic:', assumeNotNull(epic))),
				ifNull(steam, '') != '' OR ifNull(eos, '') != '' OR ifNull(epic, '') != ''
			) FROM squad_aegis.server_player_connected_events WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end),
			(SELECT count() FROM squad_aegis.server_player_chat_messages WHERE server_id = target_server AND sent_at >= range_start AND sent_at <= range_end),
			(SELECT count() FROM squad_aegis.server_player_connected_events WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end)
			  + (SELECT count() FROM squad_aegis.server_player_disconnected_events WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end),
			(SELECT count() FROM squad_aegis.server_player_died_events WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end AND teamkill = 1),
			(SELECT count() FROM squad_aegis.server_player_wounded_events WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end),
			(SELECT count() FROM squad_aegis.server_player_revived_events WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end),
			(SELECT count() FROM squad_aegis.server_player_possess_events WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end),
			(SELECT count() FROM squad_aegis.server_player_died_events WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end),
			(SELECT count() FROM squad_aegis.server_player_damaged_events WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end),
			(SELECT count() FROM squad_aegis.server_deployable_damaged_events WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end),
			(SELECT count() FROM squad_aegis.server_admin_broadcast_events WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end),
			(SELECT count() FROM squad_aegis.plugin_logs WHERE server_id = target_server AND timestamp >= range_start AND timestamp <= range_end),
			(SELECT ifNull(argMax(layer_name, play_count), '') FROM
				(SELECT assumeNotNull(layer) AS layer_name, count() AS play_count
				 FROM squad_aegis.server_game_events_unified
				 WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end
				   AND event_type = 'ROUND_ENDED' AND ifNull(layer, '') != ''
				 GROUP BY layer_name)),
			(SELECT toUInt64(max(player_count)) FROM squad_aegis.server_info_metrics WHERE server_id = target_server AND event_time >= range_start AND event_time <= range_end)
	`, serverID.String(), startTime, endTime).Scan(
		&summaryValues[0],
		&avgTickRate,
		&summaryValues[1],
		&summaryValues[2],
		&summaryValues[3],
		&summaryValues[4],
		&summaryValues[5],
		&summaryValues[6],
		&summaryValues[7],
		&summaryValues[8],
		&summaryValues[9],
		&summaryValues[10],
		&summaryValues[11],
		&summaryValues[12],
		&summaryValues[13],
		&mostPlayedMap,
		&summaryValues[14],
	)
	if err != nil {
		return ServerMetricsData{}, fmt.Errorf("sa.metrics.summary: %w", err)
	}
	metrics.Summary = MetricsSummary{
		TotalPlayers:           int(summaryValues[0]),
		AvgTickRate:            avgTickRate,
		TotalRounds:            int(summaryValues[1]),
		UniquePlayersCount:     int(summaryValues[2]),
		TotalChatMessages:      int(summaryValues[3]),
		TotalConnections:       int(summaryValues[4]),
		TotalTeamkills:         int(summaryValues[5]),
		TotalPlayerWounded:     int(summaryValues[6]),
		TotalPlayerRevived:     int(summaryValues[7]),
		TotalPlayerPossess:     int(summaryValues[8]),
		TotalPlayerDied:        int(summaryValues[9]),
		TotalPlayerDamaged:     int(summaryValues[10]),
		TotalDeployableDamaged: int(summaryValues[11]),
		TotalAdminBroadcasts:   int(summaryValues[12]),
		TotalPluginLogs:        int(summaryValues[13]),
		MostPlayedMap:          mostPlayedMap,
		PeakPlayerCount:        int(summaryValues[14]),
	}

	return metrics, nil
}

func metricsTimeRange(period string, customStartTime, customEndTime time.Time) (time.Time, time.Time) {
	if !customStartTime.IsZero() && !customEndTime.IsZero() {
		return customStartTime, customEndTime
	}

	endTime := time.Now().UTC()
	switch period {
	case "1h":
		return endTime.Add(-time.Hour), endTime
	case "6h":
		return endTime.Add(-6 * time.Hour), endTime
	case "7d":
		return endTime.Add(-7 * 24 * time.Hour), endTime
	case "30d":
		return endTime.Add(-30 * 24 * time.Hour), endTime
	default:
		return endTime.Add(-24 * time.Hour), endTime
	}
}

func metricsInterval(period string) int {
	switch period {
	case "1h":
		return 5
	case "6h":
		return 15
	case "7d":
		return 360
	case "30d":
		return 1440
	default:
		return 60
	}
}

func emptyServerMetrics(period string) ServerMetricsData {
	return ServerMetricsData{
		PlayerCount:            []MetricPoint{},
		QueueCount:             []MetricPoint{},
		TickRate:               []MetricPoint{},
		Rounds:                 []MetricPoint{},
		Maps:                   []MetricPoint{},
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
		Period:                 period,
	}
}

// fillTimeSeriesGaps fills in missing time intervals with zero values.
func fillTimeSeriesGaps(startTime, endTime time.Time, intervalMinutes int, dataMap map[int64]int) []MetricPoint {
	points := []MetricPoint{}
	interval := time.Duration(intervalMinutes) * time.Minute
	startTimestamp := startTime.Truncate(interval)
	endTimestamp := endTime.Truncate(interval)
	if endTime.After(endTimestamp) {
		endTimestamp = endTimestamp.Add(interval)
	}

	for current := startTimestamp; !current.After(endTimestamp); current = current.Add(interval) {
		timestamp := current.UnixMilli()
		value := dataMap[timestamp]
		points = append(points, MetricPoint{Timestamp: current, Value: value})
	}
	return points
}

func fillConnectionSeriesGaps(startTime, endTime time.Time, intervalMinutes int, dataMap map[int64]ConnectionMetricValue) []MetricPoint {
	points := []MetricPoint{}
	interval := time.Duration(intervalMinutes) * time.Minute
	startTimestamp := startTime.Truncate(interval)
	endTimestamp := endTime.Truncate(interval)
	if endTime.After(endTimestamp) {
		endTimestamp = endTimestamp.Add(interval)
	}

	for current := startTimestamp; !current.After(endTimestamp); current = current.Add(interval) {
		points = append(points, MetricPoint{
			Timestamp: current,
			Value:     dataMap[current.UnixMilli()],
		})
	}
	return points
}
