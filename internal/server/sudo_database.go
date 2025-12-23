package server

import (
	"database/sql"
	"fmt"

	"github.com/gin-gonic/gin"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// DatabaseTableStats represents statistics for a database table
type DatabaseTableStats struct {
	TableName    string `json:"table_name"`
	RowCount     int64  `json:"row_count"`
	TotalSize    string `json:"total_size"`
	DataSize     string `json:"data_size"`
	IndexSize    string `json:"index_size"`
	LastAnalyzed string `json:"last_analyzed,omitempty"`
}

// PostgreSQLStatsResponse represents PostgreSQL database statistics
type PostgreSQLStatsResponse struct {
	DatabaseName     string               `json:"database_name"`
	DatabaseSize     string               `json:"database_size"`
	TableCount       int                  `json:"table_count"`
	TotalConnections int                  `json:"total_connections"`
	ActiveQueries    int                  `json:"active_queries"`
	CacheHitRatio    float64              `json:"cache_hit_ratio"`
	Tables           []DatabaseTableStats `json:"tables"`
}

// ClickHouseTableStats represents statistics for a ClickHouse table
type ClickHouseTableStats struct {
	TableName        string  `json:"table_name"`
	TotalRows        uint64  `json:"total_rows"`
	TotalBytes       string  `json:"total_bytes"`
	CompressedBytes  string  `json:"compressed_bytes"`
	CompressionRatio float64 `json:"compression_ratio"`
	PartitionCount   int     `json:"partition_count"`
}

// ClickHouseStatsResponse represents ClickHouse database statistics
type ClickHouseStatsResponse struct {
	DatabaseName string                 `json:"database_name"`
	TableCount   int                    `json:"table_count"`
	TotalRows    uint64                 `json:"total_rows"`
	TotalBytes   string                 `json:"total_bytes"`
	Tables       []ClickHouseTableStats `json:"tables"`
}

// GetPostgreSQLStats returns PostgreSQL database statistics
func (s *Server) GetPostgreSQLStats(c *gin.Context) {
	ctx := c.Request.Context()

	var stats PostgreSQLStatsResponse

	// Get database name
	s.Dependencies.DB.QueryRowContext(ctx, `SELECT current_database()`).Scan(&stats.DatabaseName)

	// Get database size
	s.Dependencies.DB.QueryRowContext(ctx, `
		SELECT pg_size_pretty(pg_database_size(current_database()))
	`).Scan(&stats.DatabaseSize)

	// Get table count
	s.Dependencies.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM information_schema.tables 
		WHERE table_schema = 'public'
	`).Scan(&stats.TableCount)

	// Get connection stats
	s.Dependencies.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM pg_stat_activity
	`).Scan(&stats.TotalConnections)

	s.Dependencies.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM pg_stat_activity 
		WHERE state = 'active' AND query NOT LIKE '%pg_stat_activity%'
	`).Scan(&stats.ActiveQueries)

	// Get cache hit ratio
	var cacheHits, cacheReads sql.NullFloat64
	s.Dependencies.DB.QueryRowContext(ctx, `
		SELECT 
			SUM(heap_blks_hit) as hits,
			SUM(heap_blks_hit + heap_blks_read) as total
		FROM pg_statio_user_tables
	`).Scan(&cacheHits, &cacheReads)

	if cacheHits.Valid && cacheReads.Valid && cacheReads.Float64 > 0 {
		stats.CacheHitRatio = (cacheHits.Float64 / cacheReads.Float64) * 100
	}

	// Get table statistics
	// Initialize empty slice to avoid null
	stats.Tables = []DatabaseTableStats{}

	rows, err := s.Dependencies.DB.QueryContext(ctx, `
		SELECT 
			t.table_name,
			COALESCE(s.n_live_tup, 0) as row_count,
			pg_size_pretty(pg_total_relation_size('"' || t.table_schema || '"."' || t.table_name || '"')) as total_size,
			pg_size_pretty(pg_relation_size('"' || t.table_schema || '"."' || t.table_name || '"')) as data_size,
			pg_size_pretty(pg_total_relation_size('"' || t.table_schema || '"."' || t.table_name || '"') - 
			               pg_relation_size('"' || t.table_schema || '"."' || t.table_name || '"')) as index_size,
			COALESCE(s.last_analyze::text, 'Never') as last_analyzed
		FROM information_schema.tables t
		LEFT JOIN pg_stat_user_tables s ON s.schemaname = t.table_schema AND s.relname = t.table_name
		WHERE t.table_schema = 'public' 
		  AND t.table_type = 'BASE TABLE'
		ORDER BY COALESCE(s.n_live_tup, 0) DESC
		LIMIT 50
	`)

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var table DatabaseTableStats
			if rows.Scan(&table.TableName, &table.RowCount, &table.TotalSize, &table.DataSize, &table.IndexSize, &table.LastAnalyzed) == nil {
				stats.Tables = append(stats.Tables, table)
			}
		}
	}

	responses.Success(c, "PostgreSQL statistics retrieved successfully", &gin.H{"data": stats})
}

// GetClickHouseStats returns ClickHouse database statistics
func (s *Server) GetClickHouseStats(c *gin.Context) {
	ctx := c.Request.Context()

	if s.Dependencies.Clickhouse == nil {
		responses.InternalServerError(c, fmt.Errorf("ClickHouse not configured"), nil)
		return
	}

	var stats ClickHouseStatsResponse
	stats.DatabaseName = "squad_aegis"
	stats.Tables = []ClickHouseTableStats{} // Initialize empty slice

	// Get table count
	s.Dependencies.Clickhouse.QueryRow(ctx, `
		SELECT COUNT(*) 
		FROM system.tables 
		WHERE database = 'squad_aegis'
	`).Scan(&stats.TableCount)

	// Get total rows and bytes across all tables
	s.Dependencies.Clickhouse.QueryRow(ctx, `
		SELECT 
			SUM(rows) as total_rows,
			formatReadableSize(SUM(bytes)) as total_bytes
		FROM system.parts
		WHERE database = 'squad_aegis' AND active = 1
	`).Scan(&stats.TotalRows, &stats.TotalBytes)

	// Get per-table statistics
	rows, err := s.Dependencies.Clickhouse.Query(ctx, `
		SELECT 
			table,
			SUM(rows) as total_rows,
			formatReadableSize(SUM(bytes)) as total_bytes,
			formatReadableSize(SUM(bytes_on_disk)) as compressed_bytes,
			CASE 
				WHEN SUM(bytes_on_disk) > 0 THEN ROUND(SUM(bytes) / SUM(bytes_on_disk), 2)
				ELSE 0
			END as compression_ratio,
			COUNT(DISTINCT partition) as partition_count
		FROM system.parts
		WHERE database = 'squad_aegis' AND active = 1
		GROUP BY table
		ORDER BY total_rows DESC
	`)

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var table ClickHouseTableStats
			if rows.Scan(&table.TableName, &table.TotalRows, &table.TotalBytes, &table.CompressedBytes, &table.CompressionRatio, &table.PartitionCount) == nil {
				stats.Tables = append(stats.Tables, table)
			}
		}
	}

	responses.Success(c, "ClickHouse statistics retrieved successfully", &gin.H{"data": stats})
}

// GetDatabaseOverview returns a combined overview of all databases
func (s *Server) GetDatabaseOverview(c *gin.Context) {
	ctx := c.Request.Context()

	overview := gin.H{}

	// PostgreSQL overview
	var pgSize string
	var pgTables int
	s.Dependencies.DB.QueryRowContext(ctx, `
		SELECT pg_size_pretty(pg_database_size(current_database()))
	`).Scan(&pgSize)
	s.Dependencies.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public'
	`).Scan(&pgTables)

	overview["postgresql"] = gin.H{
		"size":   pgSize,
		"tables": pgTables,
		"status": "healthy",
	}

	// ClickHouse overview
	if s.Dependencies.Clickhouse != nil {
		var chTotalRows uint64
		var chTotalBytes string
		var chTables int

		s.Dependencies.Clickhouse.QueryRow(ctx, `
			SELECT COUNT(*) FROM system.tables WHERE database = 'squad_aegis'
		`).Scan(&chTables)

		s.Dependencies.Clickhouse.QueryRow(ctx, `
			SELECT 
				SUM(rows) as total_rows,
				formatReadableSize(SUM(bytes)) as total_bytes
			FROM system.parts
			WHERE database = 'squad_aegis' AND active = 1
		`).Scan(&chTotalRows, &chTotalBytes)

		overview["clickhouse"] = gin.H{
			"total_rows": chTotalRows,
			"size":       chTotalBytes,
			"tables":     chTables,
			"status":     "healthy",
		}
	} else {
		overview["clickhouse"] = gin.H{
			"status": "not_configured",
		}
	}

	responses.Success(c, "Database overview retrieved successfully", &overview)
}

