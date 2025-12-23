package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// GlobalAuditLogEntry represents an audit log entry with server information
type GlobalAuditLogEntry struct {
	ID         uuid.UUID       `json:"id"`
	ServerID   *uuid.UUID      `json:"server_id,omitempty"`
	ServerName string          `json:"server_name,omitempty"`
	UserID     *uuid.UUID      `json:"user_id,omitempty"`
	Username   string          `json:"username,omitempty"`
	Action     string          `json:"action"`
	Changes    json.RawMessage `json:"changes,omitempty"`
	Timestamp  time.Time       `json:"timestamp"`
}

// GlobalAuditStatsResponse represents audit log statistics
type GlobalAuditStatsResponse struct {
	TotalLogs        int64            `json:"total_logs"`
	LogsToday        int64            `json:"logs_today"`
	LogsThisWeek     int64            `json:"logs_this_week"`
	LogsThisMonth    int64            `json:"logs_this_month"`
	TopActions       []ActionCount    `json:"top_actions"`
	TopUsers         []UserActionCount `json:"top_users"`
	RecentActivity   []GlobalAuditLogEntry `json:"recent_activity"`
}

// ActionCount represents count of actions by type
type ActionCount struct {
	Action string `json:"action"`
	Count  int64  `json:"count"`
}

// UserActionCount represents count of actions by user
type UserActionCount struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	Count    int64     `json:"count"`
}

// GetGlobalAuditLogs returns audit logs across all servers
func (s *Server) GetGlobalAuditLogs(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse pagination parameters
	page := 1
	limit := 50
	if pageStr := c.Query("page"); pageStr != "" {
		fmt.Sscanf(pageStr, "%d", &page)
		if page < 1 {
			page = 1
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
		if limit < 1 || limit > 100 {
			limit = 50
		}
	}
	offset := (page - 1) * limit

	// Build the base query
	query := `
		SELECT al.id, al.server_id, s.name as server_name, al.user_id, u.username, al.action, al.changes, al.timestamp
		FROM audit_logs al
		LEFT JOIN users u ON al.user_id = u.id
		LEFT JOIN servers s ON al.server_id = s.id
		WHERE 1=1
	`
	countQuery := `
		SELECT COUNT(*)
		FROM audit_logs al
		WHERE 1=1
	`
	args := []interface{}{}
	paramCount := 0

	// Add filters
	if serverID := c.Query("server_id"); serverID != "" && serverID != "all" {
		srvID, err := uuid.Parse(serverID)
		if err == nil {
			paramCount++
			query += fmt.Sprintf(" AND al.server_id = $%d", paramCount)
			countQuery += fmt.Sprintf(" AND server_id = $%d", paramCount)
			args = append(args, srvID)
		}
	}

	if userID := c.Query("user_id"); userID != "" && userID != "all" {
		usrID, err := uuid.Parse(userID)
		if err == nil {
			paramCount++
			query += fmt.Sprintf(" AND al.user_id = $%d", paramCount)
			countQuery += fmt.Sprintf(" AND user_id = $%d", paramCount)
			args = append(args, usrID)
		}
	}

	if action := c.Query("action"); action != "" && action != "all" {
		paramCount++
		query += fmt.Sprintf(" AND al.action = $%d", paramCount)
		countQuery += fmt.Sprintf(" AND action = $%d", paramCount)
		args = append(args, action)
	}

	// Add search filter
	if search := c.Query("search"); search != "" {
		paramCount++
		searchPattern := "%" + search + "%"
		query += fmt.Sprintf(" AND (u.username ILIKE $%d OR al.action ILIKE $%d OR s.name ILIKE $%d)", paramCount, paramCount, paramCount)
		countQuery += fmt.Sprintf(" AND (EXISTS (SELECT 1 FROM users WHERE id = user_id AND username ILIKE $%d) OR action ILIKE $%d OR EXISTS (SELECT 1 FROM servers WHERE id = server_id AND name ILIKE $%d))", paramCount, paramCount, paramCount)
		args = append(args, searchPattern)
	}

	// Add date range filters
	if startDate := c.Query("start_date"); startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			paramCount++
			query += fmt.Sprintf(" AND al.timestamp >= $%d", paramCount)
			countQuery += fmt.Sprintf(" AND timestamp >= $%d", paramCount)
			args = append(args, t)
		}
	}

	if endDate := c.Query("end_date"); endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			// Add 1 day to include the entire end date
			t = t.Add(24 * time.Hour)
			paramCount++
			query += fmt.Sprintf(" AND al.timestamp < $%d", paramCount)
			countQuery += fmt.Sprintf(" AND timestamp < $%d", paramCount)
			args = append(args, t)
		}
	}

	// Add date filter shortcuts
	if dateFilter := c.Query("date_filter"); dateFilter != "" && dateFilter != "all" {
		var timeFilter time.Time
		now := time.Now()

		switch dateFilter {
		case "today":
			timeFilter = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		case "yesterday":
			yesterday := now.AddDate(0, 0, -1)
			timeFilter = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
		case "week":
			timeFilter = now.AddDate(0, 0, -7)
		case "month":
			timeFilter = now.AddDate(0, -1, 0)
		}

		if !timeFilter.IsZero() {
			paramCount++
			query += fmt.Sprintf(" AND al.timestamp >= $%d", paramCount)
			countQuery += fmt.Sprintf(" AND timestamp >= $%d", paramCount)
			args = append(args, timeFilter)
		}
	}

	// Get total count
	var totalCount int
	err := s.Dependencies.DB.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to count audit logs: %w", err), nil)
		return
	}

	// Add ordering and pagination
	paramCount++
	query += fmt.Sprintf(" ORDER BY al.timestamp DESC LIMIT $%d", paramCount)
	args = append(args, limit)
	paramCount++
	query += fmt.Sprintf(" OFFSET $%d", paramCount)
	args = append(args, offset)

	// Execute the query
	rows, err := s.Dependencies.DB.QueryContext(ctx, query, args...)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to query audit logs: %w", err), nil)
		return
	}
	defer rows.Close()

	// Parse the results
	logs := []GlobalAuditLogEntry{}
	for rows.Next() {
		var log GlobalAuditLogEntry
		var serverName, username sql.NullString

		err := rows.Scan(
			&log.ID,
			&log.ServerID,
			&serverName,
			&log.UserID,
			&username,
			&log.Action,
			&log.Changes,
			&log.Timestamp,
		)
		if err != nil {
			continue
		}

		if serverName.Valid {
			log.ServerName = serverName.String
		} else {
			log.ServerName = "N/A"
		}

		if username.Valid {
			log.Username = username.String
		} else {
			log.Username = "System"
		}

		logs = append(logs, log)
	}

	// Calculate total pages
	totalPages := (totalCount + limit - 1) / limit

	responses.Success(c, "Global audit logs retrieved successfully", &gin.H{
		"logs": logs,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       totalCount,
			"total_pages": totalPages,
		},
	})
}

