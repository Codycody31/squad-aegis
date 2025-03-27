package server

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/core"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
)

// BanListResponse represents a ban list in the API response
type BanListResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsGlobal    bool      `json:"isGlobal"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	CreatedBy   string    `json:"createdBy"`
	CreatorName string    `json:"creatorName"`
	BanCount    int       `json:"banCount"`
}

// BanListDetailResponse represents a ban list with its detailed information
type BanListDetailResponse struct {
	BanListResponse
	Bans              []ServerBanResponse `json:"bans"`
	SubscribedServers []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"subscribedServers"`
}

// BanListCreateRequest represents a request to create a new ban list
type BanListCreateRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	IsGlobal    bool   `json:"isGlobal"`
}

// BanListUpdateRequest represents a request to update an existing ban list
type BanListUpdateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsGlobal    bool   `json:"isGlobal"`
}

// ServerBanListSubscriptionRequest represents a request to subscribe or unsubscribe a server from a ban list
type ServerBanListSubscriptionRequest struct {
	ServerID  string `json:"serverId" binding:"required"`
	BanListID string `json:"banListId" binding:"required"`
}

// BanListAssignRequest represents a request to assign a ban to a ban list
type BanListAssignRequest struct {
	BanID     string `json:"banId" binding:"required"`
	BanListID string `json:"banListId" binding:"required"`
}

