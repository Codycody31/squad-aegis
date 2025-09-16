package workflow_manager

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	lua "github.com/yuin/gopher-lua"
	"go.codycody31.dev/squad-aegis/internal/clickhouse"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/rcon_manager"
)

// WorkflowManager manages workflow execution and lifecycle
type WorkflowManager struct {
	ctx              context.Context
	cancel           context.CancelFunc
	db               *sql.DB
	eventManager     *event_manager.EventManager
	rconManager      *rcon_manager.RconManager
	clickhouseClient *clickhouse.Client
	workflowDB       *WorkflowDatabase
	activeWorkflows  map[uuid.UUID]*models.ServerWorkflow
	executionContext map[uuid.UUID]*models.WorkflowExecutionContext
	mutex            sync.RWMutex
	executionMutex   sync.RWMutex
	isRunning        bool
	subscriber       *event_manager.EventSubscriber
}

// NewWorkflowManager creates a new workflow manager
func NewWorkflowManager(
	ctx context.Context,
	db *sql.DB,
	eventManager *event_manager.EventManager,
	rconManager *rcon_manager.RconManager,
	clickhouseClient *clickhouse.Client,
) *WorkflowManager {
	ctx, cancel := context.WithCancel(ctx)

	return &WorkflowManager{
		ctx:              ctx,
		cancel:           cancel,
		db:               db,
		eventManager:     eventManager,
		rconManager:      rconManager,
		clickhouseClient: clickhouseClient,
		workflowDB:       NewWorkflowDatabase(db),
		activeWorkflows:  make(map[uuid.UUID]*models.ServerWorkflow),
		executionContext: make(map[uuid.UUID]*models.WorkflowExecutionContext),
	}
}

// Start starts the workflow manager
func (wm *WorkflowManager) Start() error {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	if wm.isRunning {
		return fmt.Errorf("workflow manager is already running")
	}

	log.Debug().Msg("Starting workflow manager")

	// Load all workflows from database
	if err := wm.loadWorkflows(); err != nil {
		return fmt.Errorf("failed to load workflows: %w", err)
	}

	// Subscribe to events - subscribe to all event types by leaving Types empty
	filter := event_manager.EventFilter{
		Types: []event_manager.EventType{}, // Empty means all types
	}
	wm.subscriber = wm.eventManager.Subscribe(filter, nil, 1000)

	// Start event handler goroutine
	go wm.eventHandler()

	log.Trace().Str("subscriber_id", wm.subscriber.ID.String()).Msg("Workflow manager subscribed to events")

	wm.isRunning = true
	log.Info().Msgf("Workflow manager started with %d active workflows", len(wm.activeWorkflows))

	return nil
}

// Stop stops the workflow manager
func (wm *WorkflowManager) Stop() {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	if !wm.isRunning {
		return
	}

	log.Debug().Msg("Stopping workflow manager")

	// Cancel all running executions
	wm.executionMutex.Lock()
	for executionID := range wm.executionContext {
		log.Debug().Str("execution_id", executionID.String()).Msg("Cancelling workflow execution")
		delete(wm.executionContext, executionID)
	}
	wm.executionMutex.Unlock()

	// Unsubscribe from events
	if wm.subscriber != nil {
		wm.eventManager.Unsubscribe(wm.subscriber.ID)
	}

	wm.cancel()
	wm.isRunning = false
	log.Info().Msg("Workflow manager stopped")
}

// ReloadWorkflows reloads all workflows from the database
func (wm *WorkflowManager) ReloadWorkflows() error {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	log.Debug().Msg("Reloading workflows")
	return wm.loadWorkflows()
}

// loadWorkflows loads all enabled workflows from the database
func (wm *WorkflowManager) loadWorkflows() error {
	// Get all servers that might have workflows
	rows, err := wm.db.Query("SELECT DISTINCT server_id FROM server_workflows WHERE enabled = true")
	if err != nil {
		return fmt.Errorf("failed to query server IDs: %w", err)
	}
	defer rows.Close()

	var serverIDs []uuid.UUID
	for rows.Next() {
		var serverID uuid.UUID
		if err := rows.Scan(&serverID); err != nil {
			return fmt.Errorf("failed to scan server ID: %w", err)
		}
		serverIDs = append(serverIDs, serverID)
	}

	// Clear existing workflows
	wm.activeWorkflows = make(map[uuid.UUID]*models.ServerWorkflow)

	// Load workflows for each server
	for _, serverID := range serverIDs {
		workflows, err := wm.workflowDB.GetWorkflowsByServerID(serverID)
		if err != nil {
			log.Error().Err(err).Str("server_id", serverID.String()).Msg("Failed to load workflows for server")
			continue
		}

		log.Debug().Str("server_id", serverID.String()).Int("workflow_count", len(workflows)).Msg("Retrieved workflows from database")

		for _, workflow := range workflows {
			if workflow.Enabled {
				wm.activeWorkflows[workflow.ID] = &workflow
				log.Info().
					Str("workflow_id", workflow.ID.String()).
					Str("workflow_name", workflow.Name).
					Str("server_id", workflow.ServerID.String()).
					Int("trigger_count", len(workflow.Definition.Triggers)).
					Msg("Loaded active workflow")

				// Log each trigger for debugging
				for _, trigger := range workflow.Definition.Triggers {
					log.Debug().
						Str("workflow_id", workflow.ID.String()).
						Str("trigger_id", trigger.ID).
						Str("trigger_name", trigger.Name).
						Str("event_type", trigger.EventType).
						Bool("enabled", trigger.Enabled).
						Msg("Workflow trigger")
				}
			} else {
				log.Debug().
					Str("workflow_id", workflow.ID.String()).
					Str("workflow_name", workflow.Name).
					Msg("Skipping disabled workflow")
			}
		}
	}

	log.Info().Msgf("Loaded %d active workflows", len(wm.activeWorkflows))
	return nil
}

// eventHandler processes incoming events from the subscription
func (wm *WorkflowManager) eventHandler() {
	log.Trace().Msg("Workflow event handler started")
	for {
		select {
		case <-wm.ctx.Done():
			log.Info().Msg("Workflow event handler shutting down")
			return
		case event, ok := <-wm.subscriber.Channel:
			if !ok {
				log.Info().Msg("Workflow event handler channel closed")
				return // Channel closed
			}
			wm.handleEvent(event)
		}
	}
}

// handleEvent processes incoming events and triggers matching workflows
func (wm *WorkflowManager) handleEvent(event event_manager.Event) {
	if !wm.isRunning {
		log.Warn().Msg("Workflow manager not running, ignoring event")
		return
	}

	// Find workflows that should be triggered by this event
	wm.mutex.RLock()
	var triggeredWorkflows []*models.ServerWorkflow
	for _, workflow := range wm.activeWorkflows {
		if workflow.ServerID == event.ServerID {
			trigger := workflow.GetTriggerByEventType(string(event.Type))
			if trigger != nil {
				// Convert event data to map for condition evaluation
				eventDataMap := wm.convertEventDataToMap(event.Data)
				if wm.evaluateConditions(trigger.Conditions, eventDataMap) {
					log.Trace().
						Str("workflow_id", workflow.ID.String()).
						Str("workflow_name", workflow.Name).
						Str("trigger_name", trigger.Name).
						Str("event_type", string(event.Type)).
						Msg("Workflow trigger conditions met, adding to execution queue")
					triggeredWorkflows = append(triggeredWorkflows, workflow)
				}
			}
		}
	}
	wm.mutex.RUnlock()

	if len(triggeredWorkflows) == 0 {
		return
	}

	// Execute triggered workflows
	for _, workflow := range triggeredWorkflows {
		eventDataMap := wm.convertEventDataToMap(event.Data)
		log.Debug().
			Str("workflow_id", workflow.ID.String()).
			Str("workflow_name", workflow.Name).
			Msg("Starting workflow execution")
		go wm.executeWorkflow(workflow, eventDataMap)
	}
}

// evaluateConditions checks if an event matches workflow trigger conditions
func (wm *WorkflowManager) evaluateConditions(conditions []models.WorkflowCondition, eventData map[string]interface{}) bool {
	if len(conditions) == 0 {
		return true // No conditions means always trigger
	}

	// For now, implement basic AND logic (all conditions must be true)
	for _, condition := range conditions {
		if !wm.evaluateCondition(condition, eventData) {
			return false
		}
	}

	return true
}

// evaluateCondition evaluates a single condition
func (wm *WorkflowManager) evaluateCondition(condition models.WorkflowCondition, eventData map[string]interface{}) bool {
	// Get the field value from event data using dot notation
	fieldValue := wm.getFieldValue(condition.Field, eventData)

	switch condition.Operator {
	case models.OperatorEquals:
		result := fmt.Sprintf("%v", fieldValue) == fmt.Sprintf("%v", condition.Value)
		return result
	case models.OperatorNotEquals:
		result := fmt.Sprintf("%v", fieldValue) != fmt.Sprintf("%v", condition.Value)
		return result
	case models.OperatorContains:
		fieldStr := fmt.Sprintf("%v", fieldValue)
		valueStr := fmt.Sprintf("%v", condition.Value)
		result := len(fieldStr) > 0 && len(valueStr) > 0 && strings.Contains(fieldStr, valueStr)
		return result
	case models.OperatorNotContains:
		fieldStr := fmt.Sprintf("%v", fieldValue)
		valueStr := fmt.Sprintf("%v", condition.Value)
		result := len(fieldStr) == 0 || len(valueStr) == 0 || !strings.Contains(fieldStr, valueStr)
		return result
	case models.OperatorStartsWith:
		fieldStr := fmt.Sprintf("%v", fieldValue)
		valueStr := fmt.Sprintf("%v", condition.Value)
		result := len(fieldStr) > 0 && len(valueStr) > 0 && strings.HasPrefix(fieldStr, valueStr)
		return result
	case models.OperatorEndsWith:
		fieldStr := fmt.Sprintf("%v", fieldValue)
		valueStr := fmt.Sprintf("%v", condition.Value)
		result := len(fieldStr) > 0 && len(valueStr) > 0 && strings.HasSuffix(fieldStr, valueStr)
		return result
	case models.OperatorGreaterThan:
		return wm.compareNumbers(fieldValue, condition.Value, ">")
	case models.OperatorLessThan:
		return wm.compareNumbers(fieldValue, condition.Value, "<")
	case models.OperatorGreaterOrEqual:
		return wm.compareNumbers(fieldValue, condition.Value, ">=")
	case models.OperatorLessOrEqual:
		return wm.compareNumbers(fieldValue, condition.Value, "<=")
	case models.OperatorIn:
		return wm.evaluateInOperator(fieldValue, condition.Value)
	case models.OperatorNotIn:
		return !wm.evaluateInOperator(fieldValue, condition.Value)
	case models.OperatorRegex:
		return wm.evaluateRegexOperator(fieldValue, condition.Value)
	default:
		log.Warn().Str("operator", condition.Operator).Msg("Unsupported condition operator")
		return false
	}
}

// getFieldValue extracts a field value from event data using dot notation
func (wm *WorkflowManager) getFieldValue(field string, data map[string]interface{}) interface{} {
	// Handle empty field or data
	if field == "" || data == nil {
		return nil
	}

	// Split the field path by dots
	parts := strings.Split(field, ".")

	// Start with the root data as interface{}
	var current interface{} = data

	// Navigate through each part of the path
	for i, part := range parts {
		if current == nil {
			return nil
		}

		// Check if current is a map
		currentMap, isMap := current.(map[string]interface{})
		if !isMap {
			return nil
		}

		// Get the value for this part
		value, exists := currentMap[part]
		if !exists {
			return nil
		}

		// If this is the last part, return the value
		if i == len(parts)-1 {
			return value
		}

		// Otherwise, continue navigating with the value
		current = value
	}

	return nil
}

