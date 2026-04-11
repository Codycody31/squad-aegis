package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

func pluginUploadMaxBytes() int64 {
	if maxUploadSize := config.Config.Plugins.MaxUploadSize; maxUploadSize > 0 {
		return maxUploadSize
	}
	return 50 * 1024 * 1024
}

func (s *Server) pluginAuditActorID(c *gin.Context) *uuid.UUID {
	user := s.getUserFromSession(c)
	if user == nil {
		return nil
	}
	id := user.Id
	return &id
}

// requirePluginManager writes a 500 response and returns false when the
// plugin manager dependency is missing, so handlers can early-return with
// `if !s.requirePluginManager(c) { return }`.
func (s *Server) requirePluginManager(c *gin.Context) bool {
	if s.Dependencies.PluginManager == nil {
		responses.InternalServerError(c, errors.New("plugin manager not available"), nil)
		return false
	}
	return true
}

// PluginListAvailable returns all available plugin definitions
func (s *Server) PluginListAvailable(c *gin.Context) {
	if !s.requirePluginManager(c) {
		return
	}

	plugins := s.Dependencies.PluginManager.ListAvailablePlugins()
	responses.Success(c, "Available plugins fetched successfully", &gin.H{"plugins": plugins})
}

// PluginListInstalled returns all globally installed plugin packages.
func (s *Server) PluginListInstalled(c *gin.Context) {
	if !s.requirePluginManager(c) {
		return
	}

	packages := s.Dependencies.PluginManager.ListInstalledPluginPackages()
	responses.Success(c, "Installed plugins fetched successfully", &gin.H{"plugins": packages})
}

// PluginUpload installs a plugin package uploaded by a super admin.
func (s *Server) PluginUpload(c *gin.Context) {
	if !s.requirePluginManager(c) {
		return
	}

	maxUploadSize := pluginUploadMaxBytes()
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize+1024)

	file, err := c.FormFile("bundle")
	if err != nil {
		responses.BadRequest(c, "No plugin bundle provided", &gin.H{})
		return
	}

	if file.Size > maxUploadSize {
		responses.BadRequest(c, "Plugin bundle is too large", &gin.H{
			"file_size": file.Size,
			"max_size":  maxUploadSize,
		})
		return
	}

	openedFile, err := file.Open()
	if err != nil {
		log.Error().Err(err).Msg("Failed to open uploaded plugin bundle")
		responses.BadRequest(c, "Failed to open plugin bundle", &gin.H{})
		return
	}
	defer openedFile.Close()

	pkg, err := s.Dependencies.PluginManager.InstallPluginPackageFromBundle(c.Request.Context(), openedFile, file.Size, file.Filename)
	if err != nil {
		log.Error().Err(err).Str("filename", file.Filename).Msg("Failed to install uploaded plugin bundle")
		s.CreateAuditLog(c.Request.Context(), nil, s.pluginAuditActorID(c), "plugin:package:upload_failed", gin.H{
			"filename": file.Filename,
			"size":     file.Size,
		})
		responses.BadRequest(c, "Failed to install uploaded plugin bundle", &gin.H{})
		return
	}

	s.CreateAuditLog(c.Request.Context(), nil, s.pluginAuditActorID(c), "plugin:package:upload", gin.H{
		"plugin_id":          pkg.PluginID,
		"version":            pkg.Version,
		"checksum":           pkg.Checksum,
		"signature_verified": pkg.SignatureVerified,
		"install_state":      pkg.InstallState,
		"filename":           file.Filename,
	})

	message := "Plugin uploaded successfully"
	if pkg.InstallState == plugin_manager.PluginInstallStatePendingRestart {
		message = "Plugin uploaded successfully and will activate after restart"
	}

	responses.Success(c, message, &gin.H{"plugin": pkg})
}

