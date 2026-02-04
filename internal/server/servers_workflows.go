package server

import (
	"database/sql"
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	"go.codycody31.dev/squad-aegis/internal/workflow_manager"
)

// ServerWorkflowsList returns all workflows for a server
func (s *Server) ServerWorkflowsList(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	// TODO: Check if user has access to this server

	workflowDB := workflow_manager.NewWorkflowDatabase(s.Dependencies.DB)
	workflows, err := workflowDB.GetWorkflowsByServerID(serverID)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get workflows"})
		return
	}

	responses.Success(c, "Workflows retrieved successfully", &gin.H{
		"workflows": workflows,
	})
}

// ServerWorkflowGet returns a specific workflow
func (s *Server) ServerWorkflowGet(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		responses.BadRequest(c, "Invalid workflow ID", &gin.H{"error": err.Error()})
		return
	}

	workflowDB := workflow_manager.NewWorkflowDatabase(s.Dependencies.DB)
	workflow, err := workflowDB.GetWorkflow(workflowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Workflow not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get workflow"})
		return
	}

	// Verify workflow belongs to the server
	if workflow.ServerID != serverID {
		responses.NotFound(c, "Workflow not found", nil)
		return
	}

	responses.Success(c, "Workflow retrieved successfully", &gin.H{
		"workflow": workflow,
	})
}

// ServerWorkflowCreate creates a new workflow
func (s *Server) ServerWorkflowCreate(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	var request models.ServerWorkflowCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Validate request
	err = validation.ValidateStruct(&request,
		validation.Field(&request.Name, validation.Required, validation.Length(1, 255)),
		validation.Field(&request.Definition, validation.Required),
	)
	if err != nil {
		responses.BadRequest(c, "Validation failed", &gin.H{"errors": err})
		return
	}

	// Create workflow
	workflow := &models.ServerWorkflow{
		ID:          uuid.New(),
		ServerID:    serverID,
		Name:        request.Name,
		Description: request.Description,
		Enabled:     request.Enabled,
		Definition:  request.Definition,
		CreatedBy:   user.Id,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	workflowDB := workflow_manager.NewWorkflowDatabase(s.Dependencies.DB)
	if err := workflowDB.CreateWorkflow(workflow); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to create workflow"})
		return
	}

	// Reload workflows in the workflow manager
	if err := s.Dependencies.WorkflowManager.ReloadWorkflows(); err != nil {
		// Log error but don't fail the request since workflow was created
		// The workflow will be loaded on next restart
	}

	responses.Success(c, "Workflow created successfully", &gin.H{
		"workflow": workflow,
	})
}

// ServerWorkflowUpdate updates a workflow
func (s *Server) ServerWorkflowUpdate(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		responses.BadRequest(c, "Invalid workflow ID", &gin.H{"error": err.Error()})
		return
	}

	var request models.ServerWorkflowUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	workflowDB := workflow_manager.NewWorkflowDatabase(s.Dependencies.DB)

	// Get existing workflow
	workflow, err := workflowDB.GetWorkflow(workflowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Workflow not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get workflow"})
		return
	}

	// Verify workflow belongs to the server
	if workflow.ServerID != serverID {
		responses.NotFound(c, "Workflow not found", nil)
		return
	}

	// Update fields
	if request.Name != nil {
		workflow.Name = *request.Name
	}
	if request.Description != nil {
		workflow.Description = request.Description
	}
	if request.Enabled != nil {
		workflow.Enabled = *request.Enabled
	}
	if request.Definition != nil {
		workflow.Definition = *request.Definition
	}
	workflow.UpdatedAt = time.Now()

	// Update workflow
	if err := workflowDB.UpdateWorkflow(workflow); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to update workflow"})
		return
	}

	// Reload workflows in the workflow manager
	if err := s.Dependencies.WorkflowManager.ReloadWorkflows(); err != nil {
		// Log error but don't fail the request since workflow was updated
	}

	responses.Success(c, "Workflow updated successfully", &gin.H{
		"workflow": workflow,
	})
}

