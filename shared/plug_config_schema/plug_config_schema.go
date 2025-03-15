package plug_config_schema

import "fmt"

// FieldType defines the data type of a config field
type FieldType string

const (
	FieldTypeString = "string"
	FieldTypeInt    = "int"
	FieldTypeBool   = "bool"
	FieldTypeObject = "object"
	FieldTypeArray  = "array"
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

func (c *ConfigSchema) Validate(config map[string]interface{}) error {
	for _, field := range c.Fields {
		if field.Required && config[field.Name] == nil {
			return fmt.Errorf("required field %s is missing", field.Name)
		}
	}

	return nil
}
