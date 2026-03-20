package server

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/commands"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	"go.codycody31.dev/squad-aegis/internal/shared/utils"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
)

// Request structs for player actions
type KickPlayerRequest struct {
	SteamId string `json:"steam_id"`
	EosId   string `json:"eos_id"`
	Reason  string `json:"reason"`
}

type WarnPlayerRequest struct {
	SteamId string `json:"steam_id"`
	EosId   string `json:"eos_id"`
	Message string `json:"message" binding:"required"`
}

type MovePlayerRequest struct {
	SteamId string `json:"steam_id"`
	EosId   string `json:"eos_id"`
}

// Request structs for player actions with rule support
type PlayerActionRequest struct {
	SteamId string  `json:"steam_id"`
	EosId   string  `json:"eos_id"`
	RuleId  *string `json:"rule_id"`
}

type PlayerKickRequest struct {
	PlayerActionRequest
	Reason string `json:"reason"`
}

type PlayerBanRequest struct {
	PlayerActionRequest
	Reason   string `json:"reason" binding:"required"`
	Duration string `json:"duration"` // "0"/"permanent" for permanent, "7d", "2h", "30m", or bare number (days)
}

type PlayerWarnRequest struct {
	PlayerActionRequest
	Message string `json:"message" binding:"required"`
}