// ServerWorkflowDelete deletes a workflow
func (s *Server) ServerWorkflowDelete(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		responses.BadRequest(c, "Invalid workflow ID", &gin.H{"error": err.Error()})
		return
	}

	workflowDB := workflow_manager.NewWorkflowDatabase(s.Dependencies.DB)

	// Get existing workflow to verify ownership
	workflow, err := workflowDB.GetWorkflow(workflowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Workflow not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get workflow"})
		return
	}

	// Verify workflow belongs to the server
	if workflow.ServerID != serverID {
		responses.NotFound(c, "Workflow not found", nil)
		return
	}

	// Delete workflow
	if err := workflowDB.DeleteWorkflow(workflowID); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to delete workflow"})
		return
	}

	// Reload workflows in the workflow manager
	if err := s.Dependencies.WorkflowManager.ReloadWorkflows(); err != nil {
		// Log error but don't fail the request since workflow was deleted
	}

	responses.Success(c, "Workflow deleted successfully", nil)
}

// ServerWorkflowExecutions returns execution history for a workflow
func (s *Server) ServerWorkflowExecutions(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		responses.BadRequest(c, "Invalid workflow ID", &gin.H{"error": err.Error()})
		return
	}

	// Parse pagination parameters
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 50
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	workflowDB := workflow_manager.NewWorkflowDatabase(s.Dependencies.DB)

	// Verify workflow exists and belongs to server
	workflow, err := workflowDB.GetWorkflow(workflowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Workflow not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get workflow"})
		return
	}

	if workflow.ServerID != serverID {
		responses.NotFound(c, "Workflow not found", nil)
		return
	}

	// Get executions
	executions, err := workflowDB.GetExecutionsByWorkflowID(workflowID, limit, offset)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get executions"})
		return
	}

	responses.Success(c, "Executions retrieved successfully", &gin.H{
		"executions": executions,
		"limit":      limit,
		"offset":     offset,
	})
}

// ServerWorkflowExecutionStats returns computed statistics for a workflow
func (s *Server) ServerWorkflowExecutionStats(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		responses.BadRequest(c, "Invalid workflow ID", &gin.H{"error": err.Error()})
		return
	}

	workflowDB := workflow_manager.NewWorkflowDatabase(s.Dependencies.DB)

	// Verify workflow exists and belongs to server
	workflow, err := workflowDB.GetWorkflow(workflowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Workflow not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get workflow"})
		return
	}

	if workflow.ServerID != serverID {
		responses.NotFound(c, "Workflow not found", nil)
		return
	}

	// Get statistics
	stats, err := workflowDB.GetWorkflowExecutionStats(workflowID)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get execution statistics"})
		return
	}

	responses.Success(c, "Execution statistics retrieved successfully", &gin.H{
		"stats": stats,
	})
}

// ServerWorkflowExecutionGet returns details for a specific execution
func (s *Server) ServerWorkflowExecutionGet(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		responses.BadRequest(c, "Invalid workflow ID", &gin.H{"error": err.Error()})
		return
	}

	executionID, err := uuid.Parse(c.Param("executionId"))
	if err != nil {
		responses.BadRequest(c, "Invalid execution ID", &gin.H{"error": err.Error()})
		return
	}

	workflowDB := workflow_manager.NewWorkflowDatabase(s.Dependencies.DB)

	// Verify workflow exists and belongs to server
	workflow, err := workflowDB.GetWorkflow(workflowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Workflow not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get workflow"})
		return
	}

	if workflow.ServerID != serverID {
		responses.NotFound(c, "Workflow not found", nil)
		return
	}

	// Get execution details
	execution, err := workflowDB.GetWorkflowExecution(executionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Execution not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get execution"})
		return
	}

	// Verify execution belongs to the workflow
	if execution.WorkflowID != workflowID {
		responses.NotFound(c, "Execution not found", nil)
		return
	}

	responses.Success(c, "Execution retrieved successfully", &gin.H{
		"execution": execution,
	})
}