// getMapKeys returns the keys of a map for debugging purposes
func (wm *WorkflowManager) getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// compareNumbers compares two values numerically
func (wm *WorkflowManager) compareNumbers(fieldValue, conditionValue interface{}, operator string) bool {
	// Convert both values to float64 for comparison
	fieldFloat, fieldOk := wm.toFloat64(fieldValue)
	conditionFloat, conditionOk := wm.toFloat64(conditionValue)

	if !fieldOk || !conditionOk {
		log.Debug().
			Interface("field_value", fieldValue).
			Interface("condition_value", conditionValue).
			Msg("Failed to convert values to numbers for comparison")
		return false
	}

	switch operator {
	case ">":
		return fieldFloat > conditionFloat
	case "<":
		return fieldFloat < conditionFloat
	case ">=":
		return fieldFloat >= conditionFloat
	case "<=":
		return fieldFloat <= conditionFloat
	default:
		return false
	}
}

// toFloat64 converts an interface{} to float64
func (wm *WorkflowManager) toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		// Try to parse string as number
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			return parsed, true
		}
		return 0, false
	default:
		return 0, false
	}
}

// evaluateInOperator checks if fieldValue is in the array/slice conditionValue
func (wm *WorkflowManager) evaluateInOperator(fieldValue, conditionValue interface{}) bool {
	fieldStr := fmt.Sprintf("%v", fieldValue)

	// Handle different types of condition values
	switch v := conditionValue.(type) {
	case []interface{}:
		for _, item := range v {
			if fmt.Sprintf("%v", item) == fieldStr {
				return true
			}
		}
		return false
	case []string:
		for _, item := range v {
			if item == fieldStr {
				return true
			}
		}
		return false
	case string:
		// Treat as comma-separated values
		values := strings.Split(v, ",")
		for _, item := range values {
			if strings.TrimSpace(item) == fieldStr {
				return true
			}
		}
		return false
	default:
		// Single value comparison
		return fmt.Sprintf("%v", conditionValue) == fieldStr
	}
}

// evaluateRegexOperator performs regex matching
func (wm *WorkflowManager) evaluateRegexOperator(fieldValue, conditionValue interface{}) bool {
	fieldStr := fmt.Sprintf("%v", fieldValue)
	regexStr := fmt.Sprintf("%v", conditionValue)

	if regexStr == "" {
		return false
	}

	// Use regex package for matching
	matched, err := regexp.MatchString(regexStr, fieldStr)
	if err != nil {
		log.Warn().
			Err(err).
			Str("regex", regexStr).
			Str("field_value", fieldStr).
			Msg("Failed to evaluate regex condition")
		return false
	}

	return matched
}

// executeWorkflow executes a workflow instance
func (wm *WorkflowManager) executeWorkflow(workflow *models.ServerWorkflow, triggerEvent map[string]interface{}) {
	executionID := uuid.New()

	// Create execution context
	context := &models.WorkflowExecutionContext{
		ExecutionID:  executionID,
		WorkflowID:   workflow.ID,
		ServerID:     workflow.ServerID,
		TriggerEvent: triggerEvent,
		Metadata:     make(map[string]interface{}),
		Variables:    make(map[string]interface{}),
		StepResults:  make(map[string]interface{}),
		StartedAt:    time.Now(),
	}

	// Initialize metadata with workflow information
	context.Metadata["workflow_name"] = workflow.Name
	context.Metadata["workflow_id"] = workflow.ID.String()
	context.Metadata["server_id"] = workflow.ServerID.String()
	context.Metadata["execution_id"] = executionID.String()
	context.Metadata["started_at"] = context.StartedAt.Format(time.RFC3339)

	// Initialize variables from workflow definition
	for key, value := range workflow.Definition.Variables {
		context.Variables[key] = value
	}

	// Add workflow variables from database
	for _, variable := range workflow.Variables {
		for key, value := range variable.Value {
			context.Variables[key] = value
		}
	}

	// Store execution context
	wm.executionMutex.Lock()
	wm.executionContext[executionID] = context
	wm.executionMutex.Unlock()

	log.Debug().
		Str("execution_id", executionID.String()).
		Str("workflow_id", workflow.ID.String()).
		Str("workflow_name", workflow.Name).
		Msg("Starting workflow execution")

	// Create execution record in PostgreSQL
	pgExecution := &models.ServerWorkflowExecution{
		ID:          uuid.New(),
		WorkflowID:  workflow.ID,
		ExecutionID: executionID,
		Status:      "RUNNING",
		StartedAt:   context.StartedAt,
	}

	if err := wm.workflowDB.CreateWorkflowExecution(pgExecution); err != nil {
		log.Error().Err(err).Str("execution_id", executionID.String()).Msg("Failed to create execution record in PostgreSQL")
	}

	// Log execution start in ClickHouse
	wm.logWorkflowStep(context, workflow, "workflow_start", "WORKFLOW", 0, "RUNNING",
		map[string]interface{}{"trigger_event": triggerEvent},
		map[string]interface{}{"status": "started"}, nil, 0)

	// Initialize execution summary
	summary := &models.WorkflowExecutionSummary{
		ExecutionID:      executionID,
		WorkflowID:       workflow.ID,
		ServerID:         workflow.ServerID,
		WorkflowName:     workflow.Name,
		StartedAt:        context.StartedAt,
		Status:           "RUNNING",
		TriggerEventType: wm.getTriggerEventType(triggerEvent),
		TotalSteps:       uint32(len(workflow.Definition.Steps)),
		CompletedSteps:   0,
		FailedSteps:      0,
		SkippedSteps:     0,
		TotalDurationMs:  0,
	}

	// Execute workflow steps
	err := wm.executeWorkflowSteps(context, workflow, summary)

	// Calculate total duration
	totalDuration := time.Since(context.StartedAt)
	summary.TotalDurationMs = uint32(totalDuration.Milliseconds())

	// Update execution status
	completedAt := time.Now()
	if err != nil {
		pgExecution.Status = "FAILED"
		pgExecution.CompletedAt = &completedAt
		errorMsg := err.Error()
		pgExecution.ErrorMessage = &errorMsg
		summary.Status = "FAILED"
		summary.ErrorMessage = &errorMsg
		summary.CompletedAt = &completedAt

		log.Error().
			Err(err).
			Str("execution_id", executionID.String()).
			Str("workflow_id", workflow.ID.String()).
			Msg("Workflow execution failed")

		// Log failure in ClickHouse
		wm.logWorkflowStep(context, workflow, "workflow_failed", "WORKFLOW", uint32(len(workflow.Definition.Steps)+1), "FAILED",
			map[string]interface{}{},
			map[string]interface{}{"error": err.Error()}, &errorMsg, uint32(totalDuration.Milliseconds()))
	} else {
		pgExecution.Status = "COMPLETED"
		pgExecution.CompletedAt = &completedAt
		summary.Status = "COMPLETED"
		summary.CompletedAt = &completedAt

		log.Debug().
			Str("execution_id", executionID.String()).
			Str("workflow_id", workflow.ID.String()).
			Msg("Workflow execution completed")

		// Log completion in ClickHouse
		wm.logWorkflowStep(context, workflow, "workflow_completed", "WORKFLOW", uint32(len(workflow.Definition.Steps)+1), "COMPLETED",
			map[string]interface{}{},
			map[string]interface{}{"completed_steps": summary.CompletedSteps, "failed_steps": summary.FailedSteps}, nil, uint32(totalDuration.Milliseconds()))
	}

	// Update execution record in PostgreSQL
	if err := wm.workflowDB.UpdateWorkflowExecution(pgExecution); err != nil {
		log.Error().Err(err).Str("execution_id", executionID.String()).Msg("Failed to update execution record in PostgreSQL")
	}

	// Update execution summary in ClickHouse
	if wm.clickhouseClient != nil {
		if err := wm.clickhouseClient.UpdateWorkflowExecutionSummary(wm.ctx, summary); err != nil {
			log.Error().Err(err).Str("execution_id", executionID.String()).Msg("Failed to update execution summary in ClickHouse")
		}
	}

	// Clean up execution context
	wm.executionMutex.Lock()
	delete(wm.executionContext, executionID)
	wm.executionMutex.Unlock()
}

