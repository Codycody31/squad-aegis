package server

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/core"
	"go.codycody31.dev/squad-aegis/internal/commands"
	rcon "go.codycody31.dev/squad-aegis/internal/rcon"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
)

// Request structs for player actions
type KickPlayerRequest struct {
	SteamId string `json:"steamId" binding:"required"`
	Reason  string `json:"reason"`
}

type WarnPlayerRequest struct {
	SteamId string `json:"steamId" binding:"required"`
	Message string `json:"message" binding:"required"`
}

type MovePlayerRequest struct {
	SteamId string `json:"steamId" binding:"required"`
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

	// TODO: RCON Connection should be handled in a separate goroutine and not in the main thread
	// Then we just use a channel to send the response back to the client

	r, err := rcon.NewRcon(rcon.RconConfig{Host: server.IpAddress, Password: server.RconPassword, Port: strconv.Itoa(server.RconPort), AutoReconnect: true, AutoReconnectDelay: 5})
	if err != nil {
		responses.BadRequest(c, "Failed to connect to RCON", &gin.H{"error": err.Error()})
		return
	}
	defer r.Close()

	response, err := r.Execute(request.Command)
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

	r, err := squadRcon.NewSquadRcon(rcon.RconConfig{Host: server.IpAddress, Password: server.RconPassword, Port: strconv.Itoa(server.RconPort), AutoReconnect: true, AutoReconnectDelay: 5})
	if err != nil {
		responses.BadRequest(c, "Failed to connect to RCON", &gin.H{"error": err.Error()})
		return
	}
	defer r.Close()

	squads, teamNames, err := r.GetServerSquads()
	if err != nil {
		responses.BadRequest(c, "Failed to get server squads", &gin.H{"error": err.Error()})
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

	r, err := squadRcon.NewSquadRcon(rcon.RconConfig{Host: server.IpAddress, Password: server.RconPassword, Port: strconv.Itoa(server.RconPort), AutoReconnect: true, AutoReconnectDelay: 5})
	if err != nil {
		responses.BadRequest(c, "Failed to connect to RCON", &gin.H{"error": err.Error()})
		return
	}
	defer r.Close()

	availableLayers, err := r.GetAvailableLayers()
	if err != nil {
		responses.BadRequest(c, "Failed to get available layers", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Available layers fetched successfully", &gin.H{"layers": availableLayers})
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

	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	r, err := rcon.NewRcon(rcon.RconConfig{Host: server.IpAddress, Password: server.RconPassword, Port: strconv.Itoa(server.RconPort), AutoReconnect: true, AutoReconnectDelay: 5})
	if err != nil {
		responses.BadRequest(c, "Failed to connect to RCON", &gin.H{"error": err.Error()})
		return
	}
	defer r.Close()

	// Format the kick command
	kickCommand := "AdminKick " + request.SteamId
	if request.Reason != "" {
		kickCommand += " " + request.Reason
	}

	response, err := r.Execute(kickCommand)
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

	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	r, err := rcon.NewRcon(rcon.RconConfig{Host: server.IpAddress, Password: server.RconPassword, Port: strconv.Itoa(server.RconPort), AutoReconnect: true, AutoReconnectDelay: 5})
	if err != nil {
		responses.BadRequest(c, "Failed to connect to RCON", &gin.H{"error": err.Error()})
		return
	}
	defer r.Close()

	// Format the warning command
	warnCommand := "AdminWarn " + request.SteamId + " " + request.Message

	response, err := r.Execute(warnCommand)
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

// ServerRconMovePlayer handles moving a player to a different team
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

	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	r, err := rcon.NewRcon(rcon.RconConfig{Host: server.IpAddress, Password: server.RconPassword, Port: strconv.Itoa(server.RconPort), AutoReconnect: true, AutoReconnectDelay: 5})
	if err != nil {
		responses.BadRequest(c, "Failed to connect to RCON", &gin.H{"error": err.Error()})
		return
	}
	defer r.Close()

	// Format the move command
	moveCommand := "AdminForceTeamChange " + request.SteamId

	response, err := r.Execute(moveCommand)
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

func (s *Server) ServerRconServerInfo(c *gin.Context) {
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

	r, err := squadRcon.NewSquadRcon(rcon.RconConfig{Host: server.IpAddress, Password: server.RconPassword, Port: strconv.Itoa(server.RconPort), AutoReconnect: true, AutoReconnectDelay: 5})
	if err != nil {
		responses.BadRequest(c, "Failed to connect to RCON", &gin.H{"error": err.Error()})
		return
	}
	defer r.Close()

	serverInfo, err := r.GetServerInfo()
	if err != nil {
		responses.BadRequest(c, "Failed to get server info", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Server info fetched successfully", &gin.H{"serverInfo": serverInfo})
}
