package server

import (
	"regexp"
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

	// Ensure server is connected to RCON manager
	err = s.Dependencies.RconManager.ConnectToServer(serverId, server.IpAddress, server.RconPort, server.RconPassword)
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

	// Ensure server is connected to RCON manager
	err = s.Dependencies.RconManager.ConnectToServer(serverId, server.IpAddress, server.RconPort, server.RconPassword)
	if err != nil {
		responses.BadRequest(c, "Failed to connect to RCON", &gin.H{"error": err.Error()})
		return
	}

	// Execute ListSquads command
	squadsResponse, err := s.Dependencies.RconManager.ExecuteCommand(serverId, "ListSquads")
	if err != nil {
		responses.BadRequest(c, "Failed to get server squads", &gin.H{"error": err.Error()})
		return
	}

	// Execute ListPlayers command
	playersResponse, err := s.Dependencies.RconManager.ExecuteCommand(serverId, "ListPlayers")
	if err != nil {
		responses.BadRequest(c, "Failed to get server players", &gin.H{"error": err.Error()})
		return
	}

	// Parse responses
	r, err := squadRcon.NewSquadRcon(rcon.RconConfig{Host: server.IpAddress, Password: server.RconPassword, Port: strconv.Itoa(server.RconPort), AutoReconnect: true, AutoReconnectDelay: 5})
	if err != nil {
		responses.BadRequest(c, "Failed to create RCON parser", &gin.H{"error": err.Error()})
		return
	}
	defer r.Close()

	// Parse squads response
	squads, teamNames, err := parseSquadsResponse(squadsResponse)
	if err != nil {
		responses.BadRequest(c, "Failed to parse squads response", &gin.H{"error": err.Error()})
		return
	}

	// Parse players response
	players, err := parsePlayersResponse(playersResponse)
	if err != nil {
		responses.BadRequest(c, "Failed to parse players response", &gin.H{"error": err.Error()})
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

// Helper functions to parse RCON responses
func parseSquadsResponse(response string) ([]squadRcon.Squad, []string, error) {
	lines := strings.Split(response, "\n")
	squads := []squadRcon.Squad{}
	teamNames := []string{}
	var currentTeamID int

	// First pass: Extract teams and squads
	for _, line := range lines {
		// Match team information
		matchesTeam := regexp.MustCompile(`^Team ID: ([1|2]) \((.*)\)`)
		matchTeam := matchesTeam.FindStringSubmatch(line)

		if len(matchTeam) > 0 {
			teamId, _ := strconv.Atoi(matchTeam[1])
			currentTeamID = teamId
			teamNames = append(teamNames, matchTeam[2])
			continue
		}

		// Match squad information
		matchesSquad := regexp.MustCompile(`^ID: (\d{1,}) \| Name: (.*?) \| Size: (\d) \| Locked: (True|False)`)
		matchSquad := matchesSquad.FindStringSubmatch(line)

		if len(matchSquad) > 0 {
			squadId, _ := strconv.Atoi(matchSquad[1])
			size, _ := strconv.Atoi(matchSquad[3])

			squad := squadRcon.Squad{
				ID:      squadId,
				TeamId:  currentTeamID,
				Name:    matchSquad[2],
				Size:    size,
				Locked:  matchSquad[4] == "True",
				Players: []squadRcon.Player{},
			}

			squads = append(squads, squad)
		}
	}

	return squads, teamNames, nil
}

func parsePlayersResponse(response string) (squadRcon.PlayersData, error) {
	onlinePlayers := []squadRcon.Player{}
	disconnectedPlayers := []squadRcon.Player{}

	lines := strings.Split(response, "\n")
	for _, line := range lines {
		matchesOnline := regexp.MustCompile(`ID: ([0-9]+) \| Online IDs: EOS: (\w{32}) steam: (\d{17}) \| Name: (.+) \| Team ID: ([0-9]+) \| Squad ID: ([0-9]+|N\/A) \| Is Leader: (True|False) \| Role: (.*)`)
		matchesDisconnected := regexp.MustCompile(`^ID: (\d{1,}) \| Online IDs: EOS: (\w{32}) steam: (\d{17}) \| Since Disconnect: (\d{2,})m.(\d{2})s \| Name: (.*?)$`)

		matchOnline := matchesOnline.FindStringSubmatch(line)
		matchDisconnected := matchesDisconnected.FindStringSubmatch(line)

		if len(matchOnline) > 0 {
			playerId, _ := strconv.Atoi(matchOnline[1])
			teamId, _ := strconv.Atoi(matchOnline[5])
			squadId := 0
			if matchOnline[6] != "N/A" {
				squadId, _ = strconv.Atoi(matchOnline[6])
			}

			player := squadRcon.Player{
				Id:            playerId,
				EosId:         matchOnline[2],
				SteamId:       matchOnline[3],
				Name:          matchOnline[4],
				TeamId:        teamId,
				SquadId:       squadId,
				IsSquadLeader: matchOnline[7] == "True",
				Role:          matchOnline[8],
			}

			onlinePlayers = append(onlinePlayers, player)
		} else if len(matchDisconnected) > 0 {
			playerId, _ := strconv.Atoi(matchDisconnected[1])

			player := squadRcon.Player{
				Id:              playerId,
				EosId:           matchDisconnected[2],
				SteamId:         matchDisconnected[3],
				SinceDisconnect: matchDisconnected[4] + "m" + matchDisconnected[5] + "s",
				Name:            matchDisconnected[6],
			}

			disconnectedPlayers = append(disconnectedPlayers, player)
		}
	}

	return squadRcon.PlayersData{
		OnlinePlayers:       onlinePlayers,
		DisconnectedPlayers: disconnectedPlayers,
	}, nil
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

	// Ensure server is connected to RCON manager
	err = s.Dependencies.RconManager.ConnectToServer(serverId, server.IpAddress, server.RconPort, server.RconPassword)
	if err != nil {
		responses.BadRequest(c, "Failed to connect to RCON", &gin.H{"error": err.Error()})
		return
	}

	// Execute ListLayers command
	layersResponse, err := s.Dependencies.RconManager.ExecuteCommand(serverId, "ListLayers")
	if err != nil {
		responses.BadRequest(c, "Failed to get available layers", &gin.H{"error": err.Error()})
		return
	}

	// Parse layers response
	availableLayers := []squadRcon.Layer{}
	layersResponse = strings.Replace(layersResponse, "List of available layers :\n", "", 1)

	for _, line := range strings.Split(layersResponse, "\n") {
		if line == "" {
			continue
		}

		if strings.Contains(line, "(") {
			mod := strings.Split(line, "(")[1]
			mod = strings.Trim(mod, ")")
			availableLayers = append(availableLayers, squadRcon.Layer{
				Name:      strings.Split(line, "(")[0],
				Mod:       mod,
				IsVanilla: false,
			})
		} else {
			availableLayers = append(availableLayers, squadRcon.Layer{
				Name:      line,
				Mod:       "",
				IsVanilla: true,
			})
		}
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

	// Ensure server is connected to RCON manager
	err = s.Dependencies.RconManager.ConnectToServer(serverId, server.IpAddress, server.RconPort, server.RconPassword)
	if err != nil {
		responses.BadRequest(c, "Failed to connect to RCON", &gin.H{"error": err.Error()})
		return
	}

	// Format the kick command
	kickCommand := "AdminKick " + request.SteamId
	if request.Reason != "" {
		kickCommand += " " + request.Reason
	}

	// Execute kick command
	response, err := s.Dependencies.RconManager.ExecuteCommand(serverId, kickCommand)
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

	// Ensure server is connected to RCON manager
	err = s.Dependencies.RconManager.ConnectToServer(serverId, server.IpAddress, server.RconPort, server.RconPassword)
	if err != nil {
		responses.BadRequest(c, "Failed to connect to RCON", &gin.H{"error": err.Error()})
		return
	}

	// Format the warn command
	warnCommand := "AdminWarn " + request.SteamId + " " + request.Message

	// Execute warn command
	response, err := s.Dependencies.RconManager.ExecuteCommand(serverId, warnCommand)
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

	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Ensure server is connected to RCON manager
	err = s.Dependencies.RconManager.ConnectToServer(serverId, server.IpAddress, server.RconPort, server.RconPassword)
	if err != nil {
		responses.BadRequest(c, "Failed to connect to RCON", &gin.H{"error": err.Error()})
		return
	}

	// Format the move command
	moveCommand := "AdminForceTeamChange " + request.SteamId

	// Execute move command
	response, err := s.Dependencies.RconManager.ExecuteCommand(serverId, moveCommand)
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

	// Ensure server is connected to RCON manager
	err = s.Dependencies.RconManager.ConnectToServer(serverId, server.IpAddress, server.RconPort, server.RconPassword)
	if err != nil {
		responses.BadRequest(c, "Failed to connect to RCON", &gin.H{"error": err.Error()})
		return
	}

	// Execute ShowServerInfo command
	serverInfoResponse, err := s.Dependencies.RconManager.ExecuteCommand(serverId, "ShowServerInfo")
	if err != nil {
		responses.BadRequest(c, "Failed to get server info", &gin.H{"error": err.Error()})
		return
	}

	// Parse server info response
	serverInfo, err := squadRcon.MarshalServerInfo(serverInfoResponse)
	if err != nil {
		responses.BadRequest(c, "Failed to parse server info", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Server info fetched successfully", &gin.H{"serverInfo": serverInfo})
}