// ServerWorkflowExecutionLogs returns detailed logs for a specific execution
func (s *Server) ServerWorkflowExecutionLogs(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		responses.BadRequest(c, "Invalid workflow ID", &gin.H{"error": err.Error()})
		return
	}

	executionID, err := uuid.Parse(c.Param("executionId"))
	if err != nil {
		responses.BadRequest(c, "Invalid execution ID", &gin.H{"error": err.Error()})
		return
	}

	workflowDB := workflow_manager.NewWorkflowDatabase(s.Dependencies.DB)

	// Verify workflow exists and belongs to server
	workflow, err := workflowDB.GetWorkflow(workflowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Workflow not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get workflow"})
		return
	}

	if workflow.ServerID != serverID {
		responses.NotFound(c, "Workflow not found", nil)
		return
	}

	// Get execution details to verify it belongs to this workflow
	execution, err := workflowDB.GetWorkflowExecution(executionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Execution not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get execution"})
		return
	}

	if execution.WorkflowID != workflowID {
		responses.NotFound(c, "Execution not found", nil)
		return
	}

	// Parse pagination parameters
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Get detailed logs from ClickHouse
	logs, err := s.Dependencies.Clickhouse.GetWorkflowExecutionLogs(c.Request.Context(), executionID, limit, offset)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get execution logs"})
		return
	}

	responses.Success(c, "Execution logs retrieved successfully", &gin.H{
		"execution": execution,
		"logs":      logs,
		"limit":     limit,
		"offset":    offset,
	})
}

// ServerWorkflowExecutionMessages returns workflow log messages for a specific execution
func (s *Server) ServerWorkflowExecutionMessages(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		responses.BadRequest(c, "Invalid workflow ID", &gin.H{"error": err.Error()})
		return
	}

	executionID, err := uuid.Parse(c.Param("executionId"))
	if err != nil {
		responses.BadRequest(c, "Invalid execution ID", &gin.H{"error": err.Error()})
		return
	}

	workflowDB := workflow_manager.NewWorkflowDatabase(s.Dependencies.DB)

	// Verify workflow exists and belongs to server
	workflow, err := workflowDB.GetWorkflow(workflowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Workflow not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get workflow"})
		return
	}

	if workflow.ServerID != serverID {
		responses.NotFound(c, "Workflow not found", nil)
		return
	}

	// Get execution details to verify it belongs to this workflow
	execution, err := workflowDB.GetWorkflowExecution(executionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Execution not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get execution"})
		return
	}

	if execution.WorkflowID != workflowID {
		responses.NotFound(c, "Execution not found", nil)
		return
	}

	// Parse pagination parameters
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Get workflow log messages from ClickHouse
	messages, err := s.Dependencies.Clickhouse.GetWorkflowLogMessages(c.Request.Context(), executionID, limit, offset)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get workflow log messages"})
		return
	}

	responses.Success(c, "Workflow log messages retrieved successfully", &gin.H{
		"execution": execution,
		"messages":  messages,
		"limit":     limit,
		"offset":    offset,
	})
}

// ServerWorkflowVariablesList returns variables for a workflow
func (s *Server) ServerWorkflowVariablesList(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		responses.BadRequest(c, "Invalid workflow ID", &gin.H{"error": err.Error()})
		return
	}

	workflowDB := workflow_manager.NewWorkflowDatabase(s.Dependencies.DB)

	// Get workflow with variables
	workflow, err := workflowDB.GetWorkflow(workflowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Workflow not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get workflow"})
		return
	}

	// Verify workflow belongs to the server
	if workflow.ServerID != serverID {
		responses.NotFound(c, "Workflow not found", nil)
		return
	}

	responses.Success(c, "Variables retrieved successfully", &gin.H{
		"variables": workflow.Variables,
	})
}

// ServerWorkflowVariableCreate creates a new workflow variable
func (s *Server) ServerWorkflowVariableCreate(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		responses.BadRequest(c, "Invalid workflow ID", &gin.H{"error": err.Error()})
		return
	}

	var request models.ServerWorkflowVariableCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Validate request
	err = validation.ValidateStruct(&request,
		validation.Field(&request.Name, validation.Required, validation.Length(1, 255)),
		validation.Field(&request.Value, validation.Required),
	)
	if err != nil {
		responses.BadRequest(c, "Validation failed", &gin.H{"errors": err})
		return
	}

	workflowDB := workflow_manager.NewWorkflowDatabase(s.Dependencies.DB)

	// Verify workflow exists and belongs to server
	workflow, err := workflowDB.GetWorkflow(workflowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Workflow not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get workflow"})
		return
	}

	if workflow.ServerID != serverID {
		responses.NotFound(c, "Workflow not found", nil)
		return
	}

	// Create variable
	variable := &models.ServerWorkflowVariable{
		ID:          uuid.New(),
		WorkflowID:  workflowID,
		Name:        request.Name,
		Value:       request.Value,
		Description: request.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := workflowDB.CreateWorkflowVariable(variable); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to create variable"})
		return
	}

	// Reload workflows in the workflow manager to pick up new variable
	if err := s.Dependencies.WorkflowManager.ReloadWorkflows(); err != nil {
		// Log error but don't fail the request
	}

	responses.Success(c, "Variable created successfully", &gin.H{
		"variable": variable,
	})
}

