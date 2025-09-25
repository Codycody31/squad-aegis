package workflow_manager

import (
	"testing"

	"go.codycody31.dev/squad-aegis/internal/models"
)

func TestGetFieldValue(t *testing.T) {
	// Create a mock workflow manager
	wm := &WorkflowManager{}

	// Test data with nested structure
	testData := map[string]interface{}{
		"player": map[string]interface{}{
			"name":    "TestPlayer",
			"steamid": "76561198000000000",
			"team": map[string]interface{}{
				"id":   1,
				"name": "US Army",
			},
		},
		"event": map[string]interface{}{
			"type": "player_connected",
			"data": map[string]interface{}{
				"message":   "Player connected to server",
				"timestamp": "2025-09-16T10:00:00Z",
			},
		},
		"simple_field": "simple_value",
		"number_field": 42,
	}

	tests := []struct {
		name     string
		field    string
		expected interface{}
	}{
		{
			name:     "Simple field access",
			field:    "simple_field",
			expected: "simple_value",
		},
		{
			name:     "Number field access",
			field:    "number_field",
			expected: 42,
		},
		{
			name:     "Nested field - player name",
			field:    "player.name",
			expected: "TestPlayer",
		},
		{
			name:     "Nested field - player steamid",
			field:    "player.steamid",
			expected: "76561198000000000",
		},
		{
			name:     "Deep nested field - team name",
			field:    "player.team.name",
			expected: "US Army",
		},
		{
			name:     "Deep nested field - team id",
			field:    "player.team.id",
			expected: 1,
		},
		{
			name:     "Event data message",
			field:    "event.data.message",
			expected: "Player connected to server",
		},
		{
			name:     "Non-existent field",
			field:    "nonexistent",
			expected: nil,
		},
		{
			name:     "Non-existent nested field",
			field:    "player.nonexistent",
			expected: nil,
		},
		{
			name:     "Non-existent deep nested field",
			field:    "player.team.nonexistent",
			expected: nil,
		},
		{
			name:     "Invalid path - accessing non-map",
			field:    "simple_field.invalid",
			expected: nil,
		},
		{
			name:     "Empty field",
			field:    "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wm.getFieldValue(tt.field, testData)
			if result != tt.expected {
				t.Errorf("getFieldValue(%q) = %v, expected %v", tt.field, result, tt.expected)
			}
		})
	}
}

func TestGetFieldValueWithNilData(t *testing.T) {
	wm := &WorkflowManager{}

	result := wm.getFieldValue("any.field", nil)
	if result != nil {
		t.Errorf("getFieldValue with nil data should return nil, got %v", result)
	}
}

func TestGetValueByPath(t *testing.T) {
	// Test that getValueByPath delegates to getFieldValue
	wm := &WorkflowManager{}

	testData := map[string]interface{}{
		"nested": map[string]interface{}{
			"value": "test_value",
		},
	}

	result := wm.getValueByPath("nested.value", testData)
	expected := "test_value"

	if result != expected {
		t.Errorf("getValueByPath('nested.value') = %v, expected %v", result, expected)
	}
}

