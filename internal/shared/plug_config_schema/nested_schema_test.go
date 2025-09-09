package plug_config_schema

import (
	"testing"
)

func TestNestedObjectSchema(t *testing.T) {
	// Create a schema with nested objects similar to the command scheduler
	commandFields := []ConfigField{
		NewStringField("name", "Command name", true, ""),
		NewStringField("command", "RCON command", true, ""),
		NewBoolField("enabled", "Is enabled", false, true),
		NewIntField("interval", "Interval in seconds", false, 600),
		NewBoolField("on_new_game", "Run on new game", false, false),
	}

	schema := ConfigSchema{
		Fields: []ConfigField{
			NewArrayObjectField(
				"commands",
				"List of commands",
				true,
				commandFields,
				[]interface{}{
					CreateDefaultObject([]ConfigField{
						NewStringField("name", "", true, "TestCommand"),
						NewStringField("command", "", true, "AdminListPlayers"),
						NewBoolField("enabled", "", false, true),
						NewIntField("interval", "", false, 300),
						NewBoolField("on_new_game", "", false, false),
					}),
				},
			),
			NewIntField("check_interval", "Check interval", false, 60),
		},
	}

	// Test config with valid nested structure
	config := map[string]interface{}{
		"commands": []interface{}{
			map[string]interface{}{
				"name":        "TestCommand1",
				"command":     "AdminListPlayers",
				"enabled":     true,
				"interval":    300,
				"on_new_game": false,
			},
			map[string]interface{}{
				"name":        "TestCommand2",
				"command":     "AdminReloadServerConfig",
				"enabled":     false,
				"interval":    600,
				"on_new_game": true,
			},
		},
		"check_interval": 30,
	}

	// Test validation
	if err := schema.Validate(config); err != nil {
		t.Errorf("Expected valid config to pass validation, got error: %v", err)
	}

	// Test filling defaults
	incompleteConfig := map[string]interface{}{
		"commands": []interface{}{
			map[string]interface{}{
				"name":    "IncompleteCommand",
				"command": "AdminListPlayers",
				// Missing enabled, interval, on_new_game - should get defaults
			},
		},
		// Missing check_interval - should get default
	}

	schema.FillDefaults(incompleteConfig)

	// Verify defaults were filled
	commands := GetArrayObjectValue(incompleteConfig, "commands")
	if len(commands) != 1 {
		t.Errorf("Expected 1 command, got %d", len(commands))
	}

	cmd := commands[0]
	if GetBoolValue(cmd, "enabled") != true {
		t.Errorf("Expected enabled default to be true, got %v", GetBoolValue(cmd, "enabled"))
	}

	if GetIntValue(cmd, "interval") != 600 {
		t.Errorf("Expected interval default to be 600, got %d", GetIntValue(cmd, "interval"))
	}

	if GetBoolValue(cmd, "on_new_game") != false {
		t.Errorf("Expected on_new_game default to be false, got %v", GetBoolValue(cmd, "on_new_game"))
	}

	if GetIntValue(incompleteConfig, "check_interval") != 60 {
		t.Errorf("Expected check_interval default to be 60, got %d", GetIntValue(incompleteConfig, "check_interval"))
	}
}

func TestSchemaHelperFunctions(t *testing.T) {
	// Test NewStringField
	stringField := NewStringField("test_string", "A test string", true, "default_value")
	if stringField.Name != "test_string" || stringField.Type != FieldTypeString || stringField.Default != "default_value" {
		t.Errorf("NewStringField did not create correct field")
	}

	// Test NewIntField
	intField := NewIntField("test_int", "A test int", false, 42)
	if intField.Name != "test_int" || intField.Type != FieldTypeInt || intField.Default != 42 {
		t.Errorf("NewIntField did not create correct field")
	}

	// Test NewBoolField
	boolField := NewBoolField("test_bool", "A test bool", true, true)
	if boolField.Name != "test_bool" || boolField.Type != FieldTypeBool || boolField.Default != true {
		t.Errorf("NewBoolField did not create correct field")
	}

	// Test CreateDefaultObject
	fields := []ConfigField{
		NewStringField("name", "", true, "test_name"),
		NewIntField("value", "", false, 123),
		NewBoolField("flag", "", false, true),
	}

	defaultObj := CreateDefaultObject(fields)
	if defaultObj["name"] != "test_name" || defaultObj["value"] != 123 || defaultObj["flag"] != true {
		t.Errorf("CreateDefaultObject did not create correct object: %+v", defaultObj)
	}
}

func TestNestedValidationErrors(t *testing.T) {
	// Create schema with required nested fields
	commandFields := []ConfigField{
		NewStringField("name", "Command name", true, ""),
		NewStringField("command", "RCON command", true, ""),
	}

	schema := ConfigSchema{
		Fields: []ConfigField{
			NewArrayObjectField("commands", "List of commands", true, commandFields, nil),
		},
	}

	// Test config missing required nested field
	config := map[string]interface{}{
		"commands": []interface{}{
			map[string]interface{}{
				"name": "TestCommand",
				// Missing required "command" field
			},
		},
	}

	err := schema.Validate(config)
	if err == nil {
		t.Errorf("Expected validation error for missing required nested field, but got none")
	}

	// Test config with wrong type for nested field
	config2 := map[string]interface{}{
		"commands": []interface{}{
			map[string]interface{}{
				"name":    123, // Should be string
				"command": "AdminListPlayers",
			},
		},
	}

	err2 := schema.Validate(config2)
	if err2 == nil {
		t.Errorf("Expected validation error for wrong type in nested field, but got none")
	}
}
