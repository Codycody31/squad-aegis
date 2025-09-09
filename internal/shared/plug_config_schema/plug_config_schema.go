package plug_config_schema

import (
	"fmt"
	"strings"
)

// FieldType defines the data type of a config field
type FieldType string

const (
	FieldTypeString = "string"
	FieldTypeInt    = "int"
	FieldTypeBool   = "bool"
	FieldTypeObject = "object"
	// FieldTypeArray is deprecated, use typed arrays instead
	FieldTypeArray = "array" // Kept for backward compatibility

	// Array types
	FieldTypeArrayString = "arraystring" // array of strings
	FieldTypeArrayInt    = "arrayint"    // array of integers
	FieldTypeArrayBool   = "arraybool"   // array of booleans
	FieldTypeArrayObject = "arrayobject" // array of objects
)

// ConfigField represents a single configuration field
type ConfigField struct {
	Name        string        `json:"name"`                // Field name
	Description string        `json:"description"`         // Field description
	Required    bool          `json:"required"`            // Is this field mandatory?
	Type        FieldType     `json:"type"`                // Data type (string, int, etc.)
	Default     interface{}   `json:"default"`             // Default value
	Nested      []ConfigField `json:"nested,omitempty"`    // Nested fields for objects and array items
	Options     []interface{} `json:"options,omitempty"`   // Allowed values (for enums)
	Sensitive   bool          `json:"sensitive,omitempty"` // Is this field sensitive (e.g., passwords)?
	MinItems    *int          `json:"min_items,omitempty"` // Minimum items for arrays
	MaxItems    *int          `json:"max_items,omitempty"` // Maximum items for arrays
	Pattern     string        `json:"pattern,omitempty"`   // Regex pattern for validation
}

// ConfigSchema defines a configuration schema
type ConfigSchema struct {
	Fields []ConfigField `json:"fields"`
}

// IsArrayType checks if the given FieldType is an array type
func IsArrayType(fieldType FieldType) bool {
	return fieldType == FieldTypeArrayString ||
		fieldType == FieldTypeArrayInt ||
		fieldType == FieldTypeArrayBool ||
		fieldType == FieldTypeArrayObject ||
		strings.HasPrefix(string(fieldType), "array") // For backward compatibility
}

// GetArrayItemType returns the element type of an array
func GetArrayItemType(fieldType FieldType) FieldType {
	if fieldType == FieldTypeArrayString {
		return FieldTypeString
	}

	if fieldType == FieldTypeArrayInt {
		return FieldTypeInt
	}

	if fieldType == FieldTypeArrayBool {
		return FieldTypeBool
	}

	if fieldType == FieldTypeArrayObject {
		return FieldTypeObject
	}

	// For custom "array<type>" format or backward compatibility
	if strings.HasPrefix(string(fieldType), "array") {
		subType := strings.ToLower(string(fieldType)[5:])
		switch subType {
		case "string":
			return FieldTypeString
		case "int":
			return FieldTypeInt
		case "bool":
			return FieldTypeBool
		case "object":
			return FieldTypeObject
		default:
			return FieldTypeString // Default to string
		}
	}

	// Generic arrays (deprecated) default to string
	return FieldTypeString
}

// GetArrayValue safely retrieves an array value from a config
func GetArrayValue(config map[string]interface{}, fieldName string) []interface{} {
	if val, ok := config[fieldName]; ok {
		if arr, ok := val.([]interface{}); ok {
			return arr
		}
	}
	return []interface{}{}
}

// GetArrayStringValue retrieves a string array from config
func GetArrayStringValue(config map[string]interface{}, fieldName string) []string {
	arr := GetArrayValue(config, fieldName)
	result := make([]string, 0, len(arr))

	for _, item := range arr {
		if str, ok := item.(string); ok {
			result = append(result, str)
		}
	}

	return result
}

// GetArrayIntValue retrieves an int array from config
func GetArrayIntValue(config map[string]interface{}, fieldName string) []int {
	arr := GetArrayValue(config, fieldName)
	result := make([]int, 0, len(arr))

	for _, item := range arr {
		if n, ok := item.(int); ok {
			result = append(result, n)
		} else if f, ok := item.(float64); ok {
			// JSON numbers are sometimes parsed as float64
			result = append(result, int(f))
		}
	}

	return result
}