// BanListsList handles listing all ban lists
func (s *Server) BanListsList(c *gin.Context) {
	// Query ban lists
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
		SELECT bl.id, bl.name, bl.description, bl.is_global, bl.created_at, bl.updated_at, 
		       bl.created_by, u.username, COUNT(sb.id) as ban_count
		FROM ban_lists bl
		LEFT JOIN users u ON bl.created_by = u.id
		LEFT JOIN server_bans sb ON bl.id = sb.ban_list_id
		GROUP BY bl.id, u.username
		ORDER BY bl.created_at DESC
	`)
	if err != nil {
		responses.BadRequest(c, "Failed to query ban lists", &gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	banLists := []BanListResponse{}
	for rows.Next() {
		var banList BanListResponse
		var createdBy sql.NullString
		var creatorName sql.NullString
		var description sql.NullString

		err := rows.Scan(
			&banList.ID,
			&banList.Name,
			&description,
			&banList.IsGlobal,
			&banList.CreatedAt,
			&banList.UpdatedAt,
			&createdBy,
			&creatorName,
			&banList.BanCount,
		)
		if err != nil {
			responses.BadRequest(c, "Failed to scan ban list", &gin.H{"error": err.Error()})
			return
		}

		// Handle null values
		if description.Valid {
			banList.Description = description.String
		}
		if createdBy.Valid {
			banList.CreatedBy = createdBy.String
		}
		if creatorName.Valid {
			banList.CreatorName = creatorName.String
		}

		banLists = append(banLists, banList)
	}

	responses.Success(c, "Ban lists fetched successfully", &gin.H{
		"banLists": banLists,
	})
}

// BanListCreate handles creating a new ban list
func (s *Server) BanListCreate(c *gin.Context) {
	user := s.getUserFromSession(c)

	var request BanListCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Insert the ban list into the database
	var banListID string
	err := s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		INSERT INTO ban_lists (name, description, is_global, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, request.Name, request.Description, request.IsGlobal, user.Id).Scan(&banListID)

	if err != nil {
		responses.BadRequest(c, "Failed to create ban list", &gin.H{"error": err.Error()})
		return
	}

	// Create audit log
	auditData := map[string]interface{}{
		"banListId":   banListID,
		"name":        request.Name,
		"description": request.Description,
		"isGlobal":    request.IsGlobal,
	}
	s.CreateAuditLog(c.Request.Context(), nil, &user.Id, "banlist:create", auditData)

	responses.Success(c, "Ban list created successfully", &gin.H{
		"banListId": banListID,
	})
}

// BanListGet handles getting a single ban list with details
func (s *Server) BanListGet(c *gin.Context) {
	banListIdString := c.Param("banListId")
	banListId, err := uuid.Parse(banListIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid ban list ID", &gin.H{"error": err.Error()})
		return
	}

	// Get ban list details
	var response BanListDetailResponse
	var createdBy sql.NullString
	var creatorName sql.NullString
	var description sql.NullString

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT bl.id, bl.name, bl.description, bl.is_global, bl.created_at, bl.updated_at, 
		       bl.created_by, u.username, COUNT(sb.id) as ban_count
		FROM ban_lists bl
		LEFT JOIN users u ON bl.created_by = u.id
		LEFT JOIN server_bans sb ON bl.id = sb.ban_list_id
		WHERE bl.id = $1
		GROUP BY bl.id, u.username
	`, banListId).Scan(
		&response.ID,
		&response.Name,
		&description,
		&response.IsGlobal,
		&response.CreatedAt,
		&response.UpdatedAt,
		&createdBy,
		&creatorName,
		&response.BanCount,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			responses.NotFound(c, "Ban list not found", nil)
		} else {
			responses.BadRequest(c, "Failed to get ban list", &gin.H{"error": err.Error()})
		}
		return
	}

	// Handle null values
	if description.Valid {
		response.Description = description.String
	}
	if createdBy.Valid {
		response.CreatedBy = createdBy.String
	}
	if creatorName.Valid {
		response.CreatorName = creatorName.String
	}

	// Get all bans in this ban list
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
		SELECT sb.id, sb.server_id, sb.admin_id, u.username, sb.steam_id, sb.reason, sb.duration, sb.created_at, sb.updated_at,
			   sb.rule_id, sr.name as rule_name
		FROM server_bans sb
		JOIN users u ON sb.admin_id = u.id
		LEFT JOIN server_rules sr ON sb.rule_id = sr.id
		WHERE sb.ban_list_id = $1
		ORDER BY sb.created_at DESC
	`, banListId)
	if err != nil {
		responses.BadRequest(c, "Failed to query bans", &gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	response.Bans = []ServerBanResponse{}
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
		ban.Name = "Unknown Player"

		response.Bans = append(response.Bans, ban)
	}

	// Get all servers subscribed to this ban list
	rows, err = s.Dependencies.DB.QueryContext(c.Request.Context(), `
		SELECT s.id, s.name
		FROM servers s
		JOIN server_ban_list_subscriptions sbls ON s.id = sbls.server_id
		WHERE sbls.ban_list_id = $1
		ORDER BY s.name
	`, banListId)
	if err != nil {
		responses.BadRequest(c, "Failed to query subscribed servers", &gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	response.SubscribedServers = []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}{}
	for rows.Next() {
		var server struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		err := rows.Scan(&server.ID, &server.Name)
		if err != nil {
			responses.BadRequest(c, "Failed to scan server", &gin.H{"error": err.Error()})
			return
		}
		response.SubscribedServers = append(response.SubscribedServers, server)
	}

	responses.Success(c, "Ban list fetched successfully", &gin.H{
		"banList": response,
	})
}

// BanListUpdate handles updating a ban list
func (s *Server) BanListUpdate(c *gin.Context) {
	user := s.getUserFromSession(c)

	banListIdString := c.Param("banListId")
	banListId, err := uuid.Parse(banListIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid ban list ID", &gin.H{"error": err.Error()})
		return
	}

	var request BanListUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Update the ban list in the database
	result, err := s.Dependencies.DB.ExecContext(c.Request.Context(), `
		UPDATE ban_lists
		SET name = COALESCE($1, name),
		    description = COALESCE($2, description),
		    is_global = $3,
		    updated_at = NOW()
		WHERE id = $4
	`,
		getStringOrNull(request.Name),
		getStringOrNull(request.Description),
		request.IsGlobal,
		banListId)

	if err != nil {
		responses.BadRequest(c, "Failed to update ban list", &gin.H{"error": err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		responses.NotFound(c, "Ban list not found", nil)
		return
	}

	// Create audit log
	auditData := map[string]interface{}{
		"banListId": banListIdString,
	}
	if request.Name != "" {
		auditData["name"] = request.Name
	}
	if request.Description != "" {
		auditData["description"] = request.Description
	}
	auditData["isGlobal"] = request.IsGlobal

	s.CreateAuditLog(c.Request.Context(), nil, &user.Id, "banlist:update", auditData)

	responses.Success(c, "Ban list updated successfully", nil)
}

// BanListDelete handles deleting a ban list
func (s *Server) BanListDelete(c *gin.Context) {
	user := s.getUserFromSession(c)

	banListIdString := c.Param("banListId")
	banListId, err := uuid.Parse(banListIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid ban list ID", &gin.H{"error": err.Error()})
		return
	}

	// Delete the ban list from the database
	result, err := s.Dependencies.DB.ExecContext(c.Request.Context(), `
		DELETE FROM ban_lists
		WHERE id = $1
	`, banListId)

	if err != nil {
		responses.BadRequest(c, "Failed to delete ban list", &gin.H{"error": err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		responses.NotFound(c, "Ban list not found", nil)
		return
	}

	// Create audit log
	auditData := map[string]interface{}{
		"banListId": banListIdString,
	}
	s.CreateAuditLog(c.Request.Context(), nil, &user.Id, "banlist:delete", auditData)

	responses.Success(c, "Ban list deleted successfully", nil)
}

// ServerBanListSubscribe handles subscribing a server to a ban list
func (s *Server) ServerBanListSubscribe(c *gin.Context) {
	user := s.getUserFromSession(c)

	var request ServerBanListSubscriptionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	serverId, err := uuid.Parse(request.ServerID)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	banListId, err := uuid.Parse(request.BanListID)
	if err != nil {
		responses.BadRequest(c, "Invalid ban list ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if the user has access to the server
	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Insert the subscription into the database
	var subscriptionID string
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		INSERT INTO server_ban_list_subscriptions (server_id, ban_list_id, created_by)
		VALUES ($1, $2, $3)
		ON CONFLICT (server_id, ban_list_id) DO NOTHING
		RETURNING id
	`, serverId, banListId, user.Id).Scan(&subscriptionID)

	if err != nil && err != sql.ErrNoRows {
		responses.BadRequest(c, "Failed to create subscription", &gin.H{"error": err.Error()})
		return
	}

	// Create audit log
	auditData := map[string]interface{}{
		"serverId":  request.ServerID,
		"banListId": request.BanListID,
	}
	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:banlist:subscribe", auditData)

	// Apply all bans from the ban list to the server
	s.applyBanListToServer(c.Request.Context(), serverId, banListId, user.Id.String())

	responses.Success(c, "Server subscribed to ban list successfully", nil)
}

