package server

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// SessionInfo represents session information for display
type SessionInfo struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	Username    string     `json:"username"`
	Token       string     `json:"token"` // Masked for security
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	LastSeen    time.Time  `json:"last_seen"`
	LastSeenIP  string     `json:"last_seen_ip"`
	IsExpired   bool       `json:"is_expired"`
	TimeRemaining string    `json:"time_remaining"`
}

// SessionStatsResponse represents session statistics
type SessionStatsResponse struct {
	TotalSessions    int                 `json:"total_sessions"`
	ActiveSessions   int                 `json:"active_sessions"`
	ExpiredSessions  int                 `json:"expired_sessions"`
	SessionsByUser   []UserSessionCount  `json:"sessions_by_user"`
}

// UserSessionCount represents session count per user
type UserSessionCount struct {
	UserID       uuid.UUID `json:"user_id"`
	Username     string    `json:"username"`
	SessionCount int       `json:"session_count"`
}

// GetAllSessions returns all active sessions
func (s *Server) GetAllSessions(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse query parameters
	showExpired := c.Query("show_expired") == "true"
	userIDFilter := c.Query("user_id")

	// Build query
	query := `
		SELECT s.id, s.user_id, u.username, s.token, s.created_at, s.expires_at, s.last_seen, s.last_seen_ip
		FROM sessions s
		LEFT JOIN users u ON s.user_id = u.id
		WHERE 1=1
	`
	args := []interface{}{}
	paramCount := 0

	// Filter by active/expired
	if !showExpired {
		query += " AND (s.expires_at IS NULL OR s.expires_at > NOW())"
	}

	// Filter by user
	if userIDFilter != "" && userIDFilter != "all" {
		userID, err := uuid.Parse(userIDFilter)
		if err == nil {
			paramCount++
			query += fmt.Sprintf(" AND s.user_id = $%d", paramCount)
			args = append(args, userID)
		}
	}

	query += " ORDER BY s.last_seen DESC"

	// Execute query
	rows, err := s.Dependencies.DB.QueryContext(ctx, query, args...)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to query sessions: %w", err), nil)
		return
	}
	defer rows.Close()

	// Parse results
	sessions := []SessionInfo{}
	now := time.Now()

	for rows.Next() {
		var session SessionInfo
		var username sql.NullString
		var expiresAt sql.NullTime
		var lastSeenIP sql.NullString

		err := rows.Scan(
			&session.ID,
			&session.UserID,
			&username,
			&session.Token,
			&session.CreatedAt,
			&expiresAt,
			&session.LastSeen,
			&lastSeenIP,
		)
		if err != nil {
			continue
		}

		if username.Valid {
			session.Username = username.String
		} else {
			session.Username = "Unknown"
		}

		if lastSeenIP.Valid {
			session.LastSeenIP = lastSeenIP.String
		}

		// Mask token for security (show first 8 and last 4 characters)
		if len(session.Token) > 12 {
			session.Token = session.Token[:8] + "..." + session.Token[len(session.Token)-4:]
		}

		// Check if expired
		if expiresAt.Valid {
			session.ExpiresAt = &expiresAt.Time
			session.IsExpired = expiresAt.Time.Before(now)
			
			if !session.IsExpired {
				duration := time.Until(expiresAt.Time)
				session.TimeRemaining = formatSessionDuration(duration)
			} else {
				session.TimeRemaining = "Expired"
			}
		} else {
			session.TimeRemaining = "Never expires"
		}

		sessions = append(sessions, session)
	}

	responses.Success(c, "Sessions retrieved successfully", &gin.H{
		"sessions": sessions,
		"total":    len(sessions),
	})
}

// GetSessionStats returns session statistics
func (s *Server) GetSessionStats(c *gin.Context) {
	ctx := c.Request.Context()

	var stats SessionStatsResponse
	// Initialize empty slice to avoid null
	stats.SessionsByUser = []UserSessionCount{}

	// Get total sessions
	s.Dependencies.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sessions
	`).Scan(&stats.TotalSessions)

	// Get active sessions
	s.Dependencies.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sessions
		WHERE expires_at IS NULL OR expires_at > NOW()
	`).Scan(&stats.ActiveSessions)

	// Get expired sessions
	s.Dependencies.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sessions
		WHERE expires_at IS NOT NULL AND expires_at <= NOW()
	`).Scan(&stats.ExpiredSessions)

	// Get sessions by user
	rows, err := s.Dependencies.DB.QueryContext(ctx, `
		SELECT s.user_id, u.username, COUNT(*) as session_count
		FROM sessions s
		LEFT JOIN users u ON s.user_id = u.id
		WHERE s.expires_at IS NULL OR s.expires_at > NOW()
		GROUP BY s.user_id, u.username
		ORDER BY session_count DESC
		LIMIT 20
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var usc UserSessionCount
			if rows.Scan(&usc.UserID, &usc.Username, &usc.SessionCount) == nil {
				stats.SessionsByUser = append(stats.SessionsByUser, usc)
			}
		}
	}

	responses.Success(c, "Session statistics retrieved successfully", &gin.H{"data": stats})
}

// DeleteSession force logs out a specific session
func (s *Server) DeleteSession(c *gin.Context) {
	ctx := c.Request.Context()

	sessionIDStr := c.Param("sessionId")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid session ID", nil)
		return
	}

	// Get current session to prevent self-logout
	currentSession := c.MustGet("session").(*SessionInfo)
	if currentSession.ID == sessionID {
		responses.BadRequest(c, "Cannot delete your own session", nil)
		return
	}

	// Delete the session
	result, err := s.Dependencies.DB.ExecContext(ctx, `
		DELETE FROM sessions WHERE id = $1
	`, sessionID)

	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to delete session: %w", err), nil)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		responses.NotFound(c, "Session not found", nil)
		return
	}

	responses.SimpleSuccess(c, "Session deleted successfully")
}

// DeleteUserSessions force logs out all sessions for a specific user
func (s *Server) DeleteUserSessions(c *gin.Context) {
	ctx := c.Request.Context()

	userIDStr := c.Param("userId")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid user ID", nil)
		return
	}

	// Get current session to prevent self-logout
	currentSession := c.MustGet("session")
	if currentSession != nil {
		if sess, ok := currentSession.(*SessionInfo); ok && sess.UserID == userID {
			responses.BadRequest(c, "Cannot delete your own sessions", nil)
			return
		}
	}

	// Delete all sessions for the user
	result, err := s.Dependencies.DB.ExecContext(ctx, `
		DELETE FROM sessions WHERE user_id = $1
	`, userID)

	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to delete sessions: %w", err), nil)
		return
	}

	rowsAffected, _ := result.RowsAffected()

	responses.Success(c, "User sessions deleted successfully", &gin.H{
		"deleted_count": rowsAffected,
	})
}

// CleanupExpiredSessions removes all expired sessions from the database
func (s *Server) CleanupExpiredSessions(c *gin.Context) {
	ctx := c.Request.Context()

	result, err := s.Dependencies.DB.ExecContext(ctx, `
		DELETE FROM sessions 
		WHERE expires_at IS NOT NULL AND expires_at <= NOW()
	`)

	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to cleanup sessions: %w", err), nil)
		return
	}

	rowsAffected, _ := result.RowsAffected()

	responses.Success(c, "Expired sessions cleaned up successfully", &gin.H{
		"deleted_count": rowsAffected,
	})
}

// formatSessionDuration formats a duration into a human-readable string
func formatSessionDuration(d time.Duration) string {
	if d < 0 {
		return "Expired"
	}

	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