// executeWorkflowSteps executes the steps of a workflow
func (wm *WorkflowManager) executeWorkflowSteps(context *models.WorkflowExecutionContext, workflow *models.ServerWorkflow, summary *models.WorkflowExecutionSummary) error {
	for i, step := range workflow.Definition.Steps {
		if !step.Enabled {
			summary.SkippedSteps++
			continue
		}

		context.CurrentStep = step.ID
		stepStartTime := time.Now()

		// Log step start in ClickHouse
		wm.logWorkflowStep(context, workflow, step.Name, strings.ToUpper(step.Type), uint32(i+1), "RUNNING",
			step.Config, map[string]interface{}{}, nil, 0)

		err := wm.executeStep(context, &step, workflow)
		stepDuration := time.Since(stepStartTime)

		if err != nil {
			summary.FailedSteps++
			errorMsg := err.Error()

			log.Error().
				Err(err).
				Str("execution_id", context.ExecutionID.String()).
				Str("step_id", step.ID).
				Msg("Step execution failed")

			// Log step failure in ClickHouse
			wm.logWorkflowStep(context, workflow, step.Name, strings.ToUpper(step.Type), uint32(i+1), "FAILED",
				step.Config, map[string]interface{}{}, &errorMsg, uint32(stepDuration.Milliseconds()))

			// Handle error based on step configuration
			if step.OnError != nil {
				switch step.OnError.Action {
				case "continue":
					continue
				case "stop":
					return err
				case "retry":
					// Implement retry logic
					retryCount := step.OnError.MaxRetries
					if retryCount <= 0 {
						retryCount = 1 // Default to 1 retry
					}

					retryDelay := time.Duration(step.OnError.RetryDelay) * time.Millisecond
					if retryDelay <= 0 {
						retryDelay = 1000 * time.Millisecond // Default to 1 second
					}

					success := false
					for retry := 0; retry < retryCount; retry++ {
						log.Info().
							Str("execution_id", context.ExecutionID.String()).
							Str("step_id", step.ID).
							Int("retry_attempt", retry+1).
							Int("max_retries", retryCount).
							Msg("Retrying failed step")

						// Wait before retry (except for first attempt)
						if retry > 0 {
							time.Sleep(retryDelay)
						}

						// Log retry attempt in ClickHouse
						wm.logWorkflowStep(context, workflow, step.Name, strings.ToUpper(step.Type), uint32(i+1), "RETRYING",
							step.Config, map[string]interface{}{"retry_attempt": retry + 1, "max_retries": retryCount}, nil, 0)

						// Retry the step
						retryErr := wm.executeStep(context, &step, workflow)
						retryDuration := time.Since(stepStartTime)

						if retryErr == nil {
							// Retry succeeded
							success = true
							summary.CompletedSteps++

							log.Info().
								Str("execution_id", context.ExecutionID.String()).
								Str("step_id", step.ID).
								Int("retry_attempt", retry+1).
								Msg("Step retry succeeded")

							// Log successful retry in ClickHouse
							stepResult := context.StepResults[step.ID]
							stepOutput := map[string]interface{}{"status": "completed", "retry_attempt": retry + 1}
							if stepResult != nil {
								if resultMap, ok := stepResult.(map[string]interface{}); ok {
									for k, v := range resultMap {
										stepOutput[k] = v
									}
								}
							}
							wm.logWorkflowStep(context, workflow, step.Name, strings.ToUpper(step.Type), uint32(i+1), "COMPLETED",
								step.Config, stepOutput, nil, uint32(retryDuration.Milliseconds()))
							break
						} else {
							// Retry failed
							log.Warn().
								Err(retryErr).
								Str("execution_id", context.ExecutionID.String()).
								Str("step_id", step.ID).
								Int("retry_attempt", retry+1).
								Msg("Step retry failed")

							// Log failed retry in ClickHouse
							retryErrorMsg := retryErr.Error()
							wm.logWorkflowStep(context, workflow, step.Name, strings.ToUpper(step.Type), uint32(i+1), "RETRY_FAILED",
								step.Config, map[string]interface{}{"retry_attempt": retry + 1}, &retryErrorMsg, uint32(retryDuration.Milliseconds()))
						}
					}

					if !success {
						// All retries failed
						log.Error().
							Str("execution_id", context.ExecutionID.String()).
							Str("step_id", step.ID).
							Int("max_retries", retryCount).
							Msg("All step retries failed")
						return fmt.Errorf("step failed after %d retries: %w", retryCount, err)
					}
					continue // Step succeeded after retry
				case "goto":
					// Implement goto logic for conditional flow
					gotoStepID := step.OnError.GotoStep
					if gotoStepID == "" {
						return fmt.Errorf("goto action requires a valid goto_step")
					}

					log.Info().
						Str("execution_id", context.ExecutionID.String()).
						Str("current_step_id", step.ID).
						Str("goto_step_id", gotoStepID).
						Msg("Executing goto action after step error")

					// Execute workflow steps starting from the goto step
					return wm.executeWorkflowFromStep(context, workflow, summary, gotoStepID)
				}
			}

			// Use workflow-level error handling
			switch workflow.Definition.ErrorHandling.DefaultAction {
			case "continue":
				continue
			case "stop":
				return err
			case "retry":
				// Implement workflow-level retry logic
				maxRetries := workflow.Definition.ErrorHandling.MaxRetries
				if maxRetries <= 0 {
					maxRetries = 1 // Default to 1 retry
				}

				retryDelay := time.Duration(workflow.Definition.ErrorHandling.RetryDelay) * time.Millisecond
				if retryDelay <= 0 {
					retryDelay = 1000 * time.Millisecond // Default to 1 second
				}

				success := false
				for retry := 0; retry < maxRetries; retry++ {
					log.Info().
						Str("execution_id", context.ExecutionID.String()).
						Str("step_id", step.ID).
						Int("retry_attempt", retry+1).
						Int("max_retries", maxRetries).
						Msg("Retrying failed step (workflow-level)")

					// Wait before retry (except for first attempt)
					if retry > 0 {
						time.Sleep(retryDelay)
					}

					// Log retry attempt in ClickHouse
					wm.logWorkflowStep(context, workflow, step.Name, strings.ToUpper(step.Type), uint32(i+1), "RETRYING",
						step.Config, map[string]interface{}{"retry_attempt": retry + 1, "max_retries": maxRetries, "workflow_level": true}, nil, 0)

					// Retry the step
					retryErr := wm.executeStep(context, &step, workflow)
					retryDuration := time.Since(stepStartTime)

					if retryErr == nil {
						// Retry succeeded
						success = true
						summary.CompletedSteps++

						log.Info().
							Str("execution_id", context.ExecutionID.String()).
							Str("step_id", step.ID).
							Int("retry_attempt", retry+1).
							Msg("Step retry succeeded (workflow-level)")

						// Log successful retry in ClickHouse
						stepResult := context.StepResults[step.ID]
						stepOutput := map[string]interface{}{"status": "completed", "retry_attempt": retry + 1, "workflow_level": true}
						if stepResult != nil {
							if resultMap, ok := stepResult.(map[string]interface{}); ok {
								for k, v := range resultMap {
									stepOutput[k] = v
								}
							}
						}
						wm.logWorkflowStep(context, workflow, step.Name, strings.ToUpper(step.Type), uint32(i+1), "COMPLETED",
							step.Config, stepOutput, nil, uint32(retryDuration.Milliseconds()))
						break
					} else {
						// Retry failed
						log.Warn().
							Err(retryErr).
							Str("execution_id", context.ExecutionID.String()).
							Str("step_id", step.ID).
							Int("retry_attempt", retry+1).
							Msg("Step retry failed (workflow-level)")

						// Log failed retry in ClickHouse
						retryErrorMsg := retryErr.Error()
						wm.logWorkflowStep(context, workflow, step.Name, strings.ToUpper(step.Type), uint32(i+1), "RETRY_FAILED",
							step.Config, map[string]interface{}{"retry_attempt": retry + 1, "workflow_level": true}, &retryErrorMsg, uint32(retryDuration.Milliseconds()))
					}
				}

				if !success {
					// All retries failed
					log.Error().
						Str("execution_id", context.ExecutionID.String()).
						Str("step_id", step.ID).
						Int("max_retries", maxRetries).
						Msg("All step retries failed (workflow-level)")
					return fmt.Errorf("step failed after %d workflow-level retries: %w", maxRetries, err)
				}
				continue // Step succeeded after retry
			default:
				return err
			}
		}

		summary.CompletedSteps++

		// Log step completion in ClickHouse
		stepResult := context.StepResults[step.ID]
		stepOutput := map[string]interface{}{"status": "completed"}
		if stepResult != nil {
			if resultMap, ok := stepResult.(map[string]interface{}); ok {
				stepOutput = resultMap
			}
		}
		wm.logWorkflowStep(context, workflow, step.Name, strings.ToUpper(step.Type), uint32(i+1), "COMPLETED",
			step.Config, stepOutput, nil, uint32(stepDuration.Milliseconds()))
	}

	return nil
}

// executeWorkflowFromStep executes workflow steps starting from a specific step ID
func (wm *WorkflowManager) executeWorkflowFromStep(context *models.WorkflowExecutionContext, workflow *models.ServerWorkflow, summary *models.WorkflowExecutionSummary, startStepID string) error {
	// Find the starting step index
	startIndex := -1
	for i, step := range workflow.Definition.Steps {
		if step.ID == startStepID {
			startIndex = i
			break
		}
	}

	if startIndex == -1 {
		return fmt.Errorf("step with ID %s not found in workflow", startStepID)
	}

	log.Info().
		Str("execution_id", context.ExecutionID.String()).
		Str("start_step_id", startStepID).
		Int("start_index", startIndex).
		Msg("Starting workflow execution from specific step")

	// Execute steps starting from the found index
	for i := startIndex; i < len(workflow.Definition.Steps); i++ {
		step := workflow.Definition.Steps[i]

		if !step.Enabled {
			summary.SkippedSteps++
			continue
		}

		context.CurrentStep = step.ID
		stepStartTime := time.Now()

		// Log step start in ClickHouse
		wm.logWorkflowStep(context, workflow, step.Name, strings.ToUpper(step.Type), uint32(i+1), "RUNNING",
			step.Config, map[string]interface{}{}, nil, 0)

		err := wm.executeStep(context, &step, workflow)
		stepDuration := time.Since(stepStartTime)

		if err != nil {
			summary.FailedSteps++
			errorMsg := err.Error()

			log.Error().
				Err(err).
				Str("execution_id", context.ExecutionID.String()).
				Str("step_id", step.ID).
				Msg("Step execution failed")

			// Log step failure in ClickHouse
			wm.logWorkflowStep(context, workflow, step.Name, strings.ToUpper(step.Type), uint32(i+1), "FAILED",
				step.Config, map[string]interface{}{}, &errorMsg, uint32(stepDuration.Milliseconds()))

			// Use workflow-level error handling for subsequent failures
			switch workflow.Definition.ErrorHandling.DefaultAction {
			case "continue":
				continue
			case "stop":
				return err
			default:
				return err
			}
		}

		summary.CompletedSteps++

		// Log step completion in ClickHouse
		stepResult := context.StepResults[step.ID]
		stepOutput := map[string]interface{}{"status": "completed"}
		if stepResult != nil {
			if resultMap, ok := stepResult.(map[string]interface{}); ok {
				stepOutput = resultMap
			}
		}
		wm.logWorkflowStep(context, workflow, step.Name, strings.ToUpper(step.Type), uint32(i+1), "COMPLETED",
			step.Config, stepOutput, nil, uint32(stepDuration.Milliseconds()))
	}

	return nil
}

// executeSpecificStep executes a specific step by ID within a workflow
func (wm *WorkflowManager) executeSpecificStep(context *models.WorkflowExecutionContext, workflow *models.ServerWorkflow, stepID string) error {
	// Find the step by ID
	var targetStep *models.WorkflowStep
	var stepIndex int

	for i, step := range workflow.Definition.Steps {
		if step.ID == stepID {
			targetStep = &step
			stepIndex = i
			break
		}
	}

	if targetStep == nil {
		return fmt.Errorf("step with ID %s not found in workflow", stepID)
	}

	if !targetStep.Enabled {
		log.Debug().
			Str("execution_id", context.ExecutionID.String()).
			Str("step_id", stepID).
			Msg("Skipping disabled step")
		return nil
	}

	log.Info().
		Str("execution_id", context.ExecutionID.String()).
		Str("step_id", stepID).
		Str("step_name", targetStep.Name).
		Msg("Executing specific step")

	context.CurrentStep = targetStep.ID
	stepStartTime := time.Now()

	// Log step start in ClickHouse
	wm.logWorkflowStep(context, workflow, targetStep.Name, strings.ToUpper(targetStep.Type), uint32(stepIndex+1), "RUNNING",
		targetStep.Config, map[string]interface{}{"execution_type": "conditional_next_step"}, nil, 0)

	err := wm.executeStep(context, targetStep, workflow)
	stepDuration := time.Since(stepStartTime)

	if err != nil {
		log.Error().
			Err(err).
			Str("execution_id", context.ExecutionID.String()).
			Str("step_id", stepID).
			Msg("Specific step execution failed")

		// Log step failure in ClickHouse
		errorMsg := err.Error()
		wm.logWorkflowStep(context, workflow, targetStep.Name, strings.ToUpper(targetStep.Type), uint32(stepIndex+1), "FAILED",
			targetStep.Config, map[string]interface{}{"execution_type": "conditional_next_step"}, &errorMsg, uint32(stepDuration.Milliseconds()))

		return err
	}

	// Log step completion in ClickHouse
	stepResult := context.StepResults[targetStep.ID]
	stepOutput := map[string]interface{}{"status": "completed", "execution_type": "conditional_next_step"}
	if stepResult != nil {
		if resultMap, ok := stepResult.(map[string]interface{}); ok {
			for k, v := range resultMap {
				stepOutput[k] = v
			}
		}
	}
	wm.logWorkflowStep(context, workflow, targetStep.Name, strings.ToUpper(targetStep.Type), uint32(stepIndex+1), "COMPLETED",
		targetStep.Config, stepOutput, nil, uint32(stepDuration.Milliseconds()))

	log.Info().
		Str("execution_id", context.ExecutionID.String()).
		Str("step_id", stepID).
		Str("step_name", targetStep.Name).
		Msg("Specific step execution completed")

	return nil
}

