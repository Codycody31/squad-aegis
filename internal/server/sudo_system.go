package server

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

// SystemHealthResponse represents overall system health status
type SystemHealthResponse struct {
	PostgreSQL SystemServiceHealth `json:"postgresql"`
	ClickHouse SystemServiceHealth `json:"clickhouse"`
	Valkey     SystemServiceHealth `json:"valkey"`
	Storage    SystemServiceHealth `json:"storage"`
	Overall    string              `json:"overall"`
}

// SystemServiceHealth represents health status of a system service
type SystemServiceHealth struct {
	Status       string  `json:"status"` // healthy, degraded, unhealthy
	Latency      int64   `json:"latency"` // in milliseconds
	Message      string  `json:"message"`
	Details      gin.H   `json:"details,omitempty"`
}

// SystemConfigResponse represents sanitized configuration
type SystemConfigResponse struct {
	App        gin.H `json:"app"`
	Database   gin.H `json:"database"`
	ClickHouse gin.H `json:"clickhouse"`
	Valkey     gin.H `json:"valkey"`
	Storage    gin.H `json:"storage"`
	Log        gin.H `json:"log"`
}

// GetSystemHealth returns overall system health status
func (s *Server) GetSystemHealth(c *gin.Context) {
	ctx := c.Request.Context()

	var health SystemHealthResponse

	// Check PostgreSQL
	health.PostgreSQL = checkPostgreSQLHealth(ctx, s.Dependencies.DB)

	// Check ClickHouse
	if s.Dependencies.Clickhouse != nil {
		health.ClickHouse = checkClickHouseHealth(ctx, s.Dependencies.Clickhouse)
	} else {
		health.ClickHouse = SystemServiceHealth{
			Status:  "unhealthy",
			Message: "ClickHouse client not initialized",
		}
	}

	// Check Valkey
	if s.Dependencies.Valkey != nil {
		health.Valkey = checkValkeyHealth(ctx, s.Dependencies.Valkey)
	} else {
		health.Valkey = SystemServiceHealth{
			Status:  "unhealthy",
			Message: "Valkey client not initialized",
		}
	}

	// Check Storage
	if s.Dependencies.Storage != nil {
		health.Storage = checkStorageHealth(ctx, s)
	} else {
		health.Storage = SystemServiceHealth{
			Status:  "unhealthy",
			Message: "Storage not initialized",
		}
	}

	// Determine overall health
	health.Overall = "healthy"
	if health.PostgreSQL.Status == "unhealthy" || health.ClickHouse.Status == "unhealthy" {
		health.Overall = "unhealthy"
	} else if health.PostgreSQL.Status == "degraded" || health.ClickHouse.Status == "degraded" ||
		health.Valkey.Status == "degraded" || health.Storage.Status == "degraded" {
		health.Overall = "degraded"
	}

	responses.Success(c, "System health retrieved successfully", &gin.H{"data": health})
}

// GetSystemConfig returns sanitized system configuration
func (s *Server) GetSystemConfig(c *gin.Context) {
	cfg := config.Config

	configResponse := SystemConfigResponse{
		App: gin.H{
			"is_development": cfg.App.IsDevelopment,
			"port":           cfg.App.Port,
			"url":            cfg.App.Url,
			"in_container":   cfg.App.InContainer,
		},
		Database: gin.H{
			"host": cfg.Db.Host,
			"port": cfg.Db.Port,
			"name": cfg.Db.Name,
			"user": cfg.Db.User,
			"pass": "***REDACTED***",
		},
		ClickHouse: gin.H{
			"host":     cfg.ClickHouse.Host,
			"port":     cfg.ClickHouse.Port,
			"database": cfg.ClickHouse.Database,
			"username": cfg.ClickHouse.Username,
			"password": "***REDACTED***",
			"debug":    cfg.ClickHouse.Debug,
		},
		Valkey: gin.H{
			"host":     cfg.Valkey.Host,
			"port":     cfg.Valkey.Port,
			"password": "***REDACTED***",
			"database": cfg.Valkey.Database,
		},
		Storage: gin.H{
			"type":       cfg.Storage.Type,
			"local_path": cfg.Storage.LocalPath,
			"s3": gin.H{
				"region":            cfg.Storage.S3.Region,
				"bucket":            cfg.Storage.S3.Bucket,
				"access_key_id":     maskString(cfg.Storage.S3.AccessKeyID),
				"secret_access_key": "***REDACTED***",
				"endpoint":          cfg.Storage.S3.Endpoint,
				"use_ssl":           cfg.Storage.S3.UseSSL,
			},
		},
		Log: gin.H{
			"level":             cfg.Log.Level,
			"show_gin":          cfg.Log.ShowGin,
			"show_plugin_logs":  cfg.Log.ShowPluginLogs,
			"file":              cfg.Log.File,
		},
	}

	responses.Success(c, "System configuration retrieved successfully", &gin.H{"data": configResponse})
}

