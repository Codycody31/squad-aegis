package server

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/logwatcher_manager"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
)

func (s *Server) ServersList(c *gin.Context) {
	user := s.getUserFromSession(c)

	servers, err := core.GetServers(c.Request.Context(), s.Dependencies.DB, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get servers", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Servers fetched successfully", &gin.H{"servers": servers})
}

func (s *Server) ServersCreate(c *gin.Context) {
	var request models.ServerCreateRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	err := validation.ValidateStruct(&request,
		validation.Field(&request.Name, validation.Required),
		validation.Field(&request.IpAddress, validation.Required),
		validation.Field(&request.GamePort, validation.Required),
		validation.Field(&request.RconPort, validation.Required),
		validation.Field(&request.RconPassword, validation.Required),
	)

	if err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"errors": err})
		return
	}

	serverToCreate := models.Server{
		Id:            uuid.New(),
		Name:          request.Name,
		IpAddress:     request.IpAddress,
		GamePort:      request.GamePort,
		RconIpAddress: request.RconIpAddress,
		RconPort:      request.RconPort,
		RconPassword:  request.RconPassword,

		// Log configuration fields
		LogSourceType:    request.LogSourceType,
		LogFilePath:      request.LogFilePath,
		LogHost:          request.LogHost,
		LogPort:          request.LogPort,
		LogUsername:      request.LogUsername,
		LogPassword:      request.LogPassword,
		LogPollFrequency: request.LogPollFrequency,
		LogReadFromStart: request.LogReadFromStart,

		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	server, err := core.CreateServer(c.Request.Context(), s.Dependencies.DB, &serverToCreate)
	if err != nil {
		responses.BadRequest(c, "Failed to create server", &gin.H{"error": err.Error()})
		return
	}

	ipAddress := server.IpAddress
	if server.RconIpAddress != nil {
		ipAddress = *server.RconIpAddress
	}

	// Connect to RCON
	_ = s.Dependencies.RconManager.ConnectToServer(server.Id, ipAddress, server.RconPort, server.RconPassword)

	// Connect to logwatcher if log configuration is provided
	if server.LogSourceType != nil && server.LogFilePath != nil {
		config := logwatcher_manager.LogSourceConfig{
			Type:          logwatcher_manager.LogSourceType(*server.LogSourceType),
			FilePath:      *server.LogFilePath,
			ReadFromStart: false, // Default value
		}

		if server.LogHost != nil {
			config.Host = *server.LogHost
		}
		if server.LogPort != nil {
			config.Port = *server.LogPort
		}
		if server.LogUsername != nil {
			config.Username = *server.LogUsername
		}
		if server.LogPassword != nil {
			config.Password = *server.LogPassword
		}
		if server.LogPollFrequency != nil {
			config.PollFrequency = time.Duration(*server.LogPollFrequency) * time.Second
		}
		if server.LogReadFromStart != nil {
			config.ReadFromStart = *server.LogReadFromStart
		}

		_ = s.Dependencies.LogwatcherManager.ConnectToServer(server.Id, config)
	}

	responses.Success(c, "Server created successfully", &gin.H{"server": server})
}

// ServerGet handles retrieving a single server by ID
func (s *Server) ServerGet(c *gin.Context) {
	serverId := c.Param("serverId")
	if serverId == "" {
		responses.BadRequest(c, "Server ID is required", &gin.H{"error": "Server ID is required"})
		return
	}

	// Parse UUID
	serverUUID, err := uuid.Parse(serverId)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID format", &gin.H{"error": "Invalid server ID format"})
		return
	}

	// Get user from session
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	// Get server from database
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverUUID, user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Server not found", &gin.H{"error": "Server not found"})
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to fetch server"})
		return
	}

	responses.Success(c, "Server fetched successfully", &gin.H{
		"server": server,
	})
}

// ServerMetrics handles getting the metrics of a server
func (s *Server) ServerMetrics(c *gin.Context) {
	serverId := c.Param("serverId")
	if serverId == "" {
		responses.BadRequest(c, "Server ID is required", &gin.H{"error": "Server ID is required"})
		return
	}

	// Parse UUID
	serverUUID, err := uuid.Parse(serverId)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID format", &gin.H{"error": "Invalid server ID format"})
		return
	}

	// Get user from session
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	// Get server from database
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverUUID, user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Server not found", &gin.H{"error": "Server not found"})
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to fetch server"})
		return
	}

	// Get server status and metrics if possible
	var metrics map[string]interface{} = nil

	// Try to connect to RCON to check if server is online
	rconClient, err := squadRcon.NewSquadRconWithConnection(s.Dependencies.RconManager, serverUUID, server.IpAddress, server.RconPort, server.RconPassword)
	if err == nil {
		// Close the connection after checking
		defer rconClient.Close()

		// Get basic server info
		metrics = map[string]interface{}{}

		// Try to get player count
		playersData, err := rconClient.GetServerPlayers()
		if err == nil {
			totalTeam1 := 0
			totalTeam2 := 0

			for _, player := range playersData.OnlinePlayers {
				if player.TeamId == 1 {
					totalTeam1++
				} else if player.TeamId == 2 {
					totalTeam2++
				}
			}

			metrics["players"] = map[string]interface{}{
				"total": len(playersData.OnlinePlayers),
				"teams": map[string]interface{}{
					"1": totalTeam1,
					"2": totalTeam2,
				},
			}
		}

		// Try to get current map
		currentMap, err := rconClient.GetCurrentMap()
		if err == nil {
			metrics["current"] = currentMap
		}

		// Try to get the next map
		nextMap, err := rconClient.GetNextMap()
		if err == nil {
			metrics["next"] = nextMap
		}
	}

	responses.Success(c, "Server metrics fetched successfully", &gin.H{"metrics": metrics})
}

