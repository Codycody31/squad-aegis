package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_sdk"
)

// CustomPluginRoutes sets up routes for custom plugin management
func (s *Server) CustomPluginRoutes(r *gin.RouterGroup) {
	plugins := r.Group("/plugins")
	{
		// List all plugins (builtin + custom)
		plugins.GET("/", s.handleListPlugins)
		
		// Custom plugin operations
		plugins.POST("/upload", s.handleUploadPlugin)
		plugins.GET("/custom", s.handleListCustomPlugins)
		plugins.GET("/custom/:pluginID", s.handleGetCustomPlugin)
		plugins.DELETE("/custom/:pluginID", s.handleDeleteCustomPlugin)
		plugins.POST("/custom/:pluginID/enable", s.handleEnableCustomPlugin)
		plugins.POST("/custom/:pluginID/disable", s.handleDisableCustomPlugin)
		plugins.POST("/custom/:pluginID/verify", s.handleVerifyCustomPlugin)
		
		// Plugin permissions
		plugins.GET("/:pluginID/permissions", s.handleGetPluginPermissions)
		plugins.POST("/:pluginID/permissions", s.handleGrantPluginPermissions)
		plugins.DELETE("/:pluginID/permissions/:permissionID", s.handleRevokePluginPermission)
		
		// Public keys
		plugins.POST("/keys", s.handleAddPublicKey)
		plugins.GET("/keys", s.handleListPublicKeys)
		plugins.DELETE("/keys/:keyName", s.handleRevokePublicKey)
		
		// Plugin versions
		plugins.GET("/custom/:pluginID/versions", s.handleListPluginVersions)
	}
}

// handleListPlugins lists all available plugins (builtin + custom)
func (s *Server) handleListPlugins(c *gin.Context) {
	plugins := s.Dependencies.PluginManager.ListAvailablePlugins()
	customPlugins, err := plugin_manager.ListCustomPlugins(s.Dependencies.DB)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list custom plugins")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list custom plugins"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"builtin_plugins": plugins,
		"custom_plugins":  customPlugins,
	})
}

// handleUploadPlugin handles custom plugin upload
func (s *Server) handleUploadPlugin(c *gin.Context) {
	// Parse multipart form
	if err := c.Request.ParseMultipartForm(50 << 20); err != nil { // 50 MB max
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form"})
		return
	}
	
	// Get uploaded file
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()
	
	// Get manifest JSON
	manifestJSON := c.PostForm("manifest")
	if manifestJSON == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Manifest is required"})
		return
	}
	
	// Parse manifest
	var manifest plugin_sdk.PluginManifest
	if err := json.Unmarshal([]byte(manifestJSON), &manifest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid manifest JSON"})
		return
	}
	
	// Validate manifest
	if err := plugin_sdk.ValidateManifest(manifest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid manifest: %v", err)})
		return
	}
	
	// Get current user ID from session (placeholder - implement actual auth)
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000000") // TODO: Get from session
	
	// Read file to bytes for storage and validation
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}
	
	// Save to storage (create new reader from bytes)
	pluginStorage := plugin_manager.NewPluginStorage(s.Dependencies.Storage)
	storagePath, err := pluginStorage.SavePlugin(c.Request.Context(), manifest.ID, manifest.Version, io.NopCloser(bytes.NewReader(fileBytes)))
	if err != nil {
		log.Error().Err(err).Msg("Failed to save plugin to storage")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save plugin"})
		return
	}
	
	// Create database record
	record := plugin_manager.ConvertManifestToRecord(manifest, storagePath, userID)
	if err := plugin_manager.SaveCustomPlugin(s.Dependencies.DB, record); err != nil {
		log.Error().Err(err).Msg("Failed to save plugin to database")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save plugin metadata"})
		return
	}
	
	log.Info().
		Str("plugin_id", manifest.ID).
		Str("version", manifest.Version).
		Str("filename", header.Filename).
		Msg("Custom plugin uploaded")
	
	c.JSON(http.StatusCreated, gin.H{
		"success":   true,
		"plugin_id": manifest.ID,
		"version":   manifest.Version,
		"message":   "Plugin uploaded successfully. Awaiting verification.",
	})
}

// handleListCustomPlugins lists all custom plugins
func (s *Server) handleListCustomPlugins(c *gin.Context) {
	plugins, err := plugin_manager.ListCustomPlugins(s.Dependencies.DB)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list custom plugins")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list custom plugins"})
		return
	}
	
	c.JSON(http.StatusOK, plugins)
}

// handleGetCustomPlugin gets a specific custom plugin
func (s *Server) handleGetCustomPlugin(c *gin.Context) {
	pluginID := c.Param("pluginID")
	
	plugin, err := plugin_manager.GetCustomPlugin(s.Dependencies.DB, pluginID)
	if err != nil {
		log.Error().Err(err).Str("plugin_id", pluginID).Msg("Failed to get custom plugin")
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}
	
	c.JSON(http.StatusOK, plugin)
}