// GetArrayBoolValue retrieves a bool array from config
func GetArrayBoolValue(config map[string]interface{}, fieldName string) []bool {
	arr := GetArrayValue(config, fieldName)
	result := make([]bool, 0, len(arr))

	for _, item := range arr {
		if b, ok := item.(bool); ok {
			result = append(result, b)
		}
	}

	return result
}

// GetArrayObjectValue retrieves an array of objects from config
func GetArrayObjectValue(config map[string]interface{}, fieldName string) []map[string]interface{} {
	arr := GetArrayValue(config, fieldName)
	result := make([]map[string]interface{}, 0, len(arr))

	for _, item := range arr {
		if obj, ok := item.(map[string]interface{}); ok {
			result = append(result, obj)
		}
	}

	return result
}

// NewObjectField creates a new object field with nested fields
func NewObjectField(name, description string, required bool, nested []ConfigField, defaultValue interface{}) ConfigField {
	return ConfigField{
		Name:        name,
		Description: description,
		Required:    required,
		Type:        FieldTypeObject,
		Default:     defaultValue,
		Nested:      nested,
	}
}

// NewArrayObjectField creates a new array of objects field with nested schema
func NewArrayObjectField(name, description string, required bool, nested []ConfigField, defaultValue []interface{}) ConfigField {
	return ConfigField{
		Name:        name,
		Description: description,
		Required:    required,
		Type:        FieldTypeArrayObject,
		Default:     defaultValue,
		Nested:      nested,
	}
}

// NewStringField creates a new string field
func NewStringField(name, description string, required bool, defaultValue string) ConfigField {
	return ConfigField{
		Name:        name,
		Description: description,
		Required:    required,
		Type:        FieldTypeString,
		Default:     defaultValue,
	}
}

// NewIntField creates a new integer field
func NewIntField(name, description string, required bool, defaultValue int) ConfigField {
	return ConfigField{
		Name:        name,
		Description: description,
		Required:    required,
		Type:        FieldTypeInt,
		Default:     defaultValue,
	}
}

// NewBoolField creates a new boolean field
func NewBoolField(name, description string, required bool, defaultValue bool) ConfigField {
	return ConfigField{
		Name:        name,
		Description: description,
		Required:    required,
		Type:        FieldTypeBool,
		Default:     defaultValue,
	}
}

// CreateDefaultObject creates a map[string]interface{} from a slice of ConfigFields with their default values
func CreateDefaultObject(fields []ConfigField) map[string]interface{} {
	obj := make(map[string]interface{})
	for _, field := range fields {
		obj[field.Name] = field.Default
	}
	return obj
}

