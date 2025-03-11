package server

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/core"
	"go.codycody31.dev/squad-aegis/internal/commands"
	"go.codycody31.dev/squad-aegis/internal/models"
	rcon "go.codycody31.dev/squad-aegis/internal/rcon"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

type ServerCreateRequest struct {
	Name         string `json:"name"`
	IpAddress    string `json:"ip_address"`
	GamePort     int    `json:"game_port"`
	RconPort     int    `json:"rcon_port"`
	RconPassword string `json:"rcon_password"`
}

type ServerRconExecuteRequest struct {
	Command string `json:"command"`
}

// BannedPlayer represents a banned player
type BannedPlayer struct {
	ID        string     `json:"id"`
	SteamID   string     `json:"steamId"`
	Name      string     `json:"name"`
	Reason    string     `json:"reason"`
	BannedBy  string     `json:"bannedBy"`
	BannedAt  time.Time  `json:"bannedAt"`
	ExpiresAt *time.Time `json:"expiresAt"`
	Duration  string     `json:"duration"`
	Permanent bool       `json:"permanent"`
}

// BanPlayerRequest represents a request to ban a player
type BanPlayerRequest struct {
	SteamID  string `json:"steamId"`
	Name     string `json:"name"`
	Reason   string `json:"reason"`
	Duration string `json:"duration"` // "permanent" or a duration like "24h"
}

// UnbanPlayerRequest represents a request to unban a player
type UnbanPlayerRequest struct {
	SteamID string `json:"steamId"`
}

// ServerBan represents a ban in the database
type ServerBan struct {
	ID        string    `json:"id"`
	ServerID  string    `json:"serverId"`
	AdminID   string    `json:"adminId"`
	AdminName string    `json:"adminName"`
	SteamID   string    `json:"steamId"`
	Name      string    `json:"name"` // Not stored in DB, populated from cache or external source
	Reason    string    `json:"reason"`
	Duration  int       `json:"duration"` // In minutes, 0 means permanent
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	Permanent bool      `json:"permanent"`
}

// ServerBanCreateRequest represents a request to create a ban
type ServerBanCreateRequest struct {
	SteamID  string `json:"steamId"`
	Reason   string `json:"reason"`
	Duration int    `json:"duration"` // In days, 0 means permanent
}

