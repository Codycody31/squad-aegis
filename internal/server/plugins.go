package server

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// PluginListAvailable returns all available plugin definitions
func (s *Server) PluginListAvailable(c *gin.Context) {
	if s.Dependencies.PluginManager == nil {
		responses.InternalServerError(c, errors.New("plugin manager not available"), nil)
		return
	}

	plugins := s.Dependencies.PluginManager.ListAvailablePlugins()
	responses.Success(c, "Available plugins fetched successfully", &gin.H{"plugins": plugins})
}

// ConnectorListAvailable returns all available connector definitions
func (s *Server) ConnectorListAvailable(c *gin.Context) {
	if s.Dependencies.PluginManager == nil {
		responses.InternalServerError(c, errors.New("plugin manager not available"), nil)
		return
	}

	connectors := s.Dependencies.PluginManager.ListAvailableConnectorDefinitions()
	responses.Success(c, "Available connectors fetched successfully", &gin.H{"connectors": connectors})
}

// ConnectorList returns all configured connectors
func (s *Server) ConnectorList(c *gin.Context) {
	if s.Dependencies.PluginManager == nil {
		responses.InternalServerError(c, errors.New("plugin manager not available"), nil)
		return
	}

	connectors := s.Dependencies.PluginManager.GetConnectors()
	responses.Success(c, "Connectors fetched successfully", &gin.H{"connectors": connectors})
}

// ConnectorCreate creates a new connector instance
func (s *Server) ConnectorCreate(c *gin.Context) {
	if s.Dependencies.PluginManager == nil {
		responses.InternalServerError(c, errors.New("plugin manager not available"), nil)
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
		responses.BadRequest(c, "Failed to create connector", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Connector created successfully", &gin.H{"connector": instance})
}

// ConnectorUpdate updates a connector's configuration
func (s *Server) ConnectorUpdate(c *gin.Context) {
	if s.Dependencies.PluginManager == nil {
		responses.InternalServerError(c, errors.New("plugin manager not available"), nil)
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
		responses.BadRequest(c, "Failed to update connector", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Connector updated successfully", nil)
}

// ConnectorDelete deletes a connector instance
func (s *Server) ConnectorDelete(c *gin.Context) {
	if s.Dependencies.PluginManager == nil {
		responses.InternalServerError(c, errors.New("plugin manager not available"), nil)
		return
	}

	connectorID := c.Param("connectorId")
	if connectorID == "" {
		responses.BadRequest(c, "Connector ID is required", &gin.H{})
		return
	}

	if err := s.Dependencies.PluginManager.DeleteConnectorInstance(connectorID); err != nil {
		responses.BadRequest(c, "Failed to delete connector", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Connector deleted successfully", nil)
}

// ServerPluginList returns all plugin instances for a server
func (s *Server) ServerPluginList(c *gin.Context) {
	if s.Dependencies.PluginManager == nil {
		responses.InternalServerError(c, errors.New("plugin manager not available"), nil)
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
	if s.Dependencies.PluginManager == nil {
		responses.InternalServerError(c, errors.New("plugin manager not available"), nil)
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
	if s.Dependencies.PluginManager == nil {
		responses.InternalServerError(c, errors.New("plugin manager not available"), nil)
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

// ServerPluginUpdate updates a plugin instance's configuration
func (s *Server) ServerPluginUpdate(c *gin.Context) {
	if s.Dependencies.PluginManager == nil {
		responses.InternalServerError(c, errors.New("plugin manager not available"), nil)
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
		Config map[string]interface{} `json:"config" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	if err := s.Dependencies.PluginManager.UpdatePluginConfig(serverID, instanceID, request.Config); err != nil {
		responses.BadRequest(c, "Failed to update plugin instance", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Plugin instance updated successfully", nil)
}

// ServerPluginEnable enables a plugin instance
func (s *Server) ServerPluginEnable(c *gin.Context) {
	if s.Dependencies.PluginManager == nil {
		responses.InternalServerError(c, errors.New("plugin manager not available"), nil)
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

	responses.Success(c, "Plugin instance enabled successfully", nil)
}

// ServerPluginDisable disables a plugin instance
func (s *Server) ServerPluginDisable(c *gin.Context) {
	if s.Dependencies.PluginManager == nil {
		responses.InternalServerError(c, errors.New("plugin manager not available"), nil)
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

	responses.Success(c, "Plugin instance disabled successfully", nil)
}

// ServerPluginDelete deletes a plugin instance
func (s *Server) ServerPluginDelete(c *gin.Context) {
	if s.Dependencies.PluginManager == nil {
		responses.InternalServerError(c, errors.New("plugin manager not available"), nil)
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

	responses.Success(c, "Plugin instance deleted successfully", nil)
}

// ServerPluginLogs returns recent logs for a plugin instance
func (s *Server) ServerPluginLogs(c *gin.Context) {
	if s.Dependencies.PluginManager == nil {
		responses.InternalServerError(c, errors.New("plugin manager not available"), nil)
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
	if s.Dependencies.PluginManager == nil {
		responses.InternalServerError(c, errors.New("plugin manager not available"), nil)
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