// PluginInstalledDelete removes an installed native plugin package.
func (s *Server) PluginInstalledDelete(c *gin.Context) {
	if !s.requirePluginManager(c) {
		return
	}

	pluginID := c.Param("pluginId")
	if pluginID == "" {
		responses.BadRequest(c, "Plugin ID is required", &gin.H{})
		return
	}

	if err := s.Dependencies.PluginManager.DeleteInstalledPluginPackage(c.Request.Context(), pluginID); err != nil {
		log.Error().Err(err).Str("plugin_id", pluginID).Msg("Failed to delete installed plugin")
		s.CreateAuditLog(c.Request.Context(), nil, s.pluginAuditActorID(c), "plugin:package:delete_failed", gin.H{
			"plugin_id": pluginID,
		})
		responses.BadRequest(c, "Failed to delete installed plugin", &gin.H{})
		return
	}

	s.CreateAuditLog(c.Request.Context(), nil, s.pluginAuditActorID(c), "plugin:package:delete", gin.H{
		"plugin_id": pluginID,
	})

	responses.Success(c, "Plugin deleted successfully", nil)
}

// ServerPluginList returns all plugin instances for a server
func (s *Server) ServerPluginList(c *gin.Context) {
	if !s.requirePluginManager(c) {
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	plugins := s.Dependencies.PluginManager.GetPluginInstances(serverID)
	responses.Success(c, "Server plugins fetched successfully", &gin.H{"plugins": plugins})
}

// ServerPluginCreate creates a new plugin instance for a server
func (s *Server) ServerPluginCreate(c *gin.Context) {
	if !s.requirePluginManager(c) {
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	var request struct {
		PluginID string                 `json:"plugin_id" binding:"required"`
		Notes    string                 `json:"notes"`
		Config   map[string]interface{} `json:"config"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	if request.Config == nil {
		request.Config = make(map[string]interface{})
	}

	instance, err := s.Dependencies.PluginManager.CreatePluginInstance(serverID, request.PluginID, request.Notes, request.Config)
	if err != nil {
		responses.BadRequest(c, "Failed to create plugin instance", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Plugin instance created successfully", &gin.H{"plugin": instance})
}

// ServerPluginGet returns a specific plugin instance
func (s *Server) ServerPluginGet(c *gin.Context) {
	if !s.requirePluginManager(c) {
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	instanceID, err := uuid.Parse(c.Param("pluginId"))
	if err != nil {
		responses.BadRequest(c, "Invalid plugin instance ID", &gin.H{"error": err.Error()})
		return
	}

	instance, err := s.Dependencies.PluginManager.GetPluginInstance(serverID, instanceID)
	if err != nil {
		responses.NotFound(c, "Plugin instance not found", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Plugin instance fetched successfully", &gin.H{"plugin": instance})
}

// ServerPluginUpdate updates a plugin instance's configuration and settings
func (s *Server) ServerPluginUpdate(c *gin.Context) {
	if !s.requirePluginManager(c) {
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	instanceID, err := uuid.Parse(c.Param("pluginId"))
	if err != nil {
		responses.BadRequest(c, "Invalid plugin instance ID", &gin.H{"error": err.Error()})
		return
	}

	var request struct {
		Config   map[string]interface{} `json:"config"`
		LogLevel *string                `json:"log_level"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// At least one field must be provided
	if request.Config == nil && request.LogLevel == nil {
		responses.BadRequest(c, "At least one of config or log_level must be provided", &gin.H{})
		return
	}

	// Update config if provided
	if request.Config != nil {
		if err := s.Dependencies.PluginManager.UpdatePluginConfig(serverID, instanceID, request.Config); err != nil {
			responses.BadRequest(c, "Failed to update plugin instance config", &gin.H{"error": err.Error()})
			return
		}
	}

	// Update log level if provided
	if request.LogLevel != nil {
		if err := s.Dependencies.PluginManager.UpdatePluginLogLevel(serverID, instanceID, *request.LogLevel); err != nil {
			responses.BadRequest(c, "Failed to update plugin instance log level", &gin.H{"error": err.Error()})
			return
		}
	}

	log.Info().Str("server_id", serverID.String()).Str("plugin_id", instanceID.String()).Msg("Updated plugin instance configuration")
	responses.Success(c, "Plugin instance updated successfully", nil)
}

// ServerPluginEnable enables a plugin instance
func (s *Server) ServerPluginEnable(c *gin.Context) {
	if !s.requirePluginManager(c) {
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	instanceID, err := uuid.Parse(c.Param("pluginId"))
	if err != nil {
		responses.BadRequest(c, "Invalid plugin instance ID", &gin.H{"error": err.Error()})
		return
	}

	if err := s.Dependencies.PluginManager.EnablePluginInstance(serverID, instanceID); err != nil {
		responses.BadRequest(c, "Failed to enable plugin instance", &gin.H{"error": err.Error()})
		return
	}

	log.Info().Str("server_id", serverID.String()).Str("plugin_id", instanceID.String()).Msg("Enabled plugin instance")
	responses.Success(c, "Plugin instance enabled successfully", nil)
}

// ServerPluginDisable disables a plugin instance
func (s *Server) ServerPluginDisable(c *gin.Context) {
	if !s.requirePluginManager(c) {
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	instanceID, err := uuid.Parse(c.Param("pluginId"))
	if err != nil {
		responses.BadRequest(c, "Invalid plugin instance ID", &gin.H{"error": err.Error()})
		return
	}

	if err := s.Dependencies.PluginManager.DisablePluginInstance(serverID, instanceID); err != nil {
		responses.BadRequest(c, "Failed to disable plugin instance", &gin.H{"error": err.Error()})
		return
	}

	log.Debug().
		Str("server_id", serverID.String()).
		Str("plugin_instance_id", instanceID.String()).
		Msg("Plugin instance disabled")
	responses.Success(c, "Plugin instance disabled successfully", nil)
}

// ServerPluginDelete deletes a plugin instance
func (s *Server) ServerPluginDelete(c *gin.Context) {
	if !s.requirePluginManager(c) {
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	instanceID, err := uuid.Parse(c.Param("pluginId"))
	if err != nil {
		responses.BadRequest(c, "Invalid plugin instance ID", &gin.H{"error": err.Error()})
		return
	}

	if err := s.Dependencies.PluginManager.DeletePluginInstance(serverID, instanceID); err != nil {
		responses.BadRequest(c, "Failed to delete plugin instance", &gin.H{"error": err.Error()})
		return
	}

	log.Info().Str("server_id", serverID.String()).Str("plugin_id", instanceID.String()).Msg("Deleted plugin instance")
	responses.Success(c, "Plugin instance deleted successfully", nil)
}

// ServerPluginLogs returns recent logs for a plugin instance
func (s *Server) ServerPluginLogs(c *gin.Context) {
	if !s.requirePluginManager(c) {
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	instanceID, err := uuid.Parse(c.Param("pluginId"))
	if err != nil {
		responses.BadRequest(c, "Invalid plugin instance ID", &gin.H{"error": err.Error()})
		return
	}

	// Parse query parameters
	limit := 100 // default
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	before := c.Query("before")
	after := c.Query("after")
	order := c.Query("order")
	level := c.Query("level")
	search := c.Query("search")

	// Get logs from ClickHouse via PluginManager
	logs, err := s.Dependencies.PluginManager.GetPluginLogs(serverID, instanceID, limit, before, after, order, level, search)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to retrieve plugin logs: %w", err), nil)
		return
	}

	responses.Success(c, "Plugin logs fetched successfully", &gin.H{"logs": logs})
}

// ServerPluginLogsAll returns aggregated logs for all plugin instances for a server
func (s *Server) ServerPluginLogsAll(c *gin.Context) {
	if !s.requirePluginManager(c) {
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	// Parse query parameters
	limit := 100 // default
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	before := c.Query("before")
	after := c.Query("after")
	order := c.Query("order")
	level := c.Query("level")
	search := c.Query("search")

	// Get aggregated logs from ClickHouse via PluginManager
	logs, err := s.Dependencies.PluginManager.GetServerPluginLogs(serverID, limit, before, after, order, level, search)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to retrieve server plugin logs: %w", err), nil)
		return
	}

	responses.Success(c, "Server plugin logs fetched successfully", &gin.H{"logs": logs})
}

// ServerPluginMetrics returns metrics for a plugin instance
func (s *Server) ServerPluginMetrics(c *gin.Context) {
	// TODO: Implement plugin-specific metrics retrieval from ClickHouse
	responses.Success(c, "Plugin metrics fetched successfully", &gin.H{"metrics": &gin.H{}})
}