// executeStep executes a single workflow step
func (wm *WorkflowManager) executeStep(context *models.WorkflowExecutionContext, step *models.WorkflowStep, workflow *models.ServerWorkflow) error {
	switch step.Type {
	case models.StepTypeAction:
		return wm.executeActionStep(context, step)
	case models.StepTypeCondition:
		return wm.executeConditionStep(context, step, workflow)
	case models.StepTypeVariable:
		return wm.executeVariableStep(context, step)
	case models.StepTypeDelay:
		return wm.executeDelayStep(context, step)
	case models.StepTypeLua:
		return wm.executeLuaStep(context, step)
	default:
		return fmt.Errorf("unsupported step type: %s", step.Type)
	}
}

// executeActionStep executes an action step
func (wm *WorkflowManager) executeActionStep(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	actionType, ok := step.Config["action_type"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid action_type in step config")
	}

	switch actionType {
	case models.ActionTypeRconCommand:
		return wm.executeRconAction(context, step)
	case models.ActionTypeAdminBroadcast:
		return wm.executeAdminBroadcastAction(context, step)
	case models.ActionTypeChatMessage:
		return wm.executeChatMessageAction(context, step)
	case models.ActionTypeKickPlayer:
		return wm.executeKickPlayerAction(context, step)
	case models.ActionTypeBanPlayer:
		return wm.executeBanPlayerAction(context, step)
	case models.ActionTypeWarnPlayer:
		return wm.executeWarnPlayerAction(context, step)
	case models.ActionTypeHTTPRequest:
		return wm.executeHTTPRequestAction(context, step)
	case models.ActionTypeWebhook:
		return wm.executeWebhookAction(context, step)
	case models.ActionTypeDiscordMessage:
		return wm.executeDiscordMessageAction(context, step)
	case models.ActionTypeLogMessage:
		return wm.executeLogAction(context, step)
	case models.ActionTypeSetVariable:
		return wm.executeSetVariableAction(context, step)
	case models.ActionTypeLuaScript:
		return wm.executeLuaAction(context, step)
	default:
		return fmt.Errorf("unsupported action type: %s", actionType)
	}
}

// executeRconAction executes an RCON command action
func (wm *WorkflowManager) executeRconAction(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	command, ok := step.Config["command"].(string)
	if !ok {
		return fmt.Errorf("missing command in RCON action config")
	}

	// Replace variables in command
	command = wm.replaceVariablesWithContext(command, context.Variables, context.TriggerEvent, context.Metadata)

	log.Info().
		Str("execution_id", context.ExecutionID.String()).
		Str("server_id", context.ServerID.String()).
		Str("command", command).
		Msg("Executing RCON command")

	// Execute RCON command
	response, err := wm.rconManager.ExecuteCommand(context.ServerID, command)
	if err != nil {
		return fmt.Errorf("failed to execute RCON command: %w", err)
	}

	// Store response in step results
	context.StepResults[step.ID] = map[string]interface{}{
		"command":  command,
		"response": response,
	}

	return nil
}

// executeLogAction executes a log message action
func (wm *WorkflowManager) executeLogAction(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	message, ok := step.Config["message"].(string)
	if !ok {
		return fmt.Errorf("missing message in log action config")
	}

	// Replace variables in message
	message = wm.replaceVariablesWithContext(message, context.Variables, context.TriggerEvent, context.Metadata)

	level, _ := step.Config["level"].(string)
	if level == "" {
		level = "info"
	}

	// Normalize level to uppercase
	level = strings.ToUpper(level)

	// Use the new ClickHouse logging function
	wm.logWorkflowMessage(context, step, level, message)

	return nil
}

// executeSetVariableAction executes a set variable action
func (wm *WorkflowManager) executeSetVariableAction(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	variableName, ok := step.Config["variable_name"].(string)
	if !ok {
		return fmt.Errorf("missing variable_name in set variable action config")
	}

	variableValue := step.Config["variable_value"]
	if variableValue == nil {
		return fmt.Errorf("missing variable_value in set variable action config")
	}

	// Set the variable in context
	context.Variables[variableName] = variableValue

	log.Debug().
		Str("execution_id", context.ExecutionID.String()).
		Str("variable_name", variableName).
		Interface("variable_value", variableValue).
		Msg("Set workflow variable")

	return nil
}

// executeAdminBroadcastAction executes an admin broadcast action
func (wm *WorkflowManager) executeAdminBroadcastAction(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	message, ok := step.Config["message"].(string)
	if !ok {
		return fmt.Errorf("missing message in admin broadcast action config")
	}

	// Replace variables in message
	message = wm.replaceVariablesWithContext(message, context.Variables, context.TriggerEvent, context.Metadata)

	log.Debug().
		Str("execution_id", context.ExecutionID.String()).
		Str("server_id", context.ServerID.String()).
		Str("message", message).
		Msg("Executing admin broadcast")

	// Execute admin broadcast command
	command := fmt.Sprintf("AdminBroadcast %s", message)
	response, err := wm.rconManager.ExecuteCommand(context.ServerID, command)
	if err != nil {
		return fmt.Errorf("failed to execute admin broadcast: %w", err)
	}

	// Store response in step results
	context.StepResults[step.ID] = map[string]interface{}{
		"command":  command,
		"message":  message,
		"response": response,
	}

	return nil
}

// executeChatMessageAction executes a chat message action
func (wm *WorkflowManager) executeChatMessageAction(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	message, ok := step.Config["message"].(string)
	if !ok {
		return fmt.Errorf("missing message in chat message action config")
	}

	targetPlayer, ok := step.Config["target_player"].(string)
	if !ok {
		return fmt.Errorf("missing target_player in chat message action config")
	}

	// Replace variables in message and target player
	message = wm.replaceVariablesWithContext(message, context.Variables, context.TriggerEvent, context.Metadata)
	targetPlayer = wm.replaceVariablesWithContext(targetPlayer, context.Variables, context.TriggerEvent, context.Metadata)

	log.Info().
		Str("execution_id", context.ExecutionID.String()).
		Str("server_id", context.ServerID.String()).
		Str("target_player", targetPlayer).
		Str("message", message).
		Msg("Sending chat message to player")

	// Execute chat message command
	command := fmt.Sprintf("AdminChatMessage \"%s\" %s", targetPlayer, message)
	response, err := wm.rconManager.ExecuteCommand(context.ServerID, command)
	if err != nil {
		return fmt.Errorf("failed to send chat message: %w", err)
	}

	// Store response in step results
	context.StepResults[step.ID] = map[string]interface{}{
		"command":       command,
		"target_player": targetPlayer,
		"message":       message,
		"response":      response,
	}

	return nil
}

// executeKickPlayerAction executes a kick player action
func (wm *WorkflowManager) executeKickPlayerAction(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	playerId, ok := step.Config["player_id"].(string)
	if !ok {
		return fmt.Errorf("missing player_id in kick player action config")
	}

	reason, _ := step.Config["reason"].(string)
	if reason == "" {
		reason = "Kicked by workflow"
	}

	// Replace variables in player ID and reason
	playerId = wm.replaceVariablesWithContext(playerId, context.Variables, context.TriggerEvent, context.Metadata)
	reason = wm.replaceVariablesWithContext(reason, context.Variables, context.TriggerEvent, context.Metadata)

	log.Info().
		Str("execution_id", context.ExecutionID.String()).
		Str("server_id", context.ServerID.String()).
		Str("player_id", playerId).
		Str("reason", reason).
		Msg("Kicking player")

	// Execute kick command
	command := fmt.Sprintf("AdminKick \"%s\" %s", playerId, reason)
	response, err := wm.rconManager.ExecuteCommand(context.ServerID, command)
	if err != nil {
		return fmt.Errorf("failed to kick player: %w", err)
	}

	// Store response in step results
	context.StepResults[step.ID] = map[string]interface{}{
		"command":   command,
		"player_id": playerId,
		"reason":    reason,
		"response":  response,
	}

	return nil
}

// executeBanPlayerAction executes a ban player action
func (wm *WorkflowManager) executeBanPlayerAction(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	playerId, ok := step.Config["player_id"].(string)
	if !ok {
		return fmt.Errorf("missing player_id in ban player action config")
	}

	duration, ok := step.Config["duration"].(float64)
	if !ok {
		return fmt.Errorf("missing duration in ban player action config")
	}

	reason, _ := step.Config["reason"].(string)
	if reason == "" {
		reason = "Banned by workflow"
	}

	// Replace variables in player ID and reason
	playerId = wm.replaceVariablesWithContext(playerId, context.Variables, context.TriggerEvent, context.Metadata)
	reason = wm.replaceVariablesWithContext(reason, context.Variables, context.TriggerEvent, context.Metadata)

	log.Info().
		Str("execution_id", context.ExecutionID.String()).
		Str("server_id", context.ServerID.String()).
		Str("player_id", playerId).
		Float64("duration", duration).
		Str("reason", reason).
		Msg("Banning player")

	// Execute ban command (duration in days)
	command := fmt.Sprintf("AdminBan \"%s\" %.0f %s", playerId, duration, reason)
	response, err := wm.rconManager.ExecuteCommand(context.ServerID, command)
	if err != nil {
		return fmt.Errorf("failed to ban player: %w", err)
	}

	// Store response in step results
	context.StepResults[step.ID] = map[string]interface{}{
		"command":   command,
		"player_id": playerId,
		"duration":  duration,
		"reason":    reason,
		"response":  response,
	}

	return nil
}

// executeWarnPlayerAction executes a warn player action
func (wm *WorkflowManager) executeWarnPlayerAction(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	playerId, ok := step.Config["player_id"].(string)
	if !ok {
		return fmt.Errorf("missing player_id in warn player action config")
	}

	message, ok := step.Config["message"].(string)
	if !ok {
		return fmt.Errorf("missing message in warn player action config")
	}

	// Replace variables in player ID and message
	playerId = wm.replaceVariablesWithContext(playerId, context.Variables, context.TriggerEvent, context.Metadata)
	message = wm.replaceVariablesWithContext(message, context.Variables, context.TriggerEvent, context.Metadata)

	log.Info().
		Str("execution_id", context.ExecutionID.String()).
		Str("server_id", context.ServerID.String()).
		Str("player_id", playerId).
		Str("message", message).
		Msg("Warning player")

	// Execute warn command
	command := fmt.Sprintf("AdminWarn \"%s\" %s", playerId, message)
	response, err := wm.rconManager.ExecuteCommand(context.ServerID, command)
	if err != nil {
		return fmt.Errorf("failed to warn player: %w", err)
	}

	// Store response in step results
	context.StepResults[step.ID] = map[string]interface{}{
		"command":   command,
		"player_id": playerId,
		"message":   message,
		"response":  response,
	}

	return nil
}

// executeHTTPRequestAction executes an HTTP request action
func (wm *WorkflowManager) executeHTTPRequestAction(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	url, ok := step.Config["url"].(string)
	if !ok {
		return fmt.Errorf("missing url in HTTP request action config")
	}

	method, _ := step.Config["method"].(string)
	if method == "" {
		method = "GET"
	}

	// Replace variables in URL
	url = wm.replaceVariablesWithContext(url, context.Variables, context.TriggerEvent, context.Metadata)

	log.Info().
		Str("execution_id", context.ExecutionID.String()).
		Str("url", url).
		Str("method", method).
		Msg("Executing HTTP request")

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create request
	var body io.Reader
	if bodyData, exists := step.Config["body"]; exists && bodyData != nil {
		if bodyStr, ok := bodyData.(string); ok {
			bodyStr = wm.replaceVariablesWithContext(bodyStr, context.Variables, context.TriggerEvent, context.Metadata)
			body = strings.NewReader(bodyStr)
		}
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add headers
	if headers, exists := step.Config["headers"]; exists {
		if headersMap, ok := headers.(map[string]interface{}); ok {
			for key, value := range headersMap {
				if valueStr, ok := value.(string); ok {
					valueStr = wm.replaceVariablesWithContext(valueStr, context.Variables, context.TriggerEvent, context.Metadata)
					req.Header.Set(key, valueStr)
				}
			}
		}
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Store response in step results
	context.StepResults[step.ID] = map[string]interface{}{
		"url":              url,
		"method":           method,
		"status_code":      resp.StatusCode,
		"response_body":    string(responseBody),
		"response_headers": resp.Header,
	}

	// Check if we should fail on non-2xx status codes
	if failOnError, exists := step.Config["fail_on_error"]; exists {
		if failOnErrorBool, ok := failOnError.(bool); ok && failOnErrorBool {
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(responseBody))
			}
		}
	}

	return nil
}