// ServerRole represents a role in the server
type ServerRole struct {
	ID          string    `json:"id"`
	ServerID    string    `json:"serverId"`
	Name        string    `json:"name"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ServerAdmin represents an admin in the server
type ServerAdmin struct {
	ID           string    `json:"id"`
	ServerID     string    `json:"serverId"`
	UserID       string    `json:"userId"`
	Username     string    `json:"username"`
	ServerRoleID string    `json:"serverRoleId"`
	RoleName     string    `json:"roleName"`
	CreatedAt    time.Time `json:"createdAt"`
}

// ServerRoleCreateRequest represents a request to create a role
type ServerRoleCreateRequest struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

// ServerAdminCreateRequest represents a request to create an admin
type ServerAdminCreateRequest struct {
	UserID       string `json:"userId"`
	ServerRoleID string `json:"serverRoleId"`
}

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
	var request ServerCreateRequest

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
		Id:           uuid.New(),
		Name:         request.Name,
		IpAddress:    request.IpAddress,
		GamePort:     request.GamePort,
		RconPort:     request.RconPort,
		RconPassword: request.RconPassword,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	server, err := core.CreateServer(c.Request.Context(), s.Dependencies.DB, &serverToCreate)
	if err != nil {
		responses.BadRequest(c, "Failed to create server", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Server created successfully", &gin.H{"server": server})
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

	var request ServerRconExecuteRequest

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

	r, err := rcon.NewRcon(rcon.RconConfig{Host: server.IpAddress, Password: server.RconPassword, Port: strconv.Itoa(server.RconPort), AutoReconnect: true, AutoReconnectDelay: 5})
	if err != nil {
		responses.BadRequest(c, "Failed to connect to RCON", &gin.H{"error": err.Error()})
		return
	}
	defer r.Close()

	// Get squads information
	squadsResponse, err := r.Execute("ListSquads")
	if err != nil {
		responses.BadRequest(c, "Failed to execute ListSquads command", &gin.H{"error": err.Error()})
		return
	}

	players, err := getServerPlayers(r)
	if err != nil {
		responses.BadRequest(c, "Failed to get server players", &gin.H{"error": err.Error()})
		return
	}

	// Parse teams and squads
	teams, err := parseTeamsAndSquads(squadsResponse, players)
	if err != nil {
		responses.BadRequest(c, "Failed to parse teams and squads", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Server population fetched successfully", &gin.H{
		"teams":   teams,
		"players": players,
	})
}

// parseTeamsAndSquads parses the squads response and builds the teams/squads structure
func parseTeamsAndSquads(squadsResponse string, players PlayersData) ([]Team, error) {
	teams := []Team{}
	tempSquads := []Squad{}
	var currentTeamID int

	// First pass: Extract teams and squads
	for _, line := range strings.Split(squadsResponse, "\n") {
		// Match team information
		matchesTeam := regexp.MustCompile(`^Team ID: ([1|2]) \((.*)\)`)
		matchTeam := matchesTeam.FindStringSubmatch(line)

		if len(matchTeam) > 0 {
			teamId, _ := strconv.Atoi(matchTeam[1])
			currentTeamID = teamId

			team := Team{
				ID:      teamId,
				Name:    matchTeam[2],
				Squads:  []Squad{},
				Players: []Player{},
			}

			teams = append(teams, team)
			continue
		}

		// Match squad information
		matchesSquad := regexp.MustCompile(`^ID: (\d{1,}) \| Name: (.*?) \| Size: (\d) \| Locked: (True|False)`)
		matchSquad := matchesSquad.FindStringSubmatch(line)

		if len(matchSquad) > 0 {
			squadId, _ := strconv.Atoi(matchSquad[1])
			size, _ := strconv.Atoi(matchSquad[3])

			squad := Squad{
				ID:      squadId,
				TeamId:  currentTeamID,
				Name:    matchSquad[2],
				Size:    size,
				Locked:  matchSquad[4] == "True",
				Players: []Player{},
			}

			tempSquads = append(tempSquads, squad)
		}
	}

	// Second pass: Assign players to squads and identify squad leaders
	squadMap := make(map[int]*Squad)
	for i := range tempSquads {
		squadMap[tempSquads[i].ID] = &tempSquads[i]
	}

	for _, player := range players.OnlinePlayers {
		// Skip players with invalid squad IDs
		if player.SquadId <= 0 {
			continue
		}

		// Find the squad for this player
		if squad, ok := squadMap[player.SquadId]; ok {
			// Add player to squad
			squad.Players = append(squad.Players, player)

			// Set squad leader if applicable
			if player.IsSquadLeader {
				squad.Leader = &player
			}
		}
	}

	// Third pass: Assign squads to teams and unassigned players to teams
	teamMap := make(map[int]*Team)
	for i := range teams {
		teamMap[teams[i].ID] = &teams[i]
	}

	// Assign squads to teams
	for _, squad := range tempSquads {
		if team, ok := teamMap[squad.TeamId]; ok {
			team.Squads = append(team.Squads, squad)
		}
	}

	// Assign unassigned players to teams
	for _, player := range players.OnlinePlayers {
		// Only process players that are not in a squad (SquadId is 0 or N/A)
		if player.SquadId <= 0 {
			if team, ok := teamMap[player.TeamId]; ok {
				team.Players = append(team.Players, player)
			}
		}
	}

	return teams, nil
}

func getServerPlayers(r *rcon.Rcon) (PlayersData, error) {
	playersResponse, err := r.Execute("ListPlayers")
	if err != nil {
		return PlayersData{}, err
	}

	onlinePlayers := []Player{}
	disconnectedPlayers := []Player{}

	lines := strings.Split(playersResponse, "\n")
	for _, line := range lines {
		matchesOnline := regexp.MustCompile(`ID: ([0-9]+) \| Online IDs: EOS: (\w{32}) steam: (\d{17}) \| Name: (.+) \| Team ID: ([0-9]+) \| Squad ID: ([0-9]+|N\/A) \| Is Leader: (True|False) \| Role: (.*)`)
		matchesDisconnected := regexp.MustCompile(`^ID: (\d{1,}) \| Online IDs: EOS: (\w{32}) steam: (\d{17}) \| Since Disconnect: (\d{2,})m.(\d{2})s \| Name: (.*?)$`)

		matchOnline := matchesOnline.FindStringSubmatch(line)
		matchDisconnected := matchesDisconnected.FindStringSubmatch(line)

		if len(matchOnline) > 0 {
			playerId, _ := strconv.Atoi(matchOnline[1])
			teamId, _ := strconv.Atoi(matchOnline[5])
			squadId, _ := strconv.Atoi(matchOnline[6])

			player := Player{
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

			player := Player{
				Id:              playerId,
				EosId:           matchDisconnected[2],
				SteamId:         matchDisconnected[3],
				SinceDisconnect: matchDisconnected[4] + "m" + matchDisconnected[5] + "s",
				Name:            matchDisconnected[6],
			}

			disconnectedPlayers = append(disconnectedPlayers, player)
		}
	}

	return PlayersData{
		OnlinePlayers:       onlinePlayers,
		DisconnectedPlayers: disconnectedPlayers,
	}, nil
}

// Player represents a player in the game
type Player struct {
	Id              int    `json:"playerId"`
	EosId           string `json:"eosId"`
	SteamId         string `json:"steamId"`
	Name            string `json:"name"`
	TeamId          int    `json:"teamId"`
	SquadId         int    `json:"squadId"`
	SinceDisconnect string `json:"sinceDisconnect"`
	IsSquadLeader   bool   `json:"isSquadLeader"`
	Role            string `json:"role"`
}

// Squad represents a squad in the game
type Squad struct {
	ID      int      `json:"id"`
	TeamId  int      `json:"teamId"`
	Name    string   `json:"name"`
	Size    int      `json:"size"`
	Locked  bool     `json:"locked"`
	Leader  *Player  `json:"leader"`
	Players []Player `json:"players"`
}

// Team represents a team in the game
type Team struct {
	ID      int      `json:"id"`
	Name    string   `json:"name"`
	Squads  []Squad  `json:"squads"`
	Players []Player `json:"players"` // Unassigned players
}

// PlayersData contains online and disconnected players
type PlayersData struct {
	OnlinePlayers       []Player `json:"onlinePlayers"`
	DisconnectedPlayers []Player `json:"disconnectedPlayers"`
}

// ServerBansList handles listing all bans for a server
func (s *Server) ServerBansList(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Query the database for bans
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
		SELECT sb.id, sb.server_id, sb.admin_id, u.username, sb.steam_id, sb.reason, sb.duration, sb.created_at, sb.updated_at
		FROM server_bans sb
		JOIN users u ON sb.admin_id = u.id
		WHERE sb.server_id = $1
		ORDER BY sb.created_at DESC
	`, serverId)
	if err != nil {
		responses.BadRequest(c, "Failed to query bans", &gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	bans := []ServerBan{}
	for rows.Next() {
		var ban ServerBan
		var steamIDInt int64
		err := rows.Scan(
			&ban.ID,
			&ban.ServerID,
			&ban.AdminID,
			&ban.AdminName,
			&steamIDInt,
			&ban.Reason,
			&ban.Duration,
			&ban.CreatedAt,
			&ban.UpdatedAt,
		)
		if err != nil {
			responses.BadRequest(c, "Failed to scan ban", &gin.H{"error": err.Error()})
			return
		}

		// Convert steamID from int64 to string
		ban.SteamID = strconv.FormatInt(steamIDInt, 10)

		// Calculate if ban is permanent and expiry date
		ban.Permanent = ban.Duration == 0
		if !ban.Permanent {
			ban.ExpiresAt = ban.CreatedAt.Add(time.Duration(ban.Duration) * time.Minute)
		}

		// TODO: Fetch player name from cache or external source if needed
		// For now, we'll leave it empty or use a placeholder
		ban.Name = "Unknown Player"

		bans = append(bans, ban)
	}

	responses.Success(c, "Bans fetched successfully", &gin.H{
		"bans": bans,
	})
}

// ServerBansAdd handles adding a new ban
func (s *Server) ServerBansAdd(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	var request ServerBanCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Validate request
	if request.SteamID == "" {
		responses.BadRequest(c, "Steam ID is required", &gin.H{"error": "Steam ID is required"})
		return
	}

	if request.Reason == "" {
		responses.BadRequest(c, "Ban reason is required", &gin.H{"error": "Ban reason is required"})
		return
	}

	if request.Duration < 0 {
		responses.BadRequest(c, "Duration must be a positive integer", &gin.H{"error": "Duration must be a positive integer"})
		return
	}

	// Convert SteamID to int64
	steamID, err := strconv.ParseInt(request.SteamID, 10, 64)
	if err != nil {
		responses.BadRequest(c, "Invalid Steam ID format", &gin.H{"error": "Steam ID must be a valid 64-bit integer"})
		return
	}

	// Insert the ban into the database
	var banID string

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		INSERT INTO server_bans (server_id, admin_id, steam_id, reason, duration)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, serverId, user.Id, steamID, request.Reason, request.Duration).Scan(&banID)
	if err != nil {
		responses.BadRequest(c, "Failed to create ban", &gin.H{"error": err.Error()})
		return
	}

	// Also apply the ban via RCON if the server is online
	if server != nil {
		r, err := rcon.NewRcon(rcon.RconConfig{Host: server.IpAddress, Password: server.RconPassword, Port: strconv.Itoa(server.RconPort), AutoReconnect: true, AutoReconnectDelay: 5})
		if err == nil {
			defer r.Close()

			// Construct the ban command based on duration
			banCommand := fmt.Sprintf("AdminBan %s %dd %s", request.SteamID, request.Duration, request.Reason)

			// Execute the ban command
			_, _ = r.Execute(banCommand) // Ignore errors, as the ban is already in the database
		}
	}

	responses.Success(c, "Ban created successfully", &gin.H{
		"banId": banID,
	})
}

