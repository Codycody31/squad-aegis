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

// ExtensionResponse represents an extension in API responses
type ExtensionResponse struct {
	ID                     string                 `json:"id"`
	ServerID               string                 `json:"server_id"`
	Name                   string                 `json:"name"`
	Enabled                bool                   `json:"enabled"`
	Config                 map[string]interface{} `json:"config"`
	Notes                  *string                `json:"notes"`
	AllowMultipleInstances bool                   `json:"allow_multiple_instances"`
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
	Notes   *string                `json:"notes"`
}

// ExtensionUpdateRequest represents the request to update an extension
type ExtensionUpdateRequest struct {
	Enabled *bool                  `json:"enabled"`
	Config  map[string]interface{} `json:"config"`
	Notes   *string                `json:"notes"`
}

// ServerExtensionsList returns all extensions for a server
func (s *Server) ServerExtensionsList(c *gin.Context) {
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
		SELECT id, server_id, name, enabled, config, notes
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
		var notes *string

		if err := rows.Scan(&id, &servID, &name, &enabled, &configJSON, &notes); err != nil {
			log.Error().Err(err).Msg("Failed to scan extension row")
			continue
		}

		// Parse config JSON
		var config map[string]interface{}
		if err := json.Unmarshal(configJSON, &config); err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("Failed to unmarshal extension config")
			continue
		}

		// Get the extension registrar to access definition
		registrar, ok := s.Dependencies.ExtensionManager.GetExtension(name)
		if !ok {
			log.Error().Str("name", name).Msg("Extension registrar not found")
			continue
		}

		// Get extension definition to access AllowMultipleInstances field
		extensionDef := registrar.Define()

		// Add to response
		extensionsList = append(extensionsList, ExtensionResponse{
			ID:                     id.String(),
			ServerID:               servID.String(),
			Name:                   name,
			Enabled:                enabled,
			Config:                 config,
			Notes:                  notes,
			AllowMultipleInstances: extensionDef.AllowMultipleInstances,
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

// ServerExtensionGet returns a specific extension for a server
func (s *Server) ServerExtensionGet(c *gin.Context) {
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

	// We only need server for the access check, we don't use it directly because
	// the ExtensionManager.EnableExtension method fetches the server internally

	// Get extension from database
	var servID uuid.UUID
	var name string
	var enabled bool
	var configJSON []byte
	var notes *string

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT server_id, name, enabled, config, notes
		FROM server_extensions
		WHERE id = $1 AND server_id = $2
	`, eID, serverID).Scan(&servID, &name, &enabled, &configJSON, &notes)

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

	// Get the extension registrar to access definition
	registrar, ok := s.Dependencies.ExtensionManager.GetExtension(name)
	if !ok {
		log.Error().Str("name", name).Msg("Extension registrar not found")
		responses.InternalServerError(c, fmt.Errorf("extension type not found"), &gin.H{"error": "Extension type not found"})
		return
	}

	// Get extension definition to access AllowMultipleInstances field
	extensionDef := registrar.Define()

	// Build response
	extension := ExtensionResponse{
		ID:                     eID.String(),
		ServerID:               servID.String(),
		Name:                   name,
		Enabled:                enabled,
		Config:                 config,
		Notes:                  notes,
		AllowMultipleInstances: extensionDef.AllowMultipleInstances,
	}

	responses.Success(c, "Extension fetched successfully", &gin.H{
		"extension": extension,
	})
}

// ServerExtensionCreate creates a new extension for a server
func (s *Server) ServerExtensionCreate(c *gin.Context) {
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

	// Get extension definition
	extensionDef := registrar.Define()

	// Check if extension allows multiple instances
	if !extensionDef.AllowMultipleInstances {
		// Check if this extension is already in use for this server
		var count int
		err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
			SELECT COUNT(*) FROM server_extensions 
			WHERE server_id = $1 AND name = $2
		`, serverID, req.Name).Scan(&count)

		if err != nil {
			log.Error().Err(err).Msg("Failed to check existing extensions")
			responses.InternalServerError(c, err, &gin.H{"error": "Failed to validate extension constraints"})
			return
		}

		if count > 0 {
			responses.BadRequest(c, "This extension type does not allow multiple instances on the same server", nil)
			return
		}
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
		INSERT INTO server_extensions (id, server_id, name, enabled, config, notes)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, id, serverID, req.Name, req.Enabled, configJSON, req.Notes)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create extension")
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to create extension"})
		return
	}

	// Commit the transaction first so the extension exists in database
	if err := tx.Commit(); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to commit transaction"})
		return
	}

	// If enabled, initialize the extension using the Extension Manager
	var warningMessage string
	if req.Enabled {
		// Use the extension manager to enable the extension
		if err := s.Dependencies.ExtensionManager.EnableExtension(c.Request.Context(), serverID, req.Name); err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("Failed to enable extension")

			// Update database to mark as disabled since we couldn't enable it
			_, dbErr := s.Dependencies.DB.ExecContext(c.Request.Context(), `
				UPDATE server_extensions
				SET enabled = false
				WHERE id = $1
			`, id)
			if dbErr != nil {
				log.Error().Err(dbErr).Str("id", id.String()).Msg("Failed to update extension enabled status")
			}

			warningMessage = fmt.Sprintf("Extension created but not enabled: %s", err.Error())
		}
	}

	// Create audit log entry
	auditData := map[string]interface{}{
		"extensionId": id.String(),
		"name":        req.Name,
		"enabled":     req.Enabled,
		"config":      req.Config,
		"notes":       req.Notes,
	}
	s.CreateAuditLog(c.Request.Context(), &serverID, &user.Id, "extension:create", auditData)

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

