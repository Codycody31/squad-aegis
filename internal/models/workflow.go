package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ServerWorkflow represents a workflow definition
type ServerWorkflow struct {
	ID          uuid.UUID                `json:"id"`
	ServerID    uuid.UUID                `json:"server_id"`
	Name        string                   `json:"name"`
	Description *string                  `json:"description,omitempty"`
	Enabled     bool                     `json:"enabled"`
	Definition  WorkflowDefinition       `json:"definition"`
	CreatedBy   uuid.UUID                `json:"created_by"`
	CreatedAt   time.Time                `json:"created_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
	Variables   []ServerWorkflowVariable `json:"variables,omitempty"`
}

// WorkflowDefinition defines the structure of a workflow
type WorkflowDefinition struct {
	Version       string                 `json:"version"`   // Schema version for future compatibility
	Triggers      []WorkflowTrigger      `json:"triggers"`  // Events that start this workflow
	Variables     map[string]interface{} `json:"variables"` // Default workflow variables
	Steps         []WorkflowStep         `json:"steps"`     // Ordered list of steps to execute
	ErrorHandling WorkflowErrorHandling  `json:"error_handling,omitempty"`
}

// WorkflowTrigger defines what events trigger this workflow
type WorkflowTrigger struct {
	ID         string              `json:"id"`                   // Unique identifier for this trigger
	Name       string              `json:"name"`                 // Human-readable name
	EventType  string              `json:"event_type"`           // Event type from event_manager
	Conditions []WorkflowCondition `json:"conditions,omitempty"` // Optional conditions to filter events
	Enabled    bool                `json:"enabled"`              // Whether this trigger is active
}

// WorkflowStep represents a single step in a workflow
type WorkflowStep struct {
	ID        string                 `json:"id"`                   // Unique identifier for this step
	Name      string                 `json:"name"`                 // Human-readable name
	Type      string                 `json:"type"`                 // Step type: "condition", "action", "variable", "loop", "parallel"
	Enabled   bool                   `json:"enabled"`              // Whether this step should execute
	Config    map[string]interface{} `json:"config"`               // Step-specific configuration
	OnError   *WorkflowErrorAction   `json:"on_error,omitempty"`   // What to do if this step fails
	NextSteps []string               `json:"next_steps,omitempty"` // Next step IDs (for conditional flow)
}

// WorkflowCondition defines a condition for filtering or branching
type WorkflowCondition struct {
	Field    string      `json:"field"`    // Field to check (can use dot notation: "event.player_name")
	Operator string      `json:"operator"` // "equals", "not_equals", "contains", "regex", "greater_than", etc.
	Value    interface{} `json:"value"`    // Value to compare against
	Type     string      `json:"type"`     // "string", "number", "boolean", "regex"
}

// WorkflowErrorHandling defines how to handle errors in the workflow
type WorkflowErrorHandling struct {
	DefaultAction string         `json:"default_action"` // "continue", "stop", "retry"
	MaxRetries    int            `json:"max_retries"`
	RetryDelay    int            `json:"retry_delay_ms"`
	OnFailure     []WorkflowStep `json:"on_failure,omitempty"` // Steps to execute on failure
}

// WorkflowErrorAction defines what to do when a step fails
type WorkflowErrorAction struct {
	Action     string `json:"action"` // "continue", "stop", "retry", "goto"
	MaxRetries int    `json:"max_retries,omitempty"`
	RetryDelay int    `json:"retry_delay_ms,omitempty"`
	GotoStep   string `json:"goto_step,omitempty"` // Step ID to jump to
}

// ServerWorkflowExecution tracks workflow executions
type ServerWorkflowExecution struct {
	ID           uuid.UUID              `json:"id"`
	WorkflowID   uuid.UUID              `json:"workflow_id"`
	ExecutionID  uuid.UUID              `json:"execution_id"`  // Links to ClickHouse logs
	Status       string                 `json:"status"`        // "RUNNING", "COMPLETED", "FAILED", "CANCELLED"
	TriggerData  map[string]interface{} `json:"trigger_data"`  // Event data that triggered the workflow
	StartedAt    time.Time              `json:"started_at"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
	ErrorMessage *string                `json:"error_message,omitempty"`
}

