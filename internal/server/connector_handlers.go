package server

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// ConnectorListAvailableTypesResponse represents the response for the list available connector types endpoint
type ConnectorListAvailableTypesResponse struct {
	Types map[string]map[string]interface{} `json:"types"`
}

// ConnectorDefinitionResponse represents a connector definition in the API response
type ConnectorDefinitionResponse struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Schema      map[string]interface{} `json:"schema"`
}

// ConnectorDefinitionsResponse represents the response for the list definitions endpoint
type ConnectorDefinitionsResponse struct {
	Definitions []ConnectorDefinitionResponse `json:"definitions"`
}

// ConnectorResponse represents a connector in API responses
type ConnectorResponse struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"config"`
}

// ConnectorListGlobalResponse represents the response for the list global connectors endpoint
type ConnectorListGlobalResponse struct {
	Connectors []ConnectorResponse `json:"connectors"`
}

// ConnectorCreateRequest represents the request to create a new connector
type ConnectorCreateRequest struct {
	Name   string                 `json:"name" binding:"required"`
	Type   string                 `json:"type" binding:"required"`
	Config map[string]interface{} `json:"config" binding:"required"`
}

// ConnectorUpdateRequest represents the request to update a connector
type ConnectorUpdateRequest struct {
	Name   string                 `json:"name"`
	Config map[string]interface{} `json:"config"`
}

// ListConnectorDefinitions lists all available connector definitions
func (s *Server) ListConnectorDefinitions(c *gin.Context) {
	// Get connector definitions from connector manager
	connectorDefs := s.Dependencies.ConnectorManager.ListConnectors()

	// Build response with connector definitions
	definitionResponses := make([]ConnectorDefinitionResponse, 0, len(connectorDefs))

	for _, def := range connectorDefs {
		// Convert ConfigSchema to map
		schemaMap := make(map[string]interface{})

		// Process each field in the schema
		for _, field := range def.ConfigSchema.Fields {
			fieldInfo := map[string]interface{}{
				"description": field.Description,
				"required":    field.Required,
				"type":        string(field.Type),
			}

			if field.Default != nil {
				fieldInfo["default"] = field.Default
			}

			// Add options if present (assuming the same structure as extensions)
			if len(field.Options) > 0 {
				fieldInfo["options"] = field.Options
			}

			schemaMap[field.Name] = fieldInfo
		}

		// Create definition response
		definitionResponses = append(definitionResponses, ConnectorDefinitionResponse{
			ID:          def.ID,
			Name:        def.Name,
			Description: def.Description,
			Schema:      schemaMap,
		})
	}

	responses.Success(c, "Connector definitions fetched successfully", &gin.H{
		"definitions": definitionResponses,
	})
}

// ListConnectorTypes lists all available connector types (legacy endpoint)
func (s *Server) ListConnectorTypes(c *gin.Context) {
	// Get connector definitions from connector manager
	connectorDefs := s.Dependencies.ConnectorManager.ListConnectors()

	// Build response with all schemas
	resp := ConnectorListAvailableTypesResponse{
		Types: make(map[string]map[string]interface{}),
	}

	for _, def := range connectorDefs {
		// Convert the schema to map structure
		schemaMap := make(map[string]interface{})

		for _, field := range def.ConfigSchema.Fields {
			fieldInfo := map[string]interface{}{
				"description": field.Description,
				"required":    field.Required,
				"type":        string(field.Type),
			}

			if field.Default != nil {
				fieldInfo["default"] = field.Default
			}

			schemaMap[field.Name] = fieldInfo
		}

		resp.Types[def.ID] = schemaMap
	}

	c.JSON(http.StatusOK, resp)
}

// ListGlobalConnectors lists all global connectors
func (s *Server) ListGlobalConnectors(c *gin.Context) {
	// Get connectors from database
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
		SELECT id, name, config
		FROM connectors
		ORDER BY name
	`)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query connectors")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to query connectors",
			"code":    http.StatusInternalServerError,
		})
		return
	}
	defer rows.Close()

	// Build response
	resp := ConnectorListGlobalResponse{
		Connectors: []ConnectorResponse{},
	}

	for rows.Next() {
		var id uuid.UUID
		var name string
		var connType string
		var configJSON []byte

		if err := rows.Scan(&id, &name, &configJSON); err != nil {
			log.Error().Err(err).Msg("Failed to scan connector row")
			continue
		}

		// Parse config JSON
		var config map[string]interface{}
		if err := json.Unmarshal(configJSON, &config); err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("Failed to unmarshal connector config")
			continue
		}

		// Remove sensitive data from config
		// For Discord, remove token
		if connType == "discord" {
			if _, ok := config["token"]; ok {
				config["token"] = "********"
			}
		}

		// Add to response
		resp.Connectors = append(resp.Connectors, ConnectorResponse{
			ID:     id.String(),
			Name:   name,
			Type:   connType,
			Config: config,
		})
	}

	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating connector rows")
	}

	c.JSON(http.StatusOK, resp)
}

