package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/core"
	"go.codycody31.dev/squad-aegis/internal/connector_manager"
	"go.codycody31.dev/squad-aegis/internal/extension_manager"
	"go.codycody31.dev/squad-aegis/internal/models"
)

// ExtensionListAvailableTypesResponse represents the response for the list available extension types endpoint
type ExtensionListAvailableTypesResponse struct {
	Types map[string]map[string]interface{} `json:"types"`
}

// ExtensionResponse represents an extension in API responses
type ExtensionResponse struct {
	ID       string                 `json:"id"`
	ServerID string                 `json:"server_id"`
	Name     string                 `json:"name"`
	Enabled  bool                   `json:"enabled"`
	Config   map[string]interface{} `json:"config"`
}

// ExtensionListResponse represents the response for the list extensions endpoint
type ExtensionListResponse struct {
	Extensions []ExtensionResponse `json:"extensions"`
}

// ExtensionCreateRequest represents the request to create a new extension
type ExtensionCreateRequest struct {
	Name    string                 `json:"name" binding:"required"`
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config" binding:"required"`
}

// ExtensionUpdateRequest represents the request to update an extension
type ExtensionUpdateRequest struct {
	Enabled *bool                  `json:"enabled"`
	Config  map[string]interface{} `json:"config"`
}

// ListExtensionTypes lists all available extension types
func (s *Server) ListExtensionTypes(c *gin.Context) {
	// Get registered extensions from extension manager
	extensions := s.Dependencies.ExtensionManager.ListExtensions()

	// Build response with all schemas
	resp := ExtensionListAvailableTypesResponse{
		Types: make(map[string]map[string]interface{}),
	}

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

		resp.Types[extension.Name] = schemaMap
	}

	c.JSON(http.StatusOK, resp)
}

// ListServerExtensions returns all extensions for a server
func (s *Server) ListServerExtensions(c *gin.Context) {
	// Get server ID from URL
	serverIDStr := c.Param("serverId")

	// Validate UUID
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid server ID",
			"code":    http.StatusBadRequest,
		})
		return
	}

	// Get extensions from database
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
		SELECT id, server_id, name, enabled, config
		FROM server_extensions
		WHERE server_id = $1
		ORDER BY name
	`, serverID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query server extensions")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to query server extensions",
			"code":    http.StatusInternalServerError,
		})
		return
	}
	defer rows.Close()

	// Build response
	resp := ExtensionListResponse{
		Extensions: []ExtensionResponse{},
	}

	for rows.Next() {
		var id uuid.UUID
		var servID uuid.UUID
		var name string
		var enabled bool
		var configJSON []byte

		if err := rows.Scan(&id, &servID, &name, &enabled, &configJSON); err != nil {
			log.Error().Err(err).Msg("Failed to scan extension row")
			continue
		}

		// Parse config JSON
		var config map[string]interface{}
		if err := json.Unmarshal(configJSON, &config); err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("Failed to unmarshal extension config")
			continue
		}

		// Add to response
		resp.Extensions = append(resp.Extensions, ExtensionResponse{
			ID:       id.String(),
			ServerID: servID.String(),
			Name:     name,
			Enabled:  enabled,
			Config:   config,
		})
	}

	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating extension rows")
	}

	c.JSON(http.StatusOK, resp)
}

// GetServerExtension returns a specific extension for a server
func (s *Server) GetServerExtension(c *gin.Context) {
	// Get server ID and extension ID from URL
	serverIDStr := c.Param("serverId")
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid server ID",
		})
		return
	}

	extensionIDStr := c.Param("extensionId")
	eID, err := uuid.Parse(extensionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid extension ID",
			"code":    http.StatusBadRequest,
		})
		return
	}

	// Get extension from database
	var servID uuid.UUID
	var name string
	var enabled bool
	var configJSON []byte

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT server_id, name, enabled, config
		FROM server_extensions
		WHERE id = $1 AND server_id = $2
	`, eID, serverID).Scan(&servID, &name, &enabled, &configJSON)

	if err != nil {
		log.Error().Err(err).Str("id", eID.String()).Str("serverID", serverID.String()).Msg("Failed to get extension")
		c.JSON(http.StatusNotFound, gin.H{
			"message": "Extension not found",
			"code":    http.StatusNotFound,
		})
		return
	}

	// Parse config JSON
	var config map[string]interface{}
	if err := json.Unmarshal(configJSON, &config); err != nil {
		log.Error().Err(err).Str("id", eID.String()).Msg("Failed to unmarshal extension config")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to parse extension config",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Build response
	resp := ExtensionResponse{
		ID:       eID.String(),
		ServerID: servID.String(),
		Name:     name,
		Enabled:  enabled,
		Config:   config,
	}

	c.JSON(http.StatusOK, resp)
}

