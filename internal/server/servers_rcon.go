package server

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/commands"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
)

// Request structs for player actions
type KickPlayerRequest struct {
	SteamId string `json:"steam_id" binding:"required"`
	Reason  string `json:"reason"`
}

type WarnPlayerRequest struct {
	SteamId string `json:"steam_id" binding:"required"`
	Message string `json:"message" binding:"required"`
}

type MovePlayerRequest struct {
	SteamId string `json:"steam_id" binding:"required"`
}

// RconCommandList handles the listing of all commands that can be executed by the server
func (s *Server) RconCommandList(c *gin.Context) {
	var commandsList []commands.CommandInfo

	for _, command := range commands.CommandMatrix {
		if command.SupportsRCON {
			commandsList = append(commandsList, command)
		}
	}

	responses.Success(c, "Commands fetched successfully", &gin.H{"commands": commandsList})
}

// RconCommandAutocomplete handles the auto-complete functionality for commands
func (s *Server) RconCommandAutocomplete(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		responses.BadRequest(c, "Query parameter 'q' is required", &gin.H{"error": "Query parameter 'q' is required"})
		return
	}

	var matches []commands.CommandInfo
	for _, command := range commands.CommandMatrix {
		if strings.Contains(strings.ToLower(command.Name), strings.ToLower(query)) && command.SupportsRCON {
			matches = append(matches, command)
		}
	}

	responses.Success(c, "Commands fetched successfully", &gin.H{"commands": matches})
}

func (s *Server) ServerRconExecute(c *gin.Context) {
	user := s.getUserFromSession(c)

	var request struct {
		Command string `json:"command" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	ipAddress := server.IpAddress
	if server.RconIpAddress != nil {
		ipAddress = *server.RconIpAddress
	}

	// Ensure server is connected to RCON manager
	err = s.Dependencies.RconManager.ConnectToServer(serverId, ipAddress, server.RconPort, server.RconPassword)
	if err != nil {
		responses.BadRequest(c, "Failed to connect to RCON", &gin.H{"error": err.Error()})
		return
	}

	// Execute command using RCON manager
	response, err := s.Dependencies.RconManager.ExecuteCommand(serverId, request.Command)
	if err != nil {
		responses.BadRequest(c, "Failed to execute RCON command", &gin.H{"error": err.Error()})
		return
	}

	// Create detailed audit log
	auditData := map[string]interface{}{
		"command": request.Command,
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:rcon:execute", auditData)

	responses.Success(c, "RCON command executed successfully", &gin.H{"response": response})
}

func (s *Server) ServerRconServerPopulation(c *gin.Context) {
	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, serverId)
	squads, teamNames, err := r.GetServerSquads()
	if err != nil {
		responses.BadRequest(c, "Failed to get teams and squads", &gin.H{"error": err.Error()})
		return
	}

	players, err := r.GetServerPlayers()
	if err != nil {
		responses.BadRequest(c, "Failed to get server players", &gin.H{"error": err.Error()})
		return
	}

	teams, err := squadRcon.ParseTeamsAndSquads(squads, teamNames, players)
	if err != nil {
		responses.BadRequest(c, "Failed to parse teams and squads", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Server population fetched successfully", &gin.H{
		"teams":   teams,
		"players": players,
	})
}

func (s *Server) ServerRconAvailableLayers(c *gin.Context) {
	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, serverId)
	layers, err := r.GetAvailableLayers()
	if err != nil {
		responses.BadRequest(c, "Failed to get available layers", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Available layers fetched successfully", &gin.H{"layers": layers})
}

// ServerRconKickPlayer handles kicking a player from the server
func (s *Server) ServerRconKickPlayer(c *gin.Context) {
	user := s.getUserFromSession(c)

	var request KickPlayerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, serverId)

	// Format the kick command
	kickCommand := "AdminKick " + request.SteamId
	if request.Reason != "" {
		kickCommand += " " + request.Reason
	}

	// Execute kick command
	response, err := r.ExecuteRaw(kickCommand)
	if err != nil {
		responses.BadRequest(c, "Failed to kick player", &gin.H{"error": err.Error()})
		return
	}

	// Create detailed audit log
	auditData := map[string]interface{}{
		"steamId": request.SteamId,
		"reason":  request.Reason,
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:rcon:command:kick", auditData)

	responses.Success(c, "Player kicked successfully", &gin.H{"response": response})
}

// ServerRconWarnPlayer handles sending a warning message to a player
func (s *Server) ServerRconWarnPlayer(c *gin.Context) {
	user := s.getUserFromSession(c)

	var request WarnPlayerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, serverId)
	response, err := r.ExecuteRaw("AdminWarn " + request.SteamId + " " + request.Message)
	if err != nil {
		responses.BadRequest(c, "Failed to warn player", &gin.H{"error": err.Error()})
		return
	}

	// Create detailed audit log
	auditData := map[string]interface{}{
		"steamId": request.SteamId,
		"message": request.Message,
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:rcon:command:warn", auditData)

	responses.Success(c, "Player warned successfully", &gin.H{"response": response})
}

// ServerRconMovePlayer handles moving a player to another team
func (s *Server) ServerRconMovePlayer(c *gin.Context) {
	user := s.getUserFromSession(c)

	var request MovePlayerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, serverId)
	response, err := r.ExecuteRaw("AdminForceTeamChange " + request.SteamId)
	if err != nil {
		responses.BadRequest(c, "Failed to move player", &gin.H{"error": err.Error()})
		return
	}

	// Create detailed audit log
	auditData := map[string]interface{}{
		"steamId": request.SteamId,
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:rcon:command:move", auditData)

	responses.Success(c, "Player moved successfully", &gin.H{"response": response})
}

// ServerRconServerInfo gets the server info from the server
func (s *Server) ServerRconServerInfo(c *gin.Context) {
	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, serverId)
	serverInfo, err := r.GetServerInfo()
	if err != nil {
		responses.BadRequest(c, "Failed to get server info", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Server info fetched successfully", &gin.H{"serverInfo": serverInfo})
}

// ServerRconForceRestart forces a restart of the RCON connection for a server
func (s *Server) ServerRconForceRestart(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// First disconnect from the server
	err = s.Dependencies.RconManager.DisconnectFromServer(serverId, true)
	if err != nil && err.Error() != "server not connected" && err.Error() != "server already disconnected" {
		responses.BadRequest(c, "Failed to disconnect from RCON", &gin.H{"error": err.Error()})
		return
	}

	ipAddress := server.IpAddress
	if server.RconIpAddress != nil {
		ipAddress = *server.RconIpAddress
	}

	// Then reconnect to the server
	err = s.Dependencies.RconManager.ConnectToServer(serverId, ipAddress, server.RconPort, server.RconPassword)
	if err != nil {
		responses.BadRequest(c, "Failed to reconnect to RCON", &gin.H{"error": err.Error()})
		return
	}

	log.Info().Str("server_id", serverId.String()).Msg("RCON connection restarted")

	// Create audit log for the action
	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:rcon:force_restart", map[string]interface{}{})

	responses.Success(c, "RCON connection restarted successfully", nil)
}
