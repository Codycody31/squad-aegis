package plug_config_schema

import (
	"reflect"
	"testing"
)

func TestArrayTypes(t *testing.T) {
	// Test IsArrayType function
	tests := []struct {
		fieldType FieldType
		expected  bool
	}{
		{FieldTypeString, false},
		{FieldTypeInt, false},
		{FieldTypeBool, false},
		{FieldTypeObject, false},
		// FieldTypeArray is still recognized but deprecated
		{FieldTypeArray, true},
		{FieldTypeArrayString, true},
		{FieldTypeArrayInt, true},
		{FieldTypeArrayBool, true},
		{"arraystring", true},
		{"arrayint", true},
		{"arraybool", true},
	}

	for _, test := range tests {
		result := IsArrayType(test.fieldType)
		if result != test.expected {
			t.Errorf("IsArrayType(%s) = %v, expected %v", test.fieldType, result, test.expected)
		}
	}

	// Test GetArrayItemType function
	itemTypeTests := []struct {
		fieldType FieldType
		expected  FieldType
	}{
		// FieldTypeArray should return FieldTypeString (backward compatibility)
		{FieldTypeArray, FieldTypeString},
		{FieldTypeArrayString, FieldTypeString},
		{FieldTypeArrayInt, FieldTypeInt},
		{FieldTypeArrayBool, FieldTypeBool},
		{"arraystring", FieldTypeString},
		{"arrayint", FieldTypeInt},
		{"arraybool", FieldTypeBool},
		{"arrayunknown", FieldTypeString}, // Default to string for unknown
	}

	for _, test := range itemTypeTests {
		result := GetArrayItemType(test.fieldType)
		if result != test.expected {
			t.Errorf("GetArrayItemType(%s) = %v, expected %v", test.fieldType, result, test.expected)
		}
	}
}

func TestArrayValueRetrieval(t *testing.T) {
	// Sample config
	config := map[string]interface{}{
		"stringArray": []interface{}{"value1", "value2", "value3"},
		"intArray":    []interface{}{1, 2, 3, 4.0}, // Note: 4.0 is a float64 but should be converted to int
		"boolArray":   []interface{}{true, false, true},
		"mixedArray":  []interface{}{"string", 42, true},
		"emptyArray":  []interface{}{},
		"notArray":    "this is not an array",
		"missing":     nil,
	}

	// Test GetArrayValue
	stringArray := GetArrayValue(config, "stringArray")
	if len(stringArray) != 3 {
		t.Errorf("Expected string array length of 3, got %d", len(stringArray))
	}

	// Test GetArrayStringValue
	strings := GetArrayStringValue(config, "stringArray")
	expectedStrings := []string{"value1", "value2", "value3"}
	if !reflect.DeepEqual(strings, expectedStrings) {
		t.Errorf("GetArrayStringValue returned %v, expected %v", strings, expectedStrings)
	}

	// Test GetArrayIntValue
	ints := GetArrayIntValue(config, "intArray")
	expectedInts := []int{1, 2, 3, 4}
	if !reflect.DeepEqual(ints, expectedInts) {
		t.Errorf("GetArrayIntValue returned %v, expected %v", ints, expectedInts)
	}

	// Test GetArrayBoolValue
	bools := GetArrayBoolValue(config, "boolArray")
	expectedBools := []bool{true, false, true}
	if !reflect.DeepEqual(bools, expectedBools) {
		t.Errorf("GetArrayBoolValue returned %v, expected %v", bools, expectedBools)
	}

	// Test handling of mixed array (should filter out non-matching types)
	mixedStrings := GetArrayStringValue(config, "mixedArray")
	if len(mixedStrings) != 1 || mixedStrings[0] != "string" {
		t.Errorf("GetArrayStringValue on mixed array returned %v, expected [string]", mixedStrings)
	}

	// Test handling of empty array
	emptyBools := GetArrayBoolValue(config, "emptyArray")
	if len(emptyBools) != 0 {
		t.Errorf("GetArrayBoolValue on empty array returned %v, expected []", emptyBools)
	}

	// Test handling of non-array
	nonArrayStrings := GetArrayStringValue(config, "notArray")
	if len(nonArrayStrings) != 0 {
		t.Errorf("GetArrayStringValue on non-array returned %v, expected []", nonArrayStrings)
	}

	// Test handling of missing field
	missingInts := GetArrayIntValue(config, "missing")
	if len(missingInts) != 0 {
		t.Errorf("GetArrayIntValue on missing field returned %v, expected []", missingInts)
	}
}

