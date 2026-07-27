package workflow_manager

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.codycody31.dev/squad-aegis/internal/db"
	"go.codycody31.dev/squad-aegis/internal/models"
)

// WorkflowDatabase handles database operations for workflows
type WorkflowDatabase struct {
	db db.Executor
}

// NewWorkflowDatabase creates a new workflow database handler
func NewWorkflowDatabase(database db.Executor) *WorkflowDatabase {
	return &WorkflowDatabase{db: database}
}

// CreateWorkflow creates a new workflow in the database
func (wd *WorkflowDatabase) CreateWorkflow(ctx context.Context, workflow *models.ServerWorkflow) error {
	definitionJSON, err := json.Marshal(workflow.Definition)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow definition: %w", err)
	}

	query := `
		INSERT INTO server_workflows (id, server_id, name, description, enabled, definition, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err = wd.db.ExecContext(ctx, query,
		workflow.ID,
		workflow.ServerID,
		workflow.Name,
		workflow.Description,
		workflow.Enabled,
		definitionJSON,
		workflow.CreatedBy,
		workflow.CreatedAt,
		workflow.UpdatedAt,
	)

	return err
}

// GetWorkflow retrieves a workflow by ID
func (wd *WorkflowDatabase) GetWorkflow(ctx context.Context, workflowID uuid.UUID) (*models.ServerWorkflow, error) {
	query := `
		/* sa.workflow.get */
		SELECT id, server_id, name, description, enabled, definition, created_by, created_at, updated_at
		FROM server_workflows
		WHERE id = $1
	`

	var workflow models.ServerWorkflow
	var definitionJSON []byte
	var description sql.NullString

	err := wd.db.QueryRowContext(ctx, query, workflowID).Scan(
		&workflow.ID,
		&workflow.ServerID,
		&workflow.Name,
		&description,
		&workflow.Enabled,
		&definitionJSON,
		&workflow.CreatedBy,
		&workflow.CreatedAt,
		&workflow.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if description.Valid {
		workflow.Description = &description.String
	}

	if err := json.Unmarshal(definitionJSON, &workflow.Definition); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow definition: %w", err)
	}

	workflows := []models.ServerWorkflow{workflow}
	if err := wd.loadWorkflowVariables(ctx, workflows); err != nil {
		return nil, fmt.Errorf("failed to load workflow variables: %w", err)
	}

	return &workflows[0], nil
}

// GetWorkflowsByServerID retrieves all workflows for a server
func (wd *WorkflowDatabase) GetWorkflowsByServerID(ctx context.Context, serverID uuid.UUID) ([]models.ServerWorkflow, error) {
	query := `
		/* sa.workflow.list */
		SELECT id, server_id, name, description, enabled, definition, created_by, created_at, updated_at
		FROM server_workflows
		WHERE server_id = $1
		ORDER BY created_at DESC, id DESC
	`

	rows, err := wd.db.QueryContext(ctx, query, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workflows []models.ServerWorkflow

	for rows.Next() {
		var workflow models.ServerWorkflow
		var definitionJSON []byte
		var description sql.NullString

		err := rows.Scan(
			&workflow.ID,
			&workflow.ServerID,
			&workflow.Name,
			&description,
			&workflow.Enabled,
			&definitionJSON,
			&workflow.CreatedBy,
			&workflow.CreatedAt,
			&workflow.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		if description.Valid {
			workflow.Description = &description.String
		}

		if err := json.Unmarshal(definitionJSON, &workflow.Definition); err != nil {
			return nil, fmt.Errorf("failed to unmarshal workflow definition for workflow %s: %w", workflow.ID, err)
		}

		workflows = append(workflows, workflow)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := wd.loadWorkflowVariables(ctx, workflows); err != nil {
		return nil, fmt.Errorf("failed to load workflow variables: %w", err)
	}

	return workflows, nil
}

// GetEnabledWorkflows retrieves every enabled workflow in two queries:
// one for workflows and one batched query for their variables.
func (wd *WorkflowDatabase) GetEnabledWorkflows(ctx context.Context) ([]models.ServerWorkflow, error) {
	const query = `
		/* sa.workflow.enabled */
		SELECT id, server_id, name, description, enabled, definition, created_by, created_at, updated_at
		FROM server_workflows
		WHERE enabled
		ORDER BY server_id, created_at DESC, id DESC
	`

	rows, err := wd.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workflows []models.ServerWorkflow
	for rows.Next() {
		var workflow models.ServerWorkflow
		var definitionJSON []byte
		var description sql.NullString
		if err := rows.Scan(
			&workflow.ID,
			&workflow.ServerID,
			&workflow.Name,
			&description,
			&workflow.Enabled,
			&definitionJSON,
			&workflow.CreatedBy,
			&workflow.CreatedAt,
			&workflow.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if description.Valid {
			workflow.Description = &description.String
		}
		if err := json.Unmarshal(definitionJSON, &workflow.Definition); err != nil {
			return nil, fmt.Errorf("failed to unmarshal workflow definition for workflow %s: %w", workflow.ID, err)
		}
		workflows = append(workflows, workflow)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := wd.loadWorkflowVariables(ctx, workflows); err != nil {
		return nil, fmt.Errorf("failed to load workflow variables: %w", err)
	}

	return workflows, nil
}

// UpdateWorkflow updates a workflow in the database
func (wd *WorkflowDatabase) UpdateWorkflow(ctx context.Context, workflow *models.ServerWorkflow) error {
	definitionJSON, err := json.Marshal(workflow.Definition)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow definition: %w", err)
	}

	query := `
		UPDATE server_workflows
		SET name = $1, description = $2, enabled = $3, definition = $4, updated_at = $5
		WHERE id = $6
	`

	_, err = wd.db.ExecContext(ctx, query,
		workflow.Name,
		workflow.Description,
		workflow.Enabled,
		definitionJSON,
		workflow.UpdatedAt,
		workflow.ID,
	)

	return err
}

// DeleteWorkflow deletes a workflow from the database
func (wd *WorkflowDatabase) DeleteWorkflow(ctx context.Context, workflowID uuid.UUID) error {
	// This will cascade delete variables and executions
	query := `DELETE FROM server_workflows WHERE id = $1`
	_, err := wd.db.ExecContext(ctx, query, workflowID)
	return err
}

// CreateWorkflowVariable creates a new workflow variable
func (wd *WorkflowDatabase) CreateWorkflowVariable(ctx context.Context, variable *models.ServerWorkflowVariable) error {
	valueJSON, err := json.Marshal(variable.Value)
	if err != nil {
		return fmt.Errorf("failed to marshal variable value: %w", err)
	}

	query := `
		INSERT INTO server_workflow_variables (id, workflow_id, name, value, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = wd.db.ExecContext(ctx, query,
		variable.ID,
		variable.WorkflowID,
		variable.Name,
		valueJSON,
		variable.Description,
		variable.CreatedAt,
		variable.UpdatedAt,
	)

	return err
}