// ServerBansRemove handles removing a ban
func (s *Server) ServerBansRemove(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	banIdString := c.Param("banId")
	banId, err := uuid.Parse(banIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid ban ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Get the ban details first (to get the Steam ID for RCON unban)
	var steamIDInt int64
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT steam_id FROM server_bans
		WHERE id = $1 AND server_id = $2
	`, banId, serverId).Scan(&steamIDInt)
	if err != nil {
		if err == sql.ErrNoRows {
			responses.BadRequest(c, "Ban not found", &gin.H{"error": "Ban not found"})
		} else {
			responses.BadRequest(c, "Failed to get ban details", &gin.H{"error": err.Error()})
		}
		return
	}

	// Delete the ban from the database
	result, err := s.Dependencies.DB.ExecContext(c.Request.Context(), `
		DELETE FROM server_bans
		WHERE id = $1 AND server_id = $2
	`, banId, serverId)
	if err != nil {
		responses.BadRequest(c, "Failed to delete ban", &gin.H{"error": err.Error()})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		responses.BadRequest(c, "Failed to get rows affected", &gin.H{"error": err.Error()})
		return
	}

	if rowsAffected == 0 {
		responses.BadRequest(c, "Ban not found", &gin.H{"error": "Ban not found"})
		return
	}

	// Also remove the ban via RCON if the server is online
	steamIDStr := strconv.FormatInt(steamIDInt, 10)
	if server != nil {
		r, err := rcon.NewRcon(rcon.RconConfig{Host: server.IpAddress, Password: server.RconPassword, Port: strconv.Itoa(server.RconPort), AutoReconnect: true, AutoReconnectDelay: 5})
		if err == nil {
			defer r.Close()

			// Execute the unban command
			unbanCommand := fmt.Sprintf("AdminUnban %s", steamIDStr)
			_, _ = r.Execute(unbanCommand) // Ignore errors, as the ban is already removed from the database
		}
	}

	responses.Success(c, "Ban removed successfully", nil)
}

// ServerBansCfg handles generating the ban config file for the server
func (s *Server) ServerBansCfg(c *gin.Context) {
	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	// Query the database for active bans
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
		SELECT steam_id, reason, duration, created_at
		FROM server_bans
		WHERE server_id = $1
	`, serverId)
	if err != nil {
		responses.BadRequest(c, "Failed to query bans", &gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	// Generate the ban config file
	var banCfg strings.Builder

	now := time.Now()
	for rows.Next() {
		var steamIDInt int64
		var reason string
		var duration int
		var createdAt time.Time
		err := rows.Scan(&steamIDInt, &reason, &duration, &createdAt)
		if err != nil {
			responses.BadRequest(c, "Failed to scan ban", &gin.H{"error": err.Error()})
			return
		}

		unixTimeOfExpiry := createdAt.Add(time.Duration(duration) * (time.Hour * 24))

		// Skip expired bans
		if duration > 0 {
			if now.After(unixTimeOfExpiry) {
				continue
			}
		}

		// Format the ban entry
		steamIDStr := strconv.FormatInt(steamIDInt, 10)
		if duration == 0 {
			banCfg.WriteString(fmt.Sprintf("%s:0\n", steamIDStr))
		} else {
			banCfg.WriteString(fmt.Sprintf("%s:%d\n", steamIDStr, unixTimeOfExpiry.Unix()))
		}
	}

	// Set the content type and send the response
	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, banCfg.String())
}

// ServerRolesList handles listing all roles for a server
func (s *Server) ServerRolesList(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}
	_ = server // Ensure server is used

	// Query the database for roles
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
		SELECT id, server_id, name, permissions, created_at
		FROM server_roles
		WHERE server_id = $1
		ORDER BY name ASC
	`, serverId)
	if err != nil {
		responses.BadRequest(c, "Failed to query roles", &gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	roles := []ServerRole{}

	for rows.Next() {
		var role ServerRole
		var permissionsStr string

		err := rows.Scan(&role.ID, &role.ServerID, &role.Name, &permissionsStr, &role.CreatedAt)
		if err != nil {
			responses.BadRequest(c, "Failed to scan role", &gin.H{"error": err.Error()})
			return
		}

		// Parse permissions from comma-separated string
		role.Permissions = strings.Split(permissionsStr, ",")
		roles = append(roles, role)
	}

	responses.Success(c, "Roles fetched successfully", &gin.H{
		"roles": roles,
	})
}

// ServerRolesAdd handles adding a new role
func (s *Server) ServerRolesAdd(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}
	_ = server // Ensure server is used

	var request ServerRoleCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Validate request
	if request.Name == "" {
		responses.BadRequest(c, "Role name is required", &gin.H{"error": "Role name is required"})
		return
	}

	// Ensure name has no spaces, only allows alphanumeric and underscores
	matched, err := regexp.MatchString("^[a-zA-Z0-9_]+$", request.Name)
	if err != nil {
		responses.BadRequest(c, "Failed to validate role name", &gin.H{"error": err.Error()})
		return
	}

	if !matched {
		responses.BadRequest(c, "Role name can only contain alphanumeric characters and underscores", &gin.H{"error": "Role name can only contain alphanumeric characters and underscores"})
		return
	}

	if len(request.Permissions) == 0 {
		responses.BadRequest(c, "At least one permission is required", &gin.H{"error": "At least one permission is required"})
		return
	}

	// Convert permissions array to comma-separated string
	permissionsStr := strings.Join(request.Permissions, ",")

	// Insert the role into the database
	var roleID string
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		INSERT INTO server_roles (server_id, name, permissions)
		VALUES ($1, $2, $3)
		RETURNING id
	`, serverId, request.Name, permissionsStr).Scan(&roleID)

	if err != nil {
		responses.BadRequest(c, "Failed to create role", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Role created successfully", &gin.H{
		"roleId": roleID,
	})
}

