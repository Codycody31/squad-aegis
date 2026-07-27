package server

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// MetricsOverviewResponse represents high-level instance metrics
type MetricsOverviewResponse struct {
	TotalServers        int    `json:"total_servers"`
	ActiveServers       int    `json:"active_servers"`
	TotalPlayers        int64  `json:"total_players"`
	TotalEvents         int64  `json:"total_events"`
	EventsThisWeek      int64  `json:"events_this_week"`
	EventsThisMonth     int64  `json:"events_this_month"`
	TotalChatMessages   int64  `json:"total_chat_messages"`
	TotalWorkflowRuns   int64  `json:"total_workflow_runs"`
	StorageUsed         int64  `json:"storage_used"`
	StorageUsedReadable string `json:"storage_used_readable"`
}

// MetricsTimelinePoint represents a data point in a time series
type MetricsTimelinePoint struct {
	Timestamp string `json:"timestamp"`
	Value     int64  `json:"value"`
}

// MetricsTimelineResponse represents time-series metrics data
type MetricsTimelineResponse struct {
	EventsOverTime  []MetricsTimelinePoint `json:"events_over_time"`
	ChatOverTime    []MetricsTimelinePoint `json:"chat_over_time"`
	PlayersOverTime []MetricsTimelinePoint `json:"players_over_time"`
}

// ServerActivityResponse represents per-server activity metrics
type ServerActivityResponse struct {
	ServerID       string  `json:"server_id"`
	ServerName     string  `json:"server_name"`
	TotalEvents    int64   `json:"total_events"`
	ChatMessages   int64   `json:"chat_messages"`
	UniquePlayers  int64   `json:"unique_players"`
	WorkflowRuns   int64   `json:"workflow_runs"`
	AvgPlayerCount float64 `json:"avg_player_count"`
	LastActivity   string  `json:"last_activity"`
}

// TopServersResponse represents the most active servers
type TopServersResponse struct {
	ByEvents   []ServerActivityResponse `json:"by_events"`
	ByPlayers  []ServerActivityResponse `json:"by_players"`
	ByMessages []ServerActivityResponse `json:"by_messages"`
}