// GetWorkflowVariable retrieves a specific workflow variable
func (wd *WorkflowDatabase) GetWorkflowVariable(ctx context.Context, workflowID uuid.UUID, name string) (*models.ServerWorkflowVariable, error) {
	query := `
		SELECT id, workflow_id, name, value, description, created_at, updated_at
		FROM server_workflow_variables
		WHERE workflow_id = $1 AND name = $2
	`

	var variable models.ServerWorkflowVariable
	var valueJSON []byte
	var description sql.NullString

	err := wd.db.QueryRowContext(ctx, query, workflowID, name).Scan(
		&variable.ID,
		&variable.WorkflowID,
		&variable.Name,
		&valueJSON,
		&description,
		&variable.CreatedAt,
		&variable.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if description.Valid {
		variable.Description = &description.String
	}

	if err := json.Unmarshal(valueJSON, &variable.Value); err != nil {
		return nil, fmt.Errorf("failed to unmarshal variable value: %w", err)
	}

	return &variable, nil
}

// UpdateWorkflowVariable updates a workflow variable
func (wd *WorkflowDatabase) UpdateWorkflowVariable(ctx context.Context, variable *models.ServerWorkflowVariable) error {
	valueJSON, err := json.Marshal(variable.Value)
	if err != nil {
		return fmt.Errorf("failed to marshal variable value: %w", err)
	}

	query := `
		UPDATE server_workflow_variables
		SET value = $1, description = $2, updated_at = $3
		WHERE id = $4
	`

	_, err = wd.db.ExecContext(ctx, query, valueJSON, variable.Description, variable.UpdatedAt, variable.ID)
	return err
}

// DeleteWorkflowVariable deletes a workflow variable
func (wd *WorkflowDatabase) DeleteWorkflowVariable(ctx context.Context, variableID uuid.UUID) error {
	query := `DELETE FROM server_workflow_variables WHERE id = $1`
	_, err := wd.db.ExecContext(ctx, query, variableID)
	return err
}

// loadWorkflowVariables loads all variables for a workflow set in one query.
func (wd *WorkflowDatabase) loadWorkflowVariables(ctx context.Context, workflows []models.ServerWorkflow) error {
	if len(workflows) == 0 {
		return nil
	}

	workflowByID := make(map[uuid.UUID]*models.ServerWorkflow, len(workflows))
	workflowIDs := make([]uuid.UUID, 0, len(workflows))
	for i := range workflows {
		workflows[i].Variables = []models.ServerWorkflowVariable{}
		workflowByID[workflows[i].ID] = &workflows[i]
		workflowIDs = append(workflowIDs, workflows[i].ID)
	}

	const query = `
		/* sa.workflow.variables */
		SELECT workflow_id, id, name, value, description, created_at, updated_at
		FROM server_workflow_variables
		WHERE workflow_id = ANY($1)
		ORDER BY workflow_id, name, id
	`

	rows, err := wd.db.QueryContext(ctx, query, pq.Array(workflowIDs))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var variable models.ServerWorkflowVariable
		var valueJSON []byte
		var description sql.NullString

		err := rows.Scan(
			&variable.WorkflowID,
			&variable.ID,
			&variable.Name,
			&valueJSON,
			&description,
			&variable.CreatedAt,
			&variable.UpdatedAt,
		)

		if err != nil {
			return err
		}

		if description.Valid {
			variable.Description = &description.String
		}

		if err := json.Unmarshal(valueJSON, &variable.Value); err != nil {
			return fmt.Errorf("failed to unmarshal variable value for %s: %w", variable.Name, err)
		}

		workflow, ok := workflowByID[variable.WorkflowID]
		if !ok {
			return fmt.Errorf("workflow variable %s references unexpected workflow %s", variable.ID, variable.WorkflowID)
		}
		workflow.Variables = append(workflow.Variables, variable)
	}

	return rows.Err()
}