// GetGlobalConnector returns a specific global connector
func (s *Server) GetGlobalConnector(c *gin.Context) {
	// Get connector ID from URL
	idStr := c.Param("id")

	// Validate UUID
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid connector ID",
			"code":    http.StatusBadRequest,
		})
		return
	}

	// Get connector from database
	var name string
	var connType string
	var configJSON []byte

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT name, type, config
		FROM connectors
		WHERE id = $1
	`, id).Scan(&name, &connType, &configJSON)

	if err != nil {
		log.Error().Err(err).Str("id", id.String()).Msg("Failed to get connector")
		c.JSON(http.StatusNotFound, gin.H{
			"message": "Connector not found",
			"code":    http.StatusNotFound,
		})
		return
	}

	// Parse config JSON
	var config map[string]interface{}
	if err := json.Unmarshal(configJSON, &config); err != nil {
		log.Error().Err(err).Str("id", id.String()).Msg("Failed to unmarshal connector config")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to parse connector config",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Remove sensitive data from config
	// For Discord, remove token
	if connType == "discord" {
		if _, ok := config["token"]; ok {
			config["token"] = "********"
		}
	}

	// Build response
	resp := ConnectorResponse{
		ID:     id.String(),
		Name:   name,
		Type:   connType,
		Config: config,
	}

	c.JSON(http.StatusOK, resp)
}

// CreateGlobalConnector creates a new global connector
func (s *Server) CreateGlobalConnector(c *gin.Context) {
	var req ConnectorCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid request body",
			"code":    http.StatusBadRequest,
		})
		return
	}

	// Validate connector type
	registrar, ok := s.Dependencies.ConnectorManager.GetConnector(req.Type)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid connector type",
			"code":    http.StatusBadRequest,
		})
		return
	}

	// Add type to config
	req.Config["type"] = req.Type

	// Convert config to JSON
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal connector config")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to process connector config",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Generate UUID for new connector
	id := uuid.New()

	// Create connector in database
	_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
		INSERT INTO connectors (id, name, type, config)
		VALUES ($1, $2, $3, $4)
	`, id, req.Name, req.Type, configJSON)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create connector")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to create connector",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Create and initialize connector instance
	def := registrar.Define()
	instance, err := def.CreateInstance(id, req.Config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create connector instance")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to initialize connector",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Add the connector to the manager
	connectorType := instance.GetType()

	c.JSON(http.StatusCreated, ConnectorResponse{
		ID:     id.String(),
		Name:   req.Name,
		Type:   connectorType,
		Config: req.Config,
	})
}

