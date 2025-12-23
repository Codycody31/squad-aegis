package server

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// MetricsOverviewResponse represents high-level instance metrics
type MetricsOverviewResponse struct {
	TotalServers       int     `json:"total_servers"`
	ActiveServers      int     `json:"active_servers"`
	TotalPlayers       int64   `json:"total_players"`
	TotalEvents        int64   `json:"total_events"`
	EventsThisWeek     int64   `json:"events_this_week"`
	EventsThisMonth    int64   `json:"events_this_month"`
	TotalChatMessages  int64   `json:"total_chat_messages"`
	TotalWorkflowRuns  int64   `json:"total_workflow_runs"`
	StorageUsed        int64   `json:"storage_used"`
	StorageUsedReadable string `json:"storage_used_readable"`
}

// MetricsTimelinePoint represents a data point in a time series
type MetricsTimelinePoint struct {
	Timestamp string `json:"timestamp"`
	Value     int64  `json:"value"`
}

// MetricsTimelineResponse represents time-series metrics data
type MetricsTimelineResponse struct {
	EventsOverTime []MetricsTimelinePoint `json:"events_over_time"`
	ChatOverTime   []MetricsTimelinePoint `json:"chat_over_time"`
	PlayersOverTime []MetricsTimelinePoint `json:"players_over_time"`
}

// ServerActivityResponse represents per-server activity metrics
type ServerActivityResponse struct {
	ServerID        string `json:"server_id"`
	ServerName      string `json:"server_name"`
	TotalEvents     int64  `json:"total_events"`
	ChatMessages    int64  `json:"chat_messages"`
	UniquePlayers   int64  `json:"unique_players"`
	WorkflowRuns    int64  `json:"workflow_runs"`
	AvgPlayerCount  float64 `json:"avg_player_count"`
	LastActivity    string `json:"last_activity"`
}

// TopServersResponse represents the most active servers
type TopServersResponse struct {
	ByEvents   []ServerActivityResponse `json:"by_events"`
	ByPlayers  []ServerActivityResponse `json:"by_players"`
	ByMessages []ServerActivityResponse `json:"by_messages"`
}

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
			SELECT COUNT(DISTINCT steam_id) 
			FROM squad_aegis.server_player_connected
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
			SELECT COUNT(*) FROM squad_aegis.server_player_connected
		`).Scan(&playerCount)
		
		s.Dependencies.Clickhouse.QueryRow(ctx, `
			SELECT COUNT(*) FROM squad_aegis.server_player_wounded
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

	// Get date range from query params
	days := 7
	if daysStr := c.Query("days"); daysStr != "" {
		fmt.Sscanf(daysStr, "%d", &days)
	}
	if days > 90 {
		days = 90
	}

	var timeline MetricsTimelineResponse
	// Initialize empty slices to avoid null
	timeline.EventsOverTime = []MetricsTimelinePoint{}
	timeline.ChatOverTime = []MetricsTimelinePoint{}
	timeline.PlayersOverTime = []MetricsTimelinePoint{}

	if s.Dependencies.Clickhouse != nil {
		// Get events over time
		rows, err := s.Dependencies.Clickhouse.Query(ctx, `
			SELECT 
				toDate(sent_at) as date,
				COUNT(*) as count
			FROM squad_aegis.server_player_chat_messages
			WHERE sent_at >= now() - INTERVAL ? DAY
			GROUP BY date
			ORDER BY date
		`, days)
		
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var date time.Time
				var count int64
				if err := rows.Scan(&date, &count); err == nil {
					timeline.EventsOverTime = append(timeline.EventsOverTime, MetricsTimelinePoint{
						Timestamp: date.Format("2006-01-02"),
						Value:     count,
					})
				}
			}
		}

		// Get chat messages over time
		rows, err = s.Dependencies.Clickhouse.Query(ctx, `
			SELECT 
				toDate(sent_at) as date,
				COUNT(*) as count
			FROM squad_aegis.server_player_chat_messages
			WHERE sent_at >= now() - INTERVAL ? DAY
			GROUP BY date
			ORDER BY date
		`, days)
		
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var date time.Time
				var count int64
				if err := rows.Scan(&date, &count); err == nil {
					timeline.ChatOverTime = append(timeline.ChatOverTime, MetricsTimelinePoint{
						Timestamp: date.Format("2006-01-02"),
						Value:     count,
					})
				}
			}
		}

		// Get average player count over time
		rows, err = s.Dependencies.Clickhouse.Query(ctx, `
			SELECT 
				toDate(event_time) as date,
				toInt64(AVG(player_count)) as avg_players
			FROM squad_aegis.server_info_metrics
			WHERE event_time >= now() - INTERVAL ? DAY
			GROUP BY date
			ORDER BY date
		`, days)
		
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var date time.Time
				var count int64
				if err := rows.Scan(&date, &count); err == nil {
					timeline.PlayersOverTime = append(timeline.PlayersOverTime, MetricsTimelinePoint{
						Timestamp: date.Format("2006-01-02"),
						Value:     count,
					})
				}
			}
		}
	}

	responses.Success(c, "Timeline metrics retrieved successfully", &gin.H{"data": timeline})
}

