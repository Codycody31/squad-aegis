package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/core"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	"go.codycody31.dev/squad-aegis/shared/config"
)

// AuditLogEntry represents an audit log entry
type AuditLogEntry struct {
	ID         uuid.UUID       `json:"id"`
	UserID     *uuid.UUID      `json:"user_id,omitempty"`
	Username   string          `json:"username,omitempty"`
	Action     string          `json:"action"`
	Changes    json.RawMessage `json:"changes,omitempty"`
	Timestamp  time.Time       `json:"timestamp"`
}

// CreateAuditLog creates a new audit log entry
func (s *Server) CreateAuditLog(ctx context.Context, serverID *uuid.UUID, userID *uuid.UUID, action string, changes interface{}) error {
	// Convert changes to JSON
	changesJSON, err := json.Marshal(changes)
	if err != nil {
		return err
	}

	// Insert the audit log into the database
	_, err = s.Dependencies.DB.ExecContext(ctx, `
		INSERT INTO audit_logs (server_id, user_id, action, changes)
		VALUES ($1, $2, $3, $4)
	`, serverID, userID, action, changesJSON)

	// Track the audit log creation event
	if s.Dependencies.MetricsCollector != nil {
		data := map[string]interface{}{}

		if config.Config.App.NonAnonymousTelemetry {
			data["server_id"] = serverID
			data["user_id"] = userID
		}

		s.Dependencies.MetricsCollector.GetCountly().TrackEvent(action, 1, 0, data)
	}

	return err
}

// ServerAuditLogs handles listing audit logs for a server
func (s *Server) ServerAuditLogs(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIDString := c.Param("serverId")
	serverID, err := uuid.Parse(serverIDString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverID, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Parse pagination parameters
	page := 1
	limit := 20
	if pageStr := c.Query("page"); pageStr != "" {
		if pageInt, err := parseInt(pageStr); err == nil && pageInt > 0 {
			page = pageInt
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if limitInt, err := parseInt(limitStr); err == nil && limitInt > 0 {
			limit = limitInt
		}
	}
	offset := (page - 1) * limit

	// Build the query
	query := `
		SELECT al.id, al.user_id, u.username, al.action, al.changes, al.timestamp
		FROM audit_logs al
		LEFT JOIN users u ON al.user_id = u.id
		WHERE al.server_id = $1
	`
	countQuery := `
		SELECT COUNT(*)
		FROM audit_logs al
		WHERE al.server_id = $1
	`
	args := []interface{}{serverID}
	whereCount := 1

	// Add filters if provided
	if actionType := c.Query("actionType"); actionType != "" && actionType != "all" {
		whereCount++
		query += " AND al.action = $" + intToString(whereCount)
		countQuery += " AND action = $" + intToString(whereCount)
		args = append(args, actionType)
	}

	if userIDStr := c.Query("userId"); userIDStr != "" && userIDStr != "all" {
		userID, err := uuid.Parse(userIDStr)
		if err == nil {
			whereCount++
			query += " AND al.user_id = $" + intToString(whereCount)
			countQuery += " AND user_id = $" + intToString(whereCount)
			args = append(args, userID)
		}
	}

	// Add search if provided
	if search := c.Query("search"); search != "" {
		whereCount++
		query += " AND (u.username ILIKE $" + intToString(whereCount) + " OR al.action ILIKE $" + intToString(whereCount) + ")"
		countQuery += " AND (EXISTS (SELECT 1 FROM users WHERE id = user_id AND username ILIKE $" + intToString(whereCount) + ") OR action ILIKE $" + intToString(whereCount) + ")"
		args = append(args, "%"+search+"%")
	}

	// Add date filter if provided
	if dateFilter := c.Query("dateFilter"); dateFilter != "" && dateFilter != "all" {
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
			whereCount++
			query += " AND al.timestamp >= $" + intToString(whereCount)
			countQuery += " AND timestamp >= $" + intToString(whereCount)
			args = append(args, timeFilter)
		}
	}

	// Add ordering and pagination
	query += " ORDER BY al.timestamp DESC LIMIT $" + intToString(whereCount+1) + " OFFSET $" + intToString(whereCount+2)
	args = append(args, limit, offset)

	// Get total count
	var totalCount int
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), countQuery, args[:whereCount]...).Scan(&totalCount)
	if err != nil {
		responses.BadRequest(c, "Failed to count audit logs", &gin.H{"error": err.Error()})
		return
	}

	// Calculate total pages
	totalPages := (totalCount + limit - 1) / limit

	// Execute the query
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), query, args...)
	if err != nil {
		responses.BadRequest(c, "Failed to query audit logs", &gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	// Parse the results
	logs := []AuditLogEntry{}
	for rows.Next() {
		var log AuditLogEntry
		var username sql.NullString
		err := rows.Scan(
			&log.ID,
			&log.UserID,
			&username,
			&log.Action,
			&log.Changes,
			&log.Timestamp,
		)
		if err != nil {
			responses.BadRequest(c, "Failed to scan audit log", &gin.H{"error": err.Error()})
			return
		}

		if username.Valid {
			log.Username = username.String
		} else {
			log.Username = "System"
		}

		logs = append(logs, log)
	}

	responses.Success(c, "Audit logs fetched successfully", &gin.H{
		"logs": logs,
		"pagination": gin.H{
			"total": totalCount,
			"pages": totalPages,
			"page":  page,
			"limit": limit,
		},
	})
}

// Helper functions
func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

func intToString(i int) string {
	return strconv.Itoa(i)
}