// UpdateGlobalConnector updates an existing global connector
func (s *Server) UpdateGlobalConnector(c *gin.Context) {
	// Get connector ID from URL
	idStr := c.Param("id")

	// Validate UUID
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid connector ID",
			"code":    http.StatusBadRequest,
		})
		return
	}

	var req ConnectorUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid request body",
			"code":    http.StatusBadRequest,
		})
		return
	}

	// Get current connector info from database
	var currentName string
	var connType string
	var currentConfigJSON []byte

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT name, type, config
		FROM connectors
		WHERE id = $1
	`, id).Scan(&currentName, &connType, &currentConfigJSON)

	if err != nil {
		log.Error().Err(err).Str("id", id.String()).Msg("Failed to get connector")
		c.JSON(http.StatusNotFound, gin.H{
			"message": "Connector not found",
			"code":    http.StatusNotFound,
		})
		return
	}

	// Parse current config JSON
	var currentConfig map[string]interface{}
	if err := json.Unmarshal(currentConfigJSON, &currentConfig); err != nil {
		log.Error().Err(err).Str("id", id.String()).Msg("Failed to unmarshal connector config")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to parse connector config",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Get the connector registrar
	_, ok := s.Dependencies.ConnectorManager.GetConnector(connType)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid connector type",
			"code":    http.StatusBadRequest,
		})
		return
	}

	// Update config if provided
	if req.Config != nil {
		// Preserve type field
		req.Config["type"] = connType

		// Convert updated config to JSON
		configJSON, err := json.Marshal(req.Config)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal connector config")
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Failed to process connector config",
				"code":    http.StatusInternalServerError,
			})
			return
		}

		// Update connector in database
		if req.Name != "" {
			_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
				UPDATE connectors
				SET name = $1, config = $2
				WHERE id = $3
			`, req.Name, configJSON, id)
		} else {
			_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
				UPDATE connectors
				SET config = $1
				WHERE id = $2
			`, configJSON, id)
		}

		if err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("Failed to update connector")
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Failed to update connector",
				"code":    http.StatusInternalServerError,
			})
			return
		}

		// Restart connector instance
		if err := s.Dependencies.ConnectorManager.RestartConnector(id, req.Config); err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("Failed to restart connector")
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Connector updated but failed to restart: " + err.Error(),
				"code":    http.StatusInternalServerError,
			})
			return
		}
	} else if req.Name != "" {
		// Only update name
		_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
			UPDATE connectors
			SET name = $1
			WHERE id = $2
		`, req.Name, id)

		if err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("Failed to update connector name")
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Failed to update connector name",
				"code":    http.StatusInternalServerError,
			})
			return
		}
	}

	// Return success
	c.JSON(http.StatusOK, gin.H{
		"message": "Connector updated successfully",
	})
}

// DeleteGlobalConnector deletes a global connector
func (s *Server) DeleteGlobalConnector(c *gin.Context) {
	// Get connector ID from URL
	idStr := c.Param("id")

	// Validate UUID
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid connector ID",
			"code":    http.StatusBadRequest,
		})
		return
	}

	// Check if connector is in use by any extensions
	var extensionCount int
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT COUNT(*)
		FROM server_extensions
		WHERE config->>'connector_id' = $1
	`, id.String()).Scan(&extensionCount)

	if err != nil {
		log.Error().Err(err).Str("id", id.String()).Msg("Failed to check if connector is in use")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to check if connector is in use",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	if extensionCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Cannot delete connector that is in use by extensions",
			"code":    http.StatusBadRequest,
		})
		return
	}

	// Delete connector from database
	_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
		DELETE FROM connectors
		WHERE id = $1
	`, id)

	if err != nil {
		log.Error().Err(err).Str("id", id.String()).Msg("Failed to delete connector")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to delete connector",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Shutdown and unregister connector instance
	if err := s.Dependencies.ConnectorManager.ShutdownConnector(id); err != nil {
		log.Error().Err(err).Str("id", id.String()).Msg("Failed to shutdown connector")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Connector deleted but failed to shutdown cleanly: " + err.Error(),
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Return success
	c.JSON(http.StatusOK, gin.H{
		"message": "Connector deleted successfully",
	})
}