// executeWebhookAction executes a webhook action
func (wm *WorkflowManager) executeWebhookAction(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	url, ok := step.Config["url"].(string)
	if !ok {
		return fmt.Errorf("missing url in webhook action config")
	}

	// Replace variables in URL
	url = wm.replaceVariablesWithContext(url, context.Variables, context.TriggerEvent, context.Metadata)

	log.Info().
		Str("execution_id", context.ExecutionID.String()).
		Str("url", url).
		Msg("Executing webhook")

	// Create payload
	payload := map[string]interface{}{
		"workflow_id":   context.WorkflowID.String(),
		"execution_id":  context.ExecutionID.String(),
		"server_id":     context.ServerID.String(),
		"trigger_event": context.TriggerEvent,
		"variables":     context.Variables,
		"metadata":      context.Metadata,
		"timestamp":     time.Now().Unix(),
	}

	// Add custom payload data if specified
	if customPayload, exists := step.Config["payload"]; exists {
		if customPayloadMap, ok := customPayload.(map[string]interface{}); ok {
			for key, value := range customPayloadMap {
				if valueStr, ok := value.(string); ok {
					value = wm.replaceVariablesWithContext(valueStr, context.Variables, context.TriggerEvent, context.Metadata)
				}
				payload[key] = value
			}
		}
	}

	// Convert payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create POST request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add custom headers if specified
	if headers, exists := step.Config["headers"]; exists {
		if headersMap, ok := headers.(map[string]interface{}); ok {
			for key, value := range headersMap {
				if valueStr, ok := value.(string); ok {
					valueStr = wm.replaceVariablesWithContext(valueStr, context.Variables, context.TriggerEvent, context.Metadata)
					req.Header.Set(key, valueStr)
				}
			}
		}
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute webhook: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read webhook response: %w", err)
	}

	// Store response in step results
	context.StepResults[step.ID] = map[string]interface{}{
		"url":              url,
		"payload":          payload,
		"status_code":      resp.StatusCode,
		"response_body":    string(responseBody),
		"response_headers": resp.Header,
	}

	return nil
}