const sudoHybridEventCTEs = `
	WITH
		today() - toIntervalDay(?) AS start_day,
		today() + 1 AS end_day,
		ifNull(
			(
				SELECT argMax(first_fully_covered_day, updated_at)
				FROM squad_aegis.sql_rollup_state
				WHERE rollup_name = 'server_event_daily_counts'
			),
			toDate('2100-01-01')
		) AS event_boundary,
		ifNull(
			(
				SELECT argMax(first_fully_covered_day, updated_at)
				FROM squad_aegis.sql_rollup_state
				WHERE rollup_name = 'server_info_daily'
			),
			toDate('2100-01-01')
		) AS info_boundary,
		ifNull(
			(
				SELECT argMax(first_fully_covered_day, updated_at)
				FROM squad_aegis.sql_rollup_state
				WHERE rollup_name = 'server_player_daily_uniques'
			),
			toDate('2100-01-01')
		) AS player_boundary,
		ifNull(
			(
				SELECT argMax(first_fully_covered_day, updated_at)
				FROM squad_aegis.sql_rollup_state
				WHERE rollup_name = 'server_workflow_daily'
			),
			toDate('2100-01-01')
		) AS workflow_boundary,
		raw_event_counts AS (
			SELECT toDate(sent_at) AS day, server_id, 'chat' AS event_type, count() AS event_count
			FROM squad_aegis.server_player_chat_messages
			WHERE sent_at >= toDateTime64(start_day, 3, 'UTC')
			  AND sent_at < toDateTime64(least(end_day, event_boundary), 3, 'UTC')
			GROUP BY day, server_id

			UNION ALL

			SELECT toDate(event_time), server_id, 'connected', count()
			FROM squad_aegis.server_player_connected_events
			WHERE event_time >= toDateTime64(start_day, 3, 'UTC')
			  AND event_time < toDateTime64(least(end_day, event_boundary), 3, 'UTC')
			GROUP BY toDate(event_time), server_id

			UNION ALL

			SELECT toDate(event_time), server_id, 'disconnected', count()
			FROM squad_aegis.server_player_disconnected_events
			WHERE event_time >= toDateTime64(start_day, 3, 'UTC')
			  AND event_time < toDateTime64(least(end_day, event_boundary), 3, 'UTC')
			GROUP BY toDate(event_time), server_id

			UNION ALL

			SELECT toDate(event_time), server_id, 'wounded', count()
			FROM squad_aegis.server_player_wounded_events
			WHERE event_time >= toDateTime64(start_day, 3, 'UTC')
			  AND event_time < toDateTime64(least(end_day, event_boundary), 3, 'UTC')
			GROUP BY toDate(event_time), server_id

			UNION ALL

			SELECT toDate(event_time), server_id, 'revived', count()
			FROM squad_aegis.server_player_revived_events
			WHERE event_time >= toDateTime64(start_day, 3, 'UTC')
			  AND event_time < toDateTime64(least(end_day, event_boundary), 3, 'UTC')
			GROUP BY toDate(event_time), server_id

			UNION ALL

			SELECT toDate(event_time), server_id, 'possess', count()
			FROM squad_aegis.server_player_possess_events
			WHERE event_time >= toDateTime64(start_day, 3, 'UTC')
			  AND event_time < toDateTime64(least(end_day, event_boundary), 3, 'UTC')
			GROUP BY toDate(event_time), server_id

			UNION ALL

			SELECT toDate(event_time), server_id, 'died', count()
			FROM squad_aegis.server_player_died_events
			WHERE event_time >= toDateTime64(start_day, 3, 'UTC')
			  AND event_time < toDateTime64(least(end_day, event_boundary), 3, 'UTC')
			GROUP BY toDate(event_time), server_id

			UNION ALL

			SELECT toDate(event_time), server_id, 'damaged', count()
			FROM squad_aegis.server_player_damaged_events
			WHERE event_time >= toDateTime64(start_day, 3, 'UTC')
			  AND event_time < toDateTime64(least(end_day, event_boundary), 3, 'UTC')
			GROUP BY toDate(event_time), server_id

			UNION ALL

			SELECT toDate(event_time), server_id, 'deployable_damaged', count()
			FROM squad_aegis.server_deployable_damaged_events
			WHERE event_time >= toDateTime64(start_day, 3, 'UTC')
			  AND event_time < toDateTime64(least(end_day, event_boundary), 3, 'UTC')
			GROUP BY toDate(event_time), server_id

			UNION ALL

			SELECT toDate(event_time), server_id, 'admin_broadcast', count()
			FROM squad_aegis.server_admin_broadcast_events
			WHERE event_time >= toDateTime64(start_day, 3, 'UTC')
			  AND event_time < toDateTime64(least(end_day, event_boundary), 3, 'UTC')
			GROUP BY toDate(event_time), server_id

			UNION ALL

			SELECT toDate(event_time), server_id, 'round_ended', count()
			FROM squad_aegis.server_game_events_unified
			WHERE event_type = 'ROUND_ENDED'
			  AND event_time >= toDateTime64(start_day, 3, 'UTC')
			  AND event_time < toDateTime64(least(end_day, event_boundary), 3, 'UTC')
			GROUP BY toDate(event_time), server_id
		),
		hybrid_event_counts AS (
			SELECT day, server_id, event_type, event_count
			FROM raw_event_counts

			UNION ALL

			SELECT day, server_id, event_type, sum(event_count)
			FROM squad_aegis.server_event_daily_counts
			WHERE day >= greatest(start_day, event_boundary)
			  AND day < end_day
			GROUP BY day, server_id, event_type
		)
`