// GetExecutionsByWorkflowID retrieves workflow executions for a workflow
func (wd *WorkflowDatabase) GetExecutionsByWorkflowID(
	ctx context.Context,
	workflowID uuid.UUID,
	limit int,
	beforeStartedAt *time.Time,
	beforeID *uuid.UUID,
) ([]models.ServerWorkflowExecution, error) {
	if (beforeStartedAt == nil) != (beforeID == nil) {
		return nil, fmt.Errorf("workflow execution cursor requires both started_at and id")
	}

	query := `
		/* sa.workflow.executions */
		SELECT id, workflow_id, execution_id, status, trigger_data, started_at, completed_at, error_message
		FROM server_workflow_executions
		WHERE workflow_id = $1
	`
	args := []interface{}{workflowID}
	if beforeStartedAt != nil {
		query += ` AND (started_at, id) < ($2, $3)`
		args = append(args, *beforeStartedAt, *beforeID)
	}
	args = append(args, limit)
	query += fmt.Sprintf(" ORDER BY started_at DESC, id DESC LIMIT $%d", len(args))

	rows, err := wd.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var executions []models.ServerWorkflowExecution

	for rows.Next() {
		var execution models.ServerWorkflowExecution
		var completedAt sql.NullTime
		var errorMessage sql.NullString
		var triggerDataJSON []byte

		err := rows.Scan(
			&execution.ID,
			&execution.WorkflowID,
			&execution.ExecutionID,
			&execution.Status,
			&triggerDataJSON,
			&execution.StartedAt,
			&completedAt,
			&errorMessage,
		)

		if err != nil {
			return nil, err
		}

		if len(triggerDataJSON) > 0 {
			if err := json.Unmarshal(triggerDataJSON, &execution.TriggerData); err != nil {
				return nil, fmt.Errorf("failed to unmarshal trigger_data: %w", err)
			}
		}

		if completedAt.Valid {
			execution.CompletedAt = &completedAt.Time
		}

		if errorMessage.Valid {
			execution.ErrorMessage = &errorMessage.String
		}

		executions = append(executions, execution)
	}

	return executions, rows.Err()
}