// CreateServerExtension creates a new extension for a server
func (s *Server) CreateServerExtension(c *gin.Context) {
	// Get server ID from URL
	serverIDStr := c.Param("serverId")

	// Validate UUID
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid server ID",
			"code":    http.StatusBadRequest,
		})
		return
	}

	var req ExtensionCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid request body",
			"code":    http.StatusBadRequest,
		})
		return
	}

	// Validate extension type
	registrar, ok := s.Dependencies.ExtensionManager.GetExtension(req.Name)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid extension type",
			"code":    http.StatusBadRequest,
		})
		return
	}

	// Check if a connector_id is specified and verify it exists
	if connectorIDStr, ok := req.Config["connector_id"].(string); ok {
		connectorID, err := uuid.Parse(connectorIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Invalid connector ID format",
				"code":    http.StatusBadRequest,
			})
			return
		}

		// Check if connector exists
		var exists bool
		err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
			SELECT EXISTS(SELECT 1 FROM connectors WHERE id = $1)
		`, connectorID).Scan(&exists)

		if err != nil {
			log.Error().Err(err).Msg("Failed to check if connector exists")
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Failed to validate connector",
				"code":    http.StatusInternalServerError,
			})
			return
		}

		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Specified connector does not exist",
				"code":    http.StatusBadRequest,
			})
			return
		}
	}

	// Convert config to JSON
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal extension config")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to process extension config",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Generate UUID for new extension
	id := uuid.New()

	// Create extension in database
	_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
		INSERT INTO server_extensions (id, server_id, name, enabled, config)
		VALUES ($1, $2, $3, $4, $5)
	`, id, serverID, req.Name, req.Enabled, configJSON)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create extension")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to create extension",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// If enabled, initialize the extension
	if req.Enabled {
		// Get extension definition
		extensionDef := registrar.Define()

		// Get server
		server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverID, nil)
		if err != nil {
			log.Error().Err(err).Str("serverID", serverID.String()).Msg("Failed to get server")
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Failed to get server",
				"code":    http.StatusInternalServerError,
			})
			return
		}

		// Create the extension instance
		instance := extensionDef.CreateInstance()

		// Create dependencies
		deps, err := s.createExtensionDependencies(extensionDef, server)
		if err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("Failed to create extension dependencies")

			// Update database to mark as disabled
			_, dbErr := s.Dependencies.DB.ExecContext(c.Request.Context(), `
				UPDATE server_extensions
				SET enabled = false
				WHERE id = $1
			`, id)
			if dbErr != nil {
				log.Error().Err(dbErr).Str("id", id.String()).Msg("Failed to update extension enabled status")
			}

			c.JSON(http.StatusCreated, gin.H{
				"id":      id.String(),
				"message": "Extension created but not enabled: " + err.Error(),
				"status":  "warning",
			})
			return
		}

		// Initialize the extension
		if err := instance.Initialize(req.Config, deps); err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("Failed to initialize extension")

			// Update database to mark as disabled
			_, dbErr := s.Dependencies.DB.ExecContext(c.Request.Context(), `
				UPDATE server_extensions
				SET enabled = false
				WHERE id = $1
			`, id)
			if dbErr != nil {
				log.Error().Err(dbErr).Str("id", id.String()).Msg("Failed to update extension enabled status")
			}

			c.JSON(http.StatusCreated, gin.H{
				"id":      id.String(),
				"message": "Extension created but failed to initialize: " + err.Error(),
				"status":  "warning",
			})
			return
		}
	}

	// Return success
	c.JSON(http.StatusCreated, gin.H{
		"id":      id.String(),
		"message": "Extension created successfully",
	})
}