// ServerWorkflowVariableUpdate updates a workflow variable
func (s *Server) ServerWorkflowVariableUpdate(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		responses.BadRequest(c, "Invalid workflow ID", &gin.H{"error": err.Error()})
		return
	}

	variableID, err := uuid.Parse(c.Param("variableId"))
	if err != nil {
		responses.BadRequest(c, "Invalid variable ID", &gin.H{"error": err.Error()})
		return
	}

	var request models.ServerWorkflowVariableUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	workflowDB := workflow_manager.NewWorkflowDatabase(s.Dependencies.DB)

	// Verify workflow exists and belongs to server
	workflow, err := workflowDB.GetWorkflow(workflowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Workflow not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get workflow"})
		return
	}

	if workflow.ServerID != serverID {
		responses.NotFound(c, "Workflow not found", nil)
		return
	}

	// Find the variable
	var variable *models.ServerWorkflowVariable
	for _, v := range workflow.Variables {
		if v.ID == variableID {
			variable = &v
			break
		}
	}

	if variable == nil {
		responses.NotFound(c, "Variable not found", nil)
		return
	}

	// Update fields
	if request.Name != nil {
		variable.Name = *request.Name
	}
	if request.Value != nil {
		variable.Value = *request.Value
	}
	if request.Description != nil {
		variable.Description = request.Description
	}
	variable.UpdatedAt = time.Now()

	if err := workflowDB.UpdateWorkflowVariable(variable); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to update variable"})
		return
	}

	// Reload workflows in the workflow manager
	if err := s.Dependencies.WorkflowManager.ReloadWorkflows(); err != nil {
		// Log error but don't fail the request
	}

	responses.Success(c, "Variable updated successfully", &gin.H{
		"variable": variable,
	})
}

// ServerWorkflowVariableDelete deletes a workflow variable
func (s *Server) ServerWorkflowVariableDelete(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		responses.BadRequest(c, "Invalid workflow ID", &gin.H{"error": err.Error()})
		return
	}

	variableID, err := uuid.Parse(c.Param("variableId"))
	if err != nil {
		responses.BadRequest(c, "Invalid variable ID", &gin.H{"error": err.Error()})
		return
	}

	workflowDB := workflow_manager.NewWorkflowDatabase(s.Dependencies.DB)

	// Verify workflow exists and belongs to server
	workflow, err := workflowDB.GetWorkflow(workflowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Workflow not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get workflow"})
		return
	}

	if workflow.ServerID != serverID {
		responses.NotFound(c, "Workflow not found", nil)
		return
	}

	// Verify variable exists and belongs to workflow
	found := false
	for _, v := range workflow.Variables {
		if v.ID == variableID {
			found = true
			break
		}
	}

	if !found {
		responses.NotFound(c, "Variable not found", nil)
		return
	}

	if err := workflowDB.DeleteWorkflowVariable(variableID); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to delete variable"})
		return
	}

	// Reload workflows in the workflow manager
	if err := s.Dependencies.WorkflowManager.ReloadWorkflows(); err != nil {
		// Log error but don't fail the request
	}

	responses.Success(c, "Variable deleted successfully", nil)
}

// ServerWorkflowKVList lists all KV pairs for a workflow
func (s *Server) ServerWorkflowKVList(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		responses.BadRequest(c, "Invalid workflow ID", &gin.H{"error": err.Error()})
		return
	}

	workflowDB := workflow_manager.NewWorkflowDatabase(s.Dependencies.DB)

	// Verify workflow exists and belongs to server
	workflow, err := workflowDB.GetWorkflow(workflowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Workflow not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get workflow"})
		return
	}

	if workflow.ServerID != serverID {
		responses.NotFound(c, "Workflow not found", nil)
		return
	}

	// Get all KV pairs
	kvPairs, err := workflowDB.GetAllKVPairs(workflowID)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get KV pairs"})
		return
	}

	// Convert map to array for consistent API response
	type KVPairResponse struct {
		Key   string      `json:"key"`
		Value interface{} `json:"value"`
	}

	kvArray := make([]KVPairResponse, 0, len(kvPairs))
	for key, value := range kvPairs {
		kvArray = append(kvArray, KVPairResponse{
			Key:   key,
			Value: value,
		})
	}

	responses.Success(c, "KV pairs retrieved successfully", &gin.H{"kv_pairs": kvArray})
}

