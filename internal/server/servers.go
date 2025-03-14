package server

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/core"
	"go.codycody31.dev/squad-aegis/internal/models"
	rcon "go.codycody31.dev/squad-aegis/internal/rcon"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
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
	serverStatus := map[string]interface{}{}
	var metrics map[string]interface{} = nil

	// Try to connect to RCON to check if server is online
	rconConfig := rcon.RconConfig{
		Host:               server.IpAddress,
		Port:               fmt.Sprintf("%d", server.RconPort),
		Password:           server.RconPassword,
		AutoReconnect:      false,
		AutoReconnectDelay: 0,
	}

	rconClient, err := squadRcon.NewSquadRcon(rconConfig)
	if err == nil {
		serverStatus["rcon"] = true

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
				"max": 100, // TODO: Grab somehow, maybe from server config or via Query/Game port
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

	// Check if rcon is enabled
	rconConfig := rcon.RconConfig{
		Host:               server.IpAddress,
		Port:               fmt.Sprintf("%d", server.RconPort),
		Password:           server.RconPassword,
		AutoReconnect:      false,
		AutoReconnectDelay: 0,
	}

	rconClient, err := squadRcon.NewSquadRcon(rconConfig)
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
