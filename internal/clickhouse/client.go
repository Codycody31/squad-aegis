package clickhouse

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/models"
)

// Client represents a ClickHouse client
type Client struct {
	sql    *sql.DB
	native chdriver.Conn
	cfg    Config
}

// Config holds ClickHouse connection configuration
type Config struct {
	Host            string
	Port            int
	Database        string
	Username        string
	Password        string
	Debug           bool
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// PoolStats contains both database/sql and native driver pool statistics.
type PoolStats struct {
	SQL    sql.DBStats
	Native chdriver.Stats
}

// Rows records query latency, failures, and the number of rows exposed to the
// caller. SQL text and argument values are deliberately excluded from logs.
type Rows struct {
	rows    *sql.Rows
	label   string
	started time.Time
	count   atomic.Int64
	once    sync.Once
}

func (r *Rows) Next() bool {
	next := r.rows.Next()
	if next {
		r.count.Add(1)
		return true
	}
	r.finish(r.rows.Err())
	return false
}

func (r *Rows) Scan(dest ...any) error {
	err := r.rows.Scan(dest...)
	if err != nil {
		r.finish(err)
	}
	return err
}

func (r *Rows) Close() error {
	err := r.rows.Close()
	r.finish(err)
	return err
}

func (r *Rows) Err() error {
	err := r.rows.Err()
	if err != nil {
		r.finish(err)
	}
	return err
}

func (r *Rows) finish(err error) {
	r.once.Do(func() {
		logDatabaseQuery(r.label, time.Since(r.started), err, r.count.Load())
	})
}

type Row struct {
	row     *sql.Row
	label   string
	started time.Time
	once    sync.Once
}

func (r *Row) Scan(dest ...any) error {
	err := r.row.Scan(dest...)
	r.once.Do(func() {
		rows := int64(1)
		if err != nil {
			rows = 0
		}
		logErr := err
		if errors.Is(err, sql.ErrNoRows) {
			logErr = nil
		}
		logDatabaseQuery(r.label, time.Since(r.started), logErr, rows)
	})
	return err
}

func queryLabel(query string) string {
	trimmed := strings.TrimSpace(query)
	start := strings.Index(trimmed, "/*")
	if start < 0 {
		return "unlabeled"
	}
	comment := trimmed[start+2:]
	end := strings.Index(comment, "*/")
	if end < 0 {
		return "unlabeled"
	}
	label := strings.TrimSpace(comment[:end])
	if !strings.HasPrefix(label, "sa.") {
		return "unlabeled"
	}
	return label
}

func logDatabaseQuery(label string, duration time.Duration, err error, rows int64) {
	event := log.Debug()
	if err != nil {
		event = log.Error().Err(err)
	} else if duration >= 500*time.Millisecond {
		event = log.Warn()
	}
	event.
		Str("query", label).
		Dur("duration", duration).
		Bool("failure", err != nil).
		Int64("rows", rows).
		Msg("database query")
}

// NewClient creates a new ClickHouse client
func NewClient(cfg Config) (*Client, error) {
	// Set defaults
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}
	if cfg.Port == 0 {
		cfg.Port = 9000
	}
	if cfg.Database == "" {
		cfg.Database = "squad_aegis"
	}
	if cfg.Username == "" {
		cfg.Username = "default"
	}
	if cfg.MaxOpenConns <= 0 {
		cfg.MaxOpenConns = 10
	}
	if cfg.MaxIdleConns <= 0 {
		cfg.MaxIdleConns = 5
	}
	if cfg.ConnMaxLifetime <= 0 {
		cfg.ConnMaxLifetime = time.Hour
	}

	// Build connection options
	options := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout: 30 * time.Second,
		Debug:       cfg.Debug,
	}

	// database/sql is retained for migrations and repository reads. The native
	// connection is used for typed batches and acknowledged asynchronous writes.
	sqlConn := clickhouse.OpenDB(options)
	sqlConn.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlConn.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlConn.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	nativeOptions := *options
	nativeOptions.MaxOpenConns = cfg.MaxOpenConns
	nativeOptions.MaxIdleConns = cfg.MaxIdleConns
	nativeOptions.ConnMaxLifetime = cfg.ConnMaxLifetime
	nativeConn, err := clickhouse.Open(&nativeOptions)
	if err != nil {
		_ = sqlConn.Close()
		return nil, fmt.Errorf("failed to open native ClickHouse connection: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := sqlConn.PingContext(pingCtx); err != nil {
		_ = nativeConn.Close()
		_ = sqlConn.Close()
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}
	if err := nativeConn.Ping(pingCtx); err != nil {
		_ = nativeConn.Close()
		_ = sqlConn.Close()
		return nil, fmt.Errorf("failed to connect native ClickHouse client: %w", err)
	}

	log.Info().
		Str("host", cfg.Host).
		Int("port", cfg.Port).
		Str("database", cfg.Database).
		Msg("Connected to ClickHouse")

	return &Client{
		sql:    sqlConn,
		native: nativeConn,
		cfg:    cfg,
	}, nil
}