// ServerExtensionUpdate updates an existing extension for a server
func (s *Server) ServerExtensionUpdate(c *gin.Context) {
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
	var notes *string

	err = tx.QueryRowContext(c.Request.Context(), `
		SELECT name, enabled, config, notes
		FROM server_extensions
		WHERE id = $1 AND server_id = $2
	`, eID, serverID).Scan(&name, &enabled, &configJSON, &notes)

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

	// Update notes if provided
	notesChanged := false
	newNotes := notes
	if req.Notes != nil && (notes == nil || *req.Notes != *notes) {
		notesChanged = true
		newNotes = req.Notes
	}

	// If we're not changing anything, just return success
	if !enabledChanged && !configChanged && !notesChanged {
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
	if enabledChanged && configChanged && notesChanged {
		_, err = tx.ExecContext(c.Request.Context(), `
			UPDATE server_extensions
			SET enabled = $1, config = $2, notes = $3
			WHERE id = $4
		`, newEnabled, configJSON, newNotes, eID)
	} else if enabledChanged && configChanged {
		_, err = tx.ExecContext(c.Request.Context(), `
			UPDATE server_extensions
			SET enabled = $1, config = $2
			WHERE id = $3
		`, newEnabled, configJSON, eID)
	} else if enabledChanged && notesChanged {
		_, err = tx.ExecContext(c.Request.Context(), `
			UPDATE server_extensions
			SET enabled = $1, notes = $2
			WHERE id = $3
		`, newEnabled, newNotes, eID)
	} else if configChanged && notesChanged {
		_, err = tx.ExecContext(c.Request.Context(), `
			UPDATE server_extensions
			SET config = $1, notes = $2
			WHERE id = $3
		`, configJSON, newNotes, eID)
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
	} else if notesChanged {
		_, err = tx.ExecContext(c.Request.Context(), `
			UPDATE server_extensions
			SET notes = $1
			WHERE id = $2
		`, newNotes, eID)
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

	// For config change but keeping enabled, we need to commit the transaction
	// before restarting the extension so it reads the new config
	if configChanged && newEnabled && !enabledChanged {
		// Get extension definition ID
		extensionID := extensionDef.ID

		// First, shut down the extension
		if err := s.Dependencies.ExtensionManager.ShutdownExtension(serverID, extensionID); err != nil {
			log.Info().
				Err(err).
				Str("id", eID.String()).
				Str("extension", name).
				Str("serverID", serverID.String()).
				Msg("Error shutting down extension before update, continuing anyway")
		}

		// IMPORTANT: Commit the transaction BEFORE restarting the extension
		// so the extension reads the updated config from the database
		if err := tx.Commit(); err != nil {
			responses.InternalServerError(c, err, &gin.H{"error": "Failed to commit transaction"})
			return
		}

		// Then re-enable with new config
		if err := s.Dependencies.ExtensionManager.EnableExtension(c.Request.Context(), serverID, name); err != nil {
			log.Warn().Err(err).Str("id", eID.String()).Msg("Failed to reinitialize extension with new config")
			warningMessage = fmt.Sprintf("Extension config updated but failed to apply changes: %s", err.Error())

			// Update database to mark as disabled since we couldn't re-enable it
			_, dbErr := s.Dependencies.DB.ExecContext(c.Request.Context(), `
				UPDATE server_extensions
				SET enabled = false
				WHERE id = $1
			`, eID)
			if dbErr != nil {
				log.Warn().Err(dbErr).Str("id", eID.String()).Msg("Failed to update extension enabled status")
			}
		} else {
			log.Info().Str("id", eID.String()).Msg("Extension successfully reinitialized with updated config")
		}

		// Create audit log entry
		auditData := map[string]interface{}{
			"extensionId":   eID.String(),
			"name":          name,
			"configChanged": true,
			"newConfig":     newConfig,
		}
		if notesChanged {
			auditData["notesChanged"] = true
			auditData["newNotes"] = newNotes
		}
		s.CreateAuditLog(c.Request.Context(), &serverID, &user.Id, "extension:update", auditData)

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

		// Transaction is already committed in this case
		return
	}

	// If we're enabling, initialize
	if enabledChanged && newEnabled {
		// Log the server ID for debugging
		log.Debug().Str("serverID", serverID.String()).Msg("Enabling extension for server")

		// COMMIT TRANSACTION BEFORE ENABLING so it reads the latest config
		if err := tx.Commit(); err != nil {
			responses.InternalServerError(c, err, &gin.H{"error": "Failed to commit transaction"})
			return
		}

		if err := s.Dependencies.ExtensionManager.EnableExtension(c.Request.Context(), serverID, name); err != nil {
			log.Warn().Err(err).Str("id", eID.String()).Msg("Failed to enable extension")

			// Update database to mark as disabled
			_, dbErr := s.Dependencies.DB.ExecContext(c.Request.Context(), `
				UPDATE server_extensions
				SET enabled = false
				WHERE id = $1
			`, eID)
			if dbErr != nil {
				log.Warn().Err(dbErr).Str("id", eID.String()).Msg("Failed to update extension enabled status")
			}

			warningMessage = fmt.Sprintf("Extension updated but failed to initialize: %s", err.Error())
		}

		// Create audit log entry
		auditData := map[string]interface{}{
			"extensionId":    eID.String(),
			"name":           name,
			"enabledChanged": true,
			"newEnabled":     newEnabled,
		}
		if configChanged {
			auditData["configChanged"] = true
			auditData["newConfig"] = newConfig
		}
		s.CreateAuditLog(c.Request.Context(), &serverID, &user.Id, "extension:update", auditData)

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

		// Transaction already committed
		return
	} else if enabledChanged && !newEnabled {
		// Disabling the extension
		// Get extension definition ID
		extensionID := extensionDef.ID

		// Use the extension manager to shut down the extension
		if err := s.Dependencies.ExtensionManager.ShutdownExtension(serverID, extensionID); err != nil {
			log.Info().
				Err(err).
				Str("id", eID.String()).
				Str("extension", name).
				Str("serverID", serverID.String()).
				Msg("Error shutting down extension")
			// Continue with database update even if shutdown fails
		}
	}

	// Commit the transaction for remaining cases
	if err := tx.Commit(); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Create audit log entry for remaining cases
	auditData := map[string]interface{}{
		"extensionId": eID.String(),
		"name":        name,
	}
	if enabledChanged {
		auditData["enabledChanged"] = true
		auditData["newEnabled"] = newEnabled
	}
	if configChanged {
		auditData["configChanged"] = true
		auditData["newConfig"] = newConfig
	}
	if notesChanged {
		auditData["notesChanged"] = true
		auditData["newNotes"] = newNotes
	}
	s.CreateAuditLog(c.Request.Context(), &serverID, &user.Id, "extension:update", auditData)

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

// ServerExtensionDelete deletes an extension from a server
func (s *Server) ServerExtensionDelete(c *gin.Context) {
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
	// Get extension name from database
	var name string
	var enabled bool
	var configJSON []byte

	err = tx.QueryRowContext(c.Request.Context(), `
		SELECT name, enabled, config
		FROM server_extensions
		WHERE id = $1 AND server_id = $2
	`, eID, serverID).Scan(&name, &enabled, &configJSON)

	if err != nil {
		log.Error().Err(err).Str("id", eID.String()).Str("serverID", serverID.String()).Msg("Failed to get extension name")
	} else if enabled {
		// Get extension registrar
		registrar, ok := s.Dependencies.ExtensionManager.GetExtension(name)
		if ok {
			// Get extension definition
			extensionDef := registrar.Define()
			extensionID := extensionDef.ID

			// Try to shut down the extension
			if err := s.Dependencies.ExtensionManager.ShutdownExtension(serverID, extensionID); err != nil {
				log.Info().
					Err(err).
					Str("id", eID.String()).
					Str("extension", name).
					Str("serverID", serverID.String()).
					Msg("Error shutting down extension before delete, continuing anyway")
			} else {
				log.Info().Str("id", eID.String()).Msg("Extension shut down before deletion")
			}
		}
	}

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

	// Create audit log entry
	auditData := map[string]interface{}{
		"extensionId": eID.String(),
	}
	s.CreateAuditLog(c.Request.Context(), &serverID, &user.Id, "extension:delete", auditData)

	// Return success
	responses.Success(c, "Extension deleted successfully", nil)
}

// ServerExtensionToggle toggles an extension's enabled status
func (s *Server) ServerExtensionToggle(c *gin.Context) {
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

	// We only need server for the access check, we don't use it directly because
	// the ExtensionManager.EnableExtension method fetches the server internally

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
	var notes *string

	err = tx.QueryRowContext(c.Request.Context(), `
		SELECT name, enabled, config, notes
		FROM server_extensions
		WHERE id = $1 AND server_id = $2
	`, eID, serverID).Scan(&name, &enabled, &configJSON, &notes)

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
		// Log the server ID for debugging
		log.Debug().Str("serverID", serverID.String()).Msg("Enabling extension for server")

		if err := s.Dependencies.ExtensionManager.EnableExtension(c.Request.Context(), serverID, name); err != nil {
			log.Warn().Err(err).Str("id", eID.String()).Msg("Failed to enable extension")

			// Update database to mark as disabled
			_, dbErr := tx.ExecContext(c.Request.Context(), `
				UPDATE server_extensions
				SET enabled = false
				WHERE id = $1
			`, eID)
			if dbErr != nil {
				log.Warn().Err(dbErr).Str("id", eID.String()).Msg("Failed to update extension enabled status")
			}

			warningMessage = fmt.Sprintf("Extension not enabled: %s", err.Error())
		}
	} else {
		// Disabling the extension
		// Get extension definition ID
		extensionID := extensionDef.ID

		// Use the extension manager to shut down the extension
		if err := s.Dependencies.ExtensionManager.ShutdownExtension(serverID, extensionID); err != nil {
			log.Info().
				Err(err).
				Str("id", eID.String()).
				Str("extension", name).
				Str("serverID", serverID.String()).
				Msg("Error shutting down extension")
			// Continue with database update even if shutdown fails
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Create audit log entry
	auditData := map[string]interface{}{
		"extensionId": eID.String(),
		"name":        name,
		"enabled":     newEnabled,
	}
	s.CreateAuditLog(c.Request.Context(), &serverID, &user.Id, "extension:toggle", auditData)

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
		}
	}

	// Get all connectors for this server
	connectors := s.Dependencies.ConnectorManager.GetConnectorsByServer(server.Id)

	// Add required connectors
	for _, requiredConnector := range def.RequiredConnectors {
		found := false
		for _, connector := range connectors {
			connDef := connector.GetDefinition()
			if connDef.ID == requiredConnector {
				deps.Connectors[connDef.ID] = connector
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("required connector not found: %s", requiredConnector)
		}
	}

	// Add optional connectors if available
	for _, optionalConnector := range def.OptionalConnectors {
		for _, connector := range connectors {
			connDef := connector.GetDefinition()
			if connDef.ID == optionalConnector {
				deps.Connectors[connDef.ID] = connector
				break
			}
		}
	}

	return deps, nil
}
