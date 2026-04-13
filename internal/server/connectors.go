package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// ConnectorListAvailable returns all available connector definitions
func (s *Server) ConnectorListAvailable(c *gin.Context) {
	if !s.requirePluginManager(c) {
		return
	}

	connectors := s.Dependencies.PluginManager.ListAvailableConnectorDefinitions()
	responses.Success(c, "Available connectors fetched successfully", &gin.H{"connectors": connectors})
}

// ConnectorList returns all configured connectors
func (s *Server) ConnectorList(c *gin.Context) {
	if !s.requirePluginManager(c) {
		return
	}

	connectors := s.Dependencies.PluginManager.GetConnectors()
	responses.Success(c, "Connectors fetched successfully", &gin.H{"connectors": connectors})
}

// ConnectorCreate creates a new connector instance
func (s *Server) ConnectorCreate(c *gin.Context) {
	if !s.requirePluginManager(c) {
		return
	}

	var request struct {
		ConnectorID string                 `json:"connector_id" binding:"required"`
		Config      map[string]interface{} `json:"config" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	instance, err := s.Dependencies.PluginManager.CreateConnectorInstance(request.ConnectorID, request.Config)
	if err != nil {
		log.Error().Err(err).Str("connector_id", request.ConnectorID).Msg("Failed to create connector")
		responses.BadRequest(c, "Failed to create connector", nil)
		return
	}

	responses.Success(c, "Connector created successfully", &gin.H{"connector": instance})
}

// ConnectorUpdate updates a connector's configuration
func (s *Server) ConnectorUpdate(c *gin.Context) {
	if !s.requirePluginManager(c) {
		return
	}

	connectorID := c.Param("connectorId")
	if connectorID == "" {
		responses.BadRequest(c, "Connector ID is required", &gin.H{})
		return
	}

	var request struct {
		Config map[string]interface{} `json:"config" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	if err := s.Dependencies.PluginManager.UpdateConnectorConfig(connectorID, request.Config); err != nil {
		log.Error().Err(err).Str("connector_id", connectorID).Msg("Failed to update connector")
		responses.BadRequest(c, "Failed to update connector", nil)
		return
	}

	responses.Success(c, "Connector updated successfully", nil)
}

// ConnectorDelete deletes a connector instance
func (s *Server) ConnectorDelete(c *gin.Context) {
	if !s.requirePluginManager(c) {
		return
	}

	connectorID := c.Param("connectorId")
	if connectorID == "" {
		responses.BadRequest(c, "Connector ID is required", &gin.H{})
		return
	}

	if err := s.Dependencies.PluginManager.DeleteConnectorInstance(connectorID); err != nil {
		log.Error().Err(err).Str("connector_id", connectorID).Msg("Failed to delete connector")
		responses.BadRequest(c, "Failed to delete connector", nil)
		return
	}

	responses.Success(c, "Connector deleted successfully", nil)
}

// ConnectorPackageListInstalled lists globally installed sideloaded connector packages (native .so).
func (s *Server) ConnectorPackageListInstalled(c *gin.Context) {
	if !s.requirePluginManager(c) {
		return
	}

	packages := s.Dependencies.PluginManager.ListInstalledConnectorPackages()
	responses.Success(c, "Installed connector packages fetched successfully", &gin.H{"connectors": packages})
}

// ConnectorPackageUpload installs a sideloaded connector package (zip: manifest + .so) uploaded by a super admin.
func (s *Server) ConnectorPackageUpload(c *gin.Context) {
	if !s.requireSuperAdmin(c) {
		return
	}
	if !s.requirePluginManager(c) {
		return
	}

	maxUploadSize := pluginUploadMaxBytes()
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize+1024)

	file, err := c.FormFile("bundle")
	if err != nil {
		responses.BadRequest(c, "No connector bundle provided", &gin.H{})
		return
	}

	if !strings.HasSuffix(strings.ToLower(file.Filename), ".zip") {
		responses.BadRequest(c, "Connector bundle must be a .zip file", &gin.H{})
		return
	}

	if file.Size > maxUploadSize {
		responses.BadRequest(c, "Connector bundle is too large", &gin.H{
			"file_size": file.Size,
			"max_size":  maxUploadSize,
		})
		return
	}

	openedFile, err := file.Open()
	if err != nil {
		log.Error().Err(err).Msg("Failed to open uploaded connector bundle")
		responses.BadRequest(c, "Failed to open connector bundle", &gin.H{})
		return
	}
	defer openedFile.Close()

	pkg, err := s.Dependencies.PluginManager.InstallConnectorPackageFromBundle(c.Request.Context(), openedFile, file.Size, file.Filename)
	if err != nil {
		log.Error().Err(err).Str("filename", file.Filename).Msg("Failed to install uploaded connector bundle")
		s.CreateAuditLog(c.Request.Context(), nil, s.pluginAuditActorID(c), "connector:package:upload_failed", gin.H{
			"filename": file.Filename,
			"size":     file.Size,
			"error":    err.Error(),
		})
		responses.BadRequest(c, "Failed to install uploaded connector bundle", nil)
		return
	}

	s.CreateAuditLog(c.Request.Context(), nil, s.pluginAuditActorID(c), "connector:package:upload", gin.H{
		"connector_id":       pkg.ConnectorID,
		"version":            pkg.Version,
		"checksum":           pkg.Checksum,
		"signature_verified": pkg.SignatureVerified,
		"install_state":      pkg.InstallState,
		"filename":           file.Filename,
	})

	message := "Connector package uploaded successfully"
	if pkg.InstallState == plugin_manager.PluginInstallStatePendingRestart {
		message = "Connector package uploaded successfully and will activate after restart"
	}

	responses.Success(c, message, &gin.H{"connector": pkg})
}

// ConnectorPackageInstalledDelete removes an installed sideloaded connector package.
func (s *Server) ConnectorPackageInstalledDelete(c *gin.Context) {
	if !s.requireSuperAdmin(c) {
		return
	}
	if !s.requirePluginManager(c) {
		return
	}

	connectorID := c.Param("connectorId")
	if connectorID == "" {
		responses.BadRequest(c, "Connector ID is required", &gin.H{})
		return
	}

	if err := s.Dependencies.PluginManager.DeleteInstalledConnectorPackage(c.Request.Context(), connectorID); err != nil {
		log.Error().Err(err).Str("connector_id", connectorID).Msg("Failed to delete installed connector package")
		s.CreateAuditLog(c.Request.Context(), nil, s.pluginAuditActorID(c), "connector:package:delete_failed", gin.H{
			"connector_id": connectorID,
			"error":        err.Error(),
		})
		responses.BadRequest(c, "Failed to delete installed connector package", nil)
		return
	}

	s.CreateAuditLog(c.Request.Context(), nil, s.pluginAuditActorID(c), "connector:package:delete", gin.H{
		"connector_id": connectorID,
	})

	responses.Success(c, "Connector package deleted successfully", nil)
}