// Close closes the ClickHouse connection
func (c *Client) Close() error {
	var errs []error
	if c.native != nil {
		errs = append(errs, c.native.Close())
	}
	if c.sql != nil {
		errs = append(errs, c.sql.Close())
	}
	return errors.Join(errs...)
}

// Ping tests the connection
func (c *Client) Ping(ctx context.Context) error {
	if err := c.sql.PingContext(ctx); err != nil {
		return err
	}
	return c.native.Ping(ctx)
}

// Exec executes a query without returning results
func (c *Client) Exec(ctx context.Context, query string, args ...interface{}) error {
	started := time.Now()
	result, err := c.sql.ExecContext(ctx, query, args...)
	rows := int64(0)
	if err == nil {
		if affected, affectedErr := result.RowsAffected(); affectedErr == nil {
			rows = affected
		}
	}
	logDatabaseQuery(queryLabel(query), time.Since(started), err, rows)
	return err
}

// Query executes a query and returns rows
func (c *Client) Query(ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	started := time.Now()
	rows, err := c.sql.QueryContext(ctx, query, args...)
	if err != nil {
		logDatabaseQuery(queryLabel(query), time.Since(started), err, 0)
		return nil, err
	}
	return &Rows{rows: rows, label: queryLabel(query), started: started}, nil
}

// QueryRow executes a query that returns a single row
func (c *Client) QueryRow(ctx context.Context, query string, args ...interface{}) *Row {
	started := time.Now()
	return &Row{
		row:     c.sql.QueryRowContext(ctx, query, args...),
		label:   queryLabel(query),
		started: started,
	}
}

// Begin starts a transaction
func (c *Client) Begin(ctx context.Context) (*sql.Tx, error) {
	return c.sql.BeginTx(ctx, nil)
}

// GetConnection returns the underlying SQL connection for advanced usage
func (c *Client) GetConnection() *sql.DB {
	return c.sql
}

// PrepareBatch starts a native, typed ClickHouse insert batch.
func (c *Client) PrepareBatch(ctx context.Context, query string) (chdriver.Batch, error) {
	return c.native.PrepareBatch(ctx, query)
}

// AsyncInsert executes an acknowledged asynchronous insert.
func (c *Client) AsyncInsert(ctx context.Context, query string, args ...any) error {
	started := time.Now()
	err := c.native.AsyncInsert(ctx, query, true, args...)
	rows := int64(1)
	if err != nil {
		rows = 0
	}
	logDatabaseQuery(queryLabel(query), time.Since(started), err, rows)
	return err
}

// Stats returns pool state for health and sudo observability.
func (c *Client) Stats() PoolStats {
	return PoolStats{
		SQL:    c.sql.Stats(),
		Native: c.native.Stats(),
	}
}