// checkPostgreSQLHealth checks PostgreSQL database health
func checkPostgreSQLHealth(ctx context.Context, db *sql.DB) SystemServiceHealth {
	start := time.Now()
	
	err := db.PingContext(ctx)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return SystemServiceHealth{
			Status:  "unhealthy",
			Latency: latency,
			Message: fmt.Sprintf("Connection failed: %v", err),
		}
	}

	// Get database stats
	var dbSize string
	var tableCount int
	
	db.QueryRowContext(ctx, `
		SELECT pg_size_pretty(pg_database_size(current_database()))
	`).Scan(&dbSize)

	db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.tables 
		WHERE table_schema = 'public'
	`).Scan(&tableCount)

	// Check connection pool stats
	stats := db.Stats()

	details := gin.H{
		"database_size": dbSize,
		"table_count":   tableCount,
		"open_connections": stats.OpenConnections,
		"in_use":        stats.InUse,
		"idle":          stats.Idle,
	}

	status := "healthy"
	message := "PostgreSQL is operational"

	// Check for degraded status
	if latency > 100 {
		status = "degraded"
		message = "High latency detected"
	}

	if stats.OpenConnections > 50 {
		status = "degraded"
		message = "High connection count"
	}

	return SystemServiceHealth{
		Status:  status,
		Latency: latency,
		Message: message,
		Details: details,
	}
}

// checkClickHouseHealth checks ClickHouse health
func checkClickHouseHealth(ctx context.Context, ch interface{}) SystemServiceHealth {
	start := time.Now()

	// Type assertion to get the underlying connection
	type ClickHousePinger interface {
		Ping(context.Context) error
		Query(context.Context, string, ...interface{}) (*sql.Rows, error)
	}

	client, ok := ch.(ClickHousePinger)
	if !ok {
		return SystemServiceHealth{
			Status:  "unhealthy",
			Message: "Unable to access ClickHouse client",
		}
	}

	err := client.Ping(ctx)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return SystemServiceHealth{
			Status:  "unhealthy",
			Latency: latency,
			Message: fmt.Sprintf("Connection failed: %v", err),
		}
	}

	// Get database stats
	var totalRows, totalBytes uint64
	
	rows, err := client.Query(ctx, `
		SELECT 
			SUM(rows) as total_rows,
			SUM(bytes) as total_bytes
		FROM system.parts
		WHERE database = 'squad_aegis' AND active = 1
	`)

	if err == nil {
		defer rows.Close()
		if rows.Next() {
			rows.Scan(&totalRows, &totalBytes)
		}
	}

	details := gin.H{
		"total_rows":  totalRows,
		"total_bytes": formatBytes(int64(totalBytes)),
	}

	status := "healthy"
	message := "ClickHouse is operational"

	if latency > 200 {
		status = "degraded"
		message = "High latency detected"
	}

	return SystemServiceHealth{
		Status:  status,
		Latency: latency,
		Message: message,
		Details: details,
	}
}

// checkValkeyHealth checks Valkey health
func checkValkeyHealth(ctx context.Context, valkey interface{}) SystemServiceHealth {
	start := time.Now()

	// Type assertion to get the underlying connection
	type ValkeyPinger interface {
		Ping(context.Context) error
	}

	client, ok := valkey.(ValkeyPinger)
	if !ok {
		return SystemServiceHealth{
			Status:  "unhealthy",
			Message: "Unable to access Valkey client",
		}
	}

	err := client.Ping(ctx)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return SystemServiceHealth{
			Status:  "unhealthy",
			Latency: latency,
			Message: fmt.Sprintf("Connection failed: %v", err),
		}
	}

	status := "healthy"
	message := "Valkey is operational"

	if latency > 50 {
		status = "degraded"
		message = "High latency detected"
	}

	return SystemServiceHealth{
		Status:  status,
		Latency: latency,
		Message: message,
	}
}

// checkStorageHealth checks storage backend health
func checkStorageHealth(ctx context.Context, s *Server) SystemServiceHealth {
	start := time.Now()

	stats, err := s.Dependencies.Storage.GetStats(ctx)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return SystemServiceHealth{
			Status:  "unhealthy",
			Latency: latency,
			Message: fmt.Sprintf("Failed to get storage stats: %v", err),
		}
	}

	storageType := "local"
	details := gin.H{
		"type":        storageType,
		"total_files": stats.TotalFiles,
		"total_size":  formatBytes(stats.TotalSize),
	}

	status := "healthy"
	message := "Storage is operational"

	if latency > 500 {
		status = "degraded"
		message = "High latency detected"
	}

	return SystemServiceHealth{
		Status:  status,
		Latency: latency,
		Message: message,
		Details: details,
	}
}

// maskString masks a string for security (shows first and last 4 chars)
func maskString(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "***REDACTED***"
	}
	return s[:4] + "***" + s[len(s)-4:]
}

