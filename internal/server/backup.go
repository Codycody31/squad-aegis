package server

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/clickhouse"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// BackupMetadata contains information about a backup
type BackupMetadata struct {
	ID             uuid.UUID `json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	PostgresSize   int64     `json:"postgres_size"`
	ClickhouseSize int64     `json:"clickhouse_size"`
	TotalSize      int64     `json:"total_size"`
	Description    string    `json:"description"`
	Version        string    `json:"version"`
}

// BackupResponse represents the response for backup operations
type BackupResponse struct {
	BackupID    uuid.UUID `json:"backup_id"`
	Status      string    `json:"status"`
	Message     string    `json:"message"`
	DownloadURL string    `json:"download_url,omitempty"`
}

// RestoreRequest represents a restore request
type RestoreRequest struct {
	BackupID    uuid.UUID `json:"backup_id,omitempty"`
	Description string    `json:"description,omitempty"`
}

// CreateBackup creates a full backup of PostgreSQL and ClickHouse data
func (s *Server) CreateBackup(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Authentication required", nil)
		return
	}

	// Get optional description from request
	var req struct {
		Description string `json:"description"`
	}
	c.ShouldBindJSON(&req)

	backupID := uuid.New()
	timestamp := time.Now().UTC()

	// Create backup directory
	backupDir := fmt.Sprintf("/tmp/squad-aegis-backup-%s", backupID.String())
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		log.Error().Err(err).Msg("Failed to create backup directory")
		responses.InternalServerError(c, err, &gin.H{"error": err.Error()})
		return
	}
	defer os.RemoveAll(backupDir) // Clean up temp directory

	// Create metadata
	metadata := BackupMetadata{
		ID:          backupID,
		CreatedAt:   timestamp,
		Description: req.Description,
		Version:     "1.0", // You might want to get this from your app version
	}

	// Backup PostgreSQL data
	postgresFile := filepath.Join(backupDir, "postgres.sql")
	if err := s.backupPostgreSQL(c.Request.Context(), postgresFile); err != nil {
		log.Error().Err(err).Msg("Failed to backup PostgreSQL")
		responses.InternalServerError(c, err, &gin.H{"error": err.Error()})
		return
	}

	// Backup ClickHouse data
	clickhouseFile := filepath.Join(backupDir, "clickhouse.sql")
	if err := s.backupClickHouse(c.Request.Context(), clickhouseFile); err != nil {
		log.Error().Err(err).Msg("Failed to backup ClickHouse")
		responses.InternalServerError(c, err, &gin.H{"error": err.Error()})
		return
	}

	// Get file sizes
	if info, err := os.Stat(postgresFile); err == nil {
		metadata.PostgresSize = info.Size()
	}
	if info, err := os.Stat(clickhouseFile); err == nil {
		metadata.ClickhouseSize = info.Size()
	}

	// Save metadata
	metadataFile := filepath.Join(backupDir, "metadata.json")
	if err := s.saveMetadata(metadata, metadataFile); err != nil {
		log.Error().Err(err).Msg("Failed to save metadata")
		responses.InternalServerError(c, err, &gin.H{"error": err.Error()})
		return
	}

	// Create tar.gz archive
	archivePath := fmt.Sprintf("/tmp/squad-aegis-backup-%s.tar.gz", backupID.String())
	if err := s.createTarGz(backupDir, archivePath); err != nil {
		log.Error().Err(err).Msg("Failed to create archive")
		responses.InternalServerError(c, err, &gin.H{"error": err.Error()})
		return
	}
	defer os.Remove(archivePath) // Clean up after sending

	// Get final archive size
	if info, err := os.Stat(archivePath); err == nil {
		metadata.TotalSize = info.Size()
	}

	// Set headers for file download
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=squad-aegis-backup-%s.tar.gz", backupID.String()))
	c.Header("Content-Type", "application/gzip")

	// Send the file
	c.File(archivePath)

	log.Info().
		Str("backup_id", backupID.String()).
		Str("user_id", user.Id.String()).
		Int64("size", metadata.TotalSize).
		Msg("Backup created successfully")
}

// RestoreBackup restores data from a backup file
func (s *Server) RestoreBackup(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Authentication required", nil)
		return
	}

	// Get the uploaded file
	file, header, err := c.Request.FormFile("backup")
	if err != nil {
		responses.BadRequest(c, "No backup file provided", &gin.H{"error": err.Error()})
		return
	}
	defer file.Close()

	log.Info().
		Str("filename", header.Filename).
		Int64("size", header.Size).
		Str("user_id", user.Id.String()).
		Msg("Starting backup restore")

	// Create temporary directory for extraction
	restoreDir := fmt.Sprintf("/tmp/squad-aegis-restore-%s", uuid.New().String())
	if err := os.MkdirAll(restoreDir, 0755); err != nil {
		log.Error().Err(err).Msg("Failed to create restore directory")
		responses.InternalServerError(c, err, &gin.H{"error": err.Error()})
		return
	}
	defer os.RemoveAll(restoreDir) // Clean up temp directory

	// Save uploaded file temporarily
	tempFile := filepath.Join(restoreDir, "backup.tar.gz")
	if err := s.saveUploadedFile(file, tempFile); err != nil {
		log.Error().Err(err).Msg("Failed to save uploaded file")
		responses.InternalServerError(c, err, &gin.H{"error": err.Error()})
		return
	}

	// Extract tar.gz
	extractDir := filepath.Join(restoreDir, "extracted")
	if err := s.extractTarGz(tempFile, extractDir); err != nil {
		log.Error().Err(err).Msg("Failed to extract backup")
		responses.InternalServerError(c, err, &gin.H{"error": err.Error()})
		return
	}

	// Load and validate metadata
	metadataFile := filepath.Join(extractDir, "metadata.json")
	metadata, err := s.loadMetadata(metadataFile)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load backup metadata")
		responses.InternalServerError(c, err, &gin.H{"error": err.Error()})
		return
	}

	// Restore PostgreSQL data
	postgresFile := filepath.Join(extractDir, "postgres.sql")
	if err := s.restorePostgreSQL(c.Request.Context(), postgresFile); err != nil {
		log.Error().Err(err).Msg("Failed to restore PostgreSQL")
		responses.InternalServerError(c, err, &gin.H{"error": err.Error()})
		return
	}

	// Restore ClickHouse data
	clickhouseFile := filepath.Join(extractDir, "clickhouse.sql")
	if err := s.restoreClickHouse(c.Request.Context(), clickhouseFile); err != nil {
		log.Error().Err(err).Msg("Failed to restore ClickHouse")
		responses.InternalServerError(c, err, &gin.H{"error": err.Error()})
		return
	}

	log.Info().
		Str("backup_id", metadata.ID.String()).
		Str("user_id", user.Id.String()).
		Time("backup_created", metadata.CreatedAt).
		Msg("Backup restored successfully")

	responses.Success(c, "Backup restored successfully", &gin.H{
		"backup_id":         metadata.ID,
		"backup_created_at": metadata.CreatedAt,
		"description":       metadata.Description,
	})
}

// backupPostgreSQL creates a PostgreSQL dump
func (s *Server) backupPostgreSQL(ctx context.Context, filepath string) error {
	// Get all table names
	tables, err := s.getPostgreSQLTables(ctx)
	if err != nil {
		return fmt.Errorf("failed to get PostgreSQL tables: %w", err)
	}

	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create PostgreSQL backup file: %w", err)
	}
	defer file.Close()

	// Write backup header
	fmt.Fprintf(file, "-- Squad Aegis PostgreSQL Backup\n")
	fmt.Fprintf(file, "-- Created: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(file, "-- Tables: %v\n\n", tables)

	// Backup each table
	for _, table := range tables {
		if err := s.backupPostgreSQLTable(ctx, file, table); err != nil {
			return fmt.Errorf("failed to backup table %s: %w", table, err)
		}
	}

	return nil
}

// backupClickHouse creates a ClickHouse dump
func (s *Server) backupClickHouse(ctx context.Context, filepath string) error {
	clickhouseClient := s.Dependencies.PluginManager.GetClickHouseClient()

	// Get all table names
	tables, err := s.getClickHouseTables(ctx, clickhouseClient)
	if err != nil {
		return fmt.Errorf("failed to get ClickHouse tables: %w", err)
	}

	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create ClickHouse backup file: %w", err)
	}
	defer file.Close()

	// Write backup header
	fmt.Fprintf(file, "-- Squad Aegis ClickHouse Backup\n")
	fmt.Fprintf(file, "-- Created: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(file, "-- Tables: %v\n\n", tables)

	// Backup each table
	for _, table := range tables {
		if err := s.backupClickHouseTable(ctx, file, clickhouseClient, table); err != nil {
			log.Error().Err(err).Str("table", table).Msg("Failed to backup ClickHouse table, skipping")
			// Continue with other tables instead of failing completely
			fmt.Fprintf(file, "-- ERROR: Failed to backup table %s: %v\n\n", table, err)
		}
	}

	return nil
} // getPostgreSQLTables returns a list of all user tables
func (s *Server) getPostgreSQLTables(ctx context.Context) ([]string, error) {
	query := `
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = 'public' 
		AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`

	rows, err := s.Dependencies.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		tables = append(tables, tableName)
	}

	return tables, nil
}

// getClickHouseTables returns a list of all tables in the squad_aegis database
func (s *Server) getClickHouseTables(ctx context.Context, client *clickhouse.Client) ([]string, error) {
	query := `
		SELECT name 
		FROM system.tables 
		WHERE database = 'squad_aegis' 
		AND engine NOT LIKE '%View%'
		ORDER BY name
	`

	rows, err := client.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		tables = append(tables, tableName)
	}

	return tables, nil
}

// backupPostgreSQLTable backs up a single PostgreSQL table
func (s *Server) backupPostgreSQLTable(ctx context.Context, file *os.File, tableName string) error {
	// Get table structure
	fmt.Fprintf(file, "\n-- Table: %s\n", tableName)

	// Get CREATE TABLE statement
	createTableQuery := fmt.Sprintf(`
		SELECT 
			'CREATE TABLE ' || quote_ident(table_name) || ' (' ||
			string_agg(
				quote_ident(column_name) || ' ' || 
				CASE 
					WHEN data_type = 'character varying' THEN 'varchar' || COALESCE('(' || character_maximum_length || ')', '')
					WHEN data_type = 'timestamp without time zone' THEN 'timestamp'
					ELSE data_type 
				END ||
				CASE WHEN is_nullable = 'NO' THEN ' NOT NULL' ELSE '' END,
				', '
			) || ');'
		FROM information_schema.columns 
		WHERE table_name = '%s' AND table_schema = 'public'
		GROUP BY table_name
	`, tableName)

	var createStmt string
	err := s.Dependencies.DB.QueryRowContext(ctx, createTableQuery).Scan(&createStmt)
	if err != nil {
		return fmt.Errorf("failed to get CREATE TABLE statement: %w", err)
	}

	fmt.Fprintf(file, "DROP TABLE IF EXISTS %s CASCADE;\n", tableName)
	fmt.Fprintf(file, "%s\n\n", createStmt)

	// Get table data
	dataQuery := fmt.Sprintf("SELECT * FROM %s", tableName)
	rows, err := s.Dependencies.DB.QueryContext(ctx, dataQuery)
	if err != nil {
		return fmt.Errorf("failed to query table data: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}

	if len(columns) == 0 {
		return nil
	}

	// Create INSERT statements
	rowCount := 0
	for rows.Next() {
		if rowCount == 0 {
			fmt.Fprintf(file, "INSERT INTO %s (%s) VALUES\n", tableName,
				formatColumnNames(columns))
		}

		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		if rowCount > 0 {
			fmt.Fprint(file, ",\n")
		}

		fmt.Fprintf(file, "    (%s)", formatValues(values))
		rowCount++

		// Batch inserts every 1000 rows
		if rowCount%1000 == 0 {
			fmt.Fprintf(file, ";\n\nINSERT INTO %s (%s) VALUES\n", tableName,
				formatColumnNames(columns))
			rowCount = 0
		}
	}

	if rowCount > 0 {
		fmt.Fprint(file, ";\n")
	}

	return nil
}

// backupClickHouseTable backs up a single ClickHouse table
func (s *Server) backupClickHouseTable(ctx context.Context, file *os.File, client *clickhouse.Client, tableName string) error {
	fmt.Fprintf(file, "\n-- Table: %s\n", tableName)

	// Get CREATE TABLE statement
	createQuery := fmt.Sprintf("SHOW CREATE TABLE squad_aegis.%s", tableName)
	var createStmt string

	rows, err := client.Query(ctx, createQuery)
	if err != nil {
		return fmt.Errorf("failed to get CREATE TABLE statement: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&createStmt); err != nil {
			return fmt.Errorf("failed to scan CREATE TABLE statement: %w", err)
		}
	}

	fmt.Fprintf(file, "DROP TABLE IF EXISTS squad_aegis.%s;\n", tableName)
	fmt.Fprintf(file, "%s;\n\n", createStmt)

	// Get row count first to decide if we should backup data
	countQuery := fmt.Sprintf("SELECT count() FROM squad_aegis.%s", tableName)
	var rowCount uint64

	countRows, err := client.Query(ctx, countQuery)
	if err != nil {
		log.Error().Err(err).Str("table", tableName).Msg("Failed to get row count, skipping data backup")
		return nil
	}
	defer countRows.Close()

	if countRows.Next() {
		if err := countRows.Scan(&rowCount); err != nil {
			log.Error().Err(err).Str("table", tableName).Msg("Failed to scan row count, skipping data backup")
			return nil
		}
	}

	if rowCount == 0 {
		fmt.Fprintf(file, "-- Table %s is empty\n\n", tableName)
		return nil
	}

	fmt.Fprintf(file, "-- Table %s has %d rows\n", tableName, rowCount)

	// Get table data in smaller batches to avoid memory issues
	batchSize := 1000
	offset := 0

	for offset < int(rowCount) {
		dataQuery := fmt.Sprintf("SELECT * FROM squad_aegis.%s ORDER BY 1 LIMIT %d OFFSET %d", tableName, batchSize, offset)
		dataRows, err := client.Query(ctx, dataQuery)
		if err != nil {
			return fmt.Errorf("failed to query table data: %w", err)
		}

		// Get column names
		columns, err := dataRows.Columns()
		if err != nil {
			dataRows.Close()
			return fmt.Errorf("failed to get columns: %w", err)
		}

		if len(columns) == 0 {
			dataRows.Close()
			break
		}

		// Process rows in this batch
		batchRowCount := 0
		for dataRows.Next() {
			if batchRowCount == 0 {
				fmt.Fprintf(file, "INSERT INTO squad_aegis.%s (%s) VALUES\n", tableName,
					formatColumnNames(columns))
			}

			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := dataRows.Scan(valuePtrs...); err != nil {
				log.Error().Err(err).Str("table", tableName).Msg("Failed to scan row, skipping")
				continue
			}

			if batchRowCount > 0 {
				fmt.Fprint(file, ",\n")
			}

			fmt.Fprintf(file, "    (%s)", formatValuesForClickHouse(values))
			batchRowCount++
		}

		dataRows.Close()

		if batchRowCount > 0 {
			fmt.Fprintf(file, ";\n\n")
		}

		offset += batchSize
	}

	return nil
} // Helper functions

func formatColumnNames(columns []string) string {
	result := ""
	for i, col := range columns {
		if i > 0 {
			result += ", "
		}
		result += fmt.Sprintf(`"%s"`, col)
	}
	return result
}

func formatValues(values []interface{}) string {
	return formatValuesForDB(values, "postgres")
}

func formatValuesForClickHouse(values []interface{}) string {
	return formatValuesForDB(values, "clickhouse")
}

func formatValuesForDB(values []interface{}, dbType string) string {
	result := ""
	for i, val := range values {
		if i > 0 {
			result += ", "
		}
		if val == nil {
			result += "NULL"
		} else {
			switch v := val.(type) {
			case string:
				result += fmt.Sprintf("'%s'", escapeSQLString(v))
			case []byte:
				result += fmt.Sprintf("'%s'", escapeSQLString(string(v)))
			case time.Time:
				if dbType == "clickhouse" {
					// ClickHouse DateTime format (without milliseconds)
					result += fmt.Sprintf("'%s'", v.UTC().Format("2006-01-02 15:04:05"))
				} else {
					// PostgreSQL timestamp format (with milliseconds)
					result += fmt.Sprintf("'%s'", v.UTC().Format("2006-01-02 15:04:05.000"))
				}
			case bool:
				if v {
					result += "true"
				} else {
					result += "false"
				}
			case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
				result += fmt.Sprintf("%v", v)
			case float32, float64:
				result += fmt.Sprintf("%v", v)
			default:
				// For any other types, convert to string and escape
				str := fmt.Sprintf("%v", v)
				result += fmt.Sprintf("'%s'", escapeSQLString(str))
			}
		}
	}
	return result
}

func escapeSQLString(s string) string {
	// Comprehensive SQL string escaping for both PostgreSQL and ClickHouse
	result := ""
	for _, char := range s {
		switch char {
		case '\'':
			result += "''" // Escape single quotes
		case '\\':
			result += "\\\\" // Escape backslashes
		case '\n':
			result += "\\n" // Escape newlines
		case '\r':
			result += "\\r" // Escape carriage returns
		case '\t':
			result += "\\t" // Escape tabs
		case '\000':
			result += "\\0" // Escape null bytes
		default:
			result += string(char)
		}
	}
	return result
}

func (s *Server) saveMetadata(metadata BackupMetadata, filepath string) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath, data, 0644)
}

func (s *Server) loadMetadata(filepath string) (*BackupMetadata, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	var metadata BackupMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

func (s *Server) createTarGz(sourceDir, targetPath string) error {
	tarFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	gzWriter := gzip.NewWriter(tarFile)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the source directory itself
		if path == sourceDir {
			return nil
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		// Update the name to be relative to source directory
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// If it's a file, write the content
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(tarWriter, file)
			return err
		}

		return nil
	})
}

func (s *Server) extractTarGz(archivePath, targetDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		path := filepath.Join(targetDir, header.Name)

		// Ensure the directory exists
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		if header.Typeflag == tar.TypeReg {
			outFile, err := os.Create(path)
			if err != nil {
				return err
			}
			defer outFile.Close()

			if _, err := io.Copy(outFile, tarReader); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Server) saveUploadedFile(src multipart.File, dst string) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

// Restore functions

func (s *Server) restorePostgreSQL(ctx context.Context, filepath string) error {
	// Read the SQL file
	data, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read PostgreSQL backup file: %w", err)
	}

	// Split the SQL content into individual statements
	sqlContent := string(data)
	statements := s.splitSQLStatements(sqlContent)

	// Execute each statement separately
	for i, statement := range statements {
		statement = strings.TrimSpace(statement)
		if statement == "" || strings.HasPrefix(statement, "--") {
			continue // Skip empty lines and comments
		}

		log.Debug().Int("statement_num", i+1).Str("statement_preview", statement[:min(100, len(statement))]).Msg("Executing PostgreSQL statement")

		if _, err := s.Dependencies.DB.ExecContext(ctx, statement); err != nil {
			log.Error().Err(err).Int("statement_num", i+1).Str("statement", statement[:min(500, len(statement))]).Msg("Failed to execute PostgreSQL statement")
			return fmt.Errorf("failed to execute PostgreSQL statement %d: %w", i+1, err)
		}
	}

	return nil
}

func (s *Server) restoreClickHouse(ctx context.Context, filepath string) error {
	clickhouseClient := s.Dependencies.PluginManager.GetClickHouseClient()

	// Read the SQL file
	data, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read ClickHouse backup file: %w", err)
	}

	// Split the SQL content into individual statements
	sqlContent := string(data)
	statements := s.splitSQLStatements(sqlContent)

	// Execute each statement separately
	for i, statement := range statements {
		statement = strings.TrimSpace(statement)
		if statement == "" || strings.HasPrefix(statement, "--") {
			continue // Skip empty lines and comments
		}

		log.Debug().Int("statement_num", i+1).Str("statement_preview", statement[:min(100, len(statement))]).Msg("Executing ClickHouse statement")

		if err := clickhouseClient.Exec(ctx, statement); err != nil {
			log.Error().Err(err).Int("statement_num", i+1).Str("statement", statement[:min(500, len(statement))]).Msg("Failed to execute ClickHouse statement")
			return fmt.Errorf("failed to execute ClickHouse statement %d: %w", i+1, err)
		}
	}

	return nil
}

// splitSQLStatements splits SQL content into individual statements
func (s *Server) splitSQLStatements(sqlContent string) []string {
	var statements []string
	var currentStatement strings.Builder
	var inQuotes bool
	var quoteChar rune

	lines := strings.Split(sqlContent, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}

		// Track if we're inside quotes
		for _, char := range line {
			if !inQuotes && (char == '\'' || char == '"') {
				inQuotes = true
				quoteChar = char
			} else if inQuotes && char == quoteChar {
				inQuotes = false
			}
		}

		currentStatement.WriteString(line)
		currentStatement.WriteString(" ")

		// If line ends with semicolon and we're not in quotes, it's the end of a statement
		if !inQuotes && strings.HasSuffix(line, ";") {
			stmt := strings.TrimSpace(currentStatement.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			currentStatement.Reset()
		}
	}

	// Add any remaining statement
	if currentStatement.Len() > 0 {
		stmt := strings.TrimSpace(currentStatement.String())
		if stmt != "" {
			statements = append(statements, stmt)
		}
	}

	return statements
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