// executeDiscordMessageAction executes a Discord message action
func (wm *WorkflowManager) executeDiscordMessageAction(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	webhookUrl, ok := step.Config["webhook_url"].(string)
	if !ok {
		return fmt.Errorf("missing webhook_url in Discord message action config")
	}

	message, ok := step.Config["message"].(string)
	if !ok {
		return fmt.Errorf("missing message in Discord message action config")
	}

	// Replace variables in webhook URL and message
	webhookUrl = wm.replaceVariablesWithContext(webhookUrl, context.Variables, context.TriggerEvent, context.Metadata)
	message = wm.replaceVariablesWithContext(message, context.Variables, context.TriggerEvent, context.Metadata)

	log.Info().
		Str("execution_id", context.ExecutionID.String()).
		Str("message", message).
		Msg("Sending Discord message")

	// Create Discord webhook payload
	payload := map[string]interface{}{
		"content": message,
	}

	// Add optional fields
	if username, exists := step.Config["username"]; exists {
		if usernameStr, ok := username.(string); ok {
			usernameStr = wm.replaceVariablesWithContext(usernameStr, context.Variables, context.TriggerEvent, context.Metadata)
			payload["username"] = usernameStr
		}
	}

	if avatarUrl, exists := step.Config["avatar_url"]; exists {
		if avatarUrlStr, ok := avatarUrl.(string); ok {
			avatarUrlStr = wm.replaceVariablesWithContext(avatarUrlStr, context.Variables, context.TriggerEvent, context.Metadata)
			payload["avatar_url"] = avatarUrlStr
		}
	}

	// Convert payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Discord payload: %w", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create POST request
	req, err := http.NewRequest("POST", webhookUrl, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create Discord request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Discord message: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for debugging
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read Discord response body")
	}

	// Store response in step results
	context.StepResults[step.ID] = map[string]interface{}{
		"webhook_url":   webhookUrl,
		"message":       message,
		"payload":       payload,
		"status_code":   resp.StatusCode,
		"response_body": string(responseBody),
	}

	// Discord webhooks return 204 No Content on success
	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		return fmt.Errorf("discord webhook failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	return nil
}

// executeConditionStep executes a condition step
func (wm *WorkflowManager) executeConditionStep(context *models.WorkflowExecutionContext, step *models.WorkflowStep, workflow *models.ServerWorkflow) error {
	// Get the conditions from step config
	conditionsInterface, ok := step.Config["conditions"]
	if !ok {
		return fmt.Errorf("missing conditions in condition step config")
	}

	// Convert to slice of conditions
	var conditions []models.WorkflowCondition
	if conditionsBytes, err := json.Marshal(conditionsInterface); err != nil {
		return fmt.Errorf("failed to marshal conditions: %w", err)
	} else if err := json.Unmarshal(conditionsBytes, &conditions); err != nil {
		return fmt.Errorf("failed to unmarshal conditions: %w", err)
	}

	// Get the logic operator (AND/OR)
	logic, _ := step.Config["logic"].(string)
	if logic == "" {
		logic = "AND" // Default to AND logic
	}

	// Create data context for condition evaluation
	data := make(map[string]interface{})

	// Add variables
	for k, v := range context.Variables {
		data[k] = v
	}

	// Add trigger_event as an object
	if context.TriggerEvent != nil {
		data["trigger_event"] = context.TriggerEvent
	}

	// Add metadata as an object
	if context.Metadata != nil {
		data["metadata"] = context.Metadata
	}

	// Add step results
	if context.StepResults != nil {
		data["step_results"] = context.StepResults
	}

	// Evaluate conditions based on logic
	var result bool
	switch strings.ToUpper(logic) {
	case "AND":
		result = wm.evaluateConditions(conditions, data)
	case "OR":
		result = wm.evaluateConditionsOr(conditions, data)
	default:
		return fmt.Errorf("unsupported logic operator: %s", logic)
	}

	log.Debug().
		Str("execution_id", context.ExecutionID.String()).
		Str("step_id", step.ID).
		Str("logic", logic).
		Bool("result", result).
		Int("condition_count", len(conditions)).
		Msg("Condition step evaluation completed")

	// Store the result in step results
	context.StepResults[step.ID] = map[string]interface{}{
		"condition_result": result,
		"logic":            logic,
		"conditions":       conditions,
	}

	// Handle the condition result
	if result {
		// Condition passed - execute next steps if defined
		if len(step.NextSteps) > 0 {
			log.Debug().
				Str("execution_id", context.ExecutionID.String()).
				Str("step_id", step.ID).
				Strs("next_steps", step.NextSteps).
				Msg("Condition passed, executing next steps")

			// Execute each next step
			for _, nextStepID := range step.NextSteps {
				err := wm.executeSpecificStep(context, workflow, nextStepID)
				if err != nil {
					log.Error().
						Err(err).
						Str("execution_id", context.ExecutionID.String()).
						Str("step_id", step.ID).
						Str("next_step_id", nextStepID).
						Msg("Failed to execute next step")

					// Check if we should fail or continue
					continueOnError, _ := step.Config["continue_on_next_step_error"].(bool)
					if !continueOnError {
						return fmt.Errorf("next step %s failed: %w", nextStepID, err)
					}
				}
			}
		}
	} else {
		// Condition failed
		log.Debug().
			Str("execution_id", context.ExecutionID.String()).
			Str("step_id", step.ID).
			Msg("Condition failed")

		// Check if we should skip or fail
		skipOnFalse, _ := step.Config["skip_on_false"].(bool)
		if !skipOnFalse {
			return fmt.Errorf("condition step failed: conditions evaluated to false")
		}
	}

	return nil
}

// evaluateConditionsOr evaluates conditions with OR logic
func (wm *WorkflowManager) evaluateConditionsOr(conditions []models.WorkflowCondition, eventData map[string]interface{}) bool {
	if len(conditions) == 0 {
		return true // No conditions means always pass
	}

	// OR logic - at least one condition must be true
	for _, condition := range conditions {
		if wm.evaluateCondition(condition, eventData) {
			return true
		}
	}

	return false
}

// executeVariableStep executes a variable step
func (wm *WorkflowManager) executeVariableStep(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	// Get the operation type
	operation, ok := step.Config["operation"].(string)
	if !ok {
		return fmt.Errorf("missing operation in variable step config")
	}

	switch operation {
	case "set":
		return wm.executeVariableSet(context, step)
	case "increment":
		return wm.executeVariableIncrement(context, step)
	case "decrement":
		return wm.executeVariableDecrement(context, step)
	case "append":
		return wm.executeVariableAppend(context, step)
	case "prepend":
		return wm.executeVariablePrepend(context, step)
	case "delete":
		return wm.executeVariableDelete(context, step)
	case "copy":
		return wm.executeVariableCopy(context, step)
	case "transform":
		return wm.executeVariableTransform(context, step)
	default:
		return fmt.Errorf("unsupported variable operation: %s", operation)
	}
}

// executeVariableSet sets a variable value
func (wm *WorkflowManager) executeVariableSet(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	variableName, ok := step.Config["variable_name"].(string)
	if !ok {
		return fmt.Errorf("missing variable_name in variable set config")
	}

	var variableValue interface{}

	// Check if value is provided directly
	if value, exists := step.Config["value"]; exists {
		variableValue = value
	} else if sourceField, exists := step.Config["source_field"].(string); exists {
		// Get value from another field using dot notation
		data := wm.createDataContext(context)
		variableValue = wm.getFieldValue(sourceField, data)
	} else {
		return fmt.Errorf("missing value or source_field in variable set config")
	}

	// Handle expression evaluation
	if expression, exists := step.Config["expression"].(string); exists {
		evaluatedValue, err := wm.evaluateExpression(expression, context)
		if err != nil {
			return fmt.Errorf("failed to evaluate expression: %w", err)
		}
		variableValue = evaluatedValue
	}

	// Set the variable
	context.Variables[variableName] = variableValue

	log.Debug().
		Str("execution_id", context.ExecutionID.String()).
		Str("variable_name", variableName).
		Interface("variable_value", variableValue).
		Msg("Set workflow variable")

	// Store result
	context.StepResults[step.ID] = map[string]interface{}{
		"operation":      "set",
		"variable_name":  variableName,
		"variable_value": variableValue,
	}

	return nil
}

// executeVariableIncrement increments a numeric variable
func (wm *WorkflowManager) executeVariableIncrement(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	variableName, ok := step.Config["variable_name"].(string)
	if !ok {
		return fmt.Errorf("missing variable_name in variable increment config")
	}

	increment := 1.0 // Default increment
	if inc, exists := step.Config["increment"]; exists {
		if incFloat, ok := wm.toFloat64(inc); ok {
			increment = incFloat
		}
	}

	// Get current value
	currentValue, exists := context.Variables[variableName]
	if !exists {
		currentValue = 0.0 // Default to 0 if variable doesn't exist
	}

	currentFloat, ok := wm.toFloat64(currentValue)
	if !ok {
		return fmt.Errorf("variable %s is not numeric, cannot increment", variableName)
	}

	newValue := currentFloat + increment
	context.Variables[variableName] = newValue

	log.Debug().
		Str("execution_id", context.ExecutionID.String()).
		Str("variable_name", variableName).
		Float64("old_value", currentFloat).
		Float64("increment", increment).
		Float64("new_value", newValue).
		Msg("Incremented workflow variable")

	context.StepResults[step.ID] = map[string]interface{}{
		"operation":     "increment",
		"variable_name": variableName,
		"old_value":     currentFloat,
		"increment":     increment,
		"new_value":     newValue,
	}

	return nil
}

// executeVariableDecrement decrements a numeric variable
func (wm *WorkflowManager) executeVariableDecrement(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	variableName, ok := step.Config["variable_name"].(string)
	if !ok {
		return fmt.Errorf("missing variable_name in variable decrement config")
	}

	decrement := 1.0 // Default decrement
	if dec, exists := step.Config["decrement"]; exists {
		if decFloat, ok := wm.toFloat64(dec); ok {
			decrement = decFloat
		}
	}

	// Get current value
	currentValue, exists := context.Variables[variableName]
	if !exists {
		currentValue = 0.0 // Default to 0 if variable doesn't exist
	}

	currentFloat, ok := wm.toFloat64(currentValue)
	if !ok {
		return fmt.Errorf("variable %s is not numeric, cannot decrement", variableName)
	}

	newValue := currentFloat - decrement
	context.Variables[variableName] = newValue

	log.Debug().
		Str("execution_id", context.ExecutionID.String()).
		Str("variable_name", variableName).
		Float64("old_value", currentFloat).
		Float64("decrement", decrement).
		Float64("new_value", newValue).
		Msg("Decremented workflow variable")

	context.StepResults[step.ID] = map[string]interface{}{
		"operation":     "decrement",
		"variable_name": variableName,
		"old_value":     currentFloat,
		"decrement":     decrement,
		"new_value":     newValue,
	}

	return nil
}

// executeVariableAppend appends to a string or array variable
func (wm *WorkflowManager) executeVariableAppend(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	variableName, ok := step.Config["variable_name"].(string)
	if !ok {
		return fmt.Errorf("missing variable_name in variable append config")
	}

	appendValue, exists := step.Config["value"]
	if !exists {
		return fmt.Errorf("missing value in variable append config")
	}

	// Get current value
	currentValue, exists := context.Variables[variableName]
	if !exists {
		// Initialize as empty string or array based on append value type
		if _, ok := appendValue.(string); ok {
			currentValue = ""
		} else {
			currentValue = []interface{}{}
		}
	}

	var newValue interface{}
	switch cv := currentValue.(type) {
	case string:
		// String concatenation
		newValue = cv + fmt.Sprintf("%v", appendValue)
	case []interface{}:
		// Array append
		newValue = append(cv, appendValue)
	default:
		// Convert to string and append
		newValue = fmt.Sprintf("%v", currentValue) + fmt.Sprintf("%v", appendValue)
	}

	context.Variables[variableName] = newValue

	log.Debug().
		Str("execution_id", context.ExecutionID.String()).
		Str("variable_name", variableName).
		Interface("append_value", appendValue).
		Interface("new_value", newValue).
		Msg("Appended to workflow variable")

	context.StepResults[step.ID] = map[string]interface{}{
		"operation":     "append",
		"variable_name": variableName,
		"append_value":  appendValue,
		"new_value":     newValue,
	}

	return nil
}

// executeVariablePrepend prepends to a string variable
func (wm *WorkflowManager) executeVariablePrepend(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	variableName, ok := step.Config["variable_name"].(string)
	if !ok {
		return fmt.Errorf("missing variable_name in variable prepend config")
	}

	prependValue, exists := step.Config["value"]
	if !exists {
		return fmt.Errorf("missing value in variable prepend config")
	}

	// Get current value
	currentValue, exists := context.Variables[variableName]
	if !exists {
		currentValue = ""
	}

	// String prepend
	newValue := fmt.Sprintf("%v", prependValue) + fmt.Sprintf("%v", currentValue)
	context.Variables[variableName] = newValue

	log.Debug().
		Str("execution_id", context.ExecutionID.String()).
		Str("variable_name", variableName).
		Interface("prepend_value", prependValue).
		Interface("new_value", newValue).
		Msg("Prepended to workflow variable")

	context.StepResults[step.ID] = map[string]interface{}{
		"operation":     "prepend",
		"variable_name": variableName,
		"prepend_value": prependValue,
		"new_value":     newValue,
	}

	return nil
}

// executeVariableDelete deletes a variable
func (wm *WorkflowManager) executeVariableDelete(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	variableName, ok := step.Config["variable_name"].(string)
	if !ok {
		return fmt.Errorf("missing variable_name in variable delete config")
	}

	// Check if variable exists
	_, exists := context.Variables[variableName]

	// Delete the variable
	delete(context.Variables, variableName)

	log.Debug().
		Str("execution_id", context.ExecutionID.String()).
		Str("variable_name", variableName).
		Bool("existed", exists).
		Msg("Deleted workflow variable")

	context.StepResults[step.ID] = map[string]interface{}{
		"operation":     "delete",
		"variable_name": variableName,
		"existed":       exists,
	}

	return nil
}

// executeVariableCopy copies one variable to another
func (wm *WorkflowManager) executeVariableCopy(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	sourceVariable, ok := step.Config["source_variable"].(string)
	if !ok {
		return fmt.Errorf("missing source_variable in variable copy config")
	}

	targetVariable, ok := step.Config["target_variable"].(string)
	if !ok {
		return fmt.Errorf("missing target_variable in variable copy config")
	}

	// Get source value
	sourceValue, exists := context.Variables[sourceVariable]
	if !exists {
		return fmt.Errorf("source variable %s does not exist", sourceVariable)
	}

	// Copy to target
	context.Variables[targetVariable] = sourceValue

	log.Debug().
		Str("execution_id", context.ExecutionID.String()).
		Str("source_variable", sourceVariable).
		Str("target_variable", targetVariable).
		Interface("value", sourceValue).
		Msg("Copied workflow variable")

	context.StepResults[step.ID] = map[string]interface{}{
		"operation":       "copy",
		"source_variable": sourceVariable,
		"target_variable": targetVariable,
		"value":           sourceValue,
	}

	return nil
}

// executeVariableTransform applies a transformation to a variable
func (wm *WorkflowManager) executeVariableTransform(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	variableName, ok := step.Config["variable_name"].(string)
	if !ok {
		return fmt.Errorf("missing variable_name in variable transform config")
	}

	transformation, ok := step.Config["transformation"].(string)
	if !ok {
		return fmt.Errorf("missing transformation in variable transform config")
	}

	// Get current value
	currentValue, exists := context.Variables[variableName]
	if !exists {
		return fmt.Errorf("variable %s does not exist", variableName)
	}

	var newValue interface{}
	var err error

	switch transformation {
	case "uppercase":
		newValue = strings.ToUpper(fmt.Sprintf("%v", currentValue))
	case "lowercase":
		newValue = strings.ToLower(fmt.Sprintf("%v", currentValue))
	case "trim":
		newValue = strings.TrimSpace(fmt.Sprintf("%v", currentValue))
	case "length":
		newValue = len(fmt.Sprintf("%v", currentValue))
	case "reverse":
		str := fmt.Sprintf("%v", currentValue)
		runes := []rune(str)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		newValue = string(runes)
	case "json_encode":
		if jsonBytes, jsonErr := json.Marshal(currentValue); jsonErr == nil {
			newValue = string(jsonBytes)
		} else {
			err = fmt.Errorf("failed to JSON encode: %w", jsonErr)
		}
	case "json_decode":
		var decoded interface{}
		if jsonErr := json.Unmarshal([]byte(fmt.Sprintf("%v", currentValue)), &decoded); jsonErr == nil {
			newValue = decoded
		} else {
			err = fmt.Errorf("failed to JSON decode: %w", jsonErr)
		}
	default:
		return fmt.Errorf("unsupported transformation: %s", transformation)
	}

	if err != nil {
		return err
	}

	context.Variables[variableName] = newValue

	log.Debug().
		Str("execution_id", context.ExecutionID.String()).
		Str("variable_name", variableName).
		Str("transformation", transformation).
		Interface("old_value", currentValue).
		Interface("new_value", newValue).
		Msg("Transformed workflow variable")

	context.StepResults[step.ID] = map[string]interface{}{
		"operation":      "transform",
		"variable_name":  variableName,
		"transformation": transformation,
		"old_value":      currentValue,
		"new_value":      newValue,
	}

	return nil
}

// createDataContext creates a combined data context for field access
func (wm *WorkflowManager) createDataContext(context *models.WorkflowExecutionContext) map[string]interface{} {
	data := make(map[string]interface{})

	// Add variables
	for k, v := range context.Variables {
		data[k] = v
	}

	// Add trigger_event as an object
	if context.TriggerEvent != nil {
		data["trigger_event"] = context.TriggerEvent
	}

	// Add metadata as an object
	if context.Metadata != nil {
		data["metadata"] = context.Metadata
	}

	// Add step results
	if context.StepResults != nil {
		data["step_results"] = context.StepResults
	}

	return data
}

// evaluateExpression evaluates a simple expression (placeholder for future enhancement)
func (wm *WorkflowManager) evaluateExpression(expression string, context *models.WorkflowExecutionContext) (interface{}, error) {
	// This is a simple implementation - could be enhanced with a proper expression parser
	// For now, just replace variables in the expression
	result := wm.replaceVariablesWithContext(expression, context.Variables, context.TriggerEvent, context.Metadata)

	// Try to parse as number
	if num, err := strconv.ParseFloat(result, 64); err == nil {
		return num, nil
	}

	// Try to parse as boolean
	if result == "true" {
		return true, nil
	}
	if result == "false" {
		return false, nil
	}

	// Return as string
	return result, nil
}

// executeDelayStep executes a delay step
func (wm *WorkflowManager) executeDelayStep(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	delayMs, ok := step.Config["delay_ms"].(float64)
	if !ok {
		return fmt.Errorf("missing or invalid delay_ms in delay step config")
	}

	duration := time.Duration(delayMs) * time.Millisecond
	log.Debug().
		Str("execution_id", context.ExecutionID.String()).
		Dur("duration", duration).
		Msg("Executing delay step")

	time.Sleep(duration)
	return nil
}

// replaceVariablesWithContext replaces variable placeholders with access to trigger_event and metadata
func (wm *WorkflowManager) replaceVariablesWithContext(text string, variables map[string]interface{}, triggerEvent map[string]interface{}, metadata map[string]interface{}) string {
	// Use a simple regex to find ${variable_name} patterns
	result := text

	// Create a combined data map for easier access
	data := make(map[string]interface{})

	// Add variables directly
	for k, v := range variables {
		data[k] = v
	}

	// Add trigger_event as an object
	if triggerEvent != nil {
		data["trigger_event"] = triggerEvent
	}

	// Add metadata as an object
	if metadata != nil {
		data["metadata"] = metadata
	}

	// Simple replacement for ${variable} patterns
	i := 0
	for i < len(result) {
		start := strings.Index(result[i:], "${")
		if start == -1 {
			break
		}
		start += i

		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start

		// Extract the variable path (e.g., "trigger_event.player_name")
		varPath := result[start+2 : end]

		// Get the value using dot notation
		value := wm.getValueByPath(varPath, data)

		// Convert to string
		valueStr := ""
		if value != nil {
			valueStr = fmt.Sprintf("%v", value)
		}

		// Replace the placeholder
		result = result[:start] + valueStr + result[end+1:]
		i = start + len(valueStr)
	}

	return result
}

// getValueByPath extracts a value from nested map using dot notation
func (wm *WorkflowManager) getValueByPath(path string, data map[string]interface{}) interface{} {
	// Delegate to getFieldValue for consistent dot notation parsing
	return wm.getFieldValue(path, data)
}

// convertEventDataToMap converts EventData interface to map[string]interface{}
func (wm *WorkflowManager) convertEventDataToMap(eventData event_manager.EventData) map[string]interface{} {
	if eventData == nil {
		log.Debug().Msg("Event data is nil, returning empty map")
		return make(map[string]interface{})
	}

	// Use JSON marshaling/unmarshaling to convert the event data
	// This is a simple way to convert any struct to map[string]interface{}
	jsonBytes, err := json.Marshal(eventData)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal event data")
		return make(map[string]interface{})
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal event data to map")
		return make(map[string]interface{})
	}

	return result
}

