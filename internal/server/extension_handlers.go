package server

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/core"
	"go.codycody31.dev/squad-aegis/internal/connector_manager"
	"go.codycody31.dev/squad-aegis/internal/extension_manager"
	"go.codycody31.dev/squad-aegis/internal/models"
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

// ListServerExtensions returns all extensions for a server
func (s *Server) ListServerExtensions(c *gin.Context) {
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
	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverID, user)
	if err != nil {
		responses.NotFound(c, "Server not found", nil)
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
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to query extensions"})
		return
	}
	defer rows.Close()

	// Build response
	extensionsList := make([]ExtensionResponse, 0)

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
		extensionsList = append(extensionsList, ExtensionResponse{
			ID:       id.String(),
			ServerID: servID.String(),
			Name:     name,
			Enabled:  enabled,
			Config:   config,
		})
	}

	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating extension rows")
		responses.InternalServerError(c, err, &gin.H{"error": "Error processing extensions"})
		return
	}

	responses.Success(c, "Extensions fetched successfully", &gin.H{
		"extensions": extensionsList,
	})
}

// GetServerExtension returns a specific extension for a server
func (s *Server) GetServerExtension(c *gin.Context) {
	// Get server ID and extension ID from URL
	serverIDStr := c.Param("serverId")
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", nil)
		return
	}

	extensionIDStr := c.Param("extensionId")
	eID, err := uuid.Parse(extensionIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid extension ID", nil)
		return
	}

	// Get user from session
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	// Check if user has access to server
	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverID, user)
	if err != nil {
		responses.NotFound(c, "Server not found", nil)
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
		responses.NotFound(c, "Extension not found", nil)
		return
	}

	// Parse config JSON
	var config map[string]interface{}
	if err := json.Unmarshal(configJSON, &config); err != nil {
		log.Error().Err(err).Str("id", eID.String()).Msg("Failed to unmarshal extension config")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to parse extension config"})
		return
	}

	// Build response
	extension := ExtensionResponse{
		ID:       eID.String(),
		ServerID: servID.String(),
		Name:     name,
		Enabled:  enabled,
		Config:   config,
	}

	responses.Success(c, "Extension fetched successfully", &gin.H{
		"extension": extension,
	})
}

