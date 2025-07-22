package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/connector_manager"
	"go.codycody31.dev/squad-aegis/internal/core"
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
	Version     string                 `json:"version"`
	Author      string                 `json:"author"`
	Scope       string                 `json:"scope"`
	Flags       ConnectorFlags         `json:"flags"`
	Schema      map[string]interface{} `json:"schema"`
}

// ConnectorFlags represents the flags for a connector in API responses
type ConnectorFlags struct {
	ImplementsEvents bool `json:"implements_events"`
}

// ConnectorDefinitionsResponse represents the response for the list definitions endpoint
type ConnectorDefinitionsResponse struct {
	Definitions []ConnectorDefinitionResponse `json:"definitions"`
}

// ConnectorResponse represents a connector in API responses
type ConnectorResponse struct {
	ID          string                 `json:"id"`
	ServerID    *string                `json:"server_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	Author      string                 `json:"author"`
	Scope       string                 `json:"scope"`
	Config      map[string]interface{} `json:"config"`
}

// ConnectorListResponse represents the response for the list connectors endpoint
type ConnectorListResponse struct {
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
	// Get scope filter from query params
	scopeFilter := c.Query("scope")

	// Get connector definitions from connector manager
	connectorDefs := s.Dependencies.ConnectorManager.ListConnectors()

	// Build response with connector definitions
	definitionResponses := make([]ConnectorDefinitionResponse, 0, len(connectorDefs))

	for _, def := range connectorDefs {
		// Apply scope filter if provided
		if scopeFilter != "" && string(def.Scope) != scopeFilter {
			continue
		}

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

			// Add options if present
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
			Version:     def.Version,
			Author:      def.Author,
			Scope:       string(def.Scope),
			Flags: ConnectorFlags{
				ImplementsEvents: def.Flags.ImplementsEvents,
			},
			Schema: schemaMap,
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
	resp := ConnectorListResponse{
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

	// Get connector definition
	def := registrar.Define()

	// Validate config against schema
	if err := def.ConfigSchema.Validate(req.Config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid connector configuration: " + err.Error(),
			"code":    http.StatusBadRequest,
		})
		return
	}

	// Generate new UUID for the connector
	id := uuid.New()

	// Convert config to JSON
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal connector config")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to process connector configuration",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Insert into database
	_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
		INSERT INTO connectors (id, name, config)
		VALUES ($1, $2, $3)
	`, id, req.Name, configJSON)

	if err != nil {
		log.Error().Err(err).Msg("Failed to insert connector")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to create connector",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Create and initialize connector instance
	instance := def.CreateInstance()
	if err := instance.Initialize(req.Config); err != nil {
		log.Error().Err(err).Str("id", id.String()).Msg("Failed to initialize connector")

		// Cleanup database entry on initialization failure
		_, cleanupErr := s.Dependencies.DB.ExecContext(c.Request.Context(), `
			DELETE FROM connectors WHERE id = $1
		`, id)
		if cleanupErr != nil {
			log.Error().Err(cleanupErr).Str("id", id.String()).Msg("Failed to cleanup failed connector")
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to initialize connector: " + err.Error(),
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Build response
	resp := ConnectorResponse{
		ID:     id.String(),
		Name:   req.Name,
		Config: req.Config,
	}

	// Remove sensitive data from response
	// For Discord, remove token
	if req.Name == "discord" {
		if _, ok := resp.Config["token"]; ok {
			resp.Config["token"] = "********"
		}
	}

	responses.Success(c, "Global connector created successfully", &gin.H{
		"connector": resp,
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
	connectorType := connector.GetDefinition().ID

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

	// Process each extension to check if it requires the connector type being deleted
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

// GetConnectorsByType returns all connectors of a specific type
func (s *Server) GetConnectorsByType(c *gin.Context) {
	// Get user from session
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	// Get connector type from URL
	connectorType := c.Param("type")
	if connectorType == "" {
		responses.BadRequest(c, "Missing connector type", nil)
		return
	}

	// Get all connectors of this type
	connectors := s.Dependencies.ConnectorManager.GetConnectorsByType(connectorType)

	// Build response
	response := make([]ConnectorResponse, 0)
	for _, connector := range connectors {
		def := connector.GetDefinition()
		response = append(response, ConnectorResponse{
			ID:          def.ID,
			Name:        def.Name,
			Description: def.Description,
			Version:     def.Version,
			Author:      def.Author,
			Scope:       string(def.Scope),
		})
	}

	responses.Success(c, "Connectors fetched successfully", &gin.H{
		"connectors": response,
	})
}

// GlobalConnectorsList returns all global connectors
func (s *Server) GlobalConnectorsList(c *gin.Context) {
	// Get user from session
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	// Get all connectors
	connectors := s.Dependencies.ConnectorManager.ListConnectors()

	// Filter to only global connectors and build response
	response := make([]ConnectorResponse, 0)
	for _, def := range connectors {
		if def.Scope == connector_manager.ConnectorScopeGlobal {
			response = append(response, ConnectorResponse{
				ID:          def.ID,
				Name:        def.Name,
				Description: def.Description,
				Version:     def.Version,
				Author:      def.Author,
				Scope:       string(def.Scope),
			})
		}
	}

	responses.Success(c, "Global connectors fetched successfully", &gin.H{
		"connectors": response,
	})
}

// ServerConnectorsList returns all connectors for a server
func (s *Server) ServerConnectorsList(c *gin.Context) {
	// Get server ID from URL
	serverIDStr := c.Param("serverId")

	// Validate UUID
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", nil)
		return
	}

	// Get user from session
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	// Check if user has access to server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverID, user)
	if err != nil {
		responses.NotFound(c, "Server not found", nil)
		return
	}

	// Get query parameter for including global connectors
	includeGlobal := c.Query("include_global") == "true"

	// Build response
	response := make([]ConnectorResponse, 0)

	// First get server-specific connectors
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
		SELECT id, name, config
		FROM server_connectors
		WHERE server_id = $1
		ORDER BY name
	`, server.Id)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query server connectors")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to fetch server connectors"})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var name string
		var configJSON []byte

		if err := rows.Scan(&id, &name, &configJSON); err != nil {
			log.Error().Err(err).Msg("Failed to scan server connector row")
			continue
		}

		// Get connector definition for metadata
		registrar, ok := s.Dependencies.ConnectorManager.GetConnector(name)
		if !ok {
			log.Warn().Str("name", name).Msg("Connector type not found")
			continue
		}
		def := registrar.Define()

		// Parse config JSON
		var config map[string]interface{}
		if err := json.Unmarshal(configJSON, &config); err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("Failed to unmarshal connector config")
			continue
		}

		serverIDStr := server.Id.String()
		response = append(response, ConnectorResponse{
			ID:          id.String(),
			ServerID:    &serverIDStr,
			Name:        name,
			Description: def.Description,
			Version:     def.Version,
			Author:      def.Author,
			Scope:       string(def.Scope),
			Config:      config,
		})
	}

	if includeGlobal {
		// Get global connectors
		rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
			SELECT id, name, config
			FROM connectors
			ORDER BY name
		`)
		if err != nil {
			log.Error().Err(err).Msg("Failed to query global connectors")
			responses.InternalServerError(c, err, &gin.H{"error": "Failed to fetch global connectors"})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var id uuid.UUID
			var name string
			var configJSON []byte

			if err := rows.Scan(&id, &name, &configJSON); err != nil {
				log.Error().Err(err).Msg("Failed to scan global connector row")
				continue
			}

			// Get connector definition for metadata
			registrar, ok := s.Dependencies.ConnectorManager.GetConnector(name)
			if !ok {
				log.Warn().Str("name", name).Msg("Connector type not found")
				continue
			}
			def := registrar.Define()

			// Parse config JSON
			var config map[string]interface{}
			if err := json.Unmarshal(configJSON, &config); err != nil {
				log.Error().Err(err).Str("id", id.String()).Msg("Failed to unmarshal connector config")
				continue
			}

			response = append(response, ConnectorResponse{
				ID:          id.String(),
				Name:        name,
				Description: def.Description,
				Version:     def.Version,
				Author:      def.Author,
				Scope:       string(def.Scope),
				Config:      config,
			})
		}
	}

	responses.Success(c, "Server connectors fetched successfully", &gin.H{
		"connectors": response,
	})
}

// ConnectorsList returns all connectors, optionally filtered by scope
func (s *Server) ConnectorsList(c *gin.Context) {
	// Get user from session
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	// Get scope filter from query
	scope := c.Query("scope")
	if scope != "" && scope != string(connector_manager.ConnectorScopeGlobal) && scope != string(connector_manager.ConnectorScopeServer) {
		responses.BadRequest(c, "Invalid scope filter", nil)
		return
	}

	// Build response
	response := make([]ConnectorResponse, 0)

	// Query based on scope
	if scope == "" || scope == string(connector_manager.ConnectorScopeGlobal) {
		// Get global connectors
		rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
			SELECT id, name, config
			FROM connectors
			ORDER BY name
		`)
		if err != nil {
			log.Error().Err(err).Msg("Failed to query global connectors")
			responses.InternalServerError(c, err, &gin.H{"error": "Failed to fetch global connectors"})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var id uuid.UUID
			var name string
			var configJSON []byte

			if err := rows.Scan(&id, &name, &configJSON); err != nil {
				log.Error().Err(err).Msg("Failed to scan global connector row")
				continue
			}

			// Get connector definition for metadata
			registrar, ok := s.Dependencies.ConnectorManager.GetConnector(name)
			if !ok {
				log.Warn().Str("name", name).Msg("Connector type not found")
				continue
			}
			def := registrar.Define()

			// Parse config JSON
			var config map[string]interface{}
			if err := json.Unmarshal(configJSON, &config); err != nil {
				log.Error().Err(err).Str("id", id.String()).Msg("Failed to unmarshal connector config")
				continue
			}

			response = append(response, ConnectorResponse{
				ID:          id.String(),
				Name:        name,
				Description: def.Description,
				Version:     def.Version,
				Author:      def.Author,
				Scope:       string(def.Scope),
				Config:      config,
			})
		}
	}

	if scope == "" || scope == string(connector_manager.ConnectorScopeServer) {
		// Get server connectors
		rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
			SELECT sc.id, sc.server_id, sc.name, sc.config
			FROM server_connectors sc
			JOIN servers s ON s.id = sc.server_id
			JOIN server_admins sa ON sa.server_id = s.id
			WHERE sa.user_id = $1
			ORDER BY sc.name
		`, user.Id)
		if err != nil {
			log.Error().Err(err).Msg("Failed to query server connectors")
			responses.InternalServerError(c, err, &gin.H{"error": "Failed to fetch server connectors"})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var id uuid.UUID
			var serverID uuid.UUID
			var name string
			var configJSON []byte

			if err := rows.Scan(&id, &serverID, &name, &configJSON); err != nil {
				log.Error().Err(err).Msg("Failed to scan server connector row")
				continue
			}

			// Get connector definition for metadata
			registrar, ok := s.Dependencies.ConnectorManager.GetConnector(name)
			if !ok {
				log.Warn().Str("name", name).Msg("Connector type not found")
				continue
			}
			def := registrar.Define()

			// Parse config JSON
			var config map[string]interface{}
			if err := json.Unmarshal(configJSON, &config); err != nil {
				log.Error().Err(err).Str("id", id.String()).Msg("Failed to unmarshal connector config")
				continue
			}

			serverIDStr := serverID.String()
			response = append(response, ConnectorResponse{
				ID:          id.String(),
				ServerID:    &serverIDStr,
				Name:        name,
				Description: def.Description,
				Version:     def.Version,
				Author:      def.Author,
				Scope:       string(def.Scope),
				Config:      config,
			})
		}
	}

	responses.Success(c, "Connectors fetched successfully", &gin.H{
		"connectors": response,
	})
}

// CreateServerConnector creates a new server-specific connector
func (s *Server) CreateServerConnector(c *gin.Context) {
	// Get server ID from URL
	serverIDStr := c.Param("serverId")
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", nil)
		return
	}

	// Get user from session
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	// Check if user has access to server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverID, user)
	if err != nil {
		responses.NotFound(c, "Server not found", nil)
		return
	}

	var req ConnectorCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responses.BadRequest(c, "Invalid request body", nil)
		return
	}

	// Validate connector type using name
	registrar, ok := s.Dependencies.ConnectorManager.GetConnector(req.Name)
	if !ok {
		responses.BadRequest(c, "Invalid connector name", nil)
		return
	}

	// Get connector definition
	def := registrar.Define()

	// Verify this is a server-scoped connector
	if def.Scope != connector_manager.ConnectorScopeServer {
		responses.BadRequest(c, "This connector type cannot be created for a specific server", nil)
		return
	}

	// Validate config against schema
	if err := def.ConfigSchema.Validate(req.Config); err != nil {
		responses.BadRequest(c, "Invalid connector configuration: "+err.Error(), nil)
		return
	}

	serverConnectors := s.Dependencies.ConnectorManager.GetConnectorsByServer(server.Id)
	for _, connector := range serverConnectors {
		if connector.GetDefinition().ID == req.Name {
			responses.BadRequest(c, "Connector already exists for this server", nil)
			return
		}
	}

	// Convert config to JSON
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal connector config")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to process connector configuration"})
		return
	}

	serverConnectorId, err := s.Dependencies.ConnectorManager.CreateServerConnector(server.Id, req.Name, req.Config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create server connector")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to create connector"})
		return
	}

	// Insert into database
	_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
		INSERT INTO server_connectors (id, server_id, name, config)
		VALUES ($1, $2, $3, $4)
	`, serverConnectorId, server.Id, req.Name, configJSON)

	if err != nil {
		log.Error().Err(err).Msg("Failed to insert server connector")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to create connector"})
		return
	}

	// Build response
	resp := ConnectorResponse{
		ID:          serverConnectorId.String(),
		ServerID:    &serverIDStr,
		Name:        req.Name,
		Description: def.Description,
		Version:     def.Version,
		Author:      def.Author,
		Scope:       string(def.Scope),
		Config:      req.Config,
	}

	// Create audit log entry
	auditData := map[string]interface{}{
		"connectorId":   serverConnectorId.String(),
		"connectorType": req.Name,
		"serverId":      server.Id.String(),
	}
	s.CreateAuditLog(c.Request.Context(), &server.Id, &user.Id, "connector:create", auditData)

	responses.Success(c, "Server connector created successfully", &gin.H{
		"connector": resp,
	})
}

// UpdateServerConnector updates a server-specific connector
func (s *Server) UpdateServerConnector(c *gin.Context) {
	// Get server ID and connector ID from URL
	serverIDStr := c.Param("serverId")
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", nil)
		return
	}

	connectorIDStr := c.Param("connectorId")
	connectorID, err := uuid.Parse(connectorIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid connector ID", nil)
		return
	}

	// Get user from session
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	// Check if user has access to server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverID, user)
	if err != nil {
		responses.NotFound(c, "Server not found", nil)
		return
	}

	var req ConnectorUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responses.BadRequest(c, "Invalid request body", nil)
		return
	}

	// Get existing connector
	var name string
	var oldConfigJSON []byte
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT name, config
		FROM server_connectors
		WHERE id = $1 AND server_id = $2
	`, connectorID, server.Id).Scan(&name, &oldConfigJSON)

	if err != nil {
		responses.NotFound(c, "Server connector not found", nil)
		return
	}

	// Get connector definition
	registrar, ok := s.Dependencies.ConnectorManager.GetConnector(name)
	if !ok {
		responses.InternalServerError(c, fmt.Errorf("connector type not found"), &gin.H{"error": "Connector type not found"})
		return
	}

	def := registrar.Define()

	// Parse old config
	var oldConfig map[string]interface{}
	if err := json.Unmarshal(oldConfigJSON, &oldConfig); err != nil {
		log.Error().Err(err).Str("id", connectorID.String()).Msg("Failed to unmarshal old config")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to process existing configuration"})
		return
	}

	// Merge old config with updates
	newConfig := oldConfig
	for k, v := range req.Config {
		newConfig[k] = v
	}

	// Validate new config
	if err := def.ConfigSchema.Validate(newConfig); err != nil {
		responses.BadRequest(c, "Invalid connector configuration: "+err.Error(), nil)
		return
	}

	// Convert new config to JSON
	newConfigJSON, err := json.Marshal(newConfig)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal new config")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to process new configuration"})
		return
	}

	// Update database
	_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
		UPDATE server_connectors
		SET config = $1
		WHERE id = $2 AND server_id = $3
	`, newConfigJSON, connectorID, server.Id)

	if err != nil {
		log.Error().Err(err).Msg("Failed to update server connector")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to update connector"})
		return
	}

	// Reinitialize connector instance
	instance := def.CreateInstance()
	if err := instance.Initialize(newConfig); err != nil {
		log.Error().Err(err).Str("id", connectorID.String()).Msg("Failed to reinitialize server connector")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to reinitialize connector: " + err.Error()})
		return
	}

	// Build response
	resp := ConnectorResponse{
		ID:          connectorID.String(),
		ServerID:    &serverIDStr,
		Name:        name,
		Description: def.Description,
		Version:     def.Version,
		Author:      def.Author,
		Scope:       string(def.Scope),
		Config:      newConfig,
	}

	// Create audit log entry
	auditData := map[string]interface{}{
		"connectorId":   connectorID.String(),
		"connectorType": name,
		"serverId":      server.Id.String(),
	}
	s.CreateAuditLog(c.Request.Context(), &server.Id, &user.Id, "connector:update", auditData)

	responses.Success(c, "Server connector updated successfully", &gin.H{
		"connector": resp,
	})
}

// DeleteServerConnector deletes a server-specific connector
func (s *Server) DeleteServerConnector(c *gin.Context) {
	// Get server ID and connector ID from URL
	serverIDStr := c.Param("serverId")
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", nil)
		return
	}

	connectorIDStr := c.Param("connectorId")
	connectorID, err := uuid.Parse(connectorIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid connector ID", nil)
		return
	}

	// Get user from session
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	// Check if user has access to server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverID, user)
	if err != nil {
		responses.NotFound(c, "Server not found", nil)
		return
	}

	// Get connector to determine its type
	var name string
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT name
		FROM server_connectors
		WHERE id = $1 AND server_id = $2
	`, connectorID, server.Id).Scan(&name)

	if err != nil {
		responses.NotFound(c, "Server connector not found", nil)
		return
	}

	// Check if connector is in use by any extensions
	var extensionCount int
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT COUNT(*)
		FROM server_extensions
		WHERE server_id = $1 AND config->>'connector_id' = $2
	`, server.Id, connectorIDStr).Scan(&extensionCount)

	if err != nil {
		log.Error().Err(err).Str("id", connectorIDStr).Msg("Failed to check if connector is in use")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to check if connector is in use"})
		return
	}

	if extensionCount > 0 {
		responses.BadRequest(c, "Cannot delete connector that is in use by extensions", nil)
		return
	}

	// Delete connector from database
	_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
		DELETE FROM server_connectors
		WHERE id = $1 AND server_id = $2
	`, connectorID, server.Id)

	if err != nil {
		log.Error().Err(err).Str("id", connectorIDStr).Msg("Failed to delete server connector")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to delete connector"})
		return
	}

	// Shutdown connector instance
	if err := s.Dependencies.ConnectorManager.ShutdownConnector(connectorID); err != nil {
		log.Error().Err(err).Str("id", connectorIDStr).Msg("Failed to shutdown server connector")
		responses.InternalServerError(c, err, &gin.H{"error": "Connector deleted but failed to shutdown cleanly: " + err.Error()})
		return
	}

	// Create audit log entry
	auditData := map[string]interface{}{
		"connectorId":   connectorID.String(),
		"connectorType": name,
		"serverId":      server.Id.String(),
	}
	s.CreateAuditLog(c.Request.Context(), &server.Id, &user.Id, "connector:delete", auditData)

	responses.Success(c, "Server connector deleted successfully", nil)
}

