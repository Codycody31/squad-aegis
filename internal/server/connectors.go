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
	Config map[string]interface{} `json:"config"`
}

// ConnectorListGlobalResponse represents the response for the list global connectors endpoint
type ConnectorListGlobalResponse struct {
	Connectors []ConnectorResponse `json:"connectors"`
}

// ConnectorCreateRequest represents the request to create a new connector
type ConnectorCreateRequest struct {
	Name   string                 `json:"name" binding:"required"`
	Config map[string]interface{} `json:"config" binding:"required"`
}

// ConnectorUpdateRequest represents the request to update a connector
type ConnectorUpdateRequest struct {
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
		if name == "discord" {
			if _, ok := config["token"]; ok {
				config["token"] = "********"
			}
		}

		// Add to response
		resp.Connectors = append(resp.Connectors, ConnectorResponse{
			ID:     id.String(),
			Name:   name,
			Config: config,
		})
	}

	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating connector rows")
	}

	responses.Success(c, "Global connectors fetched successfully", &gin.H{
		"connectors": resp.Connectors,
	})
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
	var configJSON []byte

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT name, config
		FROM connectors
		WHERE id = $1
	`, id).Scan(&name, &configJSON)

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
	if name == "discord" {
		if _, ok := config["token"]; ok {
			config["token"] = "********"
		}
	}

	// Build response
	resp := ConnectorResponse{
		ID:     id.String(),
		Name:   name,
		Config: config,
	}

	responses.Success(c, "Global connector fetched successfully", &gin.H{
		"connector": resp,
	})
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

	// Validate connector type using name
	registrar, ok := s.Dependencies.ConnectorManager.GetConnector(req.Name)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid connector name",
			"code":    http.StatusBadRequest,
		})
		return
	}

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
		INSERT INTO connectors (id, name, config)
		VALUES ($1, $2, $3)
	`, id, req.Name, configJSON)

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
	_, err = def.CreateInstance(id, req.Config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create connector instance")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to initialize connector",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Get user from session
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	// Create audit log entry
	auditData := map[string]interface{}{
		"connectorId": id.String(),
		"name":        req.Name,
		"config":      req.Config,
	}
	s.CreateAuditLog(c.Request.Context(), nil, &user.Id, "connector:create", auditData)

	responses.Success(c, "Global connector created successfully", &gin.H{
		"connector": ConnectorResponse{
			ID:     id.String(),
			Name:   req.Name,
			Config: req.Config,
		},
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

	// Get connector to determine its type
	connector, exists := s.Dependencies.ConnectorManager.GetConnectorByID(id)
	if !exists {
		log.Error().Str("id", id.String()).Msg("Connector not found")
		c.JSON(http.StatusNotFound, gin.H{
			"message": "Connector not found",
			"code":    http.StatusNotFound,
		})
		return
	}

	// Get the connector type from the connector instance
	connectorType := connector.GetType()

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

	// Find all server extensions that require this connector type
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
		SELECT id, server_id, name, enabled
		FROM server_extensions
	`)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query server extensions")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to check connector dependencies",
			"code":    http.StatusInternalServerError,
		})
		return
	}
	defer rows.Close()

	// Track extensions that will be disabled
	disabledExtensions := []map[string]interface{}{}

	// Process each extension to check if it requires the connector type
	for rows.Next() {
		var extID uuid.UUID
		var serverID uuid.UUID
		var extName string
		var enabled bool

		if err := rows.Scan(&extID, &serverID, &extName, &enabled); err != nil {
			log.Error().Err(err).Msg("Failed to scan extension row")
			continue
		}

		// Skip already disabled extensions
		if !enabled {
			continue
		}

		// Get extension registrar
		registrar, ok := s.Dependencies.ExtensionManager.GetExtension(extName)
		if !ok {
			log.Warn().Str("extension", extName).Msg("Extension registrar not found")
			continue
		}

		// Get extension definition
		def := registrar.Define()

		// Check if this extension requires the connector type being deleted
		requiresConnector := false
		for _, reqConnector := range def.RequiredConnectors {
			if reqConnector == connectorType {
				requiresConnector = true
				break
			}
		}

		// If extension requires the connector, disable it
		if requiresConnector {
			// Try to shut down the extension first
			err := s.Dependencies.ExtensionManager.ShutdownExtension(serverID, def.ID)
			if err != nil {
				log.Warn().
					Err(err).
					Str("extension", extName).
					Str("serverID", serverID.String()).
					Msg("Error shutting down extension that depends on connector")
			}

			// Update the database to mark extension as disabled
			_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
				UPDATE server_extensions
				SET enabled = false
				WHERE id = $1
			`, extID)

			if err != nil {
				log.Error().
					Err(err).
					Str("extension", extName).
					Str("serverID", serverID.String()).
					Msg("Failed to disable extension that depends on connector")
			} else {
				// Track disabled extensions for audit log
				disabledExtensions = append(disabledExtensions, map[string]interface{}{
					"id":        extID.String(),
					"name":      extName,
					"server_id": serverID.String(),
				})

				log.Info().
					Str("extension", extName).
					Str("serverID", serverID.String()).
					Str("connectorType", connectorType).
					Msg("Disabled extension that depends on connector")
			}
		}
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

	// Get user from session
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	// Create audit log entry
	auditData := map[string]interface{}{
		"connectorId":        id.String(),
		"connectorType":      connectorType,
		"disabledExtensions": disabledExtensions,
	}
	s.CreateAuditLog(c.Request.Context(), nil, &user.Id, "connector:delete", auditData)

	// Return success with information about disabled extensions
	if len(disabledExtensions) > 0 {
		responses.Success(c, "Global connector deleted successfully. Some extensions were disabled.", &gin.H{
			"disabled_extensions": disabledExtensions,
		})
	} else {
		responses.Success(c, "Global connector deleted successfully", nil)
	}
}
