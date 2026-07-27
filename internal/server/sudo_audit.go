package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/pagination"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

const globalAuditCursorKind = "audit.global"

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
	TotalLogs      int64                 `json:"total_logs"`
	LogsToday      int64                 `json:"logs_today"`
	LogsThisWeek   int64                 `json:"logs_this_week"`
	LogsThisMonth  int64                 `json:"logs_this_month"`
	TopActions     []ActionCount         `json:"top_actions"`
	TopUsers       []UserActionCount     `json:"top_users"`
	RecentActivity []GlobalAuditLogEntry `json:"recent_activity"`
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

type globalAuditFilters struct {
	ServerID  *uuid.UUID `json:"server_id"`
	UserID    *uuid.UUID `json:"user_id"`
	Action    string     `json:"action"`
	Search    string     `json:"search"`
	StartDate string     `json:"start_date"`
	EndDate   string     `json:"end_date"`
	DateRange string     `json:"date_range"`
}

// GetGlobalAuditLogs returns audit logs across all servers
func (s *Server) GetGlobalAuditLogs(c *gin.Context) {
	ctx := c.Request.Context()
	limit, ok := parseCursorLimit(c, 50, 100)
	if !ok || rejectLegacyPagination(c, "page", "offset") {
		return
	}

	filters := globalAuditFilters{
		Action:    normalizedAllFilter(c.Query("action")),
		Search:    strings.ToLower(strings.TrimSpace(c.Query("search"))),
		StartDate: strings.TrimSpace(c.Query("start_date")),
		EndDate:   strings.TrimSpace(c.Query("end_date")),
		DateRange: normalizedAllFilter(c.Query("date_filter")),
	}
	if raw := normalizedAllFilter(c.Query("server_id")); raw != "" {
		value, err := uuid.Parse(raw)
		if err != nil {
			responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
			return
		}
		filters.ServerID = &value
	}
	if raw := normalizedAllFilter(c.Query("user_id")); raw != "" {
		value, err := uuid.Parse(raw)
		if err != nil {
			responses.BadRequest(c, "Invalid user ID", &gin.H{"error": err.Error()})
			return
		}
		filters.UserID = &value
	}
	filterHash := pagination.MustFilterHash(filters)
	var cursorValues *auditCursorValues
	if encodedCursor := c.Query("cursor"); encodedCursor != "" {
		cursorValues = &auditCursorValues{}
		if err := pagination.Decode(encodedCursor, globalAuditCursorKind, filterHash, cursorValues); err != nil {
			responses.BadRequest(c, "Invalid cursor", &gin.H{"error": err.Error()})
			return
		}
	}

	// Build the base query. COALESCE prefers the live username, falls back to the
	// snapshot captured at write time for deleted users.
	query := `
		/* sa.audit.list */
		SELECT al.id, al.server_id, s.name as server_name, al.user_id, COALESCE(u.username, al.username_snapshot), al.action, al.changes, al.timestamp
		FROM audit_logs al
		LEFT JOIN users u ON al.user_id = u.id
		LEFT JOIN servers s ON al.server_id = s.id
		WHERE 1=1
	`
	args := []interface{}{}
	paramCount := 0

	// Add filters
	if filters.ServerID != nil {
		paramCount++
		query += fmt.Sprintf(" AND al.server_id = $%d", paramCount)
		args = append(args, *filters.ServerID)
	}

	if filters.UserID != nil {
		paramCount++
		query += fmt.Sprintf(" AND al.user_id = $%d", paramCount)
		args = append(args, *filters.UserID)
	}

	if filters.Action != "" {
		paramCount++
		query += fmt.Sprintf(" AND al.action = $%d", paramCount)
		args = append(args, filters.Action)
	}

	// Add search filter. Also search the username snapshot so deleted users
	// remain findable by name.
	if filters.Search != "" {
		paramCount++
		searchPattern := "%" + filters.Search + "%"
		query += fmt.Sprintf(` AND (
			lower(al.action) LIKE $%[1]d OR
			lower(COALESCE(al.username_snapshot, '')) LIKE $%[1]d OR
			EXISTS (SELECT 1 FROM users au WHERE au.id = al.user_id AND lower(au.username) LIKE $%[1]d) OR
			EXISTS (SELECT 1 FROM servers aus WHERE aus.id = al.server_id AND lower(aus.name) LIKE $%[1]d)
		)`, paramCount)
		args = append(args, searchPattern)
	}

	// Add date range filters
	if filters.StartDate != "" {
		value, err := time.Parse("2006-01-02", filters.StartDate)
		if err != nil {
			responses.BadRequest(c, "Invalid start date", &gin.H{"error": err.Error()})
			return
		}
		paramCount++
		query += fmt.Sprintf(" AND al.timestamp >= $%d", paramCount)
		args = append(args, value)
	}

	if filters.EndDate != "" {
		value, err := time.Parse("2006-01-02", filters.EndDate)
		if err != nil {
			responses.BadRequest(c, "Invalid end date", &gin.H{"error": err.Error()})
			return
		}
		paramCount++
		query += fmt.Sprintf(" AND al.timestamp < $%d", paramCount)
		args = append(args, value.AddDate(0, 0, 1))
	}

	// Add date filter shortcuts
	if start := auditDateStart(filters.DateRange, time.Now()); start != nil {
		paramCount++
		query += fmt.Sprintf(" AND al.timestamp >= $%d", paramCount)
		args = append(args, *start)
	}
	if cursorValues != nil {
		paramCount++
		timestampParam := paramCount
		args = append(args, cursorValues.Timestamp)
		paramCount++
		idParam := paramCount
		args = append(args, cursorValues.ID)
		query += fmt.Sprintf(" AND (al.timestamp, al.id) < ($%d, $%d)", timestampParam, idParam)
	}

	paramCount++
	query += fmt.Sprintf(" ORDER BY al.timestamp DESC, al.id DESC LIMIT $%d", paramCount)
	args = append(args, limit+1)

	started := time.Now()
	rows, err := s.Dependencies.DB.QueryContext(ctx, query, args...)
	if err != nil {
		logAuditQuery("sa.audit.list", started, 0, err)
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
			logAuditQuery("sa.audit.list", started, len(logs), err)
			responses.InternalServerError(c, fmt.Errorf("failed to scan audit log: %w", err), nil)
			return
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
	if err := rows.Err(); err != nil {
		logAuditQuery("sa.audit.list", started, len(logs), err)
		responses.InternalServerError(c, fmt.Errorf("iterate audit logs: %w", err), nil)
		return
	}
	logs, hasMore := pagination.TrimPage(logs, limit)
	pageInfo, err := auditPageInfo(globalAuditCursorKind, filterHash, logs, hasMore)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	logAuditQuery("sa.audit.list", started, len(logs), nil)

	responses.Success(c, "Global audit logs retrieved successfully", &gin.H{
		"logs":      logs,
		"page_info": pageInfo,
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

	if err := s.Dependencies.DB.QueryRowContext(ctx, `
		/* sa.audit.stats.headline */
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE timestamp >= CURRENT_DATE),
			COUNT(*) FILTER (WHERE timestamp >= CURRENT_DATE - INTERVAL '7 days'),
			COUNT(*) FILTER (WHERE timestamp >= CURRENT_DATE - INTERVAL '30 days')
		FROM audit_logs
	`).Scan(&stats.TotalLogs, &stats.LogsToday, &stats.LogsThisWeek, &stats.LogsThisMonth); err != nil {
		responses.InternalServerError(c, fmt.Errorf("query audit headline statistics: %w", err), nil)
		return
	}

	// Get top actions
	rows, err := s.Dependencies.DB.QueryContext(ctx, `
		/* sa.audit.stats.actions */
		SELECT action, COUNT(*) as count
		FROM audit_logs
		WHERE timestamp >= CURRENT_DATE - INTERVAL '30 days'
		GROUP BY action
		ORDER BY count DESC
		LIMIT 10
	`)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("query top audit actions: %w", err), nil)
		return
	}
	for rows.Next() {
		var ac ActionCount
		if err := rows.Scan(&ac.Action, &ac.Count); err != nil {
			rows.Close()
			responses.InternalServerError(c, fmt.Errorf("scan top audit action: %w", err), nil)
			return
		}
		stats.TopActions = append(stats.TopActions, ac)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		responses.InternalServerError(c, fmt.Errorf("iterate top audit actions: %w", err), nil)
		return
	}
	rows.Close()

	// Get top users
	rows, err = s.Dependencies.DB.QueryContext(ctx, `
		/* sa.audit.stats.users */
		SELECT al.user_id,
		       COALESCE(MAX(u.username), MAX(al.username_snapshot), 'System'),
		       COUNT(*) AS count
		FROM audit_logs al
		LEFT JOIN users u ON al.user_id = u.id
		WHERE al.timestamp >= CURRENT_DATE - INTERVAL '30 days'
		  AND al.user_id IS NOT NULL
		GROUP BY al.user_id
		ORDER BY count DESC
		LIMIT 10
	`)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("query top audit users: %w", err), nil)
		return
	}
	for rows.Next() {
		var uac UserActionCount
		if err := rows.Scan(&uac.UserID, &uac.Username, &uac.Count); err != nil {
			rows.Close()
			responses.InternalServerError(c, fmt.Errorf("scan top audit user: %w", err), nil)
			return
		}
		stats.TopUsers = append(stats.TopUsers, uac)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		responses.InternalServerError(c, fmt.Errorf("iterate top audit users: %w", err), nil)
		return
	}
	rows.Close()

	// Get recent activity (last 10 logs)
	rows, err = s.Dependencies.DB.QueryContext(ctx, `
		/* sa.audit.stats.recent */
		SELECT al.id, al.server_id, s.name as server_name, al.user_id, COALESCE(u.username, al.username_snapshot), al.action, al.changes, al.timestamp
		FROM audit_logs al
		LEFT JOIN users u ON al.user_id = u.id
		LEFT JOIN servers s ON al.server_id = s.id
		ORDER BY al.timestamp DESC
		LIMIT 10
	`)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("query recent audit activity: %w", err), nil)
		return
	}
	for rows.Next() {
		var entry GlobalAuditLogEntry
		var serverName, username sql.NullString
		if err := rows.Scan(&entry.ID, &entry.ServerID, &serverName, &entry.UserID, &username, &entry.Action, &entry.Changes, &entry.Timestamp); err != nil {
			rows.Close()
			responses.InternalServerError(c, fmt.Errorf("scan recent audit activity: %w", err), nil)
			return
		}
		if serverName.Valid {
			entry.ServerName = serverName.String
		} else {
			entry.ServerName = "N/A"
		}
		if username.Valid {
			entry.Username = username.String
		} else {
			entry.Username = "System"
		}
		stats.RecentActivity = append(stats.RecentActivity, entry)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		responses.InternalServerError(c, fmt.Errorf("iterate recent audit activity: %w", err), nil)
		return
	}
	rows.Close()

	responses.Success(c, "Audit statistics retrieved successfully", &gin.H{"data": stats})
}

// ExportGlobalAuditLogs exports audit logs to CSV format
func (s *Server) ExportGlobalAuditLogs(c *gin.Context) {
	ctx := c.Request.Context()

	// Build query with filters (same as GetGlobalAuditLogs but without pagination)
	query := `
		SELECT al.id, al.server_id, s.name as server_name, al.user_id, COALESCE(u.username, al.username_snapshot), al.action, al.changes, al.timestamp
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