const sudoMetricsTimelineQuery = `/* sa.metrics.timeline */` + sudoHybridEventCTEs + `,
	raw_info_states AS (
		SELECT
			toDate(event_time) AS day,
			avgState(player_count) AS player_count_state
		FROM squad_aegis.server_info_metrics
		WHERE event_time >= toDateTime64(start_day, 3, 'UTC')
		  AND event_time < toDateTime64(least(end_day, info_boundary), 3, 'UTC')
		GROUP BY day
	),
	hybrid_info_states AS (
		SELECT day, player_count_state
		FROM raw_info_states

		UNION ALL

		SELECT day, player_count_state
		FROM squad_aegis.server_info_daily
		WHERE day >= greatest(start_day, info_boundary)
		  AND day < end_day
	),
	event_metrics AS (
		SELECT
			day,
			toInt64(sum(event_count)) AS total_events,
			toInt64(sumIf(event_count, event_type = 'chat')) AS chat_messages
		FROM hybrid_event_counts
		GROUP BY day
	),
	player_metrics AS (
		SELECT day, toInt64(avgMerge(player_count_state)) AS average_players
		FROM hybrid_info_states
		GROUP BY day
	)
	SELECT metric, day, value
	FROM
	(
		SELECT 'events' AS metric, day, total_events AS value
		FROM event_metrics

		UNION ALL

		SELECT 'chat' AS metric, day, chat_messages AS value
		FROM event_metrics

		UNION ALL

		SELECT 'players' AS metric, day, average_players AS value
		FROM player_metrics
	)
	ORDER BY day, metric
`

const sudoServerActivitiesQuery = `/* sa.metrics.server_activity */` + sudoHybridEventCTEs + `,
	raw_player_states AS (
		SELECT
			server_id,
			uniqExactState(
				multiIf(
					ifNull(steam, '') != '', concat('steam:', assumeNotNull(steam)),
					ifNull(eos, '') != '', concat('eos:', assumeNotNull(eos)),
					concat('epic:', assumeNotNull(epic))
				)
			) AS players_state
		FROM squad_aegis.server_join_succeeded_events
		WHERE event_time >= toDateTime64(start_day, 3, 'UTC')
		  AND event_time < toDateTime64(least(end_day, player_boundary), 3, 'UTC')
		  AND (
			  ifNull(steam, '') != ''
			  OR ifNull(eos, '') != ''
			  OR ifNull(epic, '') != ''
		  )
		GROUP BY server_id
	),
	hybrid_player_states AS (
		SELECT server_id, players_state
		FROM raw_player_states

		UNION ALL

		SELECT server_id, players_state
		FROM squad_aegis.server_player_daily_uniques
		WHERE day >= greatest(start_day, player_boundary)
		  AND day < end_day
	),
	raw_workflow_states AS (
		SELECT server_id, uniqExactState(execution_id) AS executions_state
		FROM squad_aegis.workflow_execution_logs
		WHERE event_time >= toDateTime64(start_day, 3, 'UTC')
		  AND event_time < toDateTime64(least(end_day, workflow_boundary), 3, 'UTC')
		GROUP BY server_id
	),
	hybrid_workflow_states AS (
		SELECT server_id, executions_state
		FROM raw_workflow_states

		UNION ALL

		SELECT server_id, executions_state
		FROM squad_aegis.server_workflow_daily
		WHERE day >= greatest(start_day, workflow_boundary)
		  AND day < end_day
	),
	raw_info_states AS (
		SELECT
			server_id,
			avgState(player_count) AS player_count_state,
			max(event_time) AS last_activity
		FROM squad_aegis.server_info_metrics
		WHERE event_time >= toDateTime64(start_day, 3, 'UTC')
		  AND event_time < toDateTime64(least(end_day, info_boundary), 3, 'UTC')
		GROUP BY server_id
	),
	hybrid_info_states AS (
		SELECT server_id, player_count_state, last_activity
		FROM raw_info_states

		UNION ALL

		SELECT server_id, player_count_state, last_activity
		FROM squad_aegis.server_info_daily
		WHERE day >= greatest(start_day, info_boundary)
		  AND day < end_day
	),
	event_metrics AS (
		SELECT
			server_id,
			toInt64(sum(event_count)) AS total_events,
			toInt64(sumIf(event_count, event_type = 'chat')) AS chat_messages
		FROM hybrid_event_counts
		GROUP BY server_id
	),
	player_metrics AS (
		SELECT server_id, toInt64(uniqExactMerge(players_state)) AS unique_players
		FROM hybrid_player_states
		GROUP BY server_id
	),
	workflow_metrics AS (
		SELECT server_id, toInt64(uniqExactMerge(executions_state)) AS workflow_runs
		FROM hybrid_workflow_states
		GROUP BY server_id
	),
	info_metrics AS (
		SELECT
			server_id,
			avgMerge(player_count_state) AS average_players,
			max(last_activity) AS last_activity
		FROM hybrid_info_states
		GROUP BY server_id
	),
	all_server_ids AS (
		SELECT DISTINCT server_id
		FROM
		(
			SELECT server_id FROM event_metrics
			UNION ALL
			SELECT server_id FROM player_metrics
			UNION ALL
			SELECT server_id FROM workflow_metrics
			UNION ALL
			SELECT server_id FROM info_metrics
		)
	)
	SELECT
		ids.server_id,
		ifNull(events.total_events, toInt64(0)) AS total_events,
		ifNull(events.chat_messages, toInt64(0)) AS chat_messages,
		ifNull(players.unique_players, toInt64(0)) AS unique_players,
		ifNull(workflows.workflow_runs, toInt64(0)) AS workflow_runs,
		ifNull(info.average_players, toFloat64(0)) AS average_players,
		info.last_activity
	FROM all_server_ids AS ids
	LEFT JOIN event_metrics AS events ON events.server_id = ids.server_id
	LEFT JOIN player_metrics AS players ON players.server_id = ids.server_id
	LEFT JOIN workflow_metrics AS workflows ON workflows.server_id = ids.server_id
	LEFT JOIN info_metrics AS info ON info.server_id = ids.server_id
	ORDER BY total_events DESC, ids.server_id
	SETTINGS join_use_nulls = 1
`