// GetExecutionsByServerID retrieves workflow executions for a server
func (wd *WorkflowDatabase) GetExecutionsByServerID(
	ctx context.Context,
	serverID uuid.UUID,
	limit int,
	beforeStartedAt *time.Time,
	beforeID *uuid.UUID,
) ([]models.ServerWorkflowExecution, error) {
	if (beforeStartedAt == nil) != (beforeID == nil) {
		return nil, fmt.Errorf("server workflow execution cursor requires both started_at and id")
	}

	query := `
		/* sa.workflow.server_executions */
		SELECT e.id, e.workflow_id, e.execution_id, e.status, e.trigger_data, e.started_at, e.completed_at, e.error_message
		FROM server_workflow_executions e
		JOIN server_workflows w ON e.workflow_id = w.id
		WHERE w.server_id = $1
	`
	args := []interface{}{serverID}
	if beforeStartedAt != nil {
		query += ` AND (e.started_at, e.id) < ($2, $3)`
		args = append(args, *beforeStartedAt, *beforeID)
	}
	args = append(args, limit)
	query += fmt.Sprintf(" ORDER BY e.started_at DESC, e.id DESC LIMIT $%d", len(args))

	rows, err := wd.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var executions []models.ServerWorkflowExecution

	for rows.Next() {
		var execution models.ServerWorkflowExecution
		var completedAt sql.NullTime
		var errorMessage sql.NullString
		var triggerDataJSON []byte

		err := rows.Scan(
			&execution.ID,
			&execution.WorkflowID,
			&execution.ExecutionID,
			&execution.Status,
			&triggerDataJSON,
			&execution.StartedAt,
			&completedAt,
			&errorMessage,
		)

		if err != nil {
			return nil, err
		}

		if len(triggerDataJSON) > 0 {
			if err := json.Unmarshal(triggerDataJSON, &execution.TriggerData); err != nil {
				return nil, fmt.Errorf("failed to unmarshal trigger_data: %w", err)
			}
		}

		if completedAt.Valid {
			execution.CompletedAt = &completedAt.Time
		}

		if errorMessage.Valid {
			execution.ErrorMessage = &errorMessage.String
		}

		executions = append(executions, execution)
	}

	return executions, rows.Err()
}