// LogWorkflowExecution logs a workflow execution step to ClickHouse
func (c *Client) LogWorkflowExecution(ctx context.Context, log *models.WorkflowExecutionLog) error {
	triggerEventDataJSON, err := json.Marshal(log.TriggerEventData)
	if err != nil {
		return fmt.Errorf("failed to marshal trigger event data: %w", err)
	}

	stepInputJSON, err := json.Marshal(log.StepInput)
	if err != nil {
		return fmt.Errorf("failed to marshal step input: %w", err)
	}

	stepOutputJSON, err := json.Marshal(log.StepOutput)
	if err != nil {
		return fmt.Errorf("failed to marshal step output: %w", err)
	}

	variablesJSON, err := json.Marshal(log.Variables)
	if err != nil {
		return fmt.Errorf("failed to marshal variables: %w", err)
	}

	metadataJSON, err := json.Marshal(log.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if log.EntryID == uuid.Nil {
		log.EntryID = uuid.New()
	}

	query := `/* sa.workflow.logs.write */
		INSERT INTO squad_aegis.workflow_execution_logs (
			entry_id, execution_id, workflow_id, server_id, event_time, trigger_event_type,
			trigger_event_data, status, step_name, step_type, step_order,
			step_status, step_input, step_output, step_error, step_duration_ms,
			variables, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	return c.AsyncInsert(ctx, query,
		log.EntryID,
		log.ExecutionID,
		log.WorkflowID,
		log.ServerID,
		log.EventTime,
		log.TriggerEventType,
		string(triggerEventDataJSON),
		log.Status,
		log.StepName,
		log.StepType,
		log.StepOrder,
		log.StepStatus,
		string(stepInputJSON),
		string(stepOutputJSON),
		log.StepError,
		log.StepDurationMs,
		string(variablesJSON),
		string(metadataJSON),
	)
}

// UpdateWorkflowExecutionSummary is retained for one rollback-compatible
// release. PostgreSQL is the execution-summary source, so no ClickHouse row is
// written.
func (c *Client) UpdateWorkflowExecutionSummary(_ context.Context, _ *models.WorkflowExecutionSummary) error {
	return nil
}

// WorkflowExecutionLogCursor is the decoded keyset position for workflow step
// logs. Opaque cursor validation is performed by the HTTP layer.
type WorkflowExecutionLogCursor struct {
	StepOrder uint32
	EventTime time.Time
	EntryID   uuid.UUID
}

// GetWorkflowExecutionLogs retrieves workflow execution logs from ClickHouse
func (c *Client) GetWorkflowExecutionLogs(
	ctx context.Context,
	serverID uuid.UUID,
	workflowID uuid.UUID,
	executionID uuid.UUID,
	limit int,
	after *WorkflowExecutionLogCursor,
) ([]models.WorkflowExecutionLog, error) {
	query := `/* sa.workflow.logs.list */
		SELECT entry_id, execution_id, workflow_id, server_id, event_time, trigger_event_type,
			   trigger_event_data, status, step_name, step_type, step_order,
			   step_status, step_input, step_output, step_error, step_duration_ms,
			   variables, metadata
		FROM squad_aegis.workflow_execution_logs
		WHERE server_id = ?
		  AND workflow_id = ?
		  AND execution_id = ?
	`

	args := []any{serverID, workflowID, executionID}
	if after != nil {
		query += `
		  AND (step_order, event_time, entry_id) > (?, ?, ?)
		`
		args = append(args, after.StepOrder, after.EventTime, after.EntryID)
	}
	query += `
		ORDER BY step_order ASC, event_time ASC, entry_id ASC
		LIMIT ?
	`
	args = append(args, limit)

	rows, err := c.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.WorkflowExecutionLog
	for rows.Next() {
		var log models.WorkflowExecutionLog
		var triggerEventDataStr, stepInputStr, stepOutputStr, variablesStr, metadataStr string

		err := rows.Scan(
			&log.EntryID,
			&log.ExecutionID,
			&log.WorkflowID,
			&log.ServerID,
			&log.EventTime,
			&log.TriggerEventType,
			&triggerEventDataStr,
			&log.Status,
			&log.StepName,
			&log.StepType,
			&log.StepOrder,
			&log.StepStatus,
			&stepInputStr,
			&stepOutputStr,
			&log.StepError,
			&log.StepDurationMs,
			&variablesStr,
			&metadataStr,
		)
		if err != nil {
			return nil, err
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal([]byte(triggerEventDataStr), &log.TriggerEventData); err != nil {
			log.TriggerEventData = make(map[string]interface{})
		}
		if err := json.Unmarshal([]byte(stepInputStr), &log.StepInput); err != nil {
			log.StepInput = make(map[string]interface{})
		}
		if err := json.Unmarshal([]byte(stepOutputStr), &log.StepOutput); err != nil {
			log.StepOutput = make(map[string]interface{})
		}
		if err := json.Unmarshal([]byte(variablesStr), &log.Variables); err != nil {
			log.Variables = make(map[string]interface{})
		}
		if err := json.Unmarshal([]byte(metadataStr), &log.Metadata); err != nil {
			log.Metadata = make(map[string]interface{})
		}

		logs = append(logs, log)
	}

	return logs, rows.Err()
}

// LogWorkflowMessage logs a workflow message to ClickHouse
func (c *Client) LogWorkflowMessage(ctx context.Context, logMsg *models.WorkflowLogMessage) error {
	// Convert variables and metadata to JSON
	variablesJSON, err := json.Marshal(logMsg.Variables)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal workflow log message variables")
		variablesJSON = []byte("{}")
	}

	metadataJSON, err := json.Marshal(logMsg.Metadata)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal workflow log message metadata")
		metadataJSON = []byte("{}")
	}

	if logMsg.EntryID == uuid.Nil {
		logMsg.EntryID = uuid.New()
	}

	query := `/* sa.workflow.messages.write */
		INSERT INTO squad_aegis.workflow_log_messages (
			entry_id, execution_id, workflow_id, server_id, step_id, step_name,
			log_time, log_level, message, variables, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	err = c.AsyncInsert(ctx, query,
		logMsg.EntryID,
		logMsg.ExecutionID,
		logMsg.WorkflowID,
		logMsg.ServerID,
		logMsg.StepID,
		logMsg.StepName,
		logMsg.LogTime,
		logMsg.LogLevel,
		logMsg.Message,
		string(variablesJSON),
		string(metadataJSON),
	)

	return err
}

// WorkflowLogMessageCursor is the decoded keyset position for workflow
// messages. Opaque cursor validation is performed by the HTTP layer.
type WorkflowLogMessageCursor struct {
	LogTime time.Time
	EntryID uuid.UUID
}

// GetWorkflowLogMessages retrieves workflow log messages from ClickHouse
func (c *Client) GetWorkflowLogMessages(
	ctx context.Context,
	serverID uuid.UUID,
	workflowID uuid.UUID,
	executionID uuid.UUID,
	limit int,
	after *WorkflowLogMessageCursor,
) ([]models.WorkflowLogMessage, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `/* sa.workflow.messages.list */
		SELECT entry_id, execution_id, workflow_id, server_id, step_id, step_name,
			   log_time, log_level, message, variables, metadata
		FROM squad_aegis.workflow_log_messages
		WHERE server_id = ?
		  AND workflow_id = ?
		  AND execution_id = ?
	`

	args := []any{serverID, workflowID, executionID}
	if after != nil {
		query += `
		  AND (log_time, entry_id) > (?, ?)
		`
		args = append(args, after.LogTime, after.EntryID)
	}
	query += `
		ORDER BY log_time ASC, entry_id ASC
		LIMIT ?
	`
	args = append(args, limit)

	rows, err := c.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.WorkflowLogMessage
	for rows.Next() {
		var msg models.WorkflowLogMessage
		var variablesJSON, metadataJSON string

		err := rows.Scan(
			&msg.EntryID,
			&msg.ExecutionID,
			&msg.WorkflowID,
			&msg.ServerID,
			&msg.StepID,
			&msg.StepName,
			&msg.LogTime,
			&msg.LogLevel,
			&msg.Message,
			&variablesJSON,
			&metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		// Parse JSON fields
		if err := json.Unmarshal([]byte(variablesJSON), &msg.Variables); err != nil {
			log.Error().Err(err).Msg("Failed to unmarshal workflow log message variables")
			msg.Variables = make(map[string]interface{})
		}

		if err := json.Unmarshal([]byte(metadataJSON), &msg.Metadata); err != nil {
			log.Error().Err(err).Msg("Failed to unmarshal workflow log message metadata")
			msg.Metadata = make(map[string]interface{})
		}

		messages = append(messages, msg)
	}

	return messages, rows.Err()
}
