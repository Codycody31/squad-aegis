package server

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/core"
	"go.codycody31.dev/squad-aegis/internal/analytics"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/shared/config"
	"go.codycody31.dev/squad-aegis/version"
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

	// Report to analytics if enabled
	if config.Config.App.Telemetry {
		// Get device info
		deviceInfo := analytics.GetDeviceInfo(!config.Config.App.NonAnonymousTelemetry)

		// Get system state
		var ramCurrent uint64
		if runtime.GOOS == "linux" {
			if data, err := os.ReadFile("/proc/self/status"); err == nil {
				scanner := bufio.NewScanner(bytes.NewReader(data))
				for scanner.Scan() {
					line := scanner.Text()
					if strings.Contains(line, "VmRSS:") {
						fields := strings.Fields(line)
						if len(fields) >= 2 {
							if mem, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
								ramCurrent = mem * 1024 // Convert from KB to bytes
							}
						}
					}
				}
			}
		}

		// Get disk info
		var diskCurrent, diskTotal uint64
		if runtime.GOOS == "linux" {
			var stat syscall.Statfs_t
			if err := syscall.Statfs("/", &stat); err == nil {
				diskTotal = stat.Blocks * uint64(stat.Bsize)
				diskCurrent = diskTotal - (stat.Bfree * uint64(stat.Bsize))
			}
		}

		s.Dependencies.MetricsCollector.GetCountly().TrackCrash(map[string]interface{}{
			// Device metrics
			"_os":          deviceInfo.OS,
			"_os_version":  deviceInfo.OSVersion,
			"_device":      deviceInfo.DeviceName,
			"_app_version": version.String(),
			"_cpu":         deviceInfo.OSArch,

			// Device state
			"_ram_current":  ramCurrent / (1024 * 1024),             // Convert to MB
			"_ram_total":    deviceInfo.MemoryTotal / (1024 * 1024), // Convert to MB
			"_disk_current": diskCurrent / (1024 * 1024),            // Convert to MB
			"_disk_total":   diskTotal / (1024 * 1024),              // Convert to MB

			// System state
			"_root":       false, // Not applicable for server
			"_online":     true,  // Server is always online when running
			"_muted":      false, // Not applicable for server
			"_background": false, // Server is always in foreground

			// Error info
			"_name":     fmt.Sprintf("%v", err),
			"_error":    string(stack),
			"_nonfatal": true,
			"_logs":     log.Logger.GetLevel().String(),

			// Custom data
			"_custom": map[string]interface{}{
				"container":  deviceInfo.Metrics["container"],
				"env":        deviceInfo.Metrics["env"],
				"hostname":   deviceInfo.Metrics["hostname"],
				"path":       c.Request.URL.Path,
				"method":     c.Request.Method,
				"client_ip":  c.ClientIP(),
				"user_agent": c.Request.UserAgent(),
			},
		})
	}

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