// UpdateServerExtension updates an existing extension for a server
func (s *Server) UpdateServerExtension(c *gin.Context) {
	// Get server ID and extension ID from URL
	serverIDStr := c.Param("serverId")
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid server ID",
		})
		return
	}

	extensionIDStr := c.Param("extensionId")
	eID, err := uuid.Parse(extensionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid extension ID",
			"code":    http.StatusBadRequest,
		})
		return
	}

	var req ExtensionUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid request body",
			"code":    http.StatusBadRequest,
		})
		return
	}

	// Get current extension info from database
	var name string
	var enabled bool
	var configJSON []byte

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT name, enabled, config
		FROM server_extensions
		WHERE id = $1 AND server_id = $2
	`, eID, serverID).Scan(&name, &enabled, &configJSON)

	if err != nil {
		log.Error().Err(err).Str("id", eID.String()).Str("serverID", serverID.String()).Msg("Failed to get extension")
		c.JSON(http.StatusNotFound, gin.H{
			"message": "Extension not found",
			"code":    http.StatusNotFound,
		})
		return
	}

	// Parse current config JSON
	var currentConfig map[string]interface{}
	if err := json.Unmarshal(configJSON, &currentConfig); err != nil {
		log.Error().Err(err).Str("id", eID.String()).Msg("Failed to unmarshal extension config")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to parse extension config",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Check if we need to update the enabled status
	enabledChanged := false
	newEnabled := enabled
	if req.Enabled != nil && *req.Enabled != enabled {
		enabledChanged = true
		newEnabled = *req.Enabled
	}

	// Update config if provided
	configChanged := false
	newConfig := currentConfig
	if req.Config != nil {
		configChanged = true
		newConfig = req.Config
	}

	// If we're not changing anything, just return success
	if !enabledChanged && !configChanged {
		c.JSON(http.StatusOK, gin.H{
			"message": "No changes requested",
		})
		return
	}

	// If we're updating config, convert to JSON
	if configChanged {
		configJSON, err = json.Marshal(newConfig)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal extension config")
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Failed to process extension config",
				"code":    http.StatusInternalServerError,
			})
			return
		}
	}

	// Update extension in database
	if enabledChanged && configChanged {
		_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
			UPDATE server_extensions
			SET enabled = $1, config = $2
			WHERE id = $3
		`, newEnabled, configJSON, eID)
	} else if enabledChanged {
		_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
			UPDATE server_extensions
			SET enabled = $1
			WHERE id = $2
		`, newEnabled, eID)
	} else if configChanged {
		_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
			UPDATE server_extensions
			SET config = $1
			WHERE id = $2
		`, configJSON, eID)
	}

	if err != nil {
		log.Error().Err(err).Str("id", eID.String()).Msg("Failed to update extension")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to update extension",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Get server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverID, nil)
	if err != nil {
		log.Error().Err(err).Str("serverID", serverID.String()).Msg("Failed to get server")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to get server",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Get extension registrar
	registrar, ok := s.Dependencies.ExtensionManager.GetExtension(name)
	if !ok {
		log.Error().Str("name", name).Msg("Extension registrar not found")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Extension type not found",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Get extension definition
	extensionDef := registrar.Define()

	// If we're enabling, initialize
	if enabledChanged && newEnabled {
		// Create the extension instance
		instance := extensionDef.CreateInstance()

		// Create dependencies
		deps, err := s.createExtensionDependencies(extensionDef, server)
		if err != nil {
			log.Error().Err(err).Str("id", eID.String()).Msg("Failed to create extension dependencies")

			// Update database to mark as disabled
			_, dbErr := s.Dependencies.DB.ExecContext(c.Request.Context(), `
				UPDATE server_extensions
				SET enabled = false
				WHERE id = $1
			`, eID)
			if dbErr != nil {
				log.Error().Err(dbErr).Str("id", eID.String()).Msg("Failed to update extension enabled status")
			}

			c.JSON(http.StatusOK, gin.H{
				"message": "Extension updated but failed to initialize: " + err.Error(),
				"status":  "warning",
			})
			return
		}

		// Initialize extension
		if err := instance.Initialize(newConfig, deps); err != nil {
			log.Error().Err(err).Str("id", eID.String()).Msg("Failed to initialize extension")

			// Update database to mark as disabled
			_, dbErr := s.Dependencies.DB.ExecContext(c.Request.Context(), `
				UPDATE server_extensions
				SET enabled = false
				WHERE id = $1
			`, eID)
			if dbErr != nil {
				log.Error().Err(dbErr).Str("id", eID.String()).Msg("Failed to update extension enabled status")
			}

			c.JSON(http.StatusOK, gin.H{
				"message": "Extension updated but failed to initialize: " + err.Error(),
				"status":  "warning",
			})
			return
		}
	}

	// If we're disabling, shutdown
	if enabledChanged && !newEnabled {
		// We would shut down the extension here
		log.Info().Str("id", eID.String()).Msg("Disabling extension")
	}

	// If we're updating config and staying enabled, restart
	if configChanged && newEnabled {
		// Restart would involve getting the current instance, shutting it down, and reinitializing
		// For now, just log a message
		log.Info().Str("id", eID.String()).Msg("Updating extension config")
	}

	// Return success
	c.JSON(http.StatusOK, gin.H{
		"message": "Extension updated successfully",
	})
}

// DeleteServerExtension deletes an extension from a server
func (s *Server) DeleteServerExtension(c *gin.Context) {
	// Get server ID and extension ID from URL
	serverIDStr := c.Param("serverId")
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid server ID",
		})
		return
	}

	extensionIDStr := c.Param("extensionId")
	eID, err := uuid.Parse(extensionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid extension ID",
			"code":    http.StatusBadRequest,
		})
		return
	}

	// Check if extension exists
	var exists bool
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT EXISTS(SELECT 1 FROM server_extensions WHERE id = $1 AND server_id = $2)
	`, eID, serverID).Scan(&exists)

	if err != nil {
		log.Error().Err(err).Str("id", eID.String()).Str("serverID", serverID.String()).Msg("Failed to check if extension exists")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to check if extension exists",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "Extension not found",
			"code":    http.StatusNotFound,
		})
		return
	}

	// For shutdown extension if it's running, we'll just log the attempt
	log.Info().Str("id", eID.String()).Msg("Shutting down extension")

	// Delete extension from database
	_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
		DELETE FROM server_extensions
		WHERE id = $1 AND server_id = $2
	`, eID, serverID)

	if err != nil {
		log.Error().Err(err).Str("id", eID.String()).Str("serverID", serverID.String()).Msg("Failed to delete extension")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to delete extension",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Return success
	c.JSON(http.StatusOK, gin.H{
		"message": "Extension deleted successfully",
	})
}