func TestArrayValidation(t *testing.T) {
	schema := ConfigSchema{
		Fields: []ConfigField{
			{
				Name:     "requiredStringArray",
				Type:     FieldTypeArrayString,
				Required: true,
			},
			{
				Name:     "optionalIntArray",
				Type:     FieldTypeArrayInt,
				Required: false,
			},
			{
				Name:     "boolArray",
				Type:     FieldTypeArrayBool,
				Required: false,
				Default:  []bool{false, false},
			},
		},
	}

	// Valid config
	validConfig := map[string]interface{}{
		"requiredStringArray": []interface{}{"value1", "value2"},
		"optionalIntArray":    []interface{}{1, 2, 3},
	}

	if err := schema.Validate(validConfig); err != nil {
		t.Errorf("Validation failed for valid config: %v", err)
	}

	// Invalid config - missing required field
	invalidConfig1 := map[string]interface{}{
		"optionalIntArray": []interface{}{1, 2, 3},
	}

	if err := schema.Validate(invalidConfig1); err == nil {
		t.Error("Validation should fail due to missing required field")
	}

	// Invalid config - wrong type (not an array)
	invalidConfig2 := map[string]interface{}{
		"requiredStringArray": "not an array",
		"optionalIntArray":    []interface{}{1, 2, 3},
	}

	if err := schema.Validate(invalidConfig2); err == nil {
		t.Error("Validation should fail due to wrong type (not an array)")
	}

	// Invalid config - wrong element type
	invalidConfig3 := map[string]interface{}{
		"requiredStringArray": []interface{}{"value1", 2}, // 2 should be a string
		"optionalIntArray":    []interface{}{1, 2, 3},
	}

	if err := schema.Validate(invalidConfig3); err == nil {
		t.Error("Validation should fail due to wrong element type")
	}

	// Test FillDefaults
	incompleteConfig := map[string]interface{}{
		"requiredStringArray": []interface{}{"value1", "value2"},
	}

	filledConfig := schema.FillDefaults(incompleteConfig)
	if filledConfig["boolArray"] == nil {
		t.Error("FillDefaults should have added default value for boolArray")
	}
}

func TestDeprecatedArrayType(t *testing.T) {
	// Test schema with a deprecated generic array
	schema := ConfigSchema{
		Fields: []ConfigField{
			{
				Name:     "deprecatedArray",
				Type:     FieldTypeArray, // Using deprecated array type
				Required: true,
				Default:  []interface{}{"default"}, // Use []interface{} not []string
			},
		},
	}

	// Config with string array
	config := map[string]interface{}{
		"deprecatedArray": []interface{}{"value1", "value2"},
	}

	// Validation should still work but with a warning
	if err := schema.ValidateWithFieldTypeCheck(config); err != nil {
		t.Errorf("Validation failed for config with deprecated array type: %v", err)
	}
}

