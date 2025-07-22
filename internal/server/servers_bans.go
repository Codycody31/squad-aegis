package server

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	rcon "github.com/SquadGO/squad-rcon-go/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
)

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
		SELECT sb.id, sb.server_id, sb.admin_id, u.username, sb.player_id, p.steam_id, p.display_name, sb.reason, sb.duration, sb.rule_id, sb.created_at, sb.updated_at
		FROM server_bans sb
		JOIN users u ON sb.admin_id = u.id
		JOIN players p ON sb.player_id = p.id
		WHERE sb.server_id = $1
		ORDER BY sb.created_at DESC
	`, serverId)
	if err != nil {
		responses.BadRequest(c, "Failed to query bans", &gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	bans := []models.ServerBan{}
	for rows.Next() {
		var ban models.ServerBan
		var steamIDInt int64
		var playerName sql.NullString
		var ruleID sql.NullString
		err := rows.Scan(
			&ban.ID,
			&ban.ServerID,
			&ban.AdminID,
			&ban.AdminName,
			&ban.PlayerID,
			&steamIDInt,
			&playerName,
			&ban.Reason,
			&ban.Duration,
			&ruleID,
			&ban.CreatedAt,
			&ban.UpdatedAt,
		)
		if err != nil {
			responses.BadRequest(c, "Failed to scan ban", &gin.H{"error": err.Error()})
			return
		}

		// Convert steamID from int64 to string
		ban.SteamID = strconv.FormatInt(steamIDInt, 10)

		// Set player name
		if playerName.Valid {
			ban.Name = playerName.String
		} else {
			ban.Name = "Unknown Player"
		}

		// Set rule ID if present
		if ruleID.Valid {
			ban.RuleID = &ruleID.String
		}

		// Calculate if ban is permanent and expiry date
		ban.Permanent = ban.Duration == 0
		if !ban.Permanent {
			ban.ExpiresAt = ban.CreatedAt.Add(time.Duration(ban.Duration) * time.Minute)
		}

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

	var request models.ServerBanCreateRequest
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

	// First, find or create the player
	var playerID uuid.UUID
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT id FROM players WHERE steam_id = $1
	`, steamID).Scan(&playerID)

	if err == sql.ErrNoRows {
		// Player doesn't exist, create them
		playerID = uuid.New()
		now := time.Now()
		_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), `
			INSERT INTO players (id, steam_id, first_seen, last_seen)
			VALUES ($1, $2, $3, $4)
		`, playerID, steamID, now, now)
		if err != nil {
			responses.BadRequest(c, "Failed to create player", &gin.H{"error": err.Error()})
			return
		}
	} else if err != nil {
		responses.BadRequest(c, "Failed to lookup player", &gin.H{"error": err.Error()})
		return
	}

	// Insert the ban into the database
	var banID string
	now := time.Now()

	query := `
		INSERT INTO server_bans (server_id, admin_id, player_id, reason, duration, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	args := []interface{}{serverId, user.Id, playerID, request.Reason, request.Duration, now, now}

	// Add rule_id if provided
	if request.RuleID != nil && *request.RuleID != "" {
		ruleUUID, err := uuid.Parse(*request.RuleID)
		if err != nil {
			responses.BadRequest(c, "Invalid rule ID format", &gin.H{"error": err.Error()})
			return
		}
		query = `
			INSERT INTO server_bans (server_id, admin_id, player_id, reason, duration, rule_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id
		`
		args = append(args[:5], ruleUUID, now, now)
	}

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), query, args...).Scan(&banID)
	if err != nil {
		responses.BadRequest(c, "Failed to create ban", &gin.H{"error": err.Error()})
		return
	}

	// Also apply the ban via RCON if the server is online
	if server != nil {
		r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, server.Id)
		_ = r.BanPlayer(request.SteamID, request.Duration, request.Reason)
	}

	// Create detailed audit log
	auditData := map[string]interface{}{
		"banId":    banID,
		"steamId":  request.SteamID,
		"reason":   request.Reason,
		"duration": request.Duration,
	}

	// Add expiry information if not permanent
	if request.Duration > 0 {
		expiresAt := time.Now().Add(time.Duration(request.Duration) * time.Minute)
		auditData["expiresAt"] = expiresAt.Format(time.RFC3339)
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:ban:create", auditData)

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
	var reason string
	var duration int
	var adminId uuid.UUID

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT p.steam_id, sb.reason, sb.duration, sb.admin_id 
		FROM server_bans sb
		JOIN players p ON sb.player_id = p.id
		WHERE sb.id = $1 AND sb.server_id = $2
	`, banId, serverId).Scan(&steamIDInt, &reason, &duration, &adminId)
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
			cmdResponse := r.Execute(unbanCommand)
			if cmdResponse == "" {
				log.Error().Msgf("Failed to execute unban command for banId %s: %s", banId.String(), unbanCommand)
			} else {
				log.Info().Msgf("Unban command executed for banId %s: %s", banId.String(), unbanCommand)
			}
		}
	}

	// Create detailed audit log
	auditData := map[string]interface{}{
		"banId":    banId.String(),
		"steamId":  steamIDStr,
		"reason":   reason,
		"duration": duration,
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:ban:delete", auditData)

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
		SELECT p.steam_id, sb.reason, sb.duration, sb.created_at
		FROM server_bans sb
		JOIN players p ON sb.player_id = p.id
		WHERE sb.server_id = $1
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