// GetServerActivities returns per-server activity breakdown
func (s *Server) GetServerActivities(c *gin.Context) {
	ctx := c.Request.Context()

	activities := []ServerActivityResponse{} // Initialize empty slice

	if s.Dependencies.Clickhouse == nil {
		responses.Success(c, "Server activities retrieved successfully", &gin.H{"data": activities})
		return
	}

	// Get server info from PostgreSQL
	serverMap := make(map[uuid.UUID]string)
	rows, err := s.Dependencies.DB.QueryContext(ctx, `SELECT id, name FROM servers`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			var name string
			if rows.Scan(&id, &name) == nil {
				serverMap[id] = name
			}
		}
	}

	// Get activity metrics from ClickHouse
	chRows, err := s.Dependencies.Clickhouse.Query(ctx, `
		SELECT 
			server_id,
			COUNT(*) as total_events,
			COUNT(DISTINCT steam_id) as unique_players,
			MAX(sent_at) as last_activity
		FROM squad_aegis.server_player_chat_messages
		WHERE sent_at >= now() - INTERVAL 30 DAY
		GROUP BY server_id
		ORDER BY total_events DESC
	`)

	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to get server activities: %w", err), nil)
		return
	}
	defer chRows.Close()

	for chRows.Next() {
		var activity ServerActivityResponse
		var serverID uuid.UUID
		var lastActivity time.Time

		if err := chRows.Scan(&serverID, &activity.TotalEvents, &activity.UniquePlayers, &lastActivity); err != nil {
			continue
		}

		activity.ServerID = serverID.String()
		activity.ServerName = serverMap[serverID]
		if activity.ServerName == "" {
			activity.ServerName = "Unknown Server"
		}
		activity.ChatMessages = activity.TotalEvents // For now, same as total events
		activity.LastActivity = lastActivity.Format("2006-01-02 15:04:05")

		// Get avg player count
		var avgPlayers sql.NullFloat64
		s.Dependencies.Clickhouse.QueryRow(ctx, `
			SELECT AVG(player_count)
			FROM squad_aegis.server_info_metrics
			WHERE server_id = ? AND event_time >= now() - INTERVAL 30 DAY
		`, serverID).Scan(&avgPlayers)
		
		if avgPlayers.Valid {
			activity.AvgPlayerCount = avgPlayers.Float64
		}

		// Get workflow runs
		s.Dependencies.Clickhouse.QueryRow(ctx, `
			SELECT COUNT(DISTINCT execution_id)
			FROM squad_aegis.workflow_execution_logs
			WHERE server_id = ? AND event_time >= now() - INTERVAL 30 DAY
		`, serverID).Scan(&activity.WorkflowRuns)

		activities = append(activities, activity)
	}

	responses.Success(c, "Server activities retrieved successfully", &gin.H{"data": activities})
}

// GetTopServers returns the most active servers by various metrics
func (s *Server) GetTopServers(c *gin.Context) {
	ctx := c.Request.Context()

	var topServers TopServersResponse
	// Initialize empty slices to avoid null
	topServers.ByEvents = []ServerActivityResponse{}
	topServers.ByPlayers = []ServerActivityResponse{}
	topServers.ByMessages = []ServerActivityResponse{}

	if s.Dependencies.Clickhouse == nil {
		responses.Success(c, "Top servers retrieved successfully", &gin.H{"data": topServers})
		return
	}

	// Get server info from PostgreSQL
	serverMap := make(map[uuid.UUID]string)
	rows, err := s.Dependencies.DB.QueryContext(ctx, `SELECT id, name FROM servers`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			var name string
			if rows.Scan(&id, &name) == nil {
				serverMap[id] = name
			}
		}
	}

	// Top by events
	chRows, err := s.Dependencies.Clickhouse.Query(ctx, `
		SELECT 
			server_id,
			COUNT(*) as event_count
		FROM squad_aegis.server_player_chat_messages
		WHERE sent_at >= now() - INTERVAL 7 DAY
		GROUP BY server_id
		ORDER BY event_count DESC
		LIMIT 10
	`)

	if err == nil {
		defer chRows.Close()
		for chRows.Next() {
			var serverID uuid.UUID
			var eventCount int64
			if chRows.Scan(&serverID, &eventCount) == nil {
				topServers.ByEvents = append(topServers.ByEvents, ServerActivityResponse{
					ServerID:    serverID.String(),
					ServerName:  serverMap[serverID],
					TotalEvents: eventCount,
				})
			}
		}
	}

	// Top by unique players
	chRows, err = s.Dependencies.Clickhouse.Query(ctx, `
		SELECT 
			server_id,
			COUNT(DISTINCT steam_id) as unique_players
		FROM squad_aegis.server_player_connected
		WHERE time >= now() - INTERVAL 7 DAY
		GROUP BY server_id
		ORDER BY unique_players DESC
		LIMIT 10
	`)

	if err == nil {
		defer chRows.Close()
		for chRows.Next() {
			var serverID uuid.UUID
			var playerCount int64
			if chRows.Scan(&serverID, &playerCount) == nil {
				topServers.ByPlayers = append(topServers.ByPlayers, ServerActivityResponse{
					ServerID:      serverID.String(),
					ServerName:    serverMap[serverID],
					UniquePlayers: playerCount,
				})
			}
		}
	}

	// Top by chat messages
	chRows, err = s.Dependencies.Clickhouse.Query(ctx, `
		SELECT 
			server_id,
			COUNT(*) as message_count
		FROM squad_aegis.server_player_chat_messages
		WHERE sent_at >= now() - INTERVAL 7 DAY
		GROUP BY server_id
		ORDER BY message_count DESC
		LIMIT 10
	`)

	if err == nil {
		defer chRows.Close()
		for chRows.Next() {
			var serverID uuid.UUID
			var messageCount int64
			if chRows.Scan(&serverID, &messageCount) == nil {
				topServers.ByMessages = append(topServers.ByMessages, ServerActivityResponse{
					ServerID:     serverID.String(),
					ServerName:   serverMap[serverID],
					ChatMessages: messageCount,
				})
			}
		}
	}

	responses.Success(c, "Top servers retrieved successfully", &gin.H{"data": topServers})
}

