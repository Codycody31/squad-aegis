package server

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

var sensitiveParamSubstrings = []string{"password", "secret", "token", "key", "api_key"}

// schemaSensitiveFieldSet returns the set of param names declared sensitive=true
// in the command schema. Nested objects are walked recursively; the path is
// joined with "." so a nested sensitive field is matched when its full path
// is the param key.
func schemaSensitiveFieldSet(schema plug_config_schema.ConfigSchema) map[string]bool {
	out := make(map[string]bool)
	walkSchemaSensitive(schema.Fields, "", out)
	return out
}

func walkSchemaSensitive(fields []plug_config_schema.ConfigField, prefix string, out map[string]bool) {
	for _, f := range fields {
		path := f.Name
		if prefix != "" {
			path = prefix + "." + f.Name
		}
		if f.Sensitive {
			out[path] = true
			out[strings.ToLower(path)] = true
		}
		if len(f.Nested) > 0 {
			walkSchemaSensitive(f.Nested, path, out)
		}
	}
}

// redactSensitiveParams returns a copy of params with sensitive fields masked.
// A field is treated as sensitive when its name (or fully-qualified path)
// appears in the schema with sensitive=true, OR when it matches one of the
// substring fallbacks (password/secret/token/...). The schema-driven path
// honors the plugin author's declared metadata and catches sensitive fields
// whose names do not contain a sensitive substring.
func redactSensitiveParams(params map[string]interface{}, schema plug_config_schema.ConfigSchema) map[string]interface{} {
	sensitiveSet := schemaSensitiveFieldSet(schema)
	redacted := make(map[string]interface{}, len(params))
	for k, v := range params {
		lower := strings.ToLower(k)
		isSensitive := sensitiveSet[k] || sensitiveSet[lower]
		if !isSensitive {
			for _, needle := range sensitiveParamSubstrings {
				if strings.Contains(lower, needle) {
					isSensitive = true
					break
				}
			}
		}
		if isSensitive {
			redacted[k] = "***REDACTED***"
		} else {
			redacted[k] = v
		}
	}
	return redacted
}

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
		"params":    redactSensitiveParams(requestBody.Params, targetCommand.Parameters),
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