// ToggleServerExtension toggles an extension's enabled status
func (s *Server) ToggleServerExtension(c *gin.Context) {
	// Get server ID and extension ID from URL
	serverIDStr := c.Param("serverId")
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid server ID",
		})
		return
	}

	extensionIDStr := c.Param("extensionId")
	eID, err := uuid.Parse(extensionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid extension ID",
			"code":    http.StatusBadRequest,
		})
		return
	}

	// Get current extension info from database
	var name string
	var enabled bool
	var configJSON []byte

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT name, enabled, config
		FROM server_extensions
		WHERE id = $1 AND server_id = $2
	`, eID, serverID).Scan(&name, &enabled, &configJSON)

	if err != nil {
		log.Error().Err(err).Str("id", eID.String()).Str("serverID", serverID.String()).Msg("Failed to get extension")
		c.JSON(http.StatusNotFound, gin.H{
			"message": "Extension not found",
			"code":    http.StatusNotFound,
		})
		return
	}

	// Parse current config JSON
	var currentConfig map[string]interface{}
	if err := json.Unmarshal(configJSON, &currentConfig); err != nil {
		log.Error().Err(err).Str("id", eID.String()).Msg("Failed to unmarshal extension config")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to parse extension config",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Toggle enabled status
	newEnabled := !enabled

	// Update extension in database
	_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
		UPDATE server_extensions
		SET enabled = $1
		WHERE id = $2
	`, newEnabled, eID)

	if err != nil {
		log.Error().Err(err).Str("id", eID.String()).Msg("Failed to update extension enabled status")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to update extension enabled status",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Get server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverID, nil)
	if err != nil {
		log.Error().Err(err).Str("serverID", serverID.String()).Msg("Failed to get server")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to get server",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Get extension registrar
	registrar, ok := s.Dependencies.ExtensionManager.GetExtension(name)
	if !ok {
		log.Error().Str("name", name).Msg("Extension registrar not found")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Extension type not found",
			"code":    http.StatusInternalServerError,
		})
		return
	}

	// Get extension definition
	extensionDef := registrar.Define()

	// If we're enabling, initialize
	if newEnabled {
		// Create the extension instance
		instance := extensionDef.CreateInstance()

		// Create dependencies
		deps, err := s.createExtensionDependencies(extensionDef, server)
		if err != nil {
			log.Error().Err(err).Str("id", eID.String()).Msg("Failed to create extension dependencies")

			// Update database to mark as disabled
			_, dbErr := s.Dependencies.DB.ExecContext(c.Request.Context(), `
				UPDATE server_extensions
				SET enabled = false
				WHERE id = $1
			`, eID)
			if dbErr != nil {
				log.Error().Err(dbErr).Str("id", eID.String()).Msg("Failed to update extension enabled status")
			}

			c.JSON(http.StatusOK, gin.H{
				"message": "Extension updated but failed to initialize: " + err.Error(),
				"status":  "warning",
			})
			return
		}

		// Initialize extension
		if err := instance.Initialize(currentConfig, deps); err != nil {
			log.Error().Err(err).Str("id", eID.String()).Msg("Failed to initialize extension")

			// Update database to mark as disabled
			_, dbErr := s.Dependencies.DB.ExecContext(c.Request.Context(), `
				UPDATE server_extensions
				SET enabled = false
				WHERE id = $1
			`, eID)
			if dbErr != nil {
				log.Error().Err(dbErr).Str("id", eID.String()).Msg("Failed to update extension enabled status")
			}

			c.JSON(http.StatusOK, gin.H{
				"message": "Extension updated but failed to initialize: " + err.Error(),
				"status":  "warning",
			})
			return
		}
	}

	// If we're disabling, shutdown
	if !newEnabled {
		// We would shut down the extension here
		log.Info().Str("id", eID.String()).Msg("Disabling extension")
	}

	// Return success
	c.JSON(http.StatusOK, gin.H{
		"message": "Extension updated successfully",
	})
}

// Helper function to create extension dependencies
func (s *Server) createExtensionDependencies(def extension_manager.ExtensionDefinition, server *models.Server) (*extension_manager.Dependencies, error) {
	deps := &extension_manager.Dependencies{
		Database:    s.Dependencies.DB,
		Server:      server,
		RconManager: s.Dependencies.RconManager,
		Connectors:  make(map[string]connector_manager.ConnectorInstance),
	}

	// Check required dependencies
	for _, depType := range def.Dependencies.Required {
		switch depType {
		case extension_manager.DependencyDatabase:
			if deps.Database == nil {
				return nil, fmt.Errorf("required dependency not available: database")
			}
		case extension_manager.DependencyServer:
			if deps.Server == nil {
				return nil, fmt.Errorf("required dependency not available: server")
			}
		case extension_manager.DependencyRconManager:
			if deps.RconManager == nil {
				return nil, fmt.Errorf("required dependency not available: rcon_manager")
			}
		case extension_manager.DependencyConnectors:
			// Get server connectors
			serverConnectors := s.Dependencies.ConnectorManager.GetConnectorsByServer(server.Id)
			for _, connector := range serverConnectors {
				deps.Connectors[connector.GetType()] = connector
			}
		}
	}

	return deps, nil
}