// ServerWorkflowVariable stores dynamic variables for workflows
type ServerWorkflowVariable struct {
	ID          uuid.UUID              `json:"id"`
	WorkflowID  uuid.UUID              `json:"workflow_id"`
	Name        string                 `json:"name"`
	Value       map[string]interface{} `json:"value"`
	Description *string                `json:"description,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// WorkflowExecutionContext holds runtime data during execution
type WorkflowExecutionContext struct {
	ExecutionID  uuid.UUID              `json:"execution_id"`
	WorkflowID   uuid.UUID              `json:"workflow_id"`
	ServerID     uuid.UUID              `json:"server_id"`
	TriggerEvent map[string]interface{} `json:"trigger_event"`
	Metadata     map[string]interface{} `json:"metadata"`
	Variables    map[string]interface{} `json:"variables"`
	StepResults  map[string]interface{} `json:"step_results"`
	CurrentStep  string                 `json:"current_step"`
	StartedAt    time.Time              `json:"started_at"`
}

// Predefined step types
const (
	StepTypeCondition = "condition"
	StepTypeAction    = "action"
	StepTypeVariable  = "variable"
	StepTypeLoop      = "loop"
	StepTypeParallel  = "parallel"
	StepTypeDelay     = "delay"
	StepTypeLua       = "lua"
)

// Predefined action types
const (
	ActionTypeRconCommand    = "rcon_command"
	ActionTypeAdminBroadcast = "admin_broadcast"
	ActionTypeChatMessage    = "chat_message"
	ActionTypeKickPlayer     = "kick_player"
	ActionTypeBanPlayer      = "ban_player"
	ActionTypeWarnPlayer     = "warn_player"
	ActionTypeHTTPRequest    = "http_request"
	ActionTypeWebhook        = "webhook"
	ActionTypeDiscordMessage = "discord_message"
	ActionTypeLogMessage     = "log_message"
	ActionTypeSetVariable    = "set_variable"
	ActionTypeLuaScript      = "lua_script"
)

// Predefined condition operators
const (
	OperatorEquals         = "equals"
	OperatorNotEquals      = "not_equals"
	OperatorContains       = "contains"
	OperatorNotContains    = "not_contains"
	OperatorStartsWith     = "starts_with"
	OperatorEndsWith       = "ends_with"
	OperatorRegex          = "regex"
	OperatorGreaterThan    = "greater_than"
	OperatorLessThan       = "less_than"
	OperatorGreaterOrEqual = "greater_or_equal"
	OperatorLessOrEqual    = "less_or_equal"
	OperatorIn             = "in"
	OperatorNotIn          = "not_in"
)

// Request/Response types for API

type ServerWorkflowCreateRequest struct {
	Name        string             `json:"name" binding:"required"`
	Description *string            `json:"description,omitempty"`
	Enabled     bool               `json:"enabled"`
	Definition  WorkflowDefinition `json:"definition" binding:"required"`
}

type ServerWorkflowUpdateRequest struct {
	Name        *string             `json:"name,omitempty"`
	Description *string             `json:"description,omitempty"`
	Enabled     *bool               `json:"enabled,omitempty"`
	Definition  *WorkflowDefinition `json:"definition,omitempty"`
}

type ServerWorkflowVariableCreateRequest struct {
	Name        string                 `json:"name" binding:"required"`
	Value       map[string]interface{} `json:"value" binding:"required"`
	Description *string                `json:"description,omitempty"`
}

type ServerWorkflowVariableUpdateRequest struct {
	Name        *string                 `json:"name,omitempty"`
	Value       *map[string]interface{} `json:"value,omitempty"`
	Description *string                 `json:"description,omitempty"`
}

type WorkflowExecuteRequest struct {
	TriggerEvent map[string]interface{} `json:"trigger_event,omitempty"`
	Variables    map[string]interface{} `json:"variables,omitempty"`
}

// WorkflowExecutionLog represents a single log entry from ClickHouse
type WorkflowExecutionLog struct {
	ExecutionID      uuid.UUID              `json:"execution_id"`
	WorkflowID       uuid.UUID              `json:"workflow_id"`
	ServerID         uuid.UUID              `json:"server_id"`
	EventTime        time.Time              `json:"event_time"`
	TriggerEventType string                 `json:"trigger_event_type"`
	TriggerEventData map[string]interface{} `json:"trigger_event_data"`
	Status           string                 `json:"status"`
	StepName         string                 `json:"step_name"`
	StepType         string                 `json:"step_type"`
	StepOrder        uint32                 `json:"step_order"`
	StepStatus       string                 `json:"step_status"`
	StepInput        map[string]interface{} `json:"step_input"`
	StepOutput       map[string]interface{} `json:"step_output"`
	StepError        *string                `json:"step_error,omitempty"`
	StepDurationMs   uint32                 `json:"step_duration_ms"`
	Variables        map[string]interface{} `json:"variables"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// WorkflowExecutionSummary represents a workflow execution summary from ClickHouse
type WorkflowExecutionSummary struct {
	ExecutionID      uuid.UUID  `json:"execution_id"`
	WorkflowID       uuid.UUID  `json:"workflow_id"`
	ServerID         uuid.UUID  `json:"server_id"`
	WorkflowName     string     `json:"workflow_name"`
	StartedAt        time.Time  `json:"started_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	Status           string     `json:"status"`
	TriggerEventType string     `json:"trigger_event_type"`
	TotalSteps       uint32     `json:"total_steps"`
	CompletedSteps   uint32     `json:"completed_steps"`
	FailedSteps      uint32     `json:"failed_steps"`
	SkippedSteps     uint32     `json:"skipped_steps"`
	TotalDurationMs  uint32     `json:"total_duration_ms"`
	ErrorMessage     *string    `json:"error_message,omitempty"`
}

// WorkflowLogMessage represents a log message from a workflow execution
type WorkflowLogMessage struct {
	ExecutionID uuid.UUID              `json:"execution_id"`
	WorkflowID  uuid.UUID              `json:"workflow_id"`
	ServerID    uuid.UUID              `json:"server_id"`
	StepID      string                 `json:"step_id"`
	StepName    string                 `json:"step_name"`
	LogTime     time.Time              `json:"log_time"`
	LogLevel    string                 `json:"log_level"` // DEBUG, INFO, WARN, ERROR
	Message     string                 `json:"message"`   // The actual log message
	Variables   map[string]interface{} `json:"variables"` // Workflow variables at log time
	Metadata    map[string]interface{} `json:"metadata"`  // Additional context
}

// Helper methods

// MarshalDefinition converts WorkflowDefinition to JSON for database storage
func (w *ServerWorkflow) MarshalDefinition() ([]byte, error) {
	return json.Marshal(w.Definition)
}

// UnmarshalDefinition converts JSON to WorkflowDefinition from database
func (w *ServerWorkflow) UnmarshalDefinition(data []byte) error {
	return json.Unmarshal(data, &w.Definition)
}

// IsActive returns true if the workflow is enabled
func (w *ServerWorkflow) IsActive() bool {
	return w.Enabled
}

// GetTriggerByEventType returns the first trigger that matches the event type
func (w *ServerWorkflow) GetTriggerByEventType(eventType string) *WorkflowTrigger {
	for _, trigger := range w.Definition.Triggers {
		if trigger.EventType == eventType && trigger.Enabled {
			return &trigger
		}
	}
	return nil
}

// GetStepByID returns a step by its ID
func (w *ServerWorkflow) GetStepByID(stepID string) *WorkflowStep {
	for _, step := range w.Definition.Steps {
		if step.ID == stepID {
			return &step
		}
	}
	return nil
}