// ServerBanListUnsubscribe handles unsubscribing a server from a ban list
func (s *Server) ServerBanListUnsubscribe(c *gin.Context) {
	user := s.getUserFromSession(c)

	var request ServerBanListSubscriptionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	serverId, err := uuid.Parse(request.ServerID)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	banListId, err := uuid.Parse(request.BanListID)
	if err != nil {
		responses.BadRequest(c, "Invalid ban list ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if the user has access to the server
	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Delete the subscription from the database
	result, err := s.Dependencies.DB.ExecContext(c.Request.Context(), `
		DELETE FROM server_ban_list_subscriptions
		WHERE server_id = $1 AND ban_list_id = $2
	`, serverId, banListId)

	if err != nil {
		responses.BadRequest(c, "Failed to delete subscription", &gin.H{"error": err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		responses.NotFound(c, "Subscription not found", nil)
		return
	}

	// Create audit log
	auditData := map[string]interface{}{
		"serverId":  request.ServerID,
		"banListId": request.BanListID,
	}
	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:banlist:unsubscribe", auditData)

	responses.Success(c, "Server unsubscribed from ban list successfully", nil)
}

// BanListAssignBan handles assigning a ban to a ban list
func (s *Server) BanListAssignBan(c *gin.Context) {
	user := s.getUserFromSession(c)

	var request BanListAssignRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	banId, err := uuid.Parse(request.BanID)
	if err != nil {
		responses.BadRequest(c, "Invalid ban ID", &gin.H{"error": err.Error()})
		return
	}

	banListId, err := uuid.Parse(request.BanListID)
	if err != nil {
		responses.BadRequest(c, "Invalid ban list ID", &gin.H{"error": err.Error()})
		return
	}

	// First, check if the ban exists and get its server ID
	var serverId string
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT server_id FROM server_bans WHERE id = $1
	`, banId).Scan(&serverId)

	if err != nil {
		if err == sql.ErrNoRows {
			responses.NotFound(c, "Ban not found", nil)
		} else {
			responses.BadRequest(c, "Failed to get ban", &gin.H{"error": err.Error()})
		}
		return
	}

	// Check if the user has access to the server
	serverUUID, _ := uuid.Parse(serverId)
	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverUUID, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Check if the ban list exists
	var banListExists int
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT COUNT(*) FROM ban_lists WHERE id = $1
	`, banListId).Scan(&banListExists)

	if err != nil {
		responses.BadRequest(c, "Failed to check ban list", &gin.H{"error": err.Error()})
		return
	}

	if banListExists == 0 {
		responses.NotFound(c, "Ban list not found", nil)
		return
	}

	// Update the ban to assign it to the ban list
	result, err := s.Dependencies.DB.ExecContext(c.Request.Context(), `
		UPDATE server_bans
		SET ban_list_id = $1, updated_at = NOW()
		WHERE id = $2
	`, banListId, banId)

	if err != nil {
		responses.BadRequest(c, "Failed to assign ban to ban list", &gin.H{"error": err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		responses.NotFound(c, "Ban not found", nil)
		return
	}

	// Create audit log
	auditData := map[string]interface{}{
		"banId":     request.BanID,
		"banListId": request.BanListID,
	}
	s.CreateAuditLog(c.Request.Context(), &serverUUID, &user.Id, "ban:assign:banlist", auditData)

	// Apply this ban to all servers subscribed to the ban list
	s.propagateBanToSubscribedServers(c.Request.Context(), banId, banListId, user.Id.String())

	responses.Success(c, "Ban assigned to ban list successfully", nil)
}

// BanListRemoveBan handles removing a ban from a ban list
func (s *Server) BanListRemoveBan(c *gin.Context) {
	user := s.getUserFromSession(c)

	banIdString := c.Param("banId")
	banId, err := uuid.Parse(banIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid ban ID", &gin.H{"error": err.Error()})
		return
	}

	// First, check if the ban exists and get its server ID and ban list ID
	var serverId string
	var banListId sql.NullString
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT server_id, ban_list_id FROM server_bans WHERE id = $1
	`, banId).Scan(&serverId, &banListId)

	if err != nil {
		if err == sql.ErrNoRows {
			responses.NotFound(c, "Ban not found", nil)
		} else {
			responses.BadRequest(c, "Failed to get ban", &gin.H{"error": err.Error()})
		}
		return
	}

	if !banListId.Valid {
		responses.BadRequest(c, "Ban is not assigned to any ban list", nil)
		return
	}

	// Check if the user has access to the server
	serverUUID, _ := uuid.Parse(serverId)
	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverUUID, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Update the ban to remove it from the ban list
	result, err := s.Dependencies.DB.ExecContext(c.Request.Context(), `
		UPDATE server_bans
		SET ban_list_id = NULL, updated_at = NOW()
		WHERE id = $1
	`, banId)

	if err != nil {
		responses.BadRequest(c, "Failed to remove ban from ban list", &gin.H{"error": err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		responses.NotFound(c, "Ban not found", nil)
		return
	}

	// Create audit log
	auditData := map[string]interface{}{
		"banId":     banIdString,
		"banListId": banListId.String,
	}
	s.CreateAuditLog(c.Request.Context(), &serverUUID, &user.Id, "ban:remove:banlist", auditData)

	responses.Success(c, "Ban removed from ban list successfully", nil)
}

// ServerBanListsList handles listing ban lists a server is subscribed to
func (s *Server) ServerBanListsList(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if the user has access to the server
	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Query ban lists the server is subscribed to
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
		SELECT bl.id, bl.name, bl.description, bl.is_global, bl.created_at, bl.updated_at, 
		       bl.created_by, u.username, COUNT(sb.id) as ban_count
		FROM ban_lists bl
		JOIN server_ban_list_subscriptions sbls ON bl.id = sbls.ban_list_id
		LEFT JOIN users u ON bl.created_by = u.id
		LEFT JOIN server_bans sb ON bl.id = sb.ban_list_id
		WHERE sbls.server_id = $1
		GROUP BY bl.id, u.username
		ORDER BY bl.created_at DESC
	`, serverId)
	if err != nil {
		responses.BadRequest(c, "Failed to query ban lists", &gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	banLists := []BanListResponse{}
	for rows.Next() {
		var banList BanListResponse
		var createdBy sql.NullString
		var creatorName sql.NullString
		var description sql.NullString

		err := rows.Scan(
			&banList.ID,
			&banList.Name,
			&description,
			&banList.IsGlobal,
			&banList.CreatedAt,
			&banList.UpdatedAt,
			&createdBy,
			&creatorName,
			&banList.BanCount,
		)
		if err != nil {
			responses.BadRequest(c, "Failed to scan ban list", &gin.H{"error": err.Error()})
			return
		}

		// Handle null values
		if description.Valid {
			banList.Description = description.String
		}
		if createdBy.Valid {
			banList.CreatedBy = createdBy.String
		}
		if creatorName.Valid {
			banList.CreatorName = creatorName.String
		}

		banLists = append(banLists, banList)
	}

	responses.Success(c, "Ban lists fetched successfully", &gin.H{
		"banLists": banLists,
	})
}

// Helper function to apply all bans from a ban list to a server
func (s *Server) applyBanListToServer(ctx context.Context, serverId uuid.UUID, banListId uuid.UUID, userId string) {
	// Get server info for RCON connection
	server, err := core.GetServerById(ctx, s.Dependencies.DB, serverId, nil)
	if err != nil {
		return
	}

	// Get all bans from the ban list
	rows, err := s.Dependencies.DB.QueryContext(ctx, `
		SELECT steam_id, reason, duration
		FROM server_bans
		WHERE ban_list_id = $1
	`, banListId)
	if err != nil {
		return
	}
	defer rows.Close()

	// Apply each ban to the server via RCON
	for rows.Next() {
		var steamIDInt int64
		var reason string
		var duration int

		err := rows.Scan(&steamIDInt, &reason, &duration)
		if err != nil {
			continue
		}

		steamID := strconv.FormatInt(steamIDInt, 10)
		durationDays := duration / (24 * 60) // Convert minutes to days

		// Apply via RCON if server is online
		if server != nil {
			r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, server.Id)
			_ = r.BanPlayer(steamID, durationDays, reason)
		}
	}
}

// Helper function to propagate a ban to all servers subscribed to a ban list
func (s *Server) propagateBanToSubscribedServers(ctx context.Context, banId uuid.UUID, banListId uuid.UUID, userId string) {
	// Get ban details
	var steamIDInt int64
	var reason string
	var duration int
	err := s.Dependencies.DB.QueryRowContext(ctx, `
		SELECT steam_id, reason, duration
		FROM server_bans
		WHERE id = $1
	`, banId).Scan(&steamIDInt, &reason, &duration)
	if err != nil {
		return
	}

	steamID := strconv.FormatInt(steamIDInt, 10)
	durationDays := duration / (24 * 60) // Convert minutes to days

	// Get all servers subscribed to this ban list
	rows, err := s.Dependencies.DB.QueryContext(ctx, `
		SELECT server_id
		FROM server_ban_list_subscriptions
		WHERE ban_list_id = $1
	`, banListId)
	if err != nil {
		return
	}
	defer rows.Close()

	// Apply the ban to each server
	for rows.Next() {
		var serverIdStr string
		err := rows.Scan(&serverIdStr)
		if err != nil {
			continue
		}

		serverId, err := uuid.Parse(serverIdStr)
		if err != nil {
			continue
		}

		// Get server info for RCON connection
		server, err := core.GetServerById(ctx, s.Dependencies.DB, serverId, nil)
		if err != nil {
			continue
		}

		// Apply via RCON if server is online
		if server != nil {
			r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, server.Id)
			_ = r.BanPlayer(steamID, durationDays, reason)
		}
	}
}

// Helper function to handle nullable strings in SQL
func getStringOrNull(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