// GetActiveWorkflows returns a list of currently active workflows
func (wm *WorkflowManager) GetActiveWorkflows() map[uuid.UUID]*models.ServerWorkflow {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()

	// Return a copy to avoid concurrent access issues
	result := make(map[uuid.UUID]*models.ServerWorkflow)
	for id, workflow := range wm.activeWorkflows {
		result[id] = workflow
	}
	return result
}

// DebugStatus returns debug information about the workflow manager
func (wm *WorkflowManager) DebugStatus() map[string]interface{} {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()

	wm.executionMutex.RLock()
	defer wm.executionMutex.RUnlock()

	workflowsByServer := make(map[string][]string)
	triggersByEventType := make(map[string][]string)

	for _, workflow := range wm.activeWorkflows {
		serverID := workflow.ServerID.String()
		if workflowsByServer[serverID] == nil {
			workflowsByServer[serverID] = []string{}
		}
		workflowsByServer[serverID] = append(workflowsByServer[serverID], workflow.Name)

		for _, trigger := range workflow.Definition.Triggers {
			if trigger.Enabled {
				if triggersByEventType[trigger.EventType] == nil {
					triggersByEventType[trigger.EventType] = []string{}
				}
				triggersByEventType[trigger.EventType] = append(triggersByEventType[trigger.EventType],
					fmt.Sprintf("%s:%s", workflow.Name, trigger.Name))
			}
		}
	}

	return map[string]interface{}{
		"is_running":               wm.isRunning,
		"active_workflows_count":   len(wm.activeWorkflows),
		"running_executions_count": len(wm.executionContext),
		"subscriber_id":            wm.subscriber.ID.String(),
		"workflows_by_server":      workflowsByServer,
		"triggers_by_event_type":   triggersByEventType,
	}
}

// GetRunningExecutions returns a list of currently running workflow executions
func (wm *WorkflowManager) GetRunningExecutions() map[uuid.UUID]*models.WorkflowExecutionContext {
	wm.executionMutex.RLock()
	defer wm.executionMutex.RUnlock()

	// Return a copy to avoid concurrent access issues
	result := make(map[uuid.UUID]*models.WorkflowExecutionContext)
	for id, context := range wm.executionContext {
		result[id] = context
	}
	return result
}

// logWorkflowStep logs a workflow step execution to ClickHouse
func (wm *WorkflowManager) logWorkflowStep(
	context *models.WorkflowExecutionContext,
	workflow *models.ServerWorkflow,
	stepName, stepType string,
	stepOrder uint32,
	stepStatus string,
	stepInput, stepOutput map[string]interface{},
	stepError *string,
	stepDurationMs uint32,
) {
	if wm.clickhouseClient == nil {
		return
	}

	executionLog := &models.WorkflowExecutionLog{
		ExecutionID:      context.ExecutionID,
		WorkflowID:       context.WorkflowID,
		ServerID:         context.ServerID,
		EventTime:        time.Now(),
		TriggerEventType: wm.getTriggerEventType(context.TriggerEvent),
		TriggerEventData: context.TriggerEvent,
		Status:           stepStatus,
		StepName:         stepName,
		StepType:         stepType,
		StepOrder:        stepOrder,
		StepStatus:       stepStatus,
		StepInput:        stepInput,
		StepOutput:       stepOutput,
		StepError:        stepError,
		StepDurationMs:   stepDurationMs,
		Variables:        context.Variables,
		Metadata: map[string]interface{}{
			"workflow_name": workflow.Name,
		},
	}

	if err := wm.clickhouseClient.LogWorkflowExecution(wm.ctx, executionLog); err != nil {
		log.Error().Err(err).
			Str("execution_id", context.ExecutionID.String()).
			Str("step_name", stepName).
			Msg("Failed to log workflow step to ClickHouse")
	}
}

// getTriggerEventType extracts the event type from trigger event data
func (wm *WorkflowManager) getTriggerEventType(triggerEvent map[string]interface{}) string {
	if eventType, ok := triggerEvent["event_type"].(string); ok {
		return eventType
	}
	if eventType, ok := triggerEvent["type"].(string); ok {
		return eventType
	}
	return "unknown"
}

// executeLuaStep executes a LUA script step
func (wm *WorkflowManager) executeLuaStep(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	script, ok := step.Config["script"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid script in step config")
	}

	return wm.executeLuaScript(context, step, script)
}

// executeLuaAction executes a LUA script action
func (wm *WorkflowManager) executeLuaAction(context *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	script, ok := step.Config["script"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid script in step config")
	}

	return wm.executeLuaScript(context, step, script)
}

// executeLuaScript executes a LUA script with access to workflow context
func (wm *WorkflowManager) executeLuaScript(workflowContext *models.WorkflowExecutionContext, step *models.WorkflowStep, script string) error {
	L := lua.NewState()
	defer L.Close()

	// Set timeout for script execution
	timeout := 30 * time.Second
	if timeoutConfig, ok := step.Config["timeout_seconds"].(float64); ok && timeoutConfig > 0 {
		timeout = time.Duration(timeoutConfig) * time.Second
	}

	// Create a context with timeout
	scriptCtx, cancel := context.WithTimeout(wm.ctx, timeout)
	defer cancel()

	// Set up the LUA environment with workflow data
	if err := wm.setupLuaEnvironment(L, workflowContext, step); err != nil {
		return fmt.Errorf("failed to setup LUA environment: %w", err)
	}

	// Execute the script in a goroutine to handle timeout
	scriptError := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				scriptError <- fmt.Errorf("LUA script panicked: %v", r)
			}
		}()

		if err := L.DoString(script); err != nil {
			scriptError <- fmt.Errorf("LUA script execution failed: %w", err)
		} else {
			scriptError <- nil
		}
	}()

	// Wait for script completion or timeout
	select {
	case err := <-scriptError:
		if err != nil {
			return err
		}
	case <-scriptCtx.Done():
		return fmt.Errorf("LUA script execution timed out after %v", timeout)
	}

	// Extract results from LUA state
	return wm.extractLuaResults(L, workflowContext, step)
}

// setupLuaEnvironment sets up the LUA environment with workflow context and utilities
func (wm *WorkflowManager) setupLuaEnvironment(L *lua.LState, workflowContext *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	// Create workflow context table
	workflowTable := L.NewTable()

	// Add execution context
	L.SetField(workflowTable, "execution_id", lua.LString(workflowContext.ExecutionID.String()))
	L.SetField(workflowTable, "workflow_id", lua.LString(workflowContext.WorkflowID.String()))
	L.SetField(workflowTable, "server_id", lua.LString(workflowContext.ServerID.String()))
	L.SetField(workflowTable, "step_id", lua.LString(step.ID))
	L.SetField(workflowTable, "step_name", lua.LString(step.Name))

	// Add variables
	variablesTable := L.NewTable()
	for key, value := range workflowContext.Variables {
		L.SetField(variablesTable, key, wm.convertToLuaValue(L, value))
	}
	L.SetField(workflowTable, "variables", variablesTable)

	// Add step results from previous steps
	stepResultsTable := L.NewTable()
	for stepID, result := range workflowContext.StepResults {
		L.SetField(stepResultsTable, stepID, wm.convertToLuaValue(L, result))
	}
	L.SetField(workflowTable, "step_results", stepResultsTable)

	// Add trigger event data
	triggerTable := L.NewTable()
	for key, value := range workflowContext.TriggerEvent {
		L.SetField(triggerTable, key, wm.convertToLuaValue(L, value))
	}
	L.SetField(workflowTable, "trigger_event", triggerTable)

	// Add metadata
	metadataTable := L.NewTable()
	for key, value := range workflowContext.Metadata {
		L.SetField(metadataTable, key, wm.convertToLuaValue(L, value))
	}
	L.SetField(workflowTable, "metadata", metadataTable)

	// Add step configuration
	configTable := L.NewTable()
	for key, value := range step.Config {
		if key != "script" && key != "timeout_seconds" { // Don't expose script and timeout to itself
			L.SetField(configTable, key, wm.convertToLuaValue(L, value))
		}
	}
	L.SetField(workflowTable, "config", configTable)

	// Set the workflow table as a global
	L.SetGlobal("workflow", workflowTable)

	// Add utility functions
	if err := wm.addLuaUtilityFunctions(L, workflowContext, step); err != nil {
		return err
	}

	// Create result table for script to populate
	resultTable := L.NewTable()
	L.SetGlobal("result", resultTable)

	return nil
}

// logWorkflowMessage logs a workflow message to ClickHouse and stdout
func (wm *WorkflowManager) logWorkflowMessage(workflowContext *models.WorkflowExecutionContext, step *models.WorkflowStep, level, message string) {
	// Log to stdout as well for real-time monitoring
	logger := log.With().
		Str("execution_id", workflowContext.ExecutionID.String()).
		Str("step_id", step.ID).
		Str("step_name", step.Name).
		Logger()

	switch level {
	case "DEBUG":
		logger.Debug().Msg(message)
	case "INFO":
		logger.Info().Msg(message)
	case "WARN":
		logger.Warn().Msg(message)
	case "ERROR":
		logger.Error().Msg(message)
	default:
		logger.Info().Msg(message)
	}

	// Log to ClickHouse if client is available
	if wm.clickhouseClient != nil {
		logMsg := &models.WorkflowLogMessage{
			ExecutionID: workflowContext.ExecutionID,
			WorkflowID:  workflowContext.WorkflowID,
			ServerID:    workflowContext.ServerID,
			StepID:      step.ID,
			StepName:    step.Name,
			LogTime:     time.Now(),
			LogLevel:    level,
			Message:     message,
			Variables:   workflowContext.Variables,
			Metadata:    workflowContext.Metadata,
		}

		if err := wm.clickhouseClient.LogWorkflowMessage(wm.ctx, logMsg); err != nil {
			log.Error().Err(err).
				Str("execution_id", workflowContext.ExecutionID.String()).
				Str("step_id", step.ID).
				Msg("Failed to log workflow message to ClickHouse")
		}
	}
}