// GetStringValue safely retrieves a string value from config
func GetStringValue(config map[string]interface{}, fieldName string) string {
	if val, ok := config[fieldName]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// GetBoolValue safely retrieves a bool value from config
func GetBoolValue(config map[string]interface{}, fieldName string) bool {
	if val, ok := config[fieldName]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// GetIntValue safely retrieves an int value from config
func GetIntValue(config map[string]interface{}, fieldName string) int {
	if val, ok := config[fieldName]; ok {
		if i, ok := val.(int); ok {
			return i
		}
		if f, ok := val.(float64); ok {
			return int(f)
		}
	}
	return 0
}

// MigrateDeprecatedArrays updates a schema to use typed arrays instead of the generic FieldTypeArray
// It analyzes the default values or tries to infer the array type from usage
func MigrateDeprecatedArrays(schema *ConfigSchema) {
	for i, field := range schema.Fields {
		if field.Type == FieldTypeArray {
			// Try to determine the actual array type from the default value if available
			if field.Default != nil {
				switch defaultVal := field.Default.(type) {
				case []interface{}:
					if len(defaultVal) > 0 {
						switch defaultVal[0].(type) {
						case string:
							schema.Fields[i].Type = FieldTypeArrayString
						case int, int32, int64, float32, float64:
							schema.Fields[i].Type = FieldTypeArrayInt
						case bool:
							schema.Fields[i].Type = FieldTypeArrayBool
						default:
							// Default to string array if can't determine
							schema.Fields[i].Type = FieldTypeArrayString
						}
					} else {
						// Empty array, default to string
						schema.Fields[i].Type = FieldTypeArrayString
					}
				case []string:
					schema.Fields[i].Type = FieldTypeArrayString
					// Convert to []interface{} for consistency
					interfaceArr := make([]interface{}, len(defaultVal))
					for j, v := range defaultVal {
						interfaceArr[j] = v
					}
					schema.Fields[i].Default = interfaceArr
				case []int:
					schema.Fields[i].Type = FieldTypeArrayInt
					// Convert to []interface{} for consistency
					interfaceArr := make([]interface{}, len(defaultVal))
					for j, v := range defaultVal {
						interfaceArr[j] = v
					}
					schema.Fields[i].Default = interfaceArr
				case []bool:
					schema.Fields[i].Type = FieldTypeArrayBool
					// Convert to []interface{} for consistency
					interfaceArr := make([]interface{}, len(defaultVal))
					for j, v := range defaultVal {
						interfaceArr[j] = v
					}
					schema.Fields[i].Default = interfaceArr
				default:
					// If still can't determine, default to string array
					schema.Fields[i].Type = FieldTypeArrayString
				}
			} else {
				// No default value, use string array as default
				schema.Fields[i].Type = FieldTypeArrayString
			}
		}
	}
}

func (c *ConfigSchema) Validate(config map[string]interface{}) error {
	for _, field := range c.Fields {
		if field.Required && config[field.Name] == nil {
			return fmt.Errorf("required field %s is missing", field.Name)
		}

		// Skip validation if field is not present and not required
		if config[field.Name] == nil {
			continue
		}

		// Validate basic types
		switch field.Type {
		case FieldTypeString:
			if _, ok := config[field.Name].(string); !ok {
				return fmt.Errorf("field %s should be a string", field.Name)
			}
		case FieldTypeInt:
			if _, ok := config[field.Name].(int); !ok {
				// Check for float64 since JSON numbers are sometimes parsed as float64
				if _, ok := config[field.Name].(float64); !ok {
					return fmt.Errorf("field %s should be an integer", field.Name)
				}
			}
		case FieldTypeBool:
			if _, ok := config[field.Name].(bool); !ok {
				return fmt.Errorf("field %s should be a boolean", field.Name)
			}
		case FieldTypeObject:
			if obj, ok := config[field.Name].(map[string]interface{}); ok {
				if len(field.Nested) > 0 {
					nestedSchema := &ConfigSchema{Fields: field.Nested}
					if err := nestedSchema.Validate(obj); err != nil {
						return fmt.Errorf("field %s: %v", field.Name, err)
					}
				}
			} else {
				return fmt.Errorf("field %s should be an object", field.Name)
			}
		}

		// Add validation for array types
		if IsArrayType(field.Type) && config[field.Name] != nil {
			arr, ok := config[field.Name].([]interface{})
			if !ok {
				return fmt.Errorf("field %s should be an array", field.Name)
			}

			// Validate array element types
			itemType := GetArrayItemType(field.Type)
			for i, item := range arr {
				switch itemType {
				case FieldTypeString:
					if _, ok := item.(string); !ok {
						return fmt.Errorf("array item %d in field %s should be a string", i, field.Name)
					}
				case FieldTypeInt:
					if _, ok := item.(int); !ok {
						// Check for float64 since JSON numbers are sometimes parsed as float64
						if _, ok := item.(float64); !ok {
							return fmt.Errorf("array item %d in field %s should be an integer", i, field.Name)
						}
					}
				case FieldTypeBool:
					if _, ok := item.(bool); !ok {
						return fmt.Errorf("array item %d in field %s should be a boolean", i, field.Name)
					}
				case FieldTypeObject:
					if _, ok := item.(map[string]interface{}); !ok {
						return fmt.Errorf("array item %d in field %s should be an object", i, field.Name)
					}
					// If the field has nested fields defined, validate them
					if len(field.Nested) > 0 {
						if obj, ok := item.(map[string]interface{}); ok {
							nestedSchema := &ConfigSchema{Fields: field.Nested}
							if err := nestedSchema.Validate(obj); err != nil {
								return fmt.Errorf("array item %d in field %s: %v", i, field.Name, err)
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// ValidateWithFieldTypeCheck validates the config with a warning if using deprecated FieldTypeArray
func (c *ConfigSchema) ValidateWithFieldTypeCheck(config map[string]interface{}) error {
	for _, field := range c.Fields {
		if field.Type == FieldTypeArray {
			fmt.Printf("Warning: Field %s is using deprecated FieldTypeArray. Use typed arrays instead (FieldTypeArrayString, FieldTypeArrayInt, FieldTypeArrayBool).\n", field.Name)
		}
	}

	return c.Validate(config)
}

func (c *ConfigSchema) FillDefaults(config map[string]interface{}) map[string]interface{} {
	for _, field := range c.Fields {
		if config[field.Name] == nil {
			config[field.Name] = field.Default
		}

		// Handle nested defaults for objects
		if field.Type == FieldTypeObject && config[field.Name] != nil {
			if obj, ok := config[field.Name].(map[string]interface{}); ok && len(field.Nested) > 0 {
				nestedSchema := &ConfigSchema{Fields: field.Nested}
				nestedSchema.FillDefaults(obj)
			}
		}

		// Handle nested defaults for array objects
		if field.Type == FieldTypeArrayObject && config[field.Name] != nil {
			if arr, ok := config[field.Name].([]interface{}); ok && len(field.Nested) > 0 {
				for _, item := range arr {
					if obj, ok := item.(map[string]interface{}); ok {
						nestedSchema := &ConfigSchema{Fields: field.Nested}
						nestedSchema.FillDefaults(obj)
					}
				}
			}
		}
	}

	return config
}

// MaskSensitiveFields masks sensitive field values for safe display/return
func (c *ConfigSchema) MaskSensitiveFields(config map[string]interface{}) map[string]interface{} {
	masked := make(map[string]interface{})

	// Copy all non-sensitive fields
	for key, value := range config {
		masked[key] = value
	}

	// Mask sensitive fields
	for _, field := range c.Fields {
		if field.Sensitive && config[field.Name] != nil {
			// Replace sensitive value with placeholder
			masked[field.Name] = "***MASKED***"
		}
	}

	return masked
}

// MergeConfigUpdates merges new config values with existing ones, handling sensitive fields properly
func (c *ConfigSchema) MergeConfigUpdates(existingConfig, newConfig map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})

	// Start with existing config
	for key, value := range existingConfig {
		merged[key] = value
	}

	// Apply updates, handling sensitive fields specially
	for _, field := range c.Fields {
		if newValue, exists := newConfig[field.Name]; exists {
			if field.Sensitive {
				// For sensitive fields, only update if the new value is not empty/masked
				if newValue != nil && newValue != "" && newValue != "***MASKED***" {
					merged[field.Name] = newValue
				}
				// If empty or masked, keep the existing value
			} else {
				// For non-sensitive fields, always update
				merged[field.Name] = newValue
			}
		}
	}

	return merged
}

// ValidateForCreation validates config for creation, ensuring sensitive required fields are provided
func (c *ConfigSchema) ValidateForCreation(config map[string]interface{}) error {
	for _, field := range c.Fields {
		if field.Required {
			value := config[field.Name]
			if value == nil {
				return fmt.Errorf("required field %s is missing", field.Name)
			}

			// For sensitive fields, ensure they're not empty when required
			if field.Sensitive {
				if str, ok := value.(string); ok && str == "" {
					return fmt.Errorf("required sensitive field %s cannot be empty", field.Name)
				}
			}
		}

		// Validate field types and arrays as before
		if IsArrayType(field.Type) && config[field.Name] != nil {
			arr, ok := config[field.Name].([]interface{})
			if !ok {
				return fmt.Errorf("field %s should be an array", field.Name)
			}

			// Validate array element types
			itemType := GetArrayItemType(field.Type)
			for i, item := range arr {
				switch itemType {
				case FieldTypeString:
					if _, ok := item.(string); !ok {
						return fmt.Errorf("array item %d in field %s should be a string", i, field.Name)
					}
				case FieldTypeInt:
					if _, ok := item.(int); !ok {
						// Check for float64 since JSON numbers are sometimes parsed as float64
						if _, ok := item.(float64); !ok {
							return fmt.Errorf("array item %d in field %s should be an integer", i, field.Name)
						}
					}
				case FieldTypeBool:
					if _, ok := item.(bool); !ok {
						return fmt.Errorf("array item %d in field %s should be a boolean", i, field.Name)
					}
				case FieldTypeObject:
					if _, ok := item.(map[string]interface{}); !ok {
						return fmt.Errorf("array item %d in field %s should be an object", i, field.Name)
					}
					// If the field has nested fields defined, validate them
					if len(field.Nested) > 0 {
						if obj, ok := item.(map[string]interface{}); ok {
							nestedSchema := &ConfigSchema{Fields: field.Nested}
							if err := nestedSchema.ValidateForCreation(obj); err != nil {
								return fmt.Errorf("array item %d in field %s: %v", i, field.Name, err)
							}
						}
					}
				}
			}
		}
	}

	return nil
}
