package server

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/core"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/rcon"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
)

// ServerBanResponse represents a ban response with additional fields
type ServerBanResponse struct {
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
	RuleID    *string   `json:"ruleId"`
	RuleName  *string   `json:"ruleName"`
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
		SELECT sb.id, sb.server_id, sb.admin_id, u.username, sb.steam_id, sb.reason, sb.duration, sb.created_at, sb.updated_at,
			   sb.rule_id, sr.name as rule_name
		FROM server_bans sb
		JOIN users u ON sb.admin_id = u.id
		LEFT JOIN server_rules sr ON sb.rule_id = sr.id
		WHERE sb.server_id = $1
		ORDER BY sb.created_at DESC
	`, serverId)
	if err != nil {
		responses.BadRequest(c, "Failed to query bans", &gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	bans := []ServerBanResponse{}
	for rows.Next() {
		var ban ServerBanResponse
		var steamIDInt int64
		var ruleId sql.NullString
		var ruleName sql.NullString
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
			&ruleId,
			&ruleName,
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

		// Set rule information if available
		if ruleId.Valid {
			ban.RuleID = &ruleId.String
		}
		if ruleName.Valid {
			ban.RuleName = &ruleName.String
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

	var request models.ServerBanCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Validate request
	if request.SteamId == "" {
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
	steamID, err := strconv.ParseInt(request.SteamId, 10, 64)
	if err != nil {
		responses.BadRequest(c, "Invalid Steam ID format", &gin.H{"error": "Steam ID must be a valid 64-bit integer"})
		return
	}

	// If rule ID is provided, verify it exists and belongs to this server
	if request.RuleId != nil {
		var count int
		err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
			SELECT COUNT(*) FROM server_rules
			WHERE id = $1 AND server_id = $2
		`, request.RuleId, serverId).Scan(&count)

		if err != nil {
			responses.BadRequest(c, "Failed to verify rule", &gin.H{"error": err.Error()})
			return
		}

		if count == 0 {
			responses.BadRequest(c, "Rule not found", &gin.H{"error": "Rule not found"})
			return
		}
	}

	// If ban list ID is provided, verify it exists
	if request.BanListId != nil {
		banListId, err := uuid.Parse(*request.BanListId)
		if err != nil {
			responses.BadRequest(c, "Invalid ban list ID", &gin.H{"error": err.Error()})
			return
		}

		var count int
		err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
			SELECT COUNT(*) FROM ban_lists
			WHERE id = $1
		`, banListId).Scan(&count)

		if err != nil {
			responses.BadRequest(c, "Failed to verify ban list", &gin.H{"error": err.Error()})
			return
		}

		if count == 0 {
			responses.BadRequest(c, "Ban list not found", &gin.H{"error": "Ban list not found"})
			return
		}
	}

	// Insert the ban into the database
	var banID string
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		INSERT INTO server_bans (server_id, admin_id, steam_id, reason, duration, rule_id, ban_list_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, serverId, user.Id, steamID, request.Reason, request.Duration*24*60, request.RuleId, request.BanListId).Scan(&banID)

	if err != nil {
		responses.BadRequest(c, "Failed to create ban", &gin.H{"error": err.Error()})
		return
	}

	// Also apply the ban via RCON if the server is online
	if server != nil {
		r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, server.Id)
		_ = r.BanPlayer(request.SteamId, request.Duration, request.Reason)
	}

	// Create detailed audit log
	auditData := map[string]interface{}{
		"banId":    banID,
		"steamId":  request.SteamId,
		"reason":   request.Reason,
		"duration": request.Duration,
	}

	if request.RuleId != nil {
		auditData["ruleId"] = *request.RuleId
	}

	if request.BanListId != nil {
		auditData["banListId"] = *request.BanListId

		// If the ban is added to a ban list, apply it to all servers subscribed to that list
		banId, err := uuid.Parse(banID)
		if err == nil {
			banListId, err := uuid.Parse(*request.BanListId)
			if err == nil {
				s.propagateBanToSubscribedServers(c.Request.Context(), banId, banListId, user.Id.String())
			}
		}
	}

	// Add expiry information if not permanent
	if request.Duration > 0 {
		expiresAt := time.Now().Add(time.Duration(request.Duration) * time.Hour * 24)
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
		SELECT steam_id, reason, duration, admin_id FROM server_bans
		WHERE id = $1 AND server_id = $2
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
			_, _ = r.Execute(unbanCommand) // Ignore errors, as the ban is already removed from the database
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