// addLuaUtilityFunctions adds utility functions to the LUA environment
func (wm *WorkflowManager) addLuaUtilityFunctions(L *lua.LState, workflowContext *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	// log function (info level)
	L.SetGlobal("log", L.NewFunction(func(L *lua.LState) int {
		message := L.CheckString(1)
		wm.logWorkflowMessage(workflowContext, step, "INFO", message)
		return 0
	}))

	// log_debug function
	L.SetGlobal("log_debug", L.NewFunction(func(L *lua.LState) int {
		message := L.CheckString(1)
		wm.logWorkflowMessage(workflowContext, step, "DEBUG", message)
		return 0
	}))

	// log_warn function
	L.SetGlobal("log_warn", L.NewFunction(func(L *lua.LState) int {
		message := L.CheckString(1)
		wm.logWorkflowMessage(workflowContext, step, "WARN", message)
		return 0
	}))

	// log_error function
	L.SetGlobal("log_error", L.NewFunction(func(L *lua.LState) int {
		message := L.CheckString(1)
		wm.logWorkflowMessage(workflowContext, step, "ERROR", message)
		return 0
	}))

	// set_variable function
	L.SetGlobal("set_variable", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		value := L.Get(2)

		goValue := wm.convertFromLuaValue(value)
		workflowContext.Variables[name] = goValue

		log.Debug().
			Str("execution_id", workflowContext.ExecutionID.String()).
			Str("variable_name", name).
			Interface("value", goValue).
			Msg("LUA script set variable")

		return 0
	}))

	// get_variable function
	L.SetGlobal("get_variable", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		if value, exists := workflowContext.Variables[name]; exists {
			L.Push(wm.convertToLuaValue(L, value))
		} else {
			L.Push(lua.LNil)
		}
		return 1
	}))

	// safe_get function - safely get a value from a table, returning default if nil
	L.SetGlobal("safe_get", L.NewFunction(func(L *lua.LState) int {
		table := L.CheckTable(1)
		key := L.CheckString(2)
		defaultValue := L.Get(3) // Optional default value

		value := L.GetField(table, key)
		if value == lua.LNil && defaultValue != lua.LNil {
			L.Push(defaultValue)
		} else {
			L.Push(value)
		}
		return 1
	}))

	// to_string function - safely convert any value to string
	L.SetGlobal("to_string", L.NewFunction(func(L *lua.LState) int {
		value := L.Get(1)
		defaultStr := L.OptString(2, "")

		if value == lua.LNil {
			L.Push(lua.LString(defaultStr))
		} else {
			L.Push(lua.LString(value.String()))
		}
		return 1
	}))

	// json_encode function
	L.SetGlobal("json_encode", L.NewFunction(func(L *lua.LState) int {
		value := L.Get(1)
		goValue := wm.convertFromLuaValue(value)

		if jsonBytes, err := json.Marshal(goValue); err == nil {
			L.Push(lua.LString(string(jsonBytes)))
		} else {
			L.Push(lua.LNil)
		}
		return 1
	}))

	// json_decode function
	L.SetGlobal("json_decode", L.NewFunction(func(L *lua.LState) int {
		jsonStr := L.CheckString(1)

		var result interface{}
		if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
			L.Push(wm.convertToLuaValue(L, result))
		} else {
			L.Push(lua.LNil)
		}
		return 1
	}))

	// rcon_execute function - execute raw RCON command
	L.SetGlobal("rcon_execute", L.NewFunction(func(L *lua.LState) int {
		command := L.CheckString(1)

		// Replace variables in command using workflow context
		command = wm.replaceVariablesWithContext(command, workflowContext.Variables, workflowContext.TriggerEvent, workflowContext.Metadata)

		log.Debug().
			Str("execution_id", workflowContext.ExecutionID.String()).
			Str("server_id", workflowContext.ServerID.String()).
			Str("command", command).
			Msg("LUA script executing RCON command")

		// Execute RCON command
		response, err := wm.rconManager.ExecuteCommand(workflowContext.ServerID, command)
		if err != nil {
			log.Error().
				Err(err).
				Str("execution_id", workflowContext.ExecutionID.String()).
				Str("command", command).
				Msg("LUA RCON command failed")
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2 // Return nil response and error message
		}

		L.Push(lua.LString(response))
		L.Push(lua.LNil) // No error
		return 2         // Return response and error (nil if successful)
	}))

	// rcon_kick function - kick a player
	L.SetGlobal("rcon_kick", L.NewFunction(func(L *lua.LState) int {
		playerId := L.CheckString(1)
		reason := L.OptString(2, "Kicked by workflow")

		// Replace variables
		playerId = wm.replaceVariablesWithContext(playerId, workflowContext.Variables, workflowContext.TriggerEvent, workflowContext.Metadata)
		reason = wm.replaceVariablesWithContext(reason, workflowContext.Variables, workflowContext.TriggerEvent, workflowContext.Metadata)

		command := fmt.Sprintf("AdminKick \"%s\" %s", playerId, reason)

		log.Info().
			Str("execution_id", workflowContext.ExecutionID.String()).
			Str("player_id", playerId).
			Str("reason", reason).
			Msg("LUA script kicking player")

		response, err := wm.rconManager.ExecuteCommand(workflowContext.ServerID, command)
		if err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}

		L.Push(lua.LBool(true))
		L.Push(lua.LString(response))
		return 2 // Return success boolean and response
	}))

	// rcon_ban function - ban a player
	L.SetGlobal("rcon_ban", L.NewFunction(func(L *lua.LState) int {
		playerId := L.CheckString(1)
		duration := L.CheckNumber(2) // Duration in days
		reason := L.OptString(3, "Banned by workflow")

		// Replace variables
		playerId = wm.replaceVariablesWithContext(playerId, workflowContext.Variables, workflowContext.TriggerEvent, workflowContext.Metadata)
		reason = wm.replaceVariablesWithContext(reason, workflowContext.Variables, workflowContext.TriggerEvent, workflowContext.Metadata)

		command := fmt.Sprintf("AdminBan \"%s\" %.0f %s", playerId, float64(duration), reason)

		log.Info().
			Str("execution_id", workflowContext.ExecutionID.String()).
			Str("player_id", playerId).
			Float64("duration", float64(duration)).
			Str("reason", reason).
			Msg("LUA script banning player")

		response, err := wm.rconManager.ExecuteCommand(workflowContext.ServerID, command)
		if err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}

		L.Push(lua.LBool(true))
		L.Push(lua.LString(response))
		return 2 // Return success boolean and response
	}))

	// rcon_warn function - warn a player
	L.SetGlobal("rcon_warn", L.NewFunction(func(L *lua.LState) int {
		playerId := L.CheckString(1)
		message := L.CheckString(2)

		// Replace variables
		playerId = wm.replaceVariablesWithContext(playerId, workflowContext.Variables, workflowContext.TriggerEvent, workflowContext.Metadata)
		message = wm.replaceVariablesWithContext(message, workflowContext.Variables, workflowContext.TriggerEvent, workflowContext.Metadata)

		command := fmt.Sprintf("AdminWarn \"%s\" %s", playerId, message)

		log.Info().
			Str("execution_id", workflowContext.ExecutionID.String()).
			Str("player_id", playerId).
			Str("message", message).
			Msg("LUA script warning player")

		response, err := wm.rconManager.ExecuteCommand(workflowContext.ServerID, command)
		if err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}

		L.Push(lua.LBool(true))
		L.Push(lua.LString(response))
		return 2 // Return success boolean and response
	}))

	// rcon_broadcast function - send admin broadcast
	L.SetGlobal("rcon_broadcast", L.NewFunction(func(L *lua.LState) int {
		message := L.CheckString(1)

		// Replace variables
		message = wm.replaceVariablesWithContext(message, workflowContext.Variables, workflowContext.TriggerEvent, workflowContext.Metadata)

		command := fmt.Sprintf("AdminBroadcast %s", message)

		log.Info().
			Str("execution_id", workflowContext.ExecutionID.String()).
			Str("message", message).
			Msg("LUA script broadcasting message")

		response, err := wm.rconManager.ExecuteCommand(workflowContext.ServerID, command)
		if err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}

		L.Push(lua.LBool(true))
		L.Push(lua.LString(response))
		return 2 // Return success boolean and response
	}))

	// rcon_chat_message function - send chat message to specific player
	L.SetGlobal("rcon_chat_message", L.NewFunction(func(L *lua.LState) int {
		playerId := L.CheckString(1)
		message := L.CheckString(2)

		// Replace variables
		playerId = wm.replaceVariablesWithContext(playerId, workflowContext.Variables, workflowContext.TriggerEvent, workflowContext.Metadata)
		message = wm.replaceVariablesWithContext(message, workflowContext.Variables, workflowContext.TriggerEvent, workflowContext.Metadata)

		command := fmt.Sprintf("AdminChatMessage \"%s\" %s", playerId, message)

		log.Info().
			Str("execution_id", workflowContext.ExecutionID.String()).
			Str("player_id", playerId).
			Str("message", message).
			Msg("LUA script sending chat message to player")

		response, err := wm.rconManager.ExecuteCommand(workflowContext.ServerID, command)
		if err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}

		L.Push(lua.LBool(true))
		L.Push(lua.LString(response))
		return 2 // Return success boolean and response
	}))

	return nil
}

// convertToLuaValue converts a Go value to a LUA value
func (wm *WorkflowManager) convertToLuaValue(L *lua.LState, value interface{}) lua.LValue {
	if value == nil {
		return lua.LNil
	}

	switch v := value.(type) {
	case string:
		return lua.LString(v)
	case int:
		return lua.LNumber(v)
	case int64:
		return lua.LNumber(v)
	case float32:
		return lua.LNumber(v)
	case float64:
		return lua.LNumber(v)
	case bool:
		return lua.LBool(v)
	case map[string]interface{}:
		table := L.NewTable()
		for key, val := range v {
			L.SetField(table, key, wm.convertToLuaValue(L, val))
		}
		return table
	case []interface{}:
		table := L.NewTable()
		for i, val := range v {
			L.SetField(table, fmt.Sprintf("%d", i+1), wm.convertToLuaValue(L, val))
		}
		return table
	default:
		// Try to convert to JSON string as fallback
		if jsonBytes, err := json.Marshal(value); err == nil {
			return lua.LString(string(jsonBytes))
		}
		// If JSON marshaling fails, convert to string
		return lua.LString(fmt.Sprintf("%v", value))
	}
}

// convertFromLuaValue converts a LUA value to a Go value
func (wm *WorkflowManager) convertFromLuaValue(value lua.LValue) interface{} {
	switch v := value.(type) {
	case *lua.LNilType:
		return nil
	case lua.LString:
		return string(v)
	case lua.LNumber:
		return float64(v)
	case lua.LBool:
		return bool(v)
	case *lua.LTable:
		result := make(map[string]interface{})
		v.ForEach(func(key, val lua.LValue) {
			keyStr := key.String()
			result[keyStr] = wm.convertFromLuaValue(val)
		})
		return result
	default:
		return value.String()
	}
}

// extractLuaResults extracts results from the LUA execution and stores them in the workflow context
func (wm *WorkflowManager) extractLuaResults(L *lua.LState, workflowContext *models.WorkflowExecutionContext, step *models.WorkflowStep) error {
	// Get the result table
	resultTable := L.GetGlobal("result")
	if resultTable != lua.LNil {
		result := wm.convertFromLuaValue(resultTable)
		workflowContext.StepResults[step.ID] = result
	} else {
		// If no result table is set, create a simple success result
		workflowContext.StepResults[step.ID] = map[string]interface{}{
			"status":  "completed",
			"message": "LUA script executed successfully",
		}
	}

	return nil
}