// ServerRolesRemove handles removing a role
func (s *Server) ServerRolesRemove(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	roleIdString := c.Param("roleId")
	roleId, err := uuid.Parse(roleIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid role ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}
	_ = server // Ensure server is used

	// Check if role is in use by any admins
	var count int
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT COUNT(*) FROM server_admins
		WHERE server_role_id = $1
	`, roleId).Scan(&count)

	if err != nil {
		responses.BadRequest(c, "Failed to check if role is in use", &gin.H{"error": err.Error()})
		return
	}

	if count > 0 {
		responses.BadRequest(c, "Role is in use by admins and cannot be removed", &gin.H{"error": "Role is in use by admins and cannot be removed"})
		return
	}

	// Delete the role
	_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
		DELETE FROM server_roles
		WHERE id = $1 AND server_id = $2
	`, roleId, serverId)

	if err != nil {
		responses.BadRequest(c, "Failed to delete role", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Role deleted successfully", nil)
}

// ServerAdminsList handles listing all admins for a server
func (s *Server) ServerAdminsList(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}
	_ = server // Ensure server is used

	// Query the database for admins
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
		SELECT sa.id, sa.server_id, sa.user_id, u.username, sa.server_role_id, sr.name as role_name, sa.created_at
		FROM server_admins sa
		JOIN users u ON sa.user_id = u.id
		JOIN server_roles sr ON sa.server_role_id = sr.id
		WHERE sa.server_id = $1
		ORDER BY u.username ASC
	`, serverId)
	if err != nil {
		responses.BadRequest(c, "Failed to query admins", &gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	admins := []ServerAdmin{}

	for rows.Next() {
		var admin ServerAdmin
		err := rows.Scan(&admin.ID, &admin.ServerID, &admin.UserID, &admin.Username, &admin.ServerRoleID, &admin.RoleName, &admin.CreatedAt)
		if err != nil {
			responses.BadRequest(c, "Failed to scan admin", &gin.H{"error": err.Error()})
			return
		}

		admins = append(admins, admin)
	}

	responses.Success(c, "Admins fetched successfully", &gin.H{
		"admins": admins,
	})
}

// ServerAdminsAdd handles adding a new admin
func (s *Server) ServerAdminsAdd(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}
	_ = server // Ensure server is used

	var request ServerAdminCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	if request.UserID == "" {
		responses.BadRequest(c, "User ID is required", &gin.H{"error": "User ID is required"})
		return
	}

	if request.ServerRoleID == "" {
		responses.BadRequest(c, "Server role ID is required", &gin.H{"error": "Server role ID is required"})
		return
	}

	// Check if user already exists as admin for this server
	var count int
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT COUNT(*) FROM server_admins
		WHERE server_id = $1 AND user_id = $2
	`, serverId, request.UserID).Scan(&count)

	if err != nil {
		responses.BadRequest(c, "Failed to check if user is already an admin", &gin.H{"error": err.Error()})
		return
	}

	if count > 0 {
		responses.BadRequest(c, "User is already an admin for this server", &gin.H{"error": "User is already an admin for this server"})
		return
	}

	// Insert the admin into the database
	var adminID string
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		INSERT INTO server_admins (server_id, user_id, server_role_id)
		VALUES ($1, $2, $3)
		RETURNING id
	`, serverId, request.UserID, request.ServerRoleID).Scan(&adminID)

	if err != nil {
		responses.BadRequest(c, "Failed to create admin", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Admin created successfully", &gin.H{
		"adminId": adminID,
	})
}

// ServerAdminsRemove handles removing an admin
func (s *Server) ServerAdminsRemove(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	adminIdString := c.Param("adminId")
	adminId, err := uuid.Parse(adminIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid admin ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}
	_ = server // Ensure server is used

	// Delete the admin
	_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
		DELETE FROM server_admins
		WHERE id = $1 AND server_id = $2
	`, adminId, serverId)

	if err != nil {
		responses.BadRequest(c, "Failed to delete admin", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Admin deleted successfully", nil)
}

// ServerAdminsCfg handles generating the admin config file
func (s *Server) ServerAdminsCfg(c *gin.Context) {
	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	// Get roles
	roles, err := core.GetServerRoles(c.Request.Context(), s.Dependencies.DB, serverId)
	if err != nil {
		responses.BadRequest(c, "Failed to get server roles", &gin.H{"error": err.Error()})
		return
	}

	// Get admins
	admins, err := core.GetServerAdmins(c.Request.Context(), s.Dependencies.DB, serverId)
	if err != nil {
		responses.BadRequest(c, "Failed to get server admins", &gin.H{"error": err.Error()})
		return
	}

	var configBuilder strings.Builder

	for _, role := range roles {
		configBuilder.WriteString(fmt.Sprintf("Group=%s:%s\n", role.Name, strings.Join(role.Permissions, ",")))
	}

	for _, admin := range admins {
		roleName := ""

		for _, role := range roles {
			if role.Id == admin.ServerRoleId {
				roleName = role.Name
				break
			}
		}

		user, err := core.GetUserById(c.Request.Context(), s.Dependencies.DB, admin.UserId, &admin.UserId)
		if err != nil {
			responses.BadRequest(c, "Failed to get user", &gin.H{"error": err.Error()})
			return
		}

		if user.SteamId == 0 {
			continue
		}

		configBuilder.WriteString(fmt.Sprintf("Admin=%d:%s\n", user.SteamId, roleName))
	}

	// Set the content type and send the response
	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, configBuilder.String())
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

	// Get server status and metrics if possible
	serverStatus := "offline"
	var metrics map[string]interface{} = nil

	// Try to connect to RCON to check if server is online
	rconConfig := rcon.RconConfig{
		Host:               server.IpAddress,
		Port:               fmt.Sprintf("%d", server.RconPort),
		Password:           server.RconPassword,
		AutoReconnect:      false,
		AutoReconnectDelay: 0,
	}

	rconClient, err := rcon.NewRcon(rconConfig)
	if err == nil {
		// Successfully connected, server is online
		serverStatus = "online"

		// Close the connection after checking
		defer rconClient.Close()

		// Get basic server info
		metrics = map[string]interface{}{}

		// Try to get player count
		playersData, err := getServerPlayers(rconClient)
		if err == nil {
			metrics["playerCount"] = len(playersData.OnlinePlayers)
			metrics["maxPlayers"] = 100 // This would ideally come from server config
		}

		// Try to get current map
		currentMap, err := rconClient.Execute("ShowCurrentMap")
		if err == nil {
			// Current level is Narva, layer is Narva_Skirmish_v1, factions CAF WPMC
			matchesMap := regexp.MustCompile(`^Current level is (.*?), layer is (.*?), factions (.*?) (.*?)$`)
			matches := matchesMap.FindStringSubmatch(currentMap)
			metrics["currentMap"] = matches[1]
			metrics["currentLayer"] = matches[2]
			metrics["currentFactions"] = []string{matches[3], matches[4]}
		}
	}

	// Prepare response with server info and status
	serverResponse := gin.H{
		"server": server,
		"status": serverStatus,
	}

	// Add metrics if available
	if metrics != nil {
		serverResponse["metrics"] = metrics
	}

	responses.Success(c, "Server fetched successfully", &serverResponse)
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

	// Delete related records first (server_admins, server_roles, server_bans)
	_, err = tx.ExecContext(c.Request.Context(), `DELETE FROM server_admins WHERE server_id = $1`, serverId)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to delete server admins"})
		return
	}

	_, err = tx.ExecContext(c.Request.Context(), `DELETE FROM server_roles WHERE server_id = $1`, serverId)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to delete server roles"})
		return
	}

	_, err = tx.ExecContext(c.Request.Context(), `DELETE FROM server_bans WHERE server_id = $1`, serverId)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to delete server bans"})
		return
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

	responses.Success(c, "Server deleted successfully", nil)
}