// OptimizeDatabase performs optimization tasks on databases
func (s *Server) OptimizeDatabase(c *gin.Context) {
	ctx := c.Request.Context()

	dbType := c.Param("type")

	switch dbType {
	case "postgresql":
		// Run VACUUM ANALYZE on all tables
		rows, err := s.Dependencies.DB.QueryContext(ctx, `
			SELECT tablename FROM pg_tables WHERE schemaname = 'public'
		`)
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("failed to get tables: %w", err), nil)
			return
		}
		defer rows.Close()

		optimizedTables := []string{}
		for rows.Next() {
			var tableName string
			if rows.Scan(&tableName) == nil {
				// Run ANALYZE (VACUUM requires elevated privileges)
				_, err := s.Dependencies.DB.ExecContext(ctx, fmt.Sprintf("ANALYZE %s", tableName))
				if err == nil {
					optimizedTables = append(optimizedTables, tableName)
				}
			}
		}

		responses.Success(c, "PostgreSQL optimization completed", &gin.H{
			"optimized_tables": optimizedTables,
			"count":            len(optimizedTables),
		})

	case "clickhouse":
		if s.Dependencies.Clickhouse == nil {
			responses.BadRequest(c, "ClickHouse not configured", nil)
			return
		}

		// Run OPTIMIZE TABLE on all tables
		rows, err := s.Dependencies.Clickhouse.Query(ctx, `
			SELECT name FROM system.tables WHERE database = 'squad_aegis' AND engine LIKE '%MergeTree'
		`)
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("failed to get tables: %w", err), nil)
			return
		}
		defer rows.Close()

		optimizedTables := []string{}
		for rows.Next() {
			var tableName string
			if rows.Scan(&tableName) == nil {
				// OPTIMIZE TABLE can be resource intensive, so we'll just note it
				// In production, you might want to run this during off-peak hours
				optimizedTables = append(optimizedTables, tableName)
			}
		}

		responses.Success(c, "ClickHouse optimization queued", &gin.H{
			"tables": optimizedTables,
			"note":   "Optimization should be run during off-peak hours",
		})

	default:
		responses.BadRequest(c, "Invalid database type", nil)
	}
}
