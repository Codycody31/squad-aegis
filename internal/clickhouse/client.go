package clickhouse

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/models"
)

// Client represents a ClickHouse client
type Client struct {
	conn *sql.DB
	cfg  Config
}

// Config holds ClickHouse connection configuration
type Config struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
	Debug    bool
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

	// Open connection
	conn := clickhouse.OpenDB(options)
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	log.Info().
		Str("host", cfg.Host).
		Int("port", cfg.Port).
		Str("database", cfg.Database).
		Msg("Connected to ClickHouse")

	return &Client{
		conn: conn,
		cfg:  cfg,
	}, nil
}

// Close closes the ClickHouse connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Ping tests the connection
func (c *Client) Ping(ctx context.Context) error {
	return c.conn.PingContext(ctx)
}

// Exec executes a query without returning results
func (c *Client) Exec(ctx context.Context, query string, args ...interface{}) error {
	_, err := c.conn.ExecContext(ctx, query, args...)
	return err
}

// Query executes a query and returns rows
func (c *Client) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return c.conn.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns a single row
func (c *Client) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return c.conn.QueryRowContext(ctx, query, args...)
}

// Begin starts a transaction
func (c *Client) Begin(ctx context.Context) (*sql.Tx, error) {
	return c.conn.BeginTx(ctx, nil)
}

// GetConnection returns the underlying SQL connection for advanced usage
func (c *Client) GetConnection() *sql.DB {
	return c.conn
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

	query := `
		INSERT INTO squad_aegis.workflow_execution_logs (
			execution_id, workflow_id, server_id, event_time, trigger_event_type,
			trigger_event_data, status, step_name, step_type, step_order,
			step_status, step_input, step_output, step_error, step_duration_ms,
			variables, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	return c.Exec(ctx, query,
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

// UpdateWorkflowExecutionSummary upserts a workflow execution summary in ClickHouse
func (c *Client) UpdateWorkflowExecutionSummary(ctx context.Context, summary *models.WorkflowExecutionSummary) error {
	query := `
		INSERT INTO squad_aegis.workflow_execution_summary (
			execution_id, workflow_id, server_id, workflow_name, started_at,
			completed_at, status, trigger_event_type, total_steps, completed_steps,
			failed_steps, skipped_steps, total_duration_ms, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	return c.Exec(ctx, query,
		summary.ExecutionID,
		summary.WorkflowID,
		summary.ServerID,
		summary.WorkflowName,
		summary.StartedAt,
		summary.CompletedAt,
		summary.Status,
		summary.TriggerEventType,
		summary.TotalSteps,
		summary.CompletedSteps,
		summary.FailedSteps,
		summary.SkippedSteps,
		summary.TotalDurationMs,
		summary.ErrorMessage,
	)
}

// GetWorkflowExecutionLogs retrieves workflow execution logs from ClickHouse
func (c *Client) GetWorkflowExecutionLogs(ctx context.Context, executionID uuid.UUID, limit, offset int) ([]models.WorkflowExecutionLog, error) {
	query := `
		SELECT execution_id, workflow_id, server_id, event_time, trigger_event_type,
			   trigger_event_data, status, step_name, step_type, step_order,
			   step_status, step_input, step_output, step_error, step_duration_ms,
			   variables, metadata
		FROM squad_aegis.workflow_execution_logs
		WHERE execution_id = ?
		ORDER BY step_order ASC, event_time ASC
		LIMIT ? OFFSET ?
	`

	rows, err := c.Query(ctx, query, executionID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.WorkflowExecutionLog
	for rows.Next() {
		var log models.WorkflowExecutionLog
		var triggerEventDataStr, stepInputStr, stepOutputStr, variablesStr, metadataStr string

		err := rows.Scan(
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

	return logs, nil
}

// GetWorkflowExecutionSummary retrieves a workflow execution summary from ClickHouse
func (c *Client) GetWorkflowExecutionSummary(ctx context.Context, executionID uuid.UUID) (*models.WorkflowExecutionSummary, error) {
	query := `
		SELECT execution_id, workflow_id, server_id, workflow_name, started_at,
			   completed_at, status, trigger_event_type, total_steps, completed_steps,
			   failed_steps, skipped_steps, total_duration_ms, error_message
		FROM squad_aegis.workflow_execution_summary
		WHERE execution_id = ?
		ORDER BY ingested_at DESC
		LIMIT 1
	`

	var summary models.WorkflowExecutionSummary
	err := c.QueryRow(ctx, query, executionID).Scan(
		&summary.ExecutionID,
		&summary.WorkflowID,
		&summary.ServerID,
		&summary.WorkflowName,
		&summary.StartedAt,
		&summary.CompletedAt,
		&summary.Status,
		&summary.TriggerEventType,
		&summary.TotalSteps,
		&summary.CompletedSteps,
		&summary.FailedSteps,
		&summary.SkippedSteps,
		&summary.TotalDurationMs,
		&summary.ErrorMessage,
	)

	if err != nil {
		return nil, err
	}

	return &summary, nil
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

	query := `
		INSERT INTO squad_aegis.workflow_log_messages (
			execution_id, workflow_id, server_id, step_id, step_name,
			log_time, log_level, message, variables, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	err = c.Exec(ctx, query,
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

// GetWorkflowLogMessages retrieves workflow log messages from ClickHouse
func (c *Client) GetWorkflowLogMessages(ctx context.Context, executionID uuid.UUID, limit, offset int) ([]models.WorkflowLogMessage, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT execution_id, workflow_id, server_id, step_id, step_name,
			   log_time, log_level, message, variables, metadata
		FROM squad_aegis.workflow_log_messages
		WHERE execution_id = ?
		ORDER BY log_time ASC
		LIMIT ? OFFSET ?
	`

	rows, err := c.Query(ctx, query, executionID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.WorkflowLogMessage
	for rows.Next() {
		var msg models.WorkflowLogMessage
		var variablesJSON, metadataJSON string

		err := rows.Scan(
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
			log.Error().Err(err).Msg("Failed to scan workflow log message row")
			continue
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

	return messages, nil
}