// ServerStatus handles getting the status of a server
func (s *Server) ServerStatus(c *gin.Context) {
	serverId := c.Param("serverId")
	if serverId == "" {
		responses.BadRequest(c, "Server ID is required", &gin.H{"error": "Server ID is required"})
		return
	}

	// Parse UUID
	serverUUID, err := uuid.Parse(serverId)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID format", &gin.H{"error": "Invalid server ID format"})
		return
	}

	// Get user from session
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	// Get server from database
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverUUID, user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Server not found", &gin.H{"error": "Server not found"})
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to fetch server"})
		return
	}

	// Get server status and metrics if possible
	serverStatus := map[string]interface{}{}

	// pinger, err := probing.NewPinger(server.IpAddress)
	// if err != nil {
	// 	panic(err)
	// }
	// err = pinger.Run() // Blocks until finished.
	// if err != nil {
	// 	serverStatus["ping"] = false
	// } else {
	// 	serverStatus["ping"] = true
	// }

	rconClient, err := squadRcon.NewSquadRconWithConnection(s.Dependencies.RconManager, serverUUID, server.IpAddress, server.RconPort, server.RconPassword)
	if err == nil {
		serverStatus["rcon"] = true
		defer rconClient.Close()
	} else {
		serverStatus["rcon"] = false
	}

	responses.Success(c, "Server status fetched successfully", &gin.H{"status": serverStatus})
}

// ServerDelete handles deleting a server
func (s *Server) ServerDelete(c *gin.Context) {
	user := s.getUserFromSession(c)

	// Only super admins can delete servers
	if !user.SuperAdmin {
		responses.Unauthorized(c, "Only super admins can delete servers", nil)
		return
	}

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	// Begin transaction
	tx, err := s.Dependencies.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to begin transaction"})
		return
	}
	defer tx.Rollback()

	chTx, err := s.Dependencies.Clickhouse.Begin(c.Request.Context())
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to begin transaction"})
		return
	}
	defer chTx.Rollback()

	// TODO: Get and shutdown plugins
	plugins := s.Dependencies.PluginManager.GetPluginInstances(serverId)
	for _, plugin := range plugins {
		err = s.Dependencies.PluginManager.DeletePluginInstance(serverId, plugin.ID)
		if err != nil {
			responses.InternalServerError(c, err, &gin.H{"error": "Failed to delete plugin"})
			return
		}

		_, err = tx.ExecContext(c.Request.Context(), `DELETE FROM plugin_data WHERE plugin_instance_id = $1`, plugin.ID)
		if err != nil {
			responses.InternalServerError(c, err, &gin.H{"error": "Failed to delete plugin data"})
			return
		}
	}

	_, err = tx.ExecContext(c.Request.Context(), `DELETE FROM plugin_instances WHERE server_id = $1`, serverId)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to delete plugin instances"})
		return
	}

	clickhouseTables := []string{
		"squad_aegis.plugin_logs",
		"squad_aegis.server_admin_broadcast_events",
		"squad_aegis.server_deployable_damaged_events",
		"squad_aegis.server_game_events_unified",
		"squad_aegis.server_join_succeeded_events",
		"squad_aegis.server_player_chat_messages",
		"squad_aegis.server_player_connected_events",
		"squad_aegis.server_player_damaged_events",
		"squad_aegis.server_player_died_events",
		"squad_aegis.server_player_possess_events",
		"squad_aegis.server_player_revived_events",
		"squad_aegis.server_player_wounded_events",
		"squad_aegis.server_tick_rate_events",
	}

	for _, table := range clickhouseTables {
		_, err = chTx.ExecContext(c.Request.Context(), fmt.Sprintf(`DELETE FROM %s WHERE server_id = $1`, table), serverId)
		if err != nil {
			responses.InternalServerError(c, err, &gin.H{"error": "Failed to delete plugin data from clickhouse"})
			return
		}
	}

	// Disconnect from RCON
	_ = s.Dependencies.RconManager.DisconnectFromServer(serverId, true)

	databaseTables := []string{
		"public.server_admins",
		"public.server_roles",
		"public.server_bans",
		"public.audit_logs",
		"public.server_ban_list_subscriptions",
	}

	for _, table := range databaseTables {
		_, err = tx.ExecContext(c.Request.Context(), fmt.Sprintf(`DELETE FROM %s WHERE server_id = $1`, table), serverId)
		if err != nil {
			responses.InternalServerError(c, err, &gin.H{"error": "Failed to delete server data from database"})
			return
		}
	}

	// Delete the server
	result, err := tx.ExecContext(c.Request.Context(), `DELETE FROM servers WHERE id = $1`, serverId)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to delete server"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to get rows affected"})
		return
	}

	if rowsAffected == 0 {
		responses.NotFound(c, "Server not found", nil)
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to commit transaction"})
		return
	}

	if err := chTx.Commit(); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to commit transaction"})
		return
	}

	responses.Success(c, "Server deleted successfully", nil)
}