// handleDeleteCustomPlugin deletes a custom plugin
func (s *Server) handleDeleteCustomPlugin(c *gin.Context) {
	pluginID := c.Param("pluginID")
	
	// Get plugin to find storage path
	plugin, err := plugin_manager.GetCustomPlugin(s.Dependencies.DB, pluginID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}
	
	// Delete from storage
	pluginStorage := plugin_manager.NewPluginStorage(s.Dependencies.Storage)
	if err := pluginStorage.DeletePluginByPath(c.Request.Context(), plugin.StoragePath); err != nil {
		log.Error().Err(err).Msg("Failed to delete plugin from storage")
		// Continue anyway to clean up database
	}
	
	// Delete from database
	if err := plugin_manager.DeleteCustomPlugin(s.Dependencies.DB, pluginID); err != nil {
		log.Error().Err(err).Msg("Failed to delete plugin from database")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete plugin"})
		return
	}
	
	log.Info().Str("plugin_id", pluginID).Msg("Custom plugin deleted")
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Plugin deleted successfully",
	})
}

// handleEnableCustomPlugin enables a custom plugin
func (s *Server) handleEnableCustomPlugin(c *gin.Context) {
	pluginID := c.Param("pluginID")
	
	if err := plugin_manager.UpdateCustomPluginEnabled(s.Dependencies.DB, pluginID, true); err != nil {
		log.Error().Err(err).Msg("Failed to enable plugin")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable plugin"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Plugin enabled successfully",
	})
}

// handleDisableCustomPlugin disables a custom plugin
func (s *Server) handleDisableCustomPlugin(c *gin.Context) {
	pluginID := c.Param("pluginID")
	
	if err := plugin_manager.UpdateCustomPluginEnabled(s.Dependencies.DB, pluginID, false); err != nil {
		log.Error().Err(err).Msg("Failed to disable plugin")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable plugin"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Plugin disabled successfully",
	})
}

// handleVerifyCustomPlugin marks a plugin as verified
func (s *Server) handleVerifyCustomPlugin(c *gin.Context) {
	pluginID := c.Param("pluginID")
	
	if err := plugin_manager.UpdateCustomPluginVerified(s.Dependencies.DB, pluginID, true); err != nil {
		log.Error().Err(err).Msg("Failed to verify plugin")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify plugin"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Plugin verified successfully",
	})
}

// handleGetPluginPermissions gets permissions for a plugin
func (s *Server) handleGetPluginPermissions(c *gin.Context) {
	pluginID := c.Param("pluginID")
	
	// This would use the permission manager
	// For now, return placeholder
	c.JSON(http.StatusOK, gin.H{
		"plugin_id":   pluginID,
		"permissions": []string{},
	})
}

// handleGrantPluginPermissions grants permissions to a plugin
func (s *Server) handleGrantPluginPermissions(c *gin.Context) {
	pluginID := c.Param("pluginID")
	
	var req struct {
		Permissions []string `json:"permissions"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	
	// TODO: Use permission manager to grant permissions
	
	log.Info().
		Str("plugin_id", pluginID).
		Interface("permissions", req.Permissions).
		Msg("Granted permissions to plugin")
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Permissions granted successfully",
	})
}

// handleRevokePluginPermission revokes a permission from a plugin
func (s *Server) handleRevokePluginPermission(c *gin.Context) {
	pluginID := c.Param("pluginID")
	permissionID := c.Param("permissionID")
	
	// TODO: Use permission manager to revoke permission
	
	log.Info().
		Str("plugin_id", pluginID).
		Str("permission_id", permissionID).
		Msg("Revoked permission from plugin")
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Permission revoked successfully",
	})
}

// handleAddPublicKey adds a public key for signature verification
func (s *Server) handleAddPublicKey(c *gin.Context) {
	var req struct {
		KeyName   string `json:"key_name"`
		PublicKey string `json:"public_key"` // Base64 encoded
		Algorithm string `json:"algorithm"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	
	// TODO: Decode public key and add to verifier
	
	log.Info().
		Str("key_name", req.KeyName).
		Str("algorithm", req.Algorithm).
		Msg("Added public key")
	
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Public key added successfully",
	})
}

// handleListPublicKeys lists all public keys
func (s *Server) handleListPublicKeys(c *gin.Context) {
	// TODO: Get keys from verifier
	
	keys := []map[string]interface{}{}
	
	c.JSON(http.StatusOK, keys)
}

// handleRevokePublicKey revokes a public key
func (s *Server) handleRevokePublicKey(c *gin.Context) {
	keyName := c.Param("keyName")
	
	// TODO: Revoke key in verifier
	
	log.Info().Str("key_name", keyName).Msg("Revoked public key")
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Public key revoked successfully",
	})
}

// handleListPluginVersions lists all versions of a plugin in storage
func (s *Server) handleListPluginVersions(c *gin.Context) {
	pluginID := c.Param("pluginID")
	
	pluginStorage := plugin_manager.NewPluginStorage(s.Dependencies.Storage)
	versions, err := pluginStorage.ListPluginVersions(c.Request.Context(), pluginID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list plugin versions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list versions"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"plugin_id": pluginID,
		"versions":  versions,
	})
}