// resolveRCONPlayerID validates the provided identifiers and returns the ID to
// use for RCON commands (prefers Steam ID). Returns the resolved ID and an
// error message if validation fails.
func resolveRCONPlayerID(steamId, eosId string) (rconID string, errMsg string) {
	if steamId == "" && eosId == "" {
		return "", "At least one player identifier (steam_id or eos_id) is required"
	}
	if steamId != "" {
		if !utils.IsSteamID(steamId) {
			return "", "Steam ID must be a valid 64-bit integer"
		}
		return steamId, ""
	}
	normalizedEOSID := utils.NormalizeEOSID(eosId)
	if !utils.IsEOSID(normalizedEOSID) {
		return "", "EOS ID must be a 32-character hex string"
	}
	return normalizedEOSID, ""
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

	// Grab the first word of the command to check if it's valid
	commandParts := strings.Fields(request.Command)
	if len(commandParts) == 0 {
		responses.BadRequest(c, "Command cannot be empty", &gin.H{"error": "Command cannot be empty"})
		return
	}
	commandName := commandParts[0]

	// Check if the command is in the command matrix and supports RCON
	var commandFound *commands.CommandInfo
	for _, cmd := range commands.CommandMatrix {
		if strings.EqualFold(cmd.Name, commandName) && cmd.SupportsRCON {
			commandFound = &cmd
			break
		}
	}

	if !user.SuperAdmin {
		perms, err := s.GetUserServerPermissions(c, user.Id, serverId)
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("failed to get user permissions: %w", err), nil)
			return
		}
		if commandFound == nil || !commands.UserHasPermissionForCommand(perms, commandFound) {
			responses.BadRequest(c, "Invalid or unsupported command", &gin.H{"error": "Invalid or unsupported command"})
			return
		}
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

	rconID, errMsg := resolveRCONPlayerID(request.SteamId, request.EosId)
	if errMsg != "" {
		responses.BadRequest(c, errMsg, nil)
		return
	}

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, serverId)

	// Format the kick command with sanitized parameters
	kickCommand := "AdminKick " + utils.SanitizeRCONParam(rconID)
	if request.Reason != "" {
		kickCommand += " " + utils.SanitizeRCONParam(request.Reason)
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
		"eosId":   request.EosId,
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

	rconID, errMsg := resolveRCONPlayerID(request.SteamId, request.EosId)
	if errMsg != "" {
		responses.BadRequest(c, errMsg, nil)
		return
	}

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, serverId)
	response, err := r.ExecuteRaw("AdminWarn " + utils.SanitizeRCONParam(rconID) + " " + utils.SanitizeRCONParam(request.Message))
	if err != nil {
		responses.BadRequest(c, "Failed to warn player", &gin.H{"error": err.Error()})
		return
	}

	// Create detailed audit log
	auditData := map[string]interface{}{
		"steamId": request.SteamId,
		"eosId":   request.EosId,
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

	rconID, errMsg := resolveRCONPlayerID(request.SteamId, request.EosId)
	if errMsg != "" {
		responses.BadRequest(c, errMsg, nil)
		return
	}

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, serverId)
	response, err := r.ExecuteRaw("AdminForceTeamChange " + utils.SanitizeRCONParam(rconID))
	if err != nil {
		responses.BadRequest(c, "Failed to move player", &gin.H{"error": err.Error()})
		return
	}

	// Create detailed audit log
	auditData := map[string]interface{}{
		"steamId": request.SteamId,
		"eosId":   request.EosId,
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
	log.Info().Str("server_id", serverId.String()).Msg("Forcing RCON connection disconnect")
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
	log.Info().Str("server_id", serverId.String()).Msg("Reconnecting to RCON")
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

// logRuleViolation logs a rule violation to ClickHouse if rule_id is provided.
// playerId may be a Steam ID (numeric) or an EOS ID (hex). ClickHouse stores
// player_steam_id as UInt64, so violations for EOS-only players are silently
// skipped until the schema is extended.
func (s *Server) logRuleViolation(ctx context.Context, serverId uuid.UUID, playerId string, ruleId *string, adminUserId *uuid.UUID, actionType string) error {
	if ruleId == nil || *ruleId == "" {
		return nil // No rule ID, skip logging
	}

	ruleUUID, err := uuid.Parse(*ruleId)
	if err != nil {
		log.Warn().Err(err).Str("rule_id", *ruleId).Msg("Invalid rule ID format, skipping violation log")
		return nil // Don't fail the action if rule ID is invalid
	}

	// Parse player ID to uint64 (ClickHouse schema requires UInt64, so EOS-only players are skipped)
	steamIdInt, err := strconv.ParseInt(playerId, 10, 64)
	if err != nil {
		log.Warn().Err(err).Str("player_id", playerId).Msg("Non-numeric player ID (likely EOS ID), skipping ClickHouse violation log")
		return nil // ClickHouse schema requires UInt64 for player_steam_id; EOS IDs not supported yet
	}

	query := `
		INSERT INTO squad_aegis.player_rule_violations
		(violation_id, server_id, player_steam_id, rule_id, admin_user_id, action_type, created_at, ingested_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	violationId := uuid.New()
	now := time.Now()

	err = s.Dependencies.Clickhouse.Exec(ctx, query,
		violationId,
		serverId,
		uint64(steamIdInt),
		&ruleUUID, // Nullable(UUID) - pass pointer
		adminUserId,
		actionType,
		now,
		time.Now(),
	)

	if err != nil {
		log.Error().Err(err).
			Str("server_id", serverId.String()).
			Str("player_id", playerId).
			Str("rule_id", *ruleId).
			Str("action_type", actionType).
			Msg("Failed to log rule violation to ClickHouse")
		return err
	}

	log.Info().
		Str("server_id", serverId.String()).
		Str("player_id", playerId).
		Str("rule_id", *ruleId).
		Str("action_type", actionType).
		Msg("Logged rule violation to ClickHouse")

	return nil
}

// ServerRconPlayerKick handles kicking a player via RCON with optional rule violation logging
func (s *Server) ServerRconPlayerKick(c *gin.Context) {
	user := s.getUserFromSession(c)

	var request PlayerKickRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	rconID, errMsg := resolveRCONPlayerID(request.SteamId, request.EosId)
	if errMsg != "" {
		responses.BadRequest(c, errMsg, nil)
		return
	}

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, serverId)

	// Format the kick command with sanitized parameters
	kickCommand := "AdminKick " + utils.SanitizeRCONParam(rconID)
	if request.Reason != "" {
		kickCommand += " " + utils.SanitizeRCONParam(request.Reason)
	}

	// Execute kick command
	response, err := r.ExecuteRaw(kickCommand)
	if err != nil {
		responses.BadRequest(c, "Failed to kick player", &gin.H{"error": err.Error()})
		return
	}

	// Log rule violation to ClickHouse if rule_id is provided
	if request.RuleId != nil && *request.RuleId != "" {
		s.logRuleViolation(c.Request.Context(), serverId, rconID, request.RuleId, &user.Id, "KICK")
	}

	// Create detailed audit log
	auditData := map[string]interface{}{
		"steamId": request.SteamId,
		"eosId":   request.EosId,
		"reason":  request.Reason,
	}
	if request.RuleId != nil && *request.RuleId != "" {
		auditData["ruleId"] = *request.RuleId
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:rcon:player:kick", auditData)

	responses.Success(c, "Player kicked successfully", &gin.H{"response": response})
}

// ServerRconPlayerBan handles banning a player via RCON with optional rule violation logging
func (s *Server) ServerRconPlayerBan(c *gin.Context) {
	user := s.getUserFromSession(c)

	var request PlayerBanRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	rconID, errMsg := resolveRCONPlayerID(request.SteamId, request.EosId)
	if errMsg != "" {
		responses.BadRequest(c, errMsg, nil)
		return
	}

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

	if request.Reason == "" {
		responses.BadRequest(c, "Ban reason is required", &gin.H{"error": "Ban reason is required"})
		return
	}

	// Parse duration string into expires_at
	expiresAt, parseErr := utils.ParseBanDuration(request.Duration)
	if parseErr != nil {
		responses.BadRequest(c, "Invalid duration format", &gin.H{"error": parseErr.Error()})
		return
	}

	// Validate and prepare identifiers for DB storage
	var steamIDVal interface{}
	var eosIDVal interface{}
	if request.SteamId != "" {
		steamID, parseErr := strconv.ParseInt(request.SteamId, 10, 64)
		if parseErr != nil {
			responses.BadRequest(c, "Invalid Steam ID format", &gin.H{"error": "Steam ID must be a valid 64-bit integer"})
			return
		}
		steamIDVal = steamID
	}
	if request.EosId != "" {
		normalizedEOSID := utils.NormalizeEOSID(request.EosId)
		if !utils.IsEOSID(normalizedEOSID) {
			responses.BadRequest(c, "Invalid EOS ID format", &gin.H{"error": "EOS ID must be a 32-character hex string"})
			return
		}
		eosIDVal = normalizedEOSID
	}

	// Build INSERT query dynamically
	var banID string
	now := time.Now()

	columns := "id, server_id, admin_id, steam_id, eos_id, reason, expires_at, created_at, updated_at"
	placeholders := "$1, $2, $3, $4, $5, $6, $7, $8, $9"
	args := []interface{}{uuid.New(), serverId, user.Id, steamIDVal, eosIDVal, request.Reason, expiresAt, now, now}
	nextParam := 10

	// Add rule_id if provided
	if request.RuleId != nil && *request.RuleId != "" {
		ruleUUID, parseErr := uuid.Parse(*request.RuleId)
		if parseErr != nil {
			responses.BadRequest(c, "Invalid rule ID format", &gin.H{"error": parseErr.Error()})
			return
		}
		columns += ", rule_id"
		placeholders += fmt.Sprintf(", $%d", nextParam)
		args = append(args, ruleUUID)
		nextParam++
	}

	query := fmt.Sprintf(`INSERT INTO server_bans (%s) VALUES (%s) RETURNING id`, columns, placeholders)

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), query, args...).Scan(&banID)
	if err != nil {
		responses.BadRequest(c, "Failed to create ban", &gin.H{"error": err.Error()})
		return
	}

	// Sync Bans.cfg, reload config, and kick the player to enforce immediately
	if err := s.syncBansCfg(c.Request.Context(), server); err != nil {
		if _, rollbackErr := s.Dependencies.DB.ExecContext(c.Request.Context(), `
			DELETE FROM server_bans
			WHERE id = $1 AND server_id = $2
		`, banID, serverId); rollbackErr != nil {
			log.Error().Err(rollbackErr).Str("banId", banID).Str("serverId", serverId.String()).Msg("Failed to roll back RCON ban after Bans.cfg sync failure")
			responses.InternalServerError(c, fmt.Errorf("failed to sync Bans.cfg after RCON ban: %w (rollback also failed: %v)", err, rollbackErr), nil)
			return
		}

		responses.InternalServerError(c, fmt.Errorf("failed to sync Bans.cfg after RCON ban: %w", err), nil)
		return
	}

	// Log rule violation to ClickHouse only after the ban has been synced successfully.
	if request.RuleId != nil && *request.RuleId != "" {
		s.logRuleViolation(c.Request.Context(), serverId, rconID, request.RuleId, &user.Id, "BAN")
	}

	r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, server.Id)
	if err := r.KickPlayer(rconID, request.Reason); err != nil {
		log.Warn().Err(err).Str("playerId", rconID).Str("serverId", serverId.String()).Msg("Failed to kick player after ban")
	}

	// Create detailed audit log after the ban has been persisted and synced.
	auditData := map[string]interface{}{
		"banId":   banID,
		"steamId": request.SteamId,
		"eosId":   request.EosId,
		"reason":  request.Reason,
	}
	if expiresAt != nil {
		auditData["expiresAt"] = expiresAt.Format(time.RFC3339)
	}
	if request.RuleId != nil && *request.RuleId != "" {
		auditData["ruleId"] = *request.RuleId
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:rcon:player:ban", auditData)

	responses.Success(c, "Player banned successfully", &gin.H{
		"banId": banID,
	})
}

// ServerRconPlayerWarn handles warning a player via RCON with optional rule violation logging
func (s *Server) ServerRconPlayerWarn(c *gin.Context) {
	user := s.getUserFromSession(c)

	var request PlayerWarnRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	rconID, errMsg := resolveRCONPlayerID(request.SteamId, request.EosId)
	if errMsg != "" {
		responses.BadRequest(c, errMsg, nil)
		return
	}

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, serverId)
	response, err := r.ExecuteRaw("AdminWarn " + utils.SanitizeRCONParam(rconID) + " " + utils.SanitizeRCONParam(request.Message))
	if err != nil {
		responses.BadRequest(c, "Failed to warn player", &gin.H{"error": err.Error()})
		return
	}

	// Log rule violation to ClickHouse if rule_id is provided
	if request.RuleId != nil && *request.RuleId != "" {
		s.logRuleViolation(c.Request.Context(), serverId, rconID, request.RuleId, &user.Id, "WARN")
	}

	// Create detailed audit log
	auditData := map[string]interface{}{
		"steamId": request.SteamId,
		"eosId":   request.EosId,
		"message": request.Message,
	}
	if request.RuleId != nil && *request.RuleId != "" {
		auditData["ruleId"] = *request.RuleId
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:rcon:player:warn", auditData)

	responses.Success(c, "Player warned successfully", &gin.H{"response": response})
}

// RuleEscalationSuggestion represents a suggested escalation action
type RuleEscalationSuggestion struct {
	SuggestedAction   string  `json:"suggested_action"`   // WARN, KICK, BAN
	SuggestedDuration *int    `json:"suggested_duration"` // Duration in days for bans
	SuggestedMessage  string  `json:"suggested_message"`
	ViolationCount    int     `json:"violation_count"`
	RuleTitle         string  `json:"rule_title"`
	RuleDescription   string  `json:"rule_description"`
	RuleId            *string `json:"rule_id"` // Rule ID for generating reason in UI
}

// calculateRuleNumber calculates the hierarchical rule number (e.g., "1.2.3") for a given rule
func calculateRuleNumber(ctx context.Context, db *sql.DB, ruleId uuid.UUID, serverId uuid.UUID) string {
	type ruleNode struct {
		id           uuid.UUID
		parentId     sql.NullString
		displayOrder int
	}

	// Build the hierarchy path from root to target rule
	path := []ruleNode{}
	currentRuleId := ruleId

	for {
		var node ruleNode
		var parentId sql.NullString
		query := `SELECT id, parent_id, display_order FROM server_rules WHERE id = $1 AND server_id = $2`
		err := db.QueryRowContext(ctx, query, currentRuleId, serverId).Scan(&node.id, &parentId, &node.displayOrder)
		if err != nil {
			break
		}

		node.parentId = parentId
		path = append([]ruleNode{node}, path...) // Prepend to build path from root to target

		if !parentId.Valid {
			break // Reached root
		}

		currentRuleId, err = uuid.Parse(parentId.String)
		if err != nil {
			break
		}
	}

	// Calculate number by traversing down the path
	var parts []string
	for i, node := range path {
		if i == 0 {
			// Root level - find its position among root rules
			query := `SELECT COUNT(*) FROM server_rules WHERE server_id = $1 AND parent_id IS NULL AND display_order < $2`
			var position int
			err := db.QueryRowContext(ctx, query, serverId, node.displayOrder).Scan(&position)
			if err != nil {
				return ""
			}
			parts = append(parts, strconv.Itoa(position+1))
		} else {
			// Child level - find its position among siblings
			parentId := path[i-1].id
			query := `SELECT COUNT(*) FROM server_rules WHERE server_id = $1 AND parent_id = $2 AND display_order < $3`
			var position int
			err := db.QueryRowContext(ctx, query, serverId, parentId, node.displayOrder).Scan(&position)
			if err != nil {
				return ""
			}
			parts = append(parts, strconv.Itoa(position+1))
		}
	}

	return strings.Join(parts, ".")
}

// formatDuration formats the duration in days to a human-readable string
func formatDuration(durationDays sql.NullInt64) string {
	if !durationDays.Valid {
		return "perm"
	}

	days := int(durationDays.Int64)
	if days == 0 {
		return "perm"
	}

	if days == 1 {
		return "1 day"
	}

	return fmt.Sprintf("%d days", days)
}

// ServerRconPlayerEscalationSuggestion checks what action should be taken for a player based on rule violations
func (s *Server) ServerRconPlayerEscalationSuggestion(c *gin.Context) {
	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	steamId := c.Query("steam_id")
	eosId := utils.NormalizeEOSID(c.Query("eos_id"))
	ruleIdStr := c.Query("rule_id")

	if steamId == "" && eosId == "" {
		responses.BadRequest(c, "Player identifier is required", &gin.H{"error": "steam_id or eos_id parameter is required"})
		return
	}

	// ClickHouse violation table uses UInt64 for player_steam_id, so we can
	// only query violation history for Steam IDs, including those linked from EOS.
	hasSteamId := false
	if steamId != "" {
		_, parseErr := strconv.ParseUint(steamId, 10, 64)
		if parseErr != nil {
			responses.BadRequest(c, "Invalid Steam ID format", &gin.H{"error": "Steam ID must be a valid 64-bit integer"})
			return
		}
		hasSteamId = true
	}
	if eosId != "" && !utils.IsEOSID(eosId) {
		responses.BadRequest(c, "Invalid EOS ID format", &gin.H{"error": "EOS ID must be a 32-character hex string"})
		return
	}

	lookupPlayerID := steamId
	lookupIsSteamID := true
	if !hasSteamId {
		lookupPlayerID = eosId
		lookupIsSteamID = false
	}

	resolvedSteamIDStrings, _ := s.resolveLinkedPlayerIdentifiers(c, lookupPlayerID, lookupIsSteamID)
	violationLookupSteamIDs := make([]uint64, 0, len(resolvedSteamIDStrings))
	seenSteamIDs := make(map[uint64]struct{}, len(resolvedSteamIDStrings))
	for _, resolvedSteamID := range resolvedSteamIDStrings {
		parsedSteamID, parseErr := strconv.ParseUint(resolvedSteamID, 10, 64)
		if parseErr != nil {
			continue
		}
		if _, seen := seenSteamIDs[parsedSteamID]; seen {
			continue
		}

		seenSteamIDs[parsedSteamID] = struct{}{}
		violationLookupSteamIDs = append(violationLookupSteamIDs, parsedSteamID)
	}

	suggestion := RuleEscalationSuggestion{
		ViolationCount: 0,
	}

	// If rule_id is provided, check violation count for that specific rule
	if ruleIdStr != "" {
		ruleId, err := uuid.Parse(ruleIdStr)
		if err != nil {
			responses.BadRequest(c, "Invalid rule ID format", &gin.H{"error": err.Error()})
			return
		}

		// Get violation count from ClickHouse for this player and rule across
		// every linked Steam ID we can resolve from the supplied identifiers.
		if len(violationLookupSteamIDs) > 0 {
			query := `
				SELECT count(*) as violation_count
				FROM squad_aegis.player_rule_violations
				WHERE server_id = ? AND player_steam_id IN (?) AND rule_id = ?
			`

			var violationCount uint64
			err = s.Dependencies.Clickhouse.QueryRow(c.Request.Context(), query, serverId, violationLookupSteamIDs, ruleId.String()).Scan(&violationCount)
			if err != nil {
				if err != sql.ErrNoRows {
					log.Warn().Err(err).Msg("Failed to query violation count, continuing without escalation suggestion")
				}
			} else {
				suggestion.ViolationCount = int(violationCount)
			}
		}

		// Get rule details and calculate rule number
		ruleQuery := `
			SELECT id, parent_id, display_order, title, description
			FROM server_rules
			WHERE id = $1 AND server_id = $2
		`
		var ruleTitle, ruleDescription string
		var ruleDisplayOrder int
		var ruleParentId sql.NullString
		var queriedRuleId uuid.UUID
		nextViolationCount := suggestion.ViolationCount
		err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), ruleQuery, ruleId, serverId).Scan(
			&queriedRuleId, &ruleParentId, &ruleDisplayOrder, &ruleTitle, &ruleDescription)
		if err == nil {
			suggestion.RuleTitle = ruleTitle
			suggestion.RuleDescription = ruleDescription
			ruleIdString := queriedRuleId.String()
			suggestion.RuleId = &ruleIdString

			// Calculate rule number by traversing up the hierarchy
			ruleNumber := calculateRuleNumber(c.Request.Context(), s.Dependencies.DB, queriedRuleId, serverId)

			// Get the action based on violation count (current count)
			actionQuery := `
				SELECT action_type, duration, message
				FROM server_rule_actions
				WHERE rule_id = $1 AND violation_count <= $2
				ORDER BY violation_count DESC
				LIMIT 1
			`
			var actionType string
			var durationDays sql.NullInt64
			var actionMessage string
			err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), actionQuery, queriedRuleId, nextViolationCount).Scan(&actionType, &durationDays, &actionMessage)
			if err == nil {
				suggestion.SuggestedAction = actionType
				if durationDays.Valid {
					durationDaysInt := int(durationDays.Int64)
					suggestion.SuggestedDuration = &durationDaysInt
				}

				// Generate message in new format: #1.2.3 | Rule Title | { 1 day/3 days/perm}
				if actionMessage != "" {
					// If custom message exists, use it as-is
					suggestion.SuggestedMessage = actionMessage
				} else {
					// Otherwise, generate the formatted message
					var durationStr string
					if actionType == "BAN" {
						durationStr = formatDuration(durationDays)
					} else {
						durationStr = "N/A"
					}
					suggestion.SuggestedMessage = fmt.Sprintf("%s | %s | %s", ruleNumber, ruleTitle, durationStr)
				}
			} else if err != sql.ErrNoRows {
				log.Warn().Err(err).Msg("Failed to query rule actions")
			}
		}

		// Update violation_count to show what it will be after this action is submitted
		suggestion.ViolationCount = nextViolationCount
	}

	responses.Success(c, "Escalation suggestion fetched successfully", &gin.H{"suggestion": suggestion})
}
