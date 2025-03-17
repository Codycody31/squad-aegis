package server

import (
	"github.com/gin-gonic/gin"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// ExtensionDefinitionResponse represents an extension definition in the API response
type ExtensionDefinitionResponse struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	Author      string                 `json:"author"`
	Schema      map[string]interface{} `json:"schema"`
}

// ExtensionDefinitionsResponse represents the response for the list definitions endpoint
type ExtensionDefinitionsResponse struct {
	Definitions []ExtensionDefinitionResponse `json:"definitions"`
}

// ListExtensionDefinitions lists all available extension definitions
// Path: /definitions (previously /types)
func (s *Server) ListExtensionDefinitions(c *gin.Context) {
	// Get registered extensions from extension manager
	extensions := s.Dependencies.ExtensionManager.ListExtensions()

	// Build response with extension definitions
	definitionResponses := make([]ExtensionDefinitionResponse, 0, len(extensions))

	for _, extension := range extensions {
		// Convert ConfigSchema to map
		schemaMap := make(map[string]interface{})

		// Process each field in the schema
		for _, field := range extension.ConfigSchema.Fields {
			fieldInfo := map[string]interface{}{
				"description": field.Description,
				"required":    field.Required,
				"type":        string(field.Type),
			}

			if field.Default != nil {
				fieldInfo["default"] = field.Default
			}

			// Add options if present
			if len(field.Options) > 0 {
				fieldInfo["options"] = field.Options
			}

			// Add nested fields if present
			if len(field.Nested) > 0 {
				nestedFields := []map[string]interface{}{}
				for _, nestedField := range field.Nested {
					nestedFieldInfo := map[string]interface{}{
						"name":        nestedField.Name,
						"description": nestedField.Description,
						"required":    nestedField.Required,
						"type":        string(nestedField.Type),
					}

					if nestedField.Default != nil {
						nestedFieldInfo["default"] = nestedField.Default
					}

					nestedFields = append(nestedFields, nestedFieldInfo)
				}
				fieldInfo["nested"] = nestedFields
			}

			schemaMap[field.Name] = fieldInfo
		}

		// Create definition response
		definitionResponses = append(definitionResponses, ExtensionDefinitionResponse{
			ID:          extension.ID,
			Name:        extension.Name,
			Description: extension.Description,
			Version:     extension.Version,
			Author:      extension.Author,
			Schema:      schemaMap,
		})
	}

	responses.Success(c, "Extension definitions fetched successfully", &gin.H{
		"definitions": definitionResponses,
	})
}