// CreateServerExtension creates a new extension for a server
func (s *Server) CreateServerExtension(c *gin.Context) {
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

	var req ExtensionCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responses.BadRequest(c, "Invalid request body", &gin.H{"error": err.Error()})
		return
	}

	// Validate extension type
	registrar, ok := s.Dependencies.ExtensionManager.GetExtension(req.Name)
	if !ok {
		responses.BadRequest(c, "Invalid extension type", nil)
		return
	}

	// Check if a connector_id is specified and verify it exists
	if connectorIDStr, ok := req.Config["connector_id"].(string); ok {
		connectorID, err := uuid.Parse(connectorIDStr)
		if err != nil {
			responses.BadRequest(c, "Invalid connector ID format", nil)
			return
		}

		// Check if connector exists
		var exists bool
		err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
			SELECT EXISTS(SELECT 1 FROM connectors WHERE id = $1)
		`, connectorID).Scan(&exists)

		if err != nil {
			log.Error().Err(err).Msg("Failed to check if connector exists")
			responses.InternalServerError(c, err, &gin.H{"error": "Failed to validate connector"})
			return
		}

		if !exists {
			responses.BadRequest(c, "Specified connector does not exist", nil)
			return
		}
	}

	// Convert config to JSON
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal extension config")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to process extension config"})
		return
	}

	// Generate UUID for new extension
	id := uuid.New()

	// Begin transaction
	tx, err := s.Dependencies.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to begin transaction"})
		return
	}
	defer tx.Rollback()

	// Create extension in database
	_, err = tx.ExecContext(c.Request.Context(), `
		INSERT INTO server_extensions (id, server_id, name, enabled, config)
		VALUES ($1, $2, $3, $4, $5)
	`, id, serverID, req.Name, req.Enabled, configJSON)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create extension")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to create extension"})
		return
	}

	// If enabled, initialize the extension
	var warningMessage string
	if req.Enabled {
		// Get extension definition
		extensionDef := registrar.Define()

		// Create the extension instance
		instance := extensionDef.CreateInstance()

		// Create dependencies
		deps, err := s.createExtensionDependencies(extensionDef, server)
		if err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("Failed to create extension dependencies")

			// Update database to mark as disabled
			_, dbErr := tx.ExecContext(c.Request.Context(), `
				UPDATE server_extensions
				SET enabled = false
				WHERE id = $1
			`, id)
			if dbErr != nil {
				log.Error().Err(dbErr).Str("id", id.String()).Msg("Failed to update extension enabled status")
			}

			warningMessage = fmt.Sprintf("Extension created but not enabled: %s", err.Error())
		} else {
			// Initialize the extension
			if err := instance.Initialize(req.Config, deps); err != nil {
				log.Error().Err(err).Str("id", id.String()).Msg("Failed to initialize extension")

				// Update database to mark as disabled
				_, dbErr := tx.ExecContext(c.Request.Context(), `
					UPDATE server_extensions
					SET enabled = false
					WHERE id = $1
				`, id)
				if dbErr != nil {
					log.Error().Err(dbErr).Str("id", id.String()).Msg("Failed to update extension enabled status")
				}

				warningMessage = fmt.Sprintf("Extension created but failed to initialize: %s", err.Error())
			}
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Return success
	if warningMessage != "" {
		responses.Success(c, warningMessage, &gin.H{
			"id":      id.String(),
			"status":  "warning",
			"enabled": false,
		})
	} else {
		responses.Success(c, "Extension created successfully", &gin.H{
			"id":      id.String(),
			"enabled": req.Enabled,
		})
	}
}

// UpdateServerExtension updates an existing extension for a server
func (s *Server) UpdateServerExtension(c *gin.Context) {
	// Get server ID and extension ID from URL
	serverIDStr := c.Param("serverId")
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", nil)
		return
	}

	extensionIDStr := c.Param("extensionId")
	eID, err := uuid.Parse(extensionIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid extension ID", nil)
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

	var req ExtensionUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responses.BadRequest(c, "Invalid request body", &gin.H{"error": err.Error()})
		return
	}

	// Begin transaction
	tx, err := s.Dependencies.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to begin transaction"})
		return
	}
	defer tx.Rollback()

	// Get current extension info from database
	var name string
	var enabled bool
	var configJSON []byte

	err = tx.QueryRowContext(c.Request.Context(), `
		SELECT name, enabled, config
		FROM server_extensions
		WHERE id = $1 AND server_id = $2
	`, eID, serverID).Scan(&name, &enabled, &configJSON)

	if err != nil {
		log.Error().Err(err).Str("id", eID.String()).Str("serverID", serverID.String()).Msg("Failed to get extension")
		responses.NotFound(c, "Extension not found", nil)
		return
	}

	// Parse current config JSON
	var currentConfig map[string]interface{}
	if err := json.Unmarshal(configJSON, &currentConfig); err != nil {
		log.Error().Err(err).Str("id", eID.String()).Msg("Failed to unmarshal extension config")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to parse extension config"})
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
		tx.Rollback() // No need to commit if no changes
		responses.Success(c, "No changes requested", nil)
		return
	}

	// If we're updating config, convert to JSON
	if configChanged {
		configJSON, err = json.Marshal(newConfig)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal extension config")
			responses.InternalServerError(c, err, &gin.H{"error": "Failed to process extension config"})
			return
		}
	}

	// Update extension in database
	if enabledChanged && configChanged {
		_, err = tx.ExecContext(c.Request.Context(), `
			UPDATE server_extensions
			SET enabled = $1, config = $2
			WHERE id = $3
		`, newEnabled, configJSON, eID)
	} else if enabledChanged {
		_, err = tx.ExecContext(c.Request.Context(), `
			UPDATE server_extensions
			SET enabled = $1
			WHERE id = $2
		`, newEnabled, eID)
	} else if configChanged {
		_, err = tx.ExecContext(c.Request.Context(), `
			UPDATE server_extensions
			SET config = $1
			WHERE id = $2
		`, configJSON, eID)
	}

	if err != nil {
		log.Error().Err(err).Str("id", eID.String()).Msg("Failed to update extension")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to update extension"})
		return
	}

	// Get extension registrar
	registrar, ok := s.Dependencies.ExtensionManager.GetExtension(name)
	if !ok {
		log.Error().Str("name", name).Msg("Extension registrar not found")
		responses.InternalServerError(c, err, &gin.H{"error": "Extension type not found"})
		return
	}

	// Get extension definition
	extensionDef := registrar.Define()

	var warningMessage string

	// If we're enabling, initialize
	if enabledChanged && newEnabled {
		// Create the extension instance
		instance := extensionDef.CreateInstance()

		// Create dependencies
		deps, err := s.createExtensionDependencies(extensionDef, server)
		if err != nil {
			log.Error().Err(err).Str("id", eID.String()).Msg("Failed to create extension dependencies")

			// Update database to mark as disabled
			_, dbErr := tx.ExecContext(c.Request.Context(), `
				UPDATE server_extensions
				SET enabled = false
				WHERE id = $1
			`, eID)
			if dbErr != nil {
				log.Error().Err(dbErr).Str("id", eID.String()).Msg("Failed to update extension enabled status")
			}

			warningMessage = fmt.Sprintf("Extension updated but failed to initialize: %s", err.Error())
		} else {
			// Initialize extension
			if err := instance.Initialize(newConfig, deps); err != nil {
				log.Error().Err(err).Str("id", eID.String()).Msg("Failed to initialize extension")

				// Update database to mark as disabled
				_, dbErr := tx.ExecContext(c.Request.Context(), `
					UPDATE server_extensions
					SET enabled = false
					WHERE id = $1
				`, eID)
				if dbErr != nil {
					log.Error().Err(dbErr).Str("id", eID.String()).Msg("Failed to update extension enabled status")
				}

				warningMessage = fmt.Sprintf("Extension updated but failed to initialize: %s", err.Error())
			}
		}
	}

	// If we're disabling, shutdown
	if enabledChanged && !newEnabled {
		// Attempt to shutdown the extension
		// For now just log it, real implementation would use extension manager
		log.Info().Str("id", eID.String()).Msg("Disabling extension")
	}

	// If we're updating config and staying enabled, restart the extension
	if configChanged && newEnabled && !enabledChanged {
		// For now just log it, real implementation would use extension manager to restart
		log.Info().Str("id", eID.String()).Msg("Updating extension config")
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Return success
	if warningMessage != "" {
		responses.Success(c, warningMessage, &gin.H{
			"status":  "warning",
			"enabled": false,
		})
	} else {
		responses.Success(c, "Extension updated successfully", &gin.H{
			"enabled": newEnabled,
		})
	}
}

// DeleteServerExtension deletes an extension from a server
func (s *Server) DeleteServerExtension(c *gin.Context) {
	// Get server ID and extension ID from URL
	serverIDStr := c.Param("serverId")
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", nil)
		return
	}

	extensionIDStr := c.Param("extensionId")
	eID, err := uuid.Parse(extensionIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid extension ID", nil)
		return
	}

	// Get user from session
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	// Check if user has access to server
	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverID, user)
	if err != nil {
		responses.NotFound(c, "Server not found", nil)
		return
	}

	// Begin transaction
	tx, err := s.Dependencies.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to begin transaction"})
		return
	}
	defer tx.Rollback()

	// Check if extension exists
	var exists bool
	err = tx.QueryRowContext(c.Request.Context(), `
		SELECT EXISTS(SELECT 1 FROM server_extensions WHERE id = $1 AND server_id = $2)
	`, eID, serverID).Scan(&exists)

	if err != nil {
		log.Error().Err(err).Str("id", eID.String()).Str("serverID", serverID.String()).Msg("Failed to check if extension exists")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to check if extension exists"})
		return
	}

	if !exists {
		responses.NotFound(c, "Extension not found", nil)
		return
	}

	// For shutdown extension if it's running
	// Note: Real implementation would use extension manager to find and shutdown the extension
	log.Info().Str("id", eID.String()).Msg("Shutting down extension")

	// Delete extension from database
	_, err = tx.ExecContext(c.Request.Context(), `
		DELETE FROM server_extensions
		WHERE id = $1 AND server_id = $2
	`, eID, serverID)

	if err != nil {
		log.Error().Err(err).Str("id", eID.String()).Str("serverID", serverID.String()).Msg("Failed to delete extension")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to delete extension"})
		return
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Return success
	responses.Success(c, "Extension deleted successfully", nil)
}

// ToggleServerExtension toggles an extension's enabled status
func (s *Server) ToggleServerExtension(c *gin.Context) {
	// Get server ID and extension ID from URL
	serverIDStr := c.Param("serverId")
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", nil)
		return
	}

	extensionIDStr := c.Param("extensionId")
	eID, err := uuid.Parse(extensionIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid extension ID", nil)
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

	// Begin transaction
	tx, err := s.Dependencies.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to begin transaction"})
		return
	}
	defer tx.Rollback()

	// Get current extension info from database
	var name string
	var enabled bool
	var configJSON []byte

	err = tx.QueryRowContext(c.Request.Context(), `
		SELECT name, enabled, config
		FROM server_extensions
		WHERE id = $1 AND server_id = $2
	`, eID, serverID).Scan(&name, &enabled, &configJSON)

	if err != nil {
		log.Error().Err(err).Str("id", eID.String()).Str("serverID", serverID.String()).Msg("Failed to get extension")
		responses.NotFound(c, "Extension not found", nil)
		return
	}

	// Parse current config JSON
	var currentConfig map[string]interface{}
	if err := json.Unmarshal(configJSON, &currentConfig); err != nil {
		log.Error().Err(err).Str("id", eID.String()).Msg("Failed to unmarshal extension config")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to parse extension config"})
		return
	}

	// Toggle enabled status
	newEnabled := !enabled

	// Update extension in database
	_, err = tx.ExecContext(c.Request.Context(), `
		UPDATE server_extensions
		SET enabled = $1
		WHERE id = $2
	`, newEnabled, eID)

	if err != nil {
		log.Error().Err(err).Str("id", eID.String()).Msg("Failed to update extension enabled status")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to update extension status"})
		return
	}

	// Get extension registrar
	registrar, ok := s.Dependencies.ExtensionManager.GetExtension(name)
	if !ok {
		log.Error().Str("name", name).Msg("Extension registrar not found")
		responses.InternalServerError(c, err, &gin.H{"error": "Extension type not found"})
		return
	}

	// Get extension definition
	extensionDef := registrar.Define()

	var warningMessage string

	// If we're enabling, initialize
	if newEnabled {
		// Create the extension instance
		instance := extensionDef.CreateInstance()

		// Create dependencies
		deps, err := s.createExtensionDependencies(extensionDef, server)
		if err != nil {
			log.Error().Err(err).Str("id", eID.String()).Msg("Failed to create extension dependencies")

			// Update database to mark as disabled
			_, dbErr := tx.ExecContext(c.Request.Context(), `
				UPDATE server_extensions
				SET enabled = false
				WHERE id = $1
			`, eID)
			if dbErr != nil {
				log.Error().Err(dbErr).Str("id", eID.String()).Msg("Failed to update extension enabled status")
			}

			warningMessage = fmt.Sprintf("Extension not enabled: %s", err.Error())
		} else {
			// Initialize extension
			if err := instance.Initialize(currentConfig, deps); err != nil {
				log.Error().Err(err).Str("id", eID.String()).Msg("Failed to initialize extension")

				// Update database to mark as disabled
				_, dbErr := tx.ExecContext(c.Request.Context(), `
					UPDATE server_extensions
					SET enabled = false
					WHERE id = $1
				`, eID)
				if dbErr != nil {
					log.Error().Err(dbErr).Str("id", eID.String()).Msg("Failed to update extension enabled status")
				}

				warningMessage = fmt.Sprintf("Extension not enabled: %s", err.Error())
			}
		}
	} else {
		// Disabling the extension
		// Real implementation would use extension manager to shut down
		// TODO: Implement disabling the extension
		log.Info().Str("id", eID.String()).Msg("Disabling extension")
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Return success
	if warningMessage != "" {
		responses.Success(c, warningMessage, &gin.H{
			"status":  "warning",
			"enabled": false,
		})
	} else {
		responses.Success(c, "Extension "+(map[bool]string{true: "enabled", false: "disabled"})[newEnabled]+" successfully", &gin.H{
			"enabled": newEnabled,
		})
	}
}

// Helper function to create extension dependencies
func (s *Server) createExtensionDependencies(def extension_manager.ExtensionDefinition, server *models.Server) (*extension_manager.Dependencies, error) {
	deps := &extension_manager.Dependencies{
		Database:    s.Dependencies.DB,
		Server:      server,
		RconManager: s.Dependencies.RconManager,
		Connectors:  make(map[string]connector_manager.Connector),
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

			// Add connectors to dependencies
			for _, requiredConnector := range def.RequiredConnectors {
				found := false
				for _, connector := range serverConnectors {
					if connector.GetType() == requiredConnector {
						deps.Connectors[connector.GetType()] = connector
						found = true
						break
					}
				}

				if !found {
					return nil, fmt.Errorf("required connector not available: %s", requiredConnector)
				}
			}

			// Add optional connectors
			for _, optionalConnector := range def.OptionalConnectors {
				for _, connector := range serverConnectors {
					if connector.GetType() == optionalConnector {
						deps.Connectors[connector.GetType()] = connector
						break
					}
				}
			}
		}
	}

	return deps, nil
}
