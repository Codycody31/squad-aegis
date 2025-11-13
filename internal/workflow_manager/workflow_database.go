package workflow_manager

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/models"
)

// WorkflowDatabase handles database operations for workflows
type WorkflowDatabase struct {
	db *sql.DB
}

// NewWorkflowDatabase creates a new workflow database handler
func NewWorkflowDatabase(db *sql.DB) *WorkflowDatabase {
	return &WorkflowDatabase{db: db}
}

// CreateWorkflow creates a new workflow in the database
func (wd *WorkflowDatabase) CreateWorkflow(workflow *models.ServerWorkflow) error {
	definitionJSON, err := json.Marshal(workflow.Definition)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow definition: %w", err)
	}

	query := `
		INSERT INTO server_workflows (id, server_id, name, description, enabled, definition, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err = wd.db.Exec(query,
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
func (wd *WorkflowDatabase) GetWorkflow(workflowID uuid.UUID) (*models.ServerWorkflow, error) {
	query := `
		SELECT id, server_id, name, description, enabled, definition, created_by, created_at, updated_at
		FROM server_workflows
		WHERE id = $1
	`

	var workflow models.ServerWorkflow
	var definitionJSON []byte
	var description sql.NullString

	err := wd.db.QueryRow(query, workflowID).Scan(
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

	// Load variables
	if err := wd.loadWorkflowVariables(&workflow); err != nil {
		return nil, fmt.Errorf("failed to load workflow variables: %w", err)
	}

	return &workflow, nil
}

// GetWorkflowsByServerID retrieves all workflows for a server
func (wd *WorkflowDatabase) GetWorkflowsByServerID(serverID uuid.UUID) ([]models.ServerWorkflow, error) {
	query := `
		SELECT id, server_id, name, description, enabled, definition, created_by, created_at, updated_at
		FROM server_workflows
		WHERE server_id = $1
		ORDER BY created_at DESC
	`

	rows, err := wd.db.Query(query, serverID)
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

		// Load variables
		if err := wd.loadWorkflowVariables(&workflow); err != nil {
			return nil, fmt.Errorf("failed to load variables for workflow %s: %w", workflow.ID, err)
		}

		workflows = append(workflows, workflow)
	}

	return workflows, nil
}

// UpdateWorkflow updates a workflow in the database
func (wd *WorkflowDatabase) UpdateWorkflow(workflow *models.ServerWorkflow) error {
	definitionJSON, err := json.Marshal(workflow.Definition)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow definition: %w", err)
	}

	query := `
		UPDATE server_workflows
		SET name = $1, description = $2, enabled = $3, definition = $4, updated_at = $5
		WHERE id = $6
	`

	_, err = wd.db.Exec(query,
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
func (wd *WorkflowDatabase) DeleteWorkflow(workflowID uuid.UUID) error {
	// This will cascade delete variables and executions
	query := `DELETE FROM server_workflows WHERE id = $1`
	_, err := wd.db.Exec(query, workflowID)
	return err
}

// CreateWorkflowVariable creates a new workflow variable
func (wd *WorkflowDatabase) CreateWorkflowVariable(variable *models.ServerWorkflowVariable) error {
	valueJSON, err := json.Marshal(variable.Value)
	if err != nil {
		return fmt.Errorf("failed to marshal variable value: %w", err)
	}

	query := `
		INSERT INTO server_workflow_variables (id, workflow_id, name, value, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = wd.db.Exec(query,
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
func (wd *WorkflowDatabase) GetWorkflowVariable(workflowID uuid.UUID, name string) (*models.ServerWorkflowVariable, error) {
	query := `
		SELECT id, workflow_id, name, value, description, created_at, updated_at
		FROM server_workflow_variables
		WHERE workflow_id = $1 AND name = $2
	`

	var variable models.ServerWorkflowVariable
	var valueJSON []byte
	var description sql.NullString

	err := wd.db.QueryRow(query, workflowID, name).Scan(
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
func (wd *WorkflowDatabase) UpdateWorkflowVariable(variable *models.ServerWorkflowVariable) error {
	valueJSON, err := json.Marshal(variable.Value)
	if err != nil {
		return fmt.Errorf("failed to marshal variable value: %w", err)
	}

	query := `
		UPDATE server_workflow_variables
		SET value = $1, description = $2, updated_at = $3
		WHERE id = $4
	`

	_, err = wd.db.Exec(query, valueJSON, variable.Description, variable.UpdatedAt, variable.ID)
	return err
}

// DeleteWorkflowVariable deletes a workflow variable
func (wd *WorkflowDatabase) DeleteWorkflowVariable(variableID uuid.UUID) error {
	query := `DELETE FROM server_workflow_variables WHERE id = $1`
	_, err := wd.db.Exec(query, variableID)
	return err
}

// loadWorkflowVariables loads all variables for a workflow
func (wd *WorkflowDatabase) loadWorkflowVariables(workflow *models.ServerWorkflow) error {
	query := `
		SELECT id, name, value, description, created_at, updated_at
		FROM server_workflow_variables
		WHERE workflow_id = $1
	`

	rows, err := wd.db.Query(query, workflow.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	workflow.Variables = []models.ServerWorkflowVariable{}

	for rows.Next() {
		var variable models.ServerWorkflowVariable
		var valueJSON []byte
		var description sql.NullString

		err := rows.Scan(
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

		variable.WorkflowID = workflow.ID
		workflow.Variables = append(workflow.Variables, variable)
	}

	return nil
}

// GetExecutionsByWorkflowID retrieves workflow executions for a workflow
func (wd *WorkflowDatabase) GetExecutionsByWorkflowID(workflowID uuid.UUID, limit, offset int) ([]models.ServerWorkflowExecution, error) {
	query := `
		SELECT id, workflow_id, execution_id, status, started_at, completed_at, error_message
		FROM server_workflow_executions
		WHERE workflow_id = $1
		ORDER BY started_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := wd.db.Query(query, workflowID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var executions []models.ServerWorkflowExecution

	for rows.Next() {
		var execution models.ServerWorkflowExecution
		var completedAt sql.NullTime
		var errorMessage sql.NullString

		err := rows.Scan(
			&execution.ID,
			&execution.WorkflowID,
			&execution.ExecutionID,
			&execution.Status,
			&execution.StartedAt,
			&completedAt,
			&errorMessage,
		)

		if err != nil {
			return nil, err
		}

		if completedAt.Valid {
			execution.CompletedAt = &completedAt.Time
		}

		if errorMessage.Valid {
			execution.ErrorMessage = &errorMessage.String
		}

		executions = append(executions, execution)
	}

	return executions, nil
}

// GetExecutionsByServerID retrieves workflow executions for a server
func (wd *WorkflowDatabase) GetExecutionsByServerID(serverID uuid.UUID, limit, offset int) ([]models.ServerWorkflowExecution, error) {
	query := `
		SELECT e.id, e.workflow_id, e.execution_id, e.status, e.started_at, e.completed_at, e.error_message
		FROM server_workflow_executions e
		JOIN server_workflows w ON e.workflow_id = w.id
		WHERE w.server_id = $1
		ORDER BY e.started_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := wd.db.Query(query, serverID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var executions []models.ServerWorkflowExecution

	for rows.Next() {
		var execution models.ServerWorkflowExecution
		var completedAt sql.NullTime
		var errorMessage sql.NullString

		err := rows.Scan(
			&execution.ID,
			&execution.WorkflowID,
			&execution.ExecutionID,
			&execution.Status,
			&execution.StartedAt,
			&completedAt,
			&errorMessage,
		)

		if err != nil {
			return nil, err
		}

		if completedAt.Valid {
			execution.CompletedAt = &completedAt.Time
		}

		if errorMessage.Valid {
			execution.ErrorMessage = &errorMessage.String
		}

		executions = append(executions, execution)
	}

	return executions, nil
}

// CreateWorkflowExecution creates a new workflow execution record
func (wd *WorkflowDatabase) CreateWorkflowExecution(execution *models.ServerWorkflowExecution) error {
	query := `
		INSERT INTO server_workflow_executions (id, workflow_id, execution_id, status, started_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := wd.db.Exec(query,
		execution.ID,
		execution.WorkflowID,
		execution.ExecutionID,
		execution.Status,
		execution.StartedAt,
	)

	return err
}

// UpdateWorkflowExecution updates a workflow execution record
func (wd *WorkflowDatabase) UpdateWorkflowExecution(execution *models.ServerWorkflowExecution) error {
	query := `
		UPDATE server_workflow_executions
		SET status = $1, completed_at = $2, error_message = $3
		WHERE execution_id = $4
	`

	_, err := wd.db.Exec(query,
		execution.Status,
		execution.CompletedAt,
		execution.ErrorMessage,
		execution.ExecutionID,
	)

	return err
}

// GetWorkflowExecution retrieves a workflow execution by execution ID
func (wd *WorkflowDatabase) GetWorkflowExecution(executionID uuid.UUID) (*models.ServerWorkflowExecution, error) {
	query := `
		SELECT id, workflow_id, execution_id, status, started_at, completed_at, error_message
		FROM server_workflow_executions
		WHERE execution_id = $1
	`

	var execution models.ServerWorkflowExecution
	var completedAt sql.NullTime
	var errorMessage sql.NullString

	err := wd.db.QueryRow(query, executionID).Scan(
		&execution.ID,
		&execution.WorkflowID,
		&execution.ExecutionID,
		&execution.Status,
		&execution.StartedAt,
		&completedAt,
		&errorMessage,
	)

	if err != nil {
		return nil, err
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
func (wd *WorkflowDatabase) GetKVValue(workflowID uuid.UUID, key string) (interface{}, error) {
	query := `
		SELECT value
		FROM server_workflow_kv_store
		WHERE workflow_id = $1 AND key = $2
	`

	var valueJSON []byte
	err := wd.db.QueryRow(query, workflowID, key).Scan(&valueJSON)
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
func (wd *WorkflowDatabase) SetKVValue(workflowID uuid.UUID, key string, value interface{}) error {
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

	_, err = wd.db.Exec(query, workflowID, key, valueJSON)
	return err
}

// DeleteKVValue deletes a value from the workflow KV store
func (wd *WorkflowDatabase) DeleteKVValue(workflowID uuid.UUID, key string) error {
	query := `DELETE FROM server_workflow_kv_store WHERE workflow_id = $1 AND key = $2`
	_, err := wd.db.Exec(query, workflowID, key)
	return err
}

// KVExists checks if a key exists in the workflow KV store
func (wd *WorkflowDatabase) KVExists(workflowID uuid.UUID, key string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM server_workflow_kv_store WHERE workflow_id = $1 AND key = $2)`

	var exists bool
	err := wd.db.QueryRow(query, workflowID, key).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// ListKVKeys returns all keys in the workflow KV store
func (wd *WorkflowDatabase) ListKVKeys(workflowID uuid.UUID) ([]string, error) {
	query := `
		SELECT key
		FROM server_workflow_kv_store
		WHERE workflow_id = $1
		ORDER BY key ASC
	`

	rows, err := wd.db.Query(query, workflowID)
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

	return keys, nil
}

// GetAllKVPairs returns all key-value pairs for a workflow
func (wd *WorkflowDatabase) GetAllKVPairs(workflowID uuid.UUID) (map[string]interface{}, error) {
	query := `
		SELECT key, value
		FROM server_workflow_kv_store
		WHERE workflow_id = $1
		ORDER BY key ASC
	`

	rows, err := wd.db.Query(query, workflowID)
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

	return kvPairs, nil
}

// ClearKVStore removes all key-value pairs for a workflow
func (wd *WorkflowDatabase) ClearKVStore(workflowID uuid.UUID) error {
	query := `DELETE FROM server_workflow_kv_store WHERE workflow_id = $1`
	_, err := wd.db.Exec(query, workflowID)
	return err
}

// CountKVPairs returns the number of key-value pairs for a workflow
func (wd *WorkflowDatabase) CountKVPairs(workflowID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM server_workflow_kv_store WHERE workflow_id = $1`

	var count int
	err := wd.db.QueryRow(query, workflowID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}