// GetMetricsOverview returns high-level instance metrics
func (s *Server) GetMetricsOverview(c *gin.Context) {
	ctx := c.Request.Context()

	var overview MetricsOverviewResponse

	// Get total and active servers from PostgreSQL
	err := s.Dependencies.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM servers
	`).Scan(&overview.TotalServers)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to get server count: %w", err), nil)
		return
	}

	// Get active servers (those with events in the last 24 hours)
	if s.Dependencies.Clickhouse != nil {
		err = s.Dependencies.Clickhouse.QueryRow(ctx, `
			SELECT COUNT(DISTINCT server_id) 
			FROM squad_aegis.server_info_metrics 
			WHERE event_time >= now() - INTERVAL 24 HOUR
		`).Scan(&overview.ActiveServers)
		if err != nil && err != sql.ErrNoRows {
			overview.ActiveServers = 0
		}

		// Get total unique players tracked
		err = s.Dependencies.Clickhouse.QueryRow(ctx, `
			SELECT uniqExact(if(ifNull(steam, '') != '', assumeNotNull(steam), assumeNotNull(eos)))
			FROM squad_aegis.server_player_connected_events
			WHERE ifNull(steam, '') != '' OR ifNull(eos, '') != ''
		`).Scan(&overview.TotalPlayers)
		if err != nil && err != sql.ErrNoRows {
			overview.TotalPlayers = 0
		}

		// Get total events (approximate from multiple tables)
		var chatCount, playerCount, woundedCount int64

		s.Dependencies.Clickhouse.QueryRow(ctx, `
			SELECT COUNT(*) FROM squad_aegis.server_player_chat_messages
		`).Scan(&chatCount)

		s.Dependencies.Clickhouse.QueryRow(ctx, `
			SELECT COUNT(*) FROM squad_aegis.server_player_connected_events
		`).Scan(&playerCount)

		s.Dependencies.Clickhouse.QueryRow(ctx, `
			SELECT COUNT(*) FROM squad_aegis.server_player_wounded_events
		`).Scan(&woundedCount)

		overview.TotalEvents = chatCount + playerCount + woundedCount
		overview.TotalChatMessages = chatCount

		// Get events this week
		s.Dependencies.Clickhouse.QueryRow(ctx, `
			SELECT COUNT(*) FROM squad_aegis.server_player_chat_messages
			WHERE sent_at >= now() - INTERVAL 7 DAY
		`).Scan(&overview.EventsThisWeek)

		// Get events this month
		s.Dependencies.Clickhouse.QueryRow(ctx, `
			SELECT COUNT(*) FROM squad_aegis.server_player_chat_messages
			WHERE sent_at >= now() - INTERVAL 30 DAY
		`).Scan(&overview.EventsThisMonth)

		// Get total workflow runs
		s.Dependencies.Clickhouse.QueryRow(ctx, `
			SELECT COUNT(DISTINCT execution_id) 
			FROM squad_aegis.workflow_execution_logs
		`).Scan(&overview.TotalWorkflowRuns)
	}

	// Get storage usage
	if s.Dependencies.Storage != nil {
		stats, err := s.Dependencies.Storage.GetStats(ctx)
		if err == nil {
			overview.StorageUsed = stats.TotalSize
			overview.StorageUsedReadable = formatBytes(stats.TotalSize)
		}
	}

	responses.Success(c, "Metrics overview retrieved successfully", &gin.H{"data": overview})
}

// GetMetricsTimeline returns time-series metrics data
func (s *Server) GetMetricsTimeline(c *gin.Context) {
	ctx := c.Request.Context()
	timeline := MetricsTimelineResponse{
		EventsOverTime:  []MetricsTimelinePoint{},
		ChatOverTime:    []MetricsTimelinePoint{},
		PlayersOverTime: []MetricsTimelinePoint{},
	}

	if s.Dependencies.Clickhouse == nil {
		responses.Success(c, "Timeline metrics retrieved successfully", &gin.H{"data": timeline})
		return
	}

	days := parseSudoMetricsDays(c.Query("days"))
	rows, err := s.Dependencies.Clickhouse.Query(ctx, sudoMetricsTimelineQuery, days-1)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to get timeline metrics: %w", err), nil)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var (
			metric string
			date   time.Time
			value  int64
		)
		if err := rows.Scan(&metric, &date, &value); err != nil {
			responses.InternalServerError(c, fmt.Errorf("failed to scan timeline metrics: %w", err), nil)
			return
		}

		point := MetricsTimelinePoint{
			Timestamp: date.Format("2006-01-02"),
			Value:     value,
		}
		switch metric {
		case "events":
			timeline.EventsOverTime = append(timeline.EventsOverTime, point)
		case "chat":
			timeline.ChatOverTime = append(timeline.ChatOverTime, point)
		case "players":
			timeline.PlayersOverTime = append(timeline.PlayersOverTime, point)
		default:
			responses.InternalServerError(c, fmt.Errorf("unknown timeline metric %q", metric), nil)
			return
		}
	}
	if err := rows.Err(); err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to read timeline metrics: %w", err), nil)
		return
	}

	responses.Success(c, "Timeline metrics retrieved successfully", &gin.H{"data": timeline})
}

// GetServerActivities returns per-server activity breakdown
func (s *Server) GetServerActivities(c *gin.Context) {
	ctx := c.Request.Context()

	if s.Dependencies.Clickhouse == nil {
		responses.Success(c, "Server activities retrieved successfully", &gin.H{"data": []ServerActivityResponse{}})
		return
	}

	activities, err := s.querySudoServerActivities(ctx, 30)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to get server activities: %w", err), nil)
		return
	}

	serverNames, err := s.loadSudoServerNames(ctx)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to load server names: %w", err), nil)
		return
	}
	applySudoServerNames(activities, serverNames)

	responses.Success(c, "Server activities retrieved successfully", &gin.H{"data": activities})
}

// GetTopServers returns the most active servers by various metrics
func (s *Server) GetTopServers(c *gin.Context) {
	ctx := c.Request.Context()

	topServers := TopServersResponse{
		ByEvents:   []ServerActivityResponse{},
		ByPlayers:  []ServerActivityResponse{},
		ByMessages: []ServerActivityResponse{},
	}

	if s.Dependencies.Clickhouse == nil {
		responses.Success(c, "Top servers retrieved successfully", &gin.H{"data": topServers})
		return
	}

	activities, err := s.querySudoServerActivities(ctx, 7)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to get top server metrics: %w", err), nil)
		return
	}
	serverNames, err := s.loadSudoServerNames(ctx)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to load server names: %w", err), nil)
		return
	}
	applySudoServerNames(activities, serverNames)

	topServers.ByEvents = rankSudoServerActivities(activities, func(activity ServerActivityResponse) int64 {
		return activity.TotalEvents
	})
	topServers.ByPlayers = rankSudoServerActivities(activities, func(activity ServerActivityResponse) int64 {
		return activity.UniquePlayers
	})
	topServers.ByMessages = rankSudoServerActivities(activities, func(activity ServerActivityResponse) int64 {
		return activity.ChatMessages
	})

	responses.Success(c, "Top servers retrieved successfully", &gin.H{"data": topServers})
}

func parseSudoMetricsDays(raw string) int {
	const (
		defaultDays = 7
		maxDays     = 90
	)

	if raw == "" {
		return defaultDays
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days < 1 {
		return defaultDays
	}
	if days > maxDays {
		return maxDays
	}
	return days
}

func (s *Server) querySudoServerActivities(ctx context.Context, days int) ([]ServerActivityResponse, error) {
	rows, err := s.Dependencies.Clickhouse.Query(ctx, sudoServerActivitiesQuery, days-1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	activities := make([]ServerActivityResponse, 0)
	for rows.Next() {
		var (
			activity     ServerActivityResponse
			serverID     uuid.UUID
			lastActivity sql.NullTime
		)
		if err := rows.Scan(
			&serverID,
			&activity.TotalEvents,
			&activity.ChatMessages,
			&activity.UniquePlayers,
			&activity.WorkflowRuns,
			&activity.AvgPlayerCount,
			&lastActivity,
		); err != nil {
			return nil, fmt.Errorf("scan server activity: %w", err)
		}

		activity.ServerID = serverID.String()
		if lastActivity.Valid {
			activity.LastActivity = lastActivity.Time.UTC().Format("2006-01-02 15:04:05")
		}
		activities = append(activities, activity)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read server activities: %w", err)
	}
	return activities, nil
}

func (s *Server) loadSudoServerNames(ctx context.Context) (map[uuid.UUID]string, error) {
	rows, err := s.Dependencies.DB.QueryContext(ctx, `SELECT id, name FROM servers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	serverNames := make(map[uuid.UUID]string)
	for rows.Next() {
		var (
			id   uuid.UUID
			name string
		)
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("scan server name: %w", err)
		}
		serverNames[id] = name
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read server names: %w", err)
	}
	return serverNames, nil
}

func applySudoServerNames(activities []ServerActivityResponse, serverNames map[uuid.UUID]string) {
	for i := range activities {
		serverID, err := uuid.Parse(activities[i].ServerID)
		if err == nil {
			activities[i].ServerName = serverNames[serverID]
		}
		if activities[i].ServerName == "" {
			activities[i].ServerName = "Unknown Server"
		}
	}
}

func rankSudoServerActivities(
	activities []ServerActivityResponse,
	value func(ServerActivityResponse) int64,
) []ServerActivityResponse {
	ranked := append([]ServerActivityResponse(nil), activities...)
	sort.SliceStable(ranked, func(i, j int) bool {
		left, right := value(ranked[i]), value(ranked[j])
		if left == right {
			return ranked[i].ServerID < ranked[j].ServerID
		}
		return left > right
	})
	if len(ranked) > 10 {
		ranked = ranked[:10]
	}
	return ranked
}
