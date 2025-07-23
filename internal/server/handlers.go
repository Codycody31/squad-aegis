package server

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/version"
)

func (s *Server) apiHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Squad Aegis API",
		"data": gin.H{
			"version": version.String(),
		},
	})
}

func (s *Server) healthHandler(c *gin.Context) {
	if err := s.Dependencies.DB.Ping(); err != nil {
		fmt.Println("DB Ping Error: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "unhealthy",
			"code":    http.StatusInternalServerError,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "healthy",
		"code":    http.StatusOK,
	})
}

func (s *Server) customRecovery(c *gin.Context, err any) {
	// Get stack trace
	stack := make([]byte, 4096)
	stack = stack[:runtime.Stack(stack, false)]

	// Log the crash
	log.Error().
		Interface("panic", err).
		Str("stack", string(stack)).
		Str("path", c.Request.URL.Path).
		Str("method", c.Request.Method).
		Str("client_ip", c.ClientIP()).
		Msg("Gin handler crashed")

	c.JSON(http.StatusInternalServerError, gin.H{
		"message": "An internal server error occurred",
		"code":    http.StatusInternalServerError,
	})
	c.Abort()
}

func (s *Server) customLoggerWithFormatter(param gin.LogFormatterParams) string {
	return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
		param.ClientIP,
		param.TimeStamp.Format(time.RFC1123),
		param.Method,
		param.Path,
		param.Request.Proto,
		param.StatusCode,
		param.Latency,
		param.Request.UserAgent(),
		param.ErrorMessage,
	)
}

func (s *Server) customUserLastSeen(c *gin.Context) {
	session, exists := c.Get("session")
	if exists {
		session := session.(*models.Session)
		_, err := s.Dependencies.DB.Exec("UPDATE sessions SET last_seen = NOW(), last_seen_ip = $1 WHERE id = $2", c.ClientIP(), session.Id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Internal Server Error",
				"code":    http.StatusInternalServerError,
			})
			c.Abort()
			return
		}
	}
}

func (s *Server) getUserFromSession(c *gin.Context) *models.User {
	session, exists := c.Get("session")
	if exists {
		session := session.(*models.Session)
		user, err := core.GetUserById(c.Copy(), s.Dependencies.DB, session.UserId, &session.UserId)
		if err != nil {
			return nil
		}
		return user
	}
	return nil
}

// HasServerPermission checks if a user has a specific permission for a server
func (s *Server) HasServerPermission(c *gin.Context, user *models.User, serverId uuid.UUID, permission string) bool {
	// Super admins have all permissions
	if user.SuperAdmin {
		return true
	}

	// Get the user's permissions for this server
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Select("sr.permissions").
		From("server_admins sa").
		Join("server_roles sr ON sa.server_role_id = sr.id").
		Where(squirrel.Eq{"sa.server_id": serverId, "sa.user_id": user.Id}).
		ToSql()
	if err != nil {
		return false
	}

	var permissionsStr string
	err = s.Dependencies.DB.QueryRowContext(c.Copy(), sql, args...).Scan(&permissionsStr)
	if err != nil {
		return false
	}

	// Parse permissions from comma-separated string
	permissions := strings.Split(permissionsStr, ",")

	// Check if the user has the required permission
	for _, p := range permissions {
		if p == permission || p == "*" {
			return true
		}
	}

	return false
}

// HasAnyServerPermission checks if a user has any of the specified permissions for a server
func (s *Server) HasAnyServerPermission(c *gin.Context, user *models.User, serverId uuid.UUID, permissions ...string) bool {
	// Super admins have all permissions
	if user.SuperAdmin {
		return true
	}

	// Get the user's permissions for this server
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Select("sr.permissions").
		From("server_admins sa").
		Join("server_roles sr ON sa.server_role_id = sr.id").
		Where(squirrel.Eq{"sa.server_id": serverId, "sa.user_id": user.Id}).
		ToSql()
	if err != nil {
		return false
	}

	var permissionsStr string
	err = s.Dependencies.DB.QueryRowContext(c.Copy(), sql, args...).Scan(&permissionsStr)
	if err != nil {
		return false
	}

	// Parse permissions from comma-separated string
	userPermissions := strings.Split(permissionsStr, ",")

	// Check if the user has any of the required permissions
	for _, userPerm := range userPermissions {
		if userPerm == "*" {
			return true
		}
		for _, requiredPerm := range permissions {
			if userPerm == requiredPerm {
				return true
			}
		}
	}

	return false
}