// GetServerConnector returns a specific server connector
func (s *Server) GetServerConnector(c *gin.Context) {
	// Get server ID and connector ID from URL
	serverIDStr := c.Param("serverId")
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", nil)
		return
	}

	connectorIDStr := c.Param("connectorId")
	connectorID, err := uuid.Parse(connectorIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid connector ID", nil)
		return
	}

	// Get user from session
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	// Check if user has access to server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverID, user)
	if err != nil {
		responses.NotFound(c, "Server not found", nil)
		return
	}

	// Get connector from database
	var name string
	var configJSON []byte
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT name, config
		FROM server_connectors
		WHERE id = $1 AND server_id = $2
	`, connectorID, server.Id).Scan(&name, &configJSON)

	if err != nil {
		responses.NotFound(c, "Server connector not found", nil)
		return
	}

	// Get connector definition
	registrar, ok := s.Dependencies.ConnectorManager.GetConnector(name)
	if !ok {
		responses.InternalServerError(c, fmt.Errorf("connector type not found"), &gin.H{"error": "Connector type not found"})
		return
	}

	def := registrar.Define()

	// Parse config JSON
	var config map[string]interface{}
	if err := json.Unmarshal(configJSON, &config); err != nil {
		log.Error().Err(err).Str("id", connectorIDStr).Msg("Failed to unmarshal connector config")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to parse connector configuration"})
		return
	}

	// Build response
	resp := ConnectorResponse{
		ID:          connectorID.String(),
		ServerID:    &serverIDStr,
		Name:        name,
		Description: def.Description,
		Version:     def.Version,
		Author:      def.Author,
		Scope:       string(def.Scope),
		Config:      config,
	}

	responses.Success(c, "Server connector fetched successfully", &gin.H{
		"connector": resp,
	})
}