// CreateWorkflowExecution creates a new workflow execution record
func (wd *WorkflowDatabase) CreateWorkflowExecution(ctx context.Context, execution *models.ServerWorkflowExecution) error {
	query := `
		INSERT INTO server_workflow_executions (id, workflow_id, execution_id, status, trigger_data, started_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	triggerDataJSON, err := json.Marshal(execution.TriggerData)
	if err != nil {
		return fmt.Errorf("failed to marshal trigger_data: %w", err)
	}

	_, err = wd.db.ExecContext(ctx, query,
		execution.ID,
		execution.WorkflowID,
		execution.ExecutionID,
		execution.Status,
		triggerDataJSON,
		execution.StartedAt,
	)

	return err
}

// UpdateWorkflowExecution updates a workflow execution record
func (wd *WorkflowDatabase) UpdateWorkflowExecution(ctx context.Context, execution *models.ServerWorkflowExecution) error {
	query := `
		UPDATE server_workflow_executions
		SET status = $1, completed_at = $2, error_message = $3
		WHERE execution_id = $4
	`

	_, err := wd.db.ExecContext(ctx, query,
		execution.Status,
		execution.CompletedAt,
		execution.ErrorMessage,
		execution.ExecutionID,
	)

	return err
}

// GetWorkflowExecution retrieves a workflow execution by execution ID
func (wd *WorkflowDatabase) GetWorkflowExecution(ctx context.Context, executionID uuid.UUID) (*models.ServerWorkflowExecution, error) {
	query := `
		SELECT id, workflow_id, execution_id, status, trigger_data, started_at, completed_at, error_message
		FROM server_workflow_executions
		WHERE execution_id = $1
	`

	var execution models.ServerWorkflowExecution
	var completedAt sql.NullTime
	var errorMessage sql.NullString
	var triggerDataJSON []byte

	err := wd.db.QueryRowContext(ctx, query, executionID).Scan(
		&execution.ID,
		&execution.WorkflowID,
		&execution.ExecutionID,
		&execution.Status,
		&triggerDataJSON,
		&execution.StartedAt,
		&completedAt,
		&errorMessage,
	)

	if err != nil {
		return nil, err
	}

	if len(triggerDataJSON) > 0 {
		if err := json.Unmarshal(triggerDataJSON, &execution.TriggerData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal trigger_data: %w", err)
		}
	}

	if completedAt.Valid {
		execution.CompletedAt = &completedAt.Time
	}

	if errorMessage.Valid {
		execution.ErrorMessage = &errorMessage.String
	}

	return &execution, nil
}

// KV Store Operations

// GetKVValue retrieves a value from the workflow KV store
func (wd *WorkflowDatabase) GetKVValue(ctx context.Context, workflowID uuid.UUID, key string) (interface{}, error) {
	query := `
		SELECT value
		FROM server_workflow_kv_store
		WHERE workflow_id = $1 AND key = $2
	`

	var valueJSON []byte
	err := wd.db.QueryRowContext(ctx, query, workflowID, key).Scan(&valueJSON)
	if err != nil {
		return nil, err
	}

	var value interface{}
	if err := json.Unmarshal(valueJSON, &value); err != nil {
		return nil, fmt.Errorf("failed to unmarshal KV value: %w", err)
	}

	return value, nil
}

// SetKVValue sets a value in the workflow KV store (creates or updates)
func (wd *WorkflowDatabase) SetKVValue(ctx context.Context, workflowID uuid.UUID, key string, value interface{}) error {
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal KV value: %w", err)
	}

	query := `
		INSERT INTO server_workflow_kv_store (id, workflow_id, key, value, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, NOW(), NOW())
		ON CONFLICT (workflow_id, key)
		DO UPDATE SET value = $3, updated_at = NOW()
	`

	_, err = wd.db.ExecContext(ctx, query, workflowID, key, valueJSON)
	return err
}

// DeleteKVValue deletes a value from the workflow KV store
func (wd *WorkflowDatabase) DeleteKVValue(ctx context.Context, workflowID uuid.UUID, key string) error {
	query := `DELETE FROM server_workflow_kv_store WHERE workflow_id = $1 AND key = $2`
	_, err := wd.db.ExecContext(ctx, query, workflowID, key)
	return err
}

// KVExists checks if a key exists in the workflow KV store
func (wd *WorkflowDatabase) KVExists(ctx context.Context, workflowID uuid.UUID, key string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM server_workflow_kv_store WHERE workflow_id = $1 AND key = $2)`

	var exists bool
	err := wd.db.QueryRowContext(ctx, query, workflowID, key).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// ListKVKeys returns all keys in the workflow KV store
func (wd *WorkflowDatabase) ListKVKeys(ctx context.Context, workflowID uuid.UUID) ([]string, error) {
	query := `
		SELECT key
		FROM server_workflow_kv_store
		WHERE workflow_id = $1
		ORDER BY key ASC
	`

	rows, err := wd.db.QueryContext(ctx, query, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}

	return keys, rows.Err()
}

// GetAllKVPairs returns all key-value pairs for a workflow
func (wd *WorkflowDatabase) GetAllKVPairs(ctx context.Context, workflowID uuid.UUID) (map[string]interface{}, error) {
	query := `
		SELECT key, value
		FROM server_workflow_kv_store
		WHERE workflow_id = $1
		ORDER BY key ASC
	`

	rows, err := wd.db.QueryContext(ctx, query, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	kvPairs := make(map[string]interface{})
	for rows.Next() {
		var key string
		var valueJSON []byte

		if err := rows.Scan(&key, &valueJSON); err != nil {
			return nil, err
		}

		var value interface{}
		if err := json.Unmarshal(valueJSON, &value); err != nil {
			return nil, fmt.Errorf("failed to unmarshal KV value for key %s: %w", key, err)
		}

		kvPairs[key] = value
	}

	return kvPairs, rows.Err()
}

// ClearKVStore removes all key-value pairs for a workflow
func (wd *WorkflowDatabase) ClearKVStore(ctx context.Context, workflowID uuid.UUID) error {
	query := `DELETE FROM server_workflow_kv_store WHERE workflow_id = $1`
	_, err := wd.db.ExecContext(ctx, query, workflowID)
	return err
}

// CountKVPairs returns the number of key-value pairs for a workflow
func (wd *WorkflowDatabase) CountKVPairs(ctx context.Context, workflowID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM server_workflow_kv_store WHERE workflow_id = $1`

	var count int
	err := wd.db.QueryRowContext(ctx, query, workflowID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// WorkflowExecutionStats holds computed statistics for a workflow
type WorkflowExecutionStats struct {
	TotalExecutions int64   `json:"total_executions"`
	SuccessRate     float64 `json:"success_rate"`
	AvgDurationMs   *int64  `json:"avg_duration_ms"`
	RunningCount    int64   `json:"running_count"`
}

// GetWorkflowExecutionStats computes aggregate statistics for a workflow
func (wd *WorkflowDatabase) GetWorkflowExecutionStats(ctx context.Context, workflowID uuid.UUID) (*WorkflowExecutionStats, error) {
	query := `
		SELECT
			COUNT(*) as total_executions,
			COALESCE(
				ROUND(
					(COUNT(*) FILTER (WHERE status = 'COMPLETED')::numeric /
					NULLIF(COUNT(*), 0)::numeric) * 100,
					2
				),
				0
			) as success_rate,
			EXTRACT(EPOCH FROM AVG(completed_at - started_at) FILTER (WHERE completed_at IS NOT NULL)) * 1000 as avg_duration_ms,
			COUNT(*) FILTER (WHERE status IN ('RUNNING', 'EXECUTING')) as running_count
		FROM server_workflow_executions
		WHERE workflow_id = $1
	`

	var stats WorkflowExecutionStats
	var avgDurationMs sql.NullFloat64

	err := wd.db.QueryRowContext(ctx, query, workflowID).Scan(
		&stats.TotalExecutions,
		&stats.SuccessRate,
		&avgDurationMs,
		&stats.RunningCount,
	)

	if err != nil {
		return nil, err
	}

	if avgDurationMs.Valid {
		ms := int64(avgDurationMs.Float64)
		stats.AvgDurationMs = &ms
	}

	return &stats, nil
}
