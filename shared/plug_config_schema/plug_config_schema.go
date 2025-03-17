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
)

// ConfigField represents a single configuration field
type ConfigField struct {
	Name        string        `json:"name"`              // Field name
	Description string        `json:"description"`       // Field description
	Required    bool          `json:"required"`          // Is this field mandatory?
	Type        FieldType     `json:"type"`              // Data type (string, int, etc.)
	Default     interface{}   `json:"default"`           // Default value
	Nested      []ConfigField `json:"nested,omitempty"`  // Nested fields for objects
	Options     []interface{} `json:"options,omitempty"` // Allowed values (for enums)
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
	}

	return config
}
