package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/pagination"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

const serverAuditCursorKind = "audit.server"

// AuditLogEntry represents an audit log entry.
type AuditLogEntry struct {
	ID        uuid.UUID       `json:"id"`
	UserID    *uuid.UUID      `json:"user_id,omitempty"`
	Username  string          `json:"username,omitempty"`
	Action    string          `json:"action"`
	Changes   json.RawMessage `json:"changes,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

type auditCursorValues struct {
	Timestamp time.Time `json:"timestamp"`
	ID        uuid.UUID `json:"id"`
}

type serverAuditFilters struct {
	ServerID  uuid.UUID  `json:"server_id"`
	Action    string     `json:"action"`
	UserID    *uuid.UUID `json:"user_id"`
	Search    string     `json:"search"`
	DateRange string     `json:"date_range"`
}

// CreateAuditLog creates a new audit log entry.
func (s *Server) CreateAuditLog(ctx context.Context, serverID *uuid.UUID, userID *uuid.UUID, action string, changes interface{}) {
	changesJSON, err := json.Marshal(changes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal changes for audit log")
		return
	}

	_, err = s.Dependencies.DB.ExecContext(ctx, `
		/* sa.audit.create */
		INSERT INTO audit_logs (id, server_id, user_id, action, changes, timestamp, username_snapshot)
		VALUES ($1, $2, $3, $4, $5, $6, (SELECT username FROM users WHERE id = $3))
	`, uuid.New(), serverID, userID, action, changesJSON, time.Now())
	if err != nil {
		log.Error().Err(err).Msg("Failed to create audit log")
	}
}

// ServerAuditLogs handles keyset-paginated audit logs for a server.
func (s *Server) ServerAuditLogs(c *gin.Context) {
	ctx := c.Request.Context()
	user := s.getUserFromSession(c)
	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}
	if _, err = core.GetServerById(ctx, s.Dependencies.DB, serverID, user); err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	limit, ok := parseCursorLimit(c, 50, 100)
	if !ok || rejectLegacyPagination(c, "page", "offset") {
		return
	}

	filters := serverAuditFilters{
		ServerID:  serverID,
		Action:    normalizedAllFilter(c.Query("actionType")),
		Search:    strings.ToLower(strings.TrimSpace(c.Query("search"))),
		DateRange: normalizedAllFilter(c.Query("dateFilter")),
	}
	if rawUserID := normalizedAllFilter(c.Query("userId")); rawUserID != "" {
		parsed, parseErr := uuid.Parse(rawUserID)
		if parseErr != nil {
			responses.BadRequest(c, "Invalid user ID", &gin.H{"error": parseErr.Error()})
			return
		}
		filters.UserID = &parsed
	}
	filterHash := pagination.MustFilterHash(filters)

	var cursorValues *auditCursorValues
	if encodedCursor := c.Query("cursor"); encodedCursor != "" {
		cursorValues = &auditCursorValues{}
		if err := pagination.Decode(encodedCursor, serverAuditCursorKind, filterHash, cursorValues); err != nil {
			responses.BadRequest(c, "Invalid cursor", &gin.H{"error": err.Error()})
			return
		}
	}

	query := `
		/* sa.audit.list */
		SELECT al.id, al.user_id, COALESCE(u.username, al.username_snapshot),
		       al.action, al.changes, al.timestamp
		FROM audit_logs al
		LEFT JOIN users u ON al.user_id = u.id
		WHERE al.server_id = $1`
	args := []any{serverID}
	addArg := func(value any) string {
		args = append(args, value)
		return "$" + strconv.Itoa(len(args))
	}

	if filters.Action != "" {
		query += " AND al.action = " + addArg(filters.Action)
	}
	if filters.UserID != nil {
		query += " AND al.user_id = " + addArg(*filters.UserID)
	}
	if filters.Search != "" {
		placeholder := addArg("%" + filters.Search + "%")
		query += ` AND (
			lower(al.action) LIKE ` + placeholder + ` OR
			lower(COALESCE(al.username_snapshot, '')) LIKE ` + placeholder + ` OR
			EXISTS (SELECT 1 FROM users au WHERE au.id = al.user_id AND lower(au.username) LIKE ` + placeholder + `)
		)`
	}
	if start := auditDateStart(filters.DateRange, time.Now()); start != nil {
		query += " AND al.timestamp >= " + addArg(*start)
	}
	if cursorValues != nil {
		timestampArg := addArg(cursorValues.Timestamp)
		idArg := addArg(cursorValues.ID)
		query += " AND (al.timestamp, al.id) < (" + timestampArg + ", " + idArg + ")"
	}
	query += " ORDER BY al.timestamp DESC, al.id DESC LIMIT " + addArg(limit+1)

	started := time.Now()
	rows, err := s.Dependencies.DB.QueryContext(ctx, query, args...)
	if err != nil {
		logAuditQuery("sa.audit.list", started, 0, err)
		responses.InternalServerError(c, fmt.Errorf("failed to query audit logs: %w", err), nil)
		return
	}
	defer rows.Close()

	logs := make([]AuditLogEntry, 0, limit+1)
	for rows.Next() {
		var entry AuditLogEntry
		var username sql.NullString
		if err := rows.Scan(&entry.ID, &entry.UserID, &username, &entry.Action, &entry.Changes, &entry.Timestamp); err != nil {
			logAuditQuery("sa.audit.list", started, len(logs), err)
			responses.InternalServerError(c, fmt.Errorf("failed to scan audit log: %w", err), nil)
			return
		}
		if username.Valid {
			entry.Username = username.String
		} else {
			entry.Username = "System"
		}
		logs = append(logs, entry)
	}
	if err := rows.Err(); err != nil {
		logAuditQuery("sa.audit.list", started, len(logs), err)
		responses.InternalServerError(c, fmt.Errorf("iterate audit logs: %w", err), nil)
		return
	}
	logs, hasMore := pagination.TrimPage(logs, limit)
	pageInfo, err := auditPageInfo(serverAuditCursorKind, filterHash, logs, hasMore)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	logAuditQuery("sa.audit.list", started, len(logs), nil)

	responses.Success(c, "Audit logs fetched successfully", &gin.H{
		"logs":      logs,
		"page_info": pageInfo,
	})
}

func auditPageInfo[T interface {
	AuditLogEntry | GlobalAuditLogEntry
}](kind, filterHash string, logs []T, hasMore bool) (pagination.PageInfo, error) {
	info := pagination.PageInfo{HasMore: hasMore}
	if !hasMore || len(logs) == 0 {
		return info, nil
	}
	var values auditCursorValues
	switch entry := any(logs[len(logs)-1]).(type) {
	case AuditLogEntry:
		values = auditCursorValues{Timestamp: entry.Timestamp, ID: entry.ID}
	case GlobalAuditLogEntry:
		values = auditCursorValues{Timestamp: entry.Timestamp, ID: entry.ID}
	default:
		return info, fmt.Errorf("unsupported audit cursor row")
	}
	encoded, err := pagination.Encode(kind, filterHash, values)
	if err != nil {
		return info, fmt.Errorf("encode audit cursor: %w", err)
	}
	info.NextCursor = &encoded
	return info, nil
}

func auditDateStart(dateFilter string, now time.Time) *time.Time {
	switch dateFilter {
	case "today":
		value := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return &value
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		value := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
		return &value
	case "week":
		value := now.AddDate(0, 0, -7)
		return &value
	case "month":
		value := now.AddDate(0, -1, 0)
		return &value
	default:
		return nil
	}
}

func logAuditQuery(label string, started time.Time, rows int, err error) {
	duration := time.Since(started)
	event := log.Debug()
	if err != nil {
		event = log.Error().Err(err)
	} else if duration >= 500*time.Millisecond {
		event = log.Warn()
	}
	event.Str("query", label).Dur("duration", duration).Int("rows", rows).Msg("database query")
}

// Helper functions retained for existing handlers.
func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

func intToString(i int) string {
	return strconv.Itoa(i)
}