func TestArrayUsageExample(t *testing.T) {
	// Example of how to use the array functions in real code
	schema := ConfigSchema{
		Fields: []ConfigField{
			{
				Name:        "servers",
				Type:        FieldTypeArrayString,
				Description: "List of server addresses",
				Required:    true,
			},
			{
				Name:        "ports",
				Type:        FieldTypeArrayInt,
				Description: "List of port numbers",
				Required:    true,
			},
			{
				Name:        "enabled_features",
				Type:        FieldTypeArrayBool,
				Description: "List of feature flags",
				Default:     []interface{}{true, false, true}, // Use []interface{} not []bool
			},
		},
	}

	config := map[string]interface{}{
		"servers": []interface{}{"server1.example.com", "server2.example.com"},
		"ports":   []interface{}{8080, 8443},
	}

	// Fill in defaults
	config = schema.FillDefaults(config)

	// Validate the config
	if err := schema.Validate(config); err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	// Get values using helper functions
	servers := GetArrayStringValue(config, "servers")
	for _, server := range servers {
		// Process each server
		if server == "" {
			t.Error("Empty server address")
		}
	}

	ports := GetArrayIntValue(config, "ports")
	for _, port := range ports {
		// Process each port
		if port <= 0 || port > 65535 {
			t.Errorf("Invalid port number: %d", port)
		}
	}

	enabledFeatures := GetArrayBoolValue(config, "enabled_features")
	for i, enabled := range enabledFeatures {
		// Process each feature flag
		if i == 0 && !enabled {
			t.Error("First feature should be enabled by default")
		}
	}
}

func TestMigrateDeprecatedArrays(t *testing.T) {
	// Test schema with various deprecated array types
	schema := ConfigSchema{
		Fields: []ConfigField{
			{
				Name:     "stringArrayWithDefault",
				Type:     FieldTypeArray,
				Required: false,
				Default:  []string{"value1", "value2"},
			},
			{
				Name:     "intArrayWithDefault",
				Type:     FieldTypeArray,
				Required: false,
				Default:  []int{1, 2, 3},
			},
			{
				Name:     "boolArrayWithDefault",
				Type:     FieldTypeArray,
				Required: false,
				Default:  []bool{true, false, true},
			},
			{
				Name:     "interfaceStringArray",
				Type:     FieldTypeArray,
				Required: false,
				Default:  []interface{}{"value1", "value2"},
			},
			{
				Name:     "interfaceIntArray",
				Type:     FieldTypeArray,
				Required: false,
				Default:  []interface{}{1, 2, 3},
			},
			{
				Name:     "interfaceBoolArray",
				Type:     FieldTypeArray,
				Required: false,
				Default:  []interface{}{true, false, true},
			},
			{
				Name:     "emptyArray",
				Type:     FieldTypeArray,
				Required: false,
				Default:  []interface{}{},
			},
			{
				Name:     "noDefaultArray",
				Type:     FieldTypeArray,
				Required: false,
			},
			{
				Name:     "nonArray", // This shouldn't be changed
				Type:     FieldTypeString,
				Required: false,
				Default:  "string value",
			},
		},
	}

	// Migrate the deprecated arrays
	MigrateDeprecatedArrays(&schema)

	// Check that each field was migrated to the correct type
	expectedTypes := map[string]FieldType{
		"stringArrayWithDefault": FieldTypeArrayString,
		"intArrayWithDefault":    FieldTypeArrayInt,
		"boolArrayWithDefault":   FieldTypeArrayBool,
		"interfaceStringArray":   FieldTypeArrayString,
		"interfaceIntArray":      FieldTypeArrayInt,
		"interfaceBoolArray":     FieldTypeArrayBool,
		"emptyArray":             FieldTypeArrayString, // Empty arrays default to string
		"noDefaultArray":         FieldTypeArrayString, // No default arrays default to string
		"nonArray":               FieldTypeString,      // Should remain unchanged
	}

	for _, field := range schema.Fields {
		expectedType, exists := expectedTypes[field.Name]
		if !exists {
			t.Errorf("Unexpected field name: %s", field.Name)
			continue
		}

		if field.Type != expectedType {
			t.Errorf("Field %s - expected type %s, got %s", field.Name, expectedType, field.Type)
		}

		// Check that defaults were converted to []interface{} for consistency
		if IsArrayType(field.Type) && field.Default != nil {
			_, ok := field.Default.([]interface{})
			if !ok {
				t.Errorf("Field %s - default value should be []interface{}, got %T", field.Name, field.Default)
			}
		}
	}
}
