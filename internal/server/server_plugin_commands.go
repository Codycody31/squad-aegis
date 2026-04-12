package server

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// ServerPluginCommandsList returns available commands for a plugin instance
func (s *Server) ServerPluginCommandsList(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

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

	commands, err := s.Dependencies.PluginManager.GetPluginCommands(serverID, instanceID)
	if err != nil {
		log.Error().Err(err).Str("server_id", serverID.String()).Str("instance_id", instanceID.String()).Msg("Failed to get plugin commands")
		responses.BadRequest(c, "Failed to get plugin commands", nil)
		return
	}

	filteredCommands := []interface{}{}
	for _, cmd := range commands {
		hasPermission := true
		if len(cmd.RequiredPermissions) > 0 {
			hasPermission, err = s.userHasAnyServerPermission(c, user.Id, serverID, cmd.RequiredPermissions)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to check user permissions for command")
				hasPermission = false
			}
		}

		if hasPermission {
			filteredCommands = append(filteredCommands, cmd)
		}
	}

	responses.Success(c, "Commands fetched successfully", &gin.H{
		"commands": filteredCommands,
	})
}

// ServerPluginCommandExecute executes a command on a plugin instance
func (s *Server) ServerPluginCommandExecute(c *gin.Context) {
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

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

	commandID := c.Param("commandId")
	if commandID == "" {
		responses.BadRequest(c, "Command ID is required", nil)
		return
	}

	var requestBody struct {
		Params map[string]interface{} `json:"params"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		responses.BadRequest(c, "Invalid request body", &gin.H{"error": err.Error()})
		return
	}

	if requestBody.Params == nil {
		requestBody.Params = make(map[string]interface{})
	}

	commands, err := s.Dependencies.PluginManager.GetPluginCommands(serverID, instanceID)
	if err != nil {
		log.Error().Err(err).Str("server_id", serverID.String()).Str("instance_id", instanceID.String()).Msg("Failed to get plugin commands")
		responses.BadRequest(c, "Failed to get plugin commands", nil)
		return
	}

	var targetCommand *plugin_manager.PluginCommand
	for i := range commands {
		if commands[i].ID == commandID {
			targetCommand = &commands[i]
			break
		}
	}

	if targetCommand == nil {
		responses.NotFound(c, "Command not found", nil)
		return
	}

	if len(targetCommand.RequiredPermissions) > 0 {
		hasPermission, err := s.userHasAnyServerPermission(c, user.Id, serverID, targetCommand.RequiredPermissions)
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("failed to check permissions: %w", err), nil)
			return
		}

		if !hasPermission {
			responses.Forbidden(c, "Insufficient permissions to execute this command", nil)
			return
		}
	}

	result, err := s.Dependencies.PluginManager.ExecutePluginCommand(serverID, instanceID, commandID, requestBody.Params)
	if err != nil {
		log.Error().Err(err).Str("server_id", serverID.String()).Str("instance_id", instanceID.String()).Str("command_id", commandID).Msg("Failed to execute plugin command")
		responses.BadRequest(c, "Failed to execute command", nil)
		return
	}

	auditData := map[string]interface{}{
		"commandId": commandID,
		"params":    requestBody.Params,
	}
	if result.ExecutionID != "" {
		auditData["executionId"] = result.ExecutionID
	}

	s.CreateAuditLog(c.Request.Context(), &serverID, &user.Id, "plugin:command:execute", auditData)

	log.Info().
		Str("server_id", serverID.String()).
		Str("plugin_instance_id", instanceID.String()).
		Str("command_id", commandID).
		Str("user_id", user.Id.String()).
		Msg("Executed plugin command")

	responses.Success(c, "Command executed successfully", &gin.H{
		"result": result,
	})
}

// ServerPluginCommandStatus gets async command execution status
func (s *Server) ServerPluginCommandStatus(c *gin.Context) {
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

	executionID := c.Param("executionId")
	if executionID == "" {
		responses.BadRequest(c, "Execution ID is required", nil)
		return
	}

	status, err := s.Dependencies.PluginManager.GetCommandExecutionStatus(serverID, instanceID, executionID)
	if err != nil {
		log.Error().Err(err).Str("server_id", serverID.String()).Str("instance_id", instanceID.String()).Str("execution_id", executionID).Msg("Failed to get command execution status")
		responses.BadRequest(c, "Failed to get command execution status", nil)
		return
	}

	responses.Success(c, "Execution status fetched successfully", &gin.H{
		"status": status,
	})
}