// ServerUserRoles handles retrieving the user's permissions for all servers they have access to
func (s *Server) ServerUserRoles(c *gin.Context) {
	session := c.MustGet("session").(*models.Session)

	// Get user's server permissions
	serverPermissions, err := core.GetUserServerPermissions(c.Copy(), s.Dependencies.DB, session.UserId)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.Success(c, "User server permissions fetched successfully", &gin.H{
		"roles": serverPermissions,
	})
}

// ServerUpdate handles updating a server
func (s *Server) ServerUpdate(c *gin.Context) {
	serverId := c.Param("serverId")
	if serverId == "" {
		responses.BadRequest(c, "Server ID is required", &gin.H{"error": "Server ID is required"})
		return
	}

	// Parse UUID
	serverUUID, err := uuid.Parse(serverId)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID format", &gin.H{"error": "Invalid server ID format"})
		return
	}

	// Get user from session
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	// Get server from database
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverUUID, user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responses.NotFound(c, "Server not found", &gin.H{"error": "Server not found"})
			return
		}
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to fetch server"})
		return
	}

	var request models.ServerUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Validate request
	err = validation.ValidateStruct(&request,
		validation.Field(&request.Name, validation.Required),
		validation.Field(&request.IpAddress, validation.Required),
		validation.Field(&request.GamePort, validation.Required),
		validation.Field(&request.RconPort, validation.Required),
	)

	if err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"errors": err})
		return
	}

	// Update server fields
	server.Name = request.Name
	server.IpAddress = request.IpAddress
	server.GamePort = request.GamePort
	server.RconIpAddress = request.RconIpAddress
	server.RconPort = request.RconPort

	if request.RconPassword != "" {
		server.RconPassword = request.RconPassword
	}

	// Update log configuration fields
	server.LogSourceType = request.LogSourceType
	server.LogFilePath = request.LogFilePath
	server.LogHost = request.LogHost
	server.LogPort = request.LogPort
	server.LogUsername = request.LogUsername
	if request.LogPassword != nil && *request.LogPassword != "" {
		server.LogPassword = request.LogPassword
	}
	server.LogPollFrequency = request.LogPollFrequency
	server.LogReadFromStart = request.LogReadFromStart

	// Update server in database
	if err := core.UpdateServer(c.Request.Context(), s.Dependencies.DB, server); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to update server"})
		return
	}

	ipAddress := server.IpAddress
	if server.RconIpAddress != nil {
		ipAddress = *server.RconIpAddress
	}

	// Reconnect RCON with new credentials
	_ = s.Dependencies.RconManager.ConnectToServer(server.Id, ipAddress, server.RconPort, server.RconPassword)

	// Reconnect logwatcher if log configuration is provided
	if server.LogSourceType != nil && server.LogFilePath != nil {
		config := logwatcher_manager.LogSourceConfig{
			Type:          logwatcher_manager.LogSourceType(*server.LogSourceType),
			FilePath:      *server.LogFilePath,
			ReadFromStart: false, // Default value
		}

		if server.LogHost != nil {
			config.Host = *server.LogHost
		}
		if server.LogPort != nil {
			config.Port = *server.LogPort
		}
		if server.LogUsername != nil {
			config.Username = *server.LogUsername
		}
		if server.LogPassword != nil {
			config.Password = *server.LogPassword
		}
		if server.LogPollFrequency != nil {
			config.PollFrequency = time.Duration(*server.LogPollFrequency) * time.Second
		}
		if server.LogReadFromStart != nil {
			config.ReadFromStart = *server.LogReadFromStart
		}

		_ = s.Dependencies.LogwatcherManager.ConnectToServer(server.Id, config)
	} else {
		// Disconnect from logwatcher if log configuration is removed
		_ = s.Dependencies.LogwatcherManager.DisconnectFromServer(server.Id)
	}

	// Create audit log entry
	auditData := map[string]interface{}{
		"serverId":    server.Id.String(),
		"name":        server.Name,
		"ipAddress":   server.IpAddress,
		"gamePort":    server.GamePort,
		"rconIp":      server.RconIpAddress,
		"rconPort":    server.RconPort,
		"rconUpdated": true,
	}
	s.CreateAuditLog(c.Request.Context(), &server.Id, &user.Id, "server:update", auditData)

	responses.Success(c, "Server updated successfully", &gin.H{"server": server})
}