// GetGlobalAuditStats returns statistics about audit logs
func (s *Server) GetGlobalAuditStats(c *gin.Context) {
	ctx := c.Request.Context()

	var stats GlobalAuditStatsResponse
	// Initialize empty slices to avoid null
	stats.TopActions = []ActionCount{}
	stats.TopUsers = []UserActionCount{}
	stats.RecentActivity = []GlobalAuditLogEntry{}

	// Get total logs
	s.Dependencies.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM audit_logs
	`).Scan(&stats.TotalLogs)

	// Get logs today
	s.Dependencies.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM audit_logs
		WHERE timestamp >= CURRENT_DATE
	`).Scan(&stats.LogsToday)

	// Get logs this week
	s.Dependencies.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM audit_logs
		WHERE timestamp >= CURRENT_DATE - INTERVAL '7 days'
	`).Scan(&stats.LogsThisWeek)

	// Get logs this month
	s.Dependencies.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM audit_logs
		WHERE timestamp >= CURRENT_DATE - INTERVAL '30 days'
	`).Scan(&stats.LogsThisMonth)

	// Get top actions
	rows, err := s.Dependencies.DB.QueryContext(ctx, `
		SELECT action, COUNT(*) as count
		FROM audit_logs
		WHERE timestamp >= CURRENT_DATE - INTERVAL '30 days'
		GROUP BY action
		ORDER BY count DESC
		LIMIT 10
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ac ActionCount
			if rows.Scan(&ac.Action, &ac.Count) == nil {
				stats.TopActions = append(stats.TopActions, ac)
			}
		}
	}

	// Get top users
	rows, err = s.Dependencies.DB.QueryContext(ctx, `
		SELECT al.user_id, u.username, COUNT(*) as count
		FROM audit_logs al
		LEFT JOIN users u ON al.user_id = u.id
		WHERE al.timestamp >= CURRENT_DATE - INTERVAL '30 days'
		  AND al.user_id IS NOT NULL
		GROUP BY al.user_id, u.username
		ORDER BY count DESC
		LIMIT 10
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var uac UserActionCount
			if rows.Scan(&uac.UserID, &uac.Username, &uac.Count) == nil {
				stats.TopUsers = append(stats.TopUsers, uac)
			}
		}
	}

	// Get recent activity (last 10 logs)
	rows, err = s.Dependencies.DB.QueryContext(ctx, `
		SELECT al.id, al.server_id, s.name as server_name, al.user_id, u.username, al.action, al.changes, al.timestamp
		FROM audit_logs al
		LEFT JOIN users u ON al.user_id = u.id
		LEFT JOIN servers s ON al.server_id = s.id
		ORDER BY al.timestamp DESC
		LIMIT 10
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var log GlobalAuditLogEntry
			var serverName, username sql.NullString

			if rows.Scan(&log.ID, &log.ServerID, &serverName, &log.UserID, &username, &log.Action, &log.Changes, &log.Timestamp) == nil {
				if serverName.Valid {
					log.ServerName = serverName.String
				} else {
					log.ServerName = "N/A"
				}
				if username.Valid {
					log.Username = username.String
				} else {
					log.Username = "System"
				}
				stats.RecentActivity = append(stats.RecentActivity, log)
			}
		}
	}

	responses.Success(c, "Audit statistics retrieved successfully", &gin.H{"data": stats})
}

// ExportGlobalAuditLogs exports audit logs to CSV format
func (s *Server) ExportGlobalAuditLogs(c *gin.Context) {
	ctx := c.Request.Context()

	// Build query with filters (same as GetGlobalAuditLogs but without pagination)
	query := `
		SELECT al.id, al.server_id, s.name as server_name, al.user_id, u.username, al.action, al.changes, al.timestamp
		FROM audit_logs al
		LEFT JOIN users u ON al.user_id = u.id
		LEFT JOIN servers s ON al.server_id = s.id
		WHERE 1=1
	`
	args := []interface{}{}
	paramCount := 0

	// Add filters (same logic as GetGlobalAuditLogs)
	if serverID := c.Query("server_id"); serverID != "" && serverID != "all" {
		srvID, err := uuid.Parse(serverID)
		if err == nil {
			paramCount++
			query += fmt.Sprintf(" AND al.server_id = $%d", paramCount)
			args = append(args, srvID)
		}
	}

	if action := c.Query("action"); action != "" && action != "all" {
		paramCount++
		query += fmt.Sprintf(" AND al.action = $%d", paramCount)
		args = append(args, action)
	}

	// Add date filter
	if dateFilter := c.Query("date_filter"); dateFilter != "" && dateFilter != "all" {
		var timeFilter time.Time
		now := time.Now()

		switch dateFilter {
		case "today":
			timeFilter = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		case "week":
			timeFilter = now.AddDate(0, 0, -7)
		case "month":
			timeFilter = now.AddDate(0, -1, 0)
		}

		if !timeFilter.IsZero() {
			paramCount++
			query += fmt.Sprintf(" AND al.timestamp >= $%d", paramCount)
			args = append(args, timeFilter)
		}
	}

	query += " ORDER BY al.timestamp DESC LIMIT 10000" // Limit to 10k rows for export

	rows, err := s.Dependencies.DB.QueryContext(ctx, query, args...)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to query audit logs: %w", err), nil)
		return
	}
	defer rows.Close()

	// Build CSV
	csv := "ID,Timestamp,Server,User,Action,Changes\n"
	for rows.Next() {
		var log GlobalAuditLogEntry
		var serverName, username sql.NullString

		if rows.Scan(&log.ID, &log.ServerID, &serverName, &log.UserID, &username, &log.Action, &log.Changes, &log.Timestamp) == nil {
			srvName := "N/A"
			if serverName.Valid {
				srvName = serverName.String
			}
			usrName := "System"
			if username.Valid {
				usrName = username.String
			}

			// Escape CSV fields
			changes := string(log.Changes)
			if len(changes) > 100 {
				changes = changes[:100] + "..."
			}
			changes = fmt.Sprintf("%q", changes)

			csv += fmt.Sprintf("%s,%s,%q,%q,%q,%s\n",
				log.ID.String(),
				log.Timestamp.Format("2006-01-02 15:04:05"),
				srvName,
				usrName,
				log.Action,
				changes,
			)
		}
	}

	// Set headers for download
	filename := fmt.Sprintf("audit_logs_%s.csv", time.Now().Format("2006-01-02"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Header("Content-Type", "text/csv")
	c.String(200, csv)
}