// ServerWorkflowKVGet gets a specific KV pair
func (s *Server) ServerWorkflowKVGet(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		responses.BadRequest(c, "Invalid workflow ID", &gin.H{"error": err.Error()})
		return
	}

	key := c.Param("key")
	if key == "" {
		responses.BadRequest(c, "Key is required", nil)
		return
	}

	workflowDB := workflow_manager.NewWorkflowDatabase(s.Dependencies.DB)

	// Verify workflow exists and belongs to server
	workflow, err := workflowDB.GetWorkflow(workflowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Workflow not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get workflow"})
		return
	}

	if workflow.ServerID != serverID {
		responses.NotFound(c, "Workflow not found", nil)
		return
	}

	// Get KV value
	value, err := workflowDB.GetKVValue(workflowID, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Key not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get KV value"})
		return
	}

	responses.Success(c, "KV pair retrieved successfully", &gin.H{
		"key":   key,
		"value": value,
	})
}

// ServerWorkflowKVSet sets a KV pair
func (s *Server) ServerWorkflowKVSet(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		responses.BadRequest(c, "Invalid workflow ID", &gin.H{"error": err.Error()})
		return
	}

	var req struct {
		Key   string      `json:"key" binding:"required"`
		Value interface{} `json:"value" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		responses.BadRequest(c, "Invalid request body", &gin.H{"error": err.Error()})
		return
	}

	// Validate key length
	if len(req.Key) > 255 {
		responses.BadRequest(c, "Key is too long (max 255 characters)", nil)
		return
	}

	workflowDB := workflow_manager.NewWorkflowDatabase(s.Dependencies.DB)

	// Verify workflow exists and belongs to server
	workflow, err := workflowDB.GetWorkflow(workflowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Workflow not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get workflow"})
		return
	}

	if workflow.ServerID != serverID {
		responses.NotFound(c, "Workflow not found", nil)
		return
	}

	// Set KV value
	if err := workflowDB.SetKVValue(workflowID, req.Key, req.Value); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to set KV value"})
		return
	}

	responses.Success(c, "KV pair set successfully", &gin.H{
		"key":   req.Key,
		"value": req.Value,
	})
}

// ServerWorkflowKVDelete deletes a KV pair
func (s *Server) ServerWorkflowKVDelete(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		responses.BadRequest(c, "Invalid workflow ID", &gin.H{"error": err.Error()})
		return
	}

	key := c.Param("key")
	if key == "" {
		responses.BadRequest(c, "Key is required", nil)
		return
	}

	workflowDB := workflow_manager.NewWorkflowDatabase(s.Dependencies.DB)

	// Verify workflow exists and belongs to server
	workflow, err := workflowDB.GetWorkflow(workflowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Workflow not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get workflow"})
		return
	}

	if workflow.ServerID != serverID {
		responses.NotFound(c, "Workflow not found", nil)
		return
	}

	// Delete KV value
	if err := workflowDB.DeleteKVValue(workflowID, key); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to delete KV value"})
		return
	}

	responses.Success(c, "KV pair deleted successfully", nil)
}

// ServerWorkflowKVClear clears all KV pairs for a workflow
func (s *Server) ServerWorkflowKVClear(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		responses.BadRequest(c, "Invalid workflow ID", &gin.H{"error": err.Error()})
		return
	}

	workflowDB := workflow_manager.NewWorkflowDatabase(s.Dependencies.DB)

	// Verify workflow exists and belongs to server
	workflow, err := workflowDB.GetWorkflow(workflowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Workflow not found", nil)
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get workflow"})
		return
	}

	if workflow.ServerID != serverID {
		responses.NotFound(c, "Workflow not found", nil)
		return
	}

	// Clear KV store
	if err := workflowDB.ClearKVStore(workflowID); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to clear KV store"})
		return
	}

	responses.Success(c, "KV store cleared successfully", nil)
}