func TestEvaluateCondition(t *testing.T) {
	wm := &WorkflowManager{}

	testData := map[string]interface{}{
		"player": map[string]interface{}{
			"name":  "TestPlayer",
			"level": 5,
			"score": 100.5,
		},
		"tags":    []interface{}{"admin", "vip", "moderator"},
		"message": "Hello World",
	}

	tests := []struct {
		name      string
		condition models.WorkflowCondition
		expected  bool
	}{
		{
			name: "String equals",
			condition: models.WorkflowCondition{
				Field:    "player.name",
				Operator: models.OperatorEquals,
				Value:    "TestPlayer",
			},
			expected: true,
		},
		{
			name: "String not equals",
			condition: models.WorkflowCondition{
				Field:    "player.name",
				Operator: models.OperatorNotEquals,
				Value:    "OtherPlayer",
			},
			expected: true,
		},
		{
			name: "String contains",
			condition: models.WorkflowCondition{
				Field:    "message",
				Operator: models.OperatorContains,
				Value:    "World",
			},
			expected: true,
		},
		{
			name: "String starts with",
			condition: models.WorkflowCondition{
				Field:    "message",
				Operator: models.OperatorStartsWith,
				Value:    "Hello",
			},
			expected: true,
		},
		{
			name: "String ends with",
			condition: models.WorkflowCondition{
				Field:    "message",
				Operator: models.OperatorEndsWith,
				Value:    "World",
			},
			expected: true,
		},
		{
			name: "Number greater than",
			condition: models.WorkflowCondition{
				Field:    "player.level",
				Operator: models.OperatorGreaterThan,
				Value:    3,
			},
			expected: true,
		},
		{
			name: "Number less than",
			condition: models.WorkflowCondition{
				Field:    "player.level",
				Operator: models.OperatorLessThan,
				Value:    10,
			},
			expected: true,
		},
		{
			name: "Float greater or equal",
			condition: models.WorkflowCondition{
				Field:    "player.score",
				Operator: models.OperatorGreaterOrEqual,
				Value:    100.5,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wm.evaluateCondition(tt.condition, testData)
			if result != tt.expected {
				t.Errorf("evaluateCondition(%+v) = %v, expected %v", tt.condition, result, tt.expected)
			}
		})
	}
}

func TestCompareNumbers(t *testing.T) {
	wm := &WorkflowManager{}

	tests := []struct {
		name      string
		field     interface{}
		condition interface{}
		operator  string
		expected  bool
	}{
		{"int greater than", 10, 5, ">", true},
		{"int less than", 5, 10, "<", true},
		{"float greater equal", 10.5, 10.5, ">=", true},
		{"float less equal", 5.2, 10.8, "<=", true},
		{"string number comparison", "15", 10, ">", true},
		{"invalid comparison", "abc", 10, ">", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wm.compareNumbers(tt.field, tt.condition, tt.operator)
			if result != tt.expected {
				t.Errorf("compareNumbers(%v, %v, %s) = %v, expected %v",
					tt.field, tt.condition, tt.operator, result, tt.expected)
			}
		})
	}
}

func TestEvaluateInOperator(t *testing.T) {
	wm := &WorkflowManager{}

	tests := []struct {
		name      string
		field     interface{}
		condition interface{}
		expected  bool
	}{
		{"string in slice", "admin", []interface{}{"admin", "user", "guest"}, true},
		{"string not in slice", "banned", []interface{}{"admin", "user", "guest"}, false},
		{"string in string array", "vip", []string{"admin", "vip", "moderator"}, true},
		{"string in comma separated", "red", "red,green,blue", true},
		{"single value match", "test", "test", true},
		{"single value no match", "test", "other", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wm.evaluateInOperator(tt.field, tt.condition)
			if result != tt.expected {
				t.Errorf("evaluateInOperator(%v, %v) = %v, expected %v",
					tt.field, tt.condition, result, tt.expected)
			}
		})
	}
}

func TestEvaluateRegexOperator(t *testing.T) {
	wm := &WorkflowManager{}

	tests := []struct {
		name     string
		field    interface{}
		pattern  interface{}
		expected bool
	}{
		{"simple pattern match", "hello123", `hello\d+`, true},
		{"email pattern", "user@example.com", `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`, true},
		{"no match", "hello", `\d+`, false},
		{"invalid regex", "test", `[`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wm.evaluateRegexOperator(tt.field, tt.pattern)
			if result != tt.expected {
				t.Errorf("evaluateRegexOperator(%v, %v) = %v, expected %v",
					tt.field, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestToFloat64(t *testing.T) {
	wm := &WorkflowManager{}

	tests := []struct {
		name     string
		input    interface{}
		expected float64
		ok       bool
	}{
		{"float64", 10.5, 10.5, true},
		{"float32", float32(5.2), float64(float32(5.2)), true},
		{"int", 42, 42.0, true},
		{"int64", int64(100), 100.0, true},
		{"string number", "15.5", 15.5, true},
		{"string non-number", "abc", 0, false},
		{"nil", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := wm.toFloat64(tt.input)
			if ok != tt.ok || (ok && result != tt.expected) {
				t.Errorf("toFloat64(%v) = %v, %v, expected %v, %v",
					tt.input, result, ok, tt.expected, tt.ok)
			}
		})
	}
}
