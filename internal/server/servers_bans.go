package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/core"
	dbpkg "go.codycody31.dev/squad-aegis/internal/db"
	"go.codycody31.dev/squad-aegis/internal/file_upload"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	"go.codycody31.dev/squad-aegis/internal/shared/utils"
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
		SELECT sb.id, sb.server_id, sb.admin_id, COALESCE(u.username, 'System') as admin_name, sb.steam_id, sb.eos_id, sb.reason, sb.expires_at, sb.rule_id, sr.title as rule_title,  sb.ban_list_id, bl.name as ban_list_name, sb.evidence_text, sb.created_at, sb.updated_at
		FROM server_bans sb
		LEFT JOIN users u ON sb.admin_id = u.id
		LEFT JOIN ban_lists bl ON sb.ban_list_id = bl.id
		LEFT JOIN server_rules sr ON sb.rule_id = sr.id
		WHERE sb.server_id = $1
		ORDER BY sb.created_at DESC
	`, serverId)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	defer rows.Close()

	bans := []models.ServerBan{}
	steamIDs := []string{}
	for rows.Next() {
		var ban models.ServerBan
		var adminIDStr sql.NullString
		var steamIDInt sql.NullInt64
		var eosIDStr sql.NullString
		var expiresAt sql.NullTime
		var ruleID sql.NullString
		var ruleTitle sql.NullString
		var banListID sql.NullString
		var banListName sql.NullString
		var evidenceText sql.NullString
		err := rows.Scan(
			&ban.ID,
			&ban.ServerID,
			&adminIDStr,
			&ban.AdminName,
			&steamIDInt,
			&eosIDStr,
			&ban.Reason,
			&expiresAt,
			&ruleID,
			&ruleTitle,
			&banListID,
			&banListName,
			&evidenceText,
			&ban.CreatedAt,
			&ban.UpdatedAt,
		)
		if err != nil {
			responses.InternalServerError(c, err, nil)
			return
		}

		if adminIDStr.Valid {
			adminID, parseErr := uuid.Parse(adminIDStr.String)
			if parseErr != nil {
				responses.InternalServerError(c, parseErr, nil)
				return
			}
			ban.AdminID = &adminID
		}
		if steamIDInt.Valid {
			ban.SteamID = strconv.FormatInt(steamIDInt.Int64, 10)
		}
		if eosIDStr.Valid {
			ban.EOSID = utils.NormalizeEOSID(eosIDStr.String)
		}

		// Collect steam IDs for batch lookup
		if ban.SteamID != "" {
			steamIDs = append(steamIDs, ban.SteamID)
		}

		// Set rule ID if present
		if ruleID.Valid {
			ban.RuleID = &ruleID.String
		}

		// Set rule name if present
		if ruleTitle.Valid {
			ban.RuleName = &ruleTitle.String
		}

		// Set ban list information if present
		if banListID.Valid {
			ban.BanListID = &banListID.String
		}
		if banListName.Valid {
			ban.BanListName = &banListName.String
		}

		// Set evidence text if present
		if evidenceText.Valid {
			ban.EvidenceText = &evidenceText.String
		}

		// Set expires_at and compute permanent flag
		if expiresAt.Valid {
			ban.ExpiresAt = &expiresAt.Time
		}
		ban.Permanent = ban.ExpiresAt == nil

		// Load evidence records for this ban
		evidence, evidenceErr := s.loadBanEvidence(c.Request.Context(), ban.ID)
		if evidenceErr != nil {
			log.Error().Err(evidenceErr).Str("ban_id", ban.ID).Msg("Failed to load ban evidence")
		}
		ban.Evidence = evidence

		bans = append(bans, ban)
	}
	if err := rows.Err(); err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	// Batch lookup player names from ClickHouse
	playerNames := s.lookupPlayerNamesBatch(c.Request.Context(), steamIDs)

	// Assign player names to bans
	for i := range bans {
		if name, ok := playerNames[bans[i].SteamID]; ok {
			bans[i].Name = name
		} else if bans[i].SteamID != "" {
			bans[i].Name = bans[i].SteamID
		} else {
			bans[i].Name = bans[i].EOSID
		}
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

	// Validate request - at least one player identifier required
	if request.SteamID == "" && request.EOSID == "" {
		responses.BadRequest(c, "Steam ID or EOS ID is required", &gin.H{"error": "At least one player identifier (steam_id or eos_id) is required"})
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

	// Detect and validate player ID types
	var steamIDVal interface{}
	var eosIDVal interface{}
	normalizedEOSID := utils.NormalizeEOSID(request.EOSID)
	if request.SteamID != "" {
		steamID, parseErr := strconv.ParseInt(request.SteamID, 10, 64)
		if parseErr != nil {
			responses.BadRequest(c, "Invalid Steam ID format", &gin.H{"error": "Steam ID must be a valid 64-bit integer"})
			return
		}
		steamIDVal = steamID
	}
	if normalizedEOSID != "" {
		if !utils.IsEOSID(normalizedEOSID) {
			responses.BadRequest(c, "Invalid EOS ID format", &gin.H{"error": "EOS ID must be a 32-character hex string"})
			return
		}
		eosIDVal = normalizedEOSID
	}

	// Determine the player ID to use for RCON commands (prefer Steam ID)
	rconPlayerID := request.SteamID
	if rconPlayerID == "" {
		rconPlayerID = normalizedEOSID
	}

	restoreExcludedSteamIDs := map[string]bool(nil)
	if request.SteamID != "" {
		restoreExcludedSteamIDs = map[string]bool{request.SteamID: true}
	}
	restoreExcludedEOSIDs := map[string]bool(nil)
	if normalizedEOSID != "" {
		restoreExcludedEOSIDs = map[string]bool{normalizedEOSID: true}
	}

	// Build INSERT query dynamically
	var banID uuid.UUID = uuid.New()
	now := time.Now()

	columns := "id, server_id, admin_id, steam_id, eos_id, reason, expires_at, evidence_text, created_at, updated_at"
	placeholders := "$1, $2, $3, $4, $5, $6, $7, $8, $9, $10"
	args := []interface{}{banID, serverId, user.Id, steamIDVal, eosIDVal, request.Reason, expiresAt, request.EvidenceText, now, now}
	nextParam := 11

	if request.RuleID != nil && *request.RuleID != "" {
		ruleUUID, parseErr := uuid.Parse(*request.RuleID)
		if parseErr != nil {
			responses.BadRequest(c, "Invalid rule ID format", &gin.H{"error": parseErr.Error()})
			return
		}
		columns += ", rule_id"
		placeholders += fmt.Sprintf(", $%d", nextParam)
		args = append(args, ruleUUID)
		nextParam++
	}

	if request.BanListID != nil && *request.BanListID != "" {
		banListUUID, parseErr := uuid.Parse(*request.BanListID)
		if parseErr != nil {
			responses.BadRequest(c, "Invalid ban list ID format", &gin.H{"error": parseErr.Error()})
			return
		}
		columns += ", ban_list_id"
		placeholders += fmt.Sprintf(", $%d", nextParam)
		args = append(args, banListUUID)
		nextParam++
	}

	query := fmt.Sprintf(`INSERT INTO server_bans (%s) VALUES (%s) RETURNING id`, columns, placeholders)

	tx, err := s.Dependencies.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	var returnedBanID string
	err = tx.QueryRowContext(c.Request.Context(), query, args...).Scan(&returnedBanID)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	if err := s.syncBansCfgWithExecutor(c.Request.Context(), tx, server); err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to sync Bans.cfg after ban: %w", err), nil)
		return
	}

	if err := tx.Commit(); err != nil {
		if restoreErr := s.syncBansCfgWithExcludedIDs(c.Request.Context(), server, restoreExcludedSteamIDs, restoreExcludedEOSIDs); restoreErr != nil {
			log.Warn().Err(restoreErr).Str("banId", banID.String()).Str("serverId", serverId.String()).Msg("Failed to restore Bans.cfg after ban commit error")
		}
		responses.InternalServerError(c, fmt.Errorf("failed to commit ban after syncing Bans.cfg: %w", err), nil)
		return
	}

	// Insert evidence records after the ban has been persisted and synced.
	if len(request.Evidence) > 0 {
		err = s.createBanEvidence(c.Request.Context(), banID.String(), serverId, request.Evidence)
		if err != nil {
			log.Error().Err(err).Str("banId", banID.String()).Msg("Failed to create ban evidence")
			// Don't fail the entire ban creation, just log the error
		}
	}

	// Log rule violation to ClickHouse if rule ID is provided (Steam ID only - ClickHouse schema requires UInt64)
	if request.RuleID != nil && *request.RuleID != "" && request.SteamID != "" {
		if err := s.logRuleViolation(c.Request.Context(), serverId, request.SteamID, request.RuleID, &user.Id, "BAN"); err != nil {
			log.Warn().Err(err).Str("steamId", request.SteamID).Str("ruleId", *request.RuleID).Msg("Failed to log rule violation for manual ban")
		}
	}

	// Create detailed audit log after the ban has been persisted and synced.
	auditData := map[string]interface{}{
		"banId":         banID.String(),
		"reason":        request.Reason,
		"evidenceCount": len(request.Evidence),
	}
	if request.SteamID != "" {
		auditData["steamId"] = request.SteamID
	}
	if request.EOSID != "" {
		auditData["eosId"] = request.EOSID
	}
	if request.RuleID != nil && *request.RuleID != "" {
		auditData["ruleId"] = *request.RuleID
	}
	if expiresAt != nil {
		auditData["expiresAt"] = expiresAt.Format(time.RFC3339)
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:ban:create", auditData)

	r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, server.Id)
	if err := r.BanPlayer(rconPlayerID, request.Reason); err != nil {
		log.Warn().Err(err).Str("playerID", rconPlayerID).Str("serverId", serverId.String()).Msg("Failed to kick player after ban")
	}

	responses.Success(c, "Ban created successfully", &gin.H{
		"banId": banID.String(),
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

	// Get the ban details first so we can build audit data and clean up evidence
	// after the transaction commits.
	var steamIDInt sql.NullInt64
	var eosIDStr sql.NullString
	var reason string

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT sb.steam_id, sb.eos_id, sb.reason
		FROM server_bans sb
		WHERE sb.id = $1 AND sb.server_id = $2
	`, banId, serverId).Scan(&steamIDInt, &eosIDStr, &reason)
	if err != nil {
		if err == sql.ErrNoRows {
			responses.BadRequest(c, "Ban not found", &gin.H{"error": "Ban not found"})
		} else {
			responses.InternalServerError(c, err, nil)
		}
		return
	}

	var steamIDStr string
	if steamIDInt.Valid {
		steamIDStr = strconv.FormatInt(steamIDInt.Int64, 10)
	}
	var eosID string
	if eosIDStr.Valid {
		eosID = utils.NormalizeEOSID(eosIDStr.String)
	}

	evidenceFilePaths, evidenceErr := s.getBanEvidenceFilePaths(c.Request.Context(), banId.String())
	if evidenceErr != nil {
		log.Warn().Err(evidenceErr).Str("banId", banId.String()).Msg("Failed to collect evidence file paths before ban deletion")
	}

	tx, err := s.Dependencies.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(c.Request.Context(), `
		DELETE FROM server_bans
		WHERE id = $1 AND server_id = $2
	`, banId, serverId)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	if rowsAffected == 0 {
		responses.BadRequest(c, "Ban not found", &gin.H{"error": "Ban not found"})
		return
	}

	if err := s.syncBansCfgWithExecutor(c.Request.Context(), tx, server); err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to sync Bans.cfg after unban for server %s (steam_id=%s eos_id=%s): %w", serverId.String(), steamIDStr, eosID, err), nil)
		return
	}

	if err := tx.Commit(); err != nil {
		if restoreErr := s.syncBansCfg(c.Request.Context(), server); restoreErr != nil {
			log.Warn().Err(restoreErr).Str("banId", banId.String()).Str("serverId", serverId.String()).Msg("Failed to restore Bans.cfg after unban commit error")
		}
		responses.InternalServerError(c, fmt.Errorf("failed to commit unban after syncing Bans.cfg: %w", err), nil)
		return
	}

	// Delete evidence files from storage after the DB row is gone.
	if err := s.deleteEvidenceFilesFromStorage(c.Request.Context(), banId.String(), evidenceFilePaths); err != nil {
		log.Warn().Err(err).Str("banId", banId.String()).Msg("Failed to delete some evidence files from storage, continuing with ban deletion")
	}

	// Reload server config so the game server picks up the updated Bans.cfg
	r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, server.Id)
	if _, err := r.ExecuteRaw("AdminReloadServerConfig"); err != nil {
		log.Warn().Err(err).Str("serverId", serverId.String()).Msg("Failed to reload server config after unban")
	}

	// Create detailed audit log
	auditData := map[string]interface{}{
		"banId":   banId.String(),
		"steamId": steamIDStr,
		"eosId":   eosID,
		"reason":  reason,
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:ban:delete", auditData)

	responses.Success(c, "Ban removed successfully", nil)
}

// SyncBansCfgByID looks up a server by ID and regenerates its Bans.cfg.
// Intended for use as a callback from plugin/workflow ban paths.
func (s *Server) SyncBansCfgByID(ctx context.Context, serverID uuid.UUID) error {
	srv, err := core.GetServerById(ctx, s.Dependencies.DB, serverID, nil)
	if err != nil {
		return fmt.Errorf("failed to look up server %s: %w", serverID, err)
	}
	return s.syncBansCfg(ctx, srv)
}

// syncBansCfg writes a Bans.cfg for this server, merging DB-tracked bans with
// any external entries (auto-bans, in-game manual bans) already present in the file.
func (s *Server) syncBansCfg(ctx context.Context, server *models.Server) error {
	return s.syncBansCfgWithExecutor(ctx, s.Dependencies.DB, server)
}

func (s *Server) syncBansCfgWithExcludedIDs(ctx context.Context, server *models.Server, excludedSteamIDs, excludedEOSIDs map[string]bool) error {
	return s.syncBansCfgWithExcludedIDsUsingExecutor(ctx, s.Dependencies.DB, server, excludedSteamIDs, excludedEOSIDs)
}

func (s *Server) syncBansCfgWithExecutor(ctx context.Context, executor dbpkg.Executor, server *models.Server) error {
	return s.syncBansCfgWithExcludedIDsUsingExecutor(ctx, executor, server, nil, nil)
}

func (s *Server) syncBansCfgWithExcludedIDsUsingExecutor(ctx context.Context, executor dbpkg.Executor, server *models.Server, excludedSteamIDs, excludedEOSIDs map[string]bool) error {
	if server == nil || server.SquadGamePath == nil || *server.SquadGamePath == "" {
		return nil // No base path configured, nothing to do
	}
	if server.LogSourceType == nil || *server.LogSourceType == "" {
		return nil // No log source configured, can't access files
	}

	// Acquire a per-server mutex to serialize read-modify-write cycles on
	// Bans.cfg. Without this, concurrent ban/unban requests for the same
	// server could race and silently drop entries.
	muVal, _ := s.bansCfgMu.LoadOrStore(server.Id.String(), &sync.Mutex{})
	mu := muVal.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	managedBans, err := s.getEffectiveServerBansWithExecutor(ctx, executor, server.Id)
	if err != nil {
		return err
	}
	filteredManagedBans := filterServerBansByExcludedIDs(managedBans, excludedSteamIDs, excludedEOSIDs)

	// Preserve entries from the existing Bans.cfg that are not tracked in the DB
	// (e.g., automatic teamkill bans, in-game manual bans not yet imported).
	existingContent, readErr := s.readBansCfg(ctx, server)
	content, err := buildMergedServerBansCfgContent(filteredManagedBans, existingContent, readErr, excludedSteamIDs, excludedEOSIDs)
	if err != nil {
		return err
	}

	bansCfgPath := buildBansCfgPath(*server.SquadGamePath, server.LogSourceType)

	switch *server.LogSourceType {
	case "sftp", "ftp":
		if server.LogHost == nil || server.LogUsername == nil || server.LogPassword == nil {
			return fmt.Errorf("missing SFTP/FTP credentials for Bans.cfg editing")
		}
		port := 22
		if *server.LogSourceType == "ftp" {
			port = 21
		}
		if server.LogPort != nil {
			port = *server.LogPort
		}

		uploader, err := file_upload.NewUploader(file_upload.UploadConfig{
			Protocol: *server.LogSourceType,
			Host:     *server.LogHost,
			Port:     port,
			Username: *server.LogUsername,
			Password: *server.LogPassword,
			FilePath: bansCfgPath,
		})
		if err != nil {
			return fmt.Errorf("failed to connect for Bans.cfg editing: %w", err)
		}
		defer uploader.Close()

		atomicUploader, ok := uploader.(interface {
			UploadAtomically(context.Context, string) error
		})
		if !ok {
			return fmt.Errorf("Bans.cfg uploader does not support atomic replacement")
		}

		if err := atomicUploader.UploadAtomically(ctx, content); err != nil {
			return fmt.Errorf("failed to write Bans.cfg: %w", err)
		}

		log.Info().Str("serverId", server.Id.String()).Msg("Synced Bans.cfg via " + *server.LogSourceType)

	case "local":
		if err := writeFileAtomically(bansCfgPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write local Bans.cfg: %w", err)
		}

		log.Info().Str("serverId", server.Id.String()).Msg("Synced local Bans.cfg")
	}

	return nil
}

func writeFileAtomically(path string, content []byte, mode os.FileMode) error {
	tempFile, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".aegis-tmp-*")
	if err != nil {
		return err
	}

	tempPath := tempFile.Name()
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tempPath)
		}
	}()

	if err := tempFile.Chmod(mode); err != nil {
		_ = tempFile.Close()
		return err
	}
	if _, err := tempFile.Write(content); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tempPath, path); err == nil {
		cleanupTemp = false
		return nil
	}

	backupPath := filepath.Join(
		filepath.Dir(path),
		fmt.Sprintf(".%s.aegis-bak-%d", filepath.Base(path), time.Now().UnixNano()),
	)
	if err := os.Rename(path, backupPath); err != nil {
		return err
	}

	if err := os.Rename(tempPath, path); err != nil {
		restoreErr := os.Rename(backupPath, path)
		if restoreErr != nil {
			return fmt.Errorf("failed to replace file and failed to restore original: replace: %w; restore: %v", err, restoreErr)
		}
		return err
	}

	cleanupTemp = false
	_ = os.Remove(backupPath)
	return nil
}

// ServerBansUpdate handles updating an existing ban
func (s *Server) ServerBansUpdate(c *gin.Context) {
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

	var request models.ServerBanUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Get the current ban details first
	var currentBan models.ServerBan
	var adminIDStr sql.NullString
	var steamIDInt sql.NullInt64
	var eosIDStr sql.NullString
	var ruleID sql.NullString
	var ruleTitle sql.NullString
	var banListID sql.NullString
	var banListName sql.NullString
	var evidenceText sql.NullString

	var updateExpiresAt sql.NullTime
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT sb.id, sb.server_id, sb.admin_id, COALESCE(u.username, 'System') as admin_name, sb.steam_id, sb.eos_id, sb.reason, sb.expires_at, sb.rule_id, sr.title as rule_title,  sb.ban_list_id, bl.name as ban_list_name, sb.evidence_text, sb.created_at, sb.updated_at
		FROM server_bans sb
		LEFT JOIN users u ON sb.admin_id = u.id
		LEFT JOIN ban_lists bl ON sb.ban_list_id = bl.id
		LEFT JOIN server_rules sr ON sb.rule_id = sr.id
		WHERE sb.id = $1 AND sb.server_id = $2
	`, banId, serverId).Scan(
		&currentBan.ID,
		&currentBan.ServerID,
		&adminIDStr,
		&currentBan.AdminName,
		&steamIDInt,
		&eosIDStr,
		&currentBan.Reason,
		&updateExpiresAt,
		&ruleID,
		&ruleTitle,
		&banListID,
		&banListName,
		&evidenceText,
		&currentBan.CreatedAt,
		&currentBan.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			responses.BadRequest(c, "Ban not found", &gin.H{"error": "Ban not found"})
		} else {
			responses.InternalServerError(c, err, nil)
		}
		return
	}

	if adminIDStr.Valid {
		adminID, parseErr := uuid.Parse(adminIDStr.String)
		if parseErr != nil {
			responses.InternalServerError(c, parseErr, nil)
			return
		}
		currentBan.AdminID = &adminID
	}
	if steamIDInt.Valid {
		currentBan.SteamID = strconv.FormatInt(steamIDInt.Int64, 10)
	}
	if eosIDStr.Valid {
		currentBan.EOSID = utils.NormalizeEOSID(eosIDStr.String)
	}
	if currentBan.SteamID != "" {
		currentBan.Name = currentBan.SteamID
	} else {
		currentBan.Name = currentBan.EOSID
	}

	// Set rule ID if present
	if ruleID.Valid {
		currentBan.RuleID = &ruleID.String
	}

	// Set rule name if present
	if ruleTitle.Valid {
		currentBan.RuleName = &ruleTitle.String
	}

	// Set ban list information if present
	if banListID.Valid {
		currentBan.BanListID = &banListID.String
	}
	if banListName.Valid {
		currentBan.BanListName = &banListName.String
	}

	// Set evidence text if present
	if evidenceText.Valid {
		currentBan.EvidenceText = &evidenceText.String
	}

	// Set expires_at and compute permanent flag
	if updateExpiresAt.Valid {
		currentBan.ExpiresAt = &updateExpiresAt.Time
	}
	currentBan.Permanent = currentBan.ExpiresAt == nil

	// Build update query dynamically based on provided fields
	updateFields := []string{}
	updateArgs := []interface{}{}
	argIndex := 1

	if request.Reason != nil && *request.Reason != "" {
		updateFields = append(updateFields, fmt.Sprintf("reason = $%d", argIndex))
		updateArgs = append(updateArgs, *request.Reason)
		argIndex++
	}

	if request.Duration != nil {
		newExpiresAt, parseErr := utils.ParseBanDuration(*request.Duration)
		if parseErr != nil {
			responses.BadRequest(c, "Invalid duration format", &gin.H{"error": parseErr.Error()})
			return
		}
		updateFields = append(updateFields, fmt.Sprintf("expires_at = $%d", argIndex))
		updateArgs = append(updateArgs, newExpiresAt)
		argIndex++
	}

	if request.BanListID != nil {
		if *request.BanListID == "" {
			// Remove from ban list
			updateFields = append(updateFields, fmt.Sprintf("ban_list_id = $%d", argIndex))
			updateArgs = append(updateArgs, nil)
			argIndex++
		} else {
			// Add to ban list
			banListUUID, err := uuid.Parse(*request.BanListID)
			if err != nil {
				responses.BadRequest(c, "Invalid ban list ID format", &gin.H{"error": err.Error()})
				return
			}
			updateFields = append(updateFields, fmt.Sprintf("ban_list_id = $%d", argIndex))
			updateArgs = append(updateArgs, banListUUID)
			argIndex++
		}
	}

	if request.RuleID != nil {
		updateFields = append(updateFields, fmt.Sprintf("rule_id = $%d", argIndex))
		updateArgs = append(updateArgs, *request.RuleID)
		argIndex++
	}

	if request.EvidenceText != nil {
		updateFields = append(updateFields, fmt.Sprintf("evidence_text = $%d", argIndex))
		updateArgs = append(updateArgs, *request.EvidenceText)
		argIndex++
	}

	// Check if evidence is being updated (separate from ban fields)
	// Evidence can be nil (not updating), empty array (clearing evidence), or have items (updating evidence)
	hasEvidenceUpdate := request.Evidence != nil

	// If no fields to update and no evidence update, return error
	if len(updateFields) == 0 && !hasEvidenceUpdate {
		responses.BadRequest(c, "No fields to update", &gin.H{"error": "At least one field must be provided for update"})
		return
	}

	// Add updated_at timestamp
	now := time.Now()
	updateFields = append(updateFields, fmt.Sprintf("updated_at = $%d", argIndex))
	updateArgs = append(updateArgs, now)
	argIndex++

	// Build the final query
	query := fmt.Sprintf("UPDATE server_bans SET %s WHERE id = $%d AND server_id = $%d",
		strings.Join(updateFields, ", "), argIndex, argIndex+1)
	updateArgs = append(updateArgs, banId, serverId)

	filesToDelete := []string(nil)
	if request.Evidence != nil {
		existingFiles, err := s.getExistingEvidenceFiles(c.Request.Context(), banId.String())
		if err != nil {
			log.Error().Err(err).Str("banId", banId.String()).Msg("Failed to query existing evidence files")
			responses.BadRequest(c, "Failed to update evidence", &gin.H{"error": "Failed to query existing evidence"})
			return
		}

		newFilePaths := make(map[string]bool)
		for _, ev := range *request.Evidence {
			if ev.EvidenceType == "file_upload" && ev.FilePath != nil && *ev.FilePath != "" {
				newFilePaths[*ev.FilePath] = true
			}
		}

		for _, existingFile := range existingFiles {
			if !newFilePaths[existingFile] {
				filesToDelete = append(filesToDelete, existingFile)
			}
		}
	}

	tx, err := s.Dependencies.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(c.Request.Context(), query, updateArgs...)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	if rowsAffected == 0 {
		responses.BadRequest(c, "Ban not found", &gin.H{"error": "Ban not found"})
		return
	}

	if request.Evidence != nil {
		if _, err := tx.ExecContext(c.Request.Context(), `
			DELETE FROM ban_evidence WHERE ban_id = $1
		`, banId); err != nil {
			log.Error().Err(err).Str("banId", banId.String()).Msg("Failed to delete old ban evidence")
			responses.BadRequest(c, "Failed to update evidence", &gin.H{"error": "Failed to delete existing evidence"})
			return
		}

		if len(*request.Evidence) > 0 {
			if err := s.createBanEvidenceWithTx(c.Request.Context(), tx, banId.String(), serverId, *request.Evidence); err != nil {
				log.Error().Err(err).Str("banId", banId.String()).Msg("Failed to create updated ban evidence")
				responses.BadRequest(c, "Failed to update evidence", &gin.H{"error": err.Error()})
				return
			}
		}
	}

	if err := s.syncBansCfgWithExecutor(c.Request.Context(), tx, server); err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to sync Bans.cfg after ban update: %w", err), nil)
		return
	}

	if err := tx.Commit(); err != nil {
		if restoreErr := s.syncBansCfg(c.Request.Context(), server); restoreErr != nil {
			log.Warn().Err(restoreErr).Str("banId", banId.String()).Str("serverId", serverId.String()).Msg("Failed to restore Bans.cfg after ban update commit error")
		}
		responses.InternalServerError(c, fmt.Errorf("failed to commit ban update after syncing Bans.cfg: %w", err), nil)
		return
	}

	if len(filesToDelete) > 0 {
		if err := s.deleteSpecificEvidenceFiles(c.Request.Context(), banId.String(), filesToDelete); err != nil {
			log.Warn().Err(err).Str("banId", banId.String()).Msg("Failed to delete some evidence files from storage after ban update")
		}
	}

	// Get updated ban details for response and RCON
	var updatedBan models.ServerBan
	var updatedAdminIDStr sql.NullString
	var updatedSteamIDInt sql.NullInt64
	var updatedEOSIDStr sql.NullString
	var updatedRuleID sql.NullString
	var updatedRuleTitle sql.NullString
	var updatedBanListID sql.NullString
	var updatedBanListName sql.NullString
	var updatedEvidenceText sql.NullString

	var updatedBanExpiresAt sql.NullTime
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT sb.id, sb.server_id, sb.admin_id, COALESCE(u.username, 'System') as admin_name, sb.steam_id, sb.eos_id, sb.reason, sb.expires_at, sb.rule_id, sr.title as rule_title,  sb.ban_list_id, bl.name as ban_list_name, sb.evidence_text, sb.created_at, sb.updated_at
		FROM server_bans sb
		LEFT JOIN users u ON sb.admin_id = u.id
		LEFT JOIN ban_lists bl ON sb.ban_list_id = bl.id
		LEFT JOIN server_rules sr ON sb.rule_id = sr.id
		WHERE sb.id = $1
	`, banId).Scan(
		&updatedBan.ID,
		&updatedBan.ServerID,
		&updatedAdminIDStr,
		&updatedBan.AdminName,
		&updatedSteamIDInt,
		&updatedEOSIDStr,
		&updatedBan.Reason,
		&updatedBanExpiresAt,
		&updatedRuleID,
		&updatedRuleTitle,
		&updatedBanListID,
		&updatedBanListName,
		&updatedEvidenceText,
		&updatedBan.CreatedAt,
		&updatedBan.UpdatedAt,
	)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	if updatedAdminIDStr.Valid {
		adminID, parseErr := uuid.Parse(updatedAdminIDStr.String)
		if parseErr != nil {
			responses.InternalServerError(c, parseErr, nil)
			return
		}
		updatedBan.AdminID = &adminID
	}
	if updatedSteamIDInt.Valid {
		updatedBan.SteamID = strconv.FormatInt(updatedSteamIDInt.Int64, 10)
	}
	if updatedEOSIDStr.Valid {
		updatedBan.EOSID = utils.NormalizeEOSID(updatedEOSIDStr.String)
	}
	if updatedBan.SteamID != "" {
		updatedBan.Name = updatedBan.SteamID
	} else {
		updatedBan.Name = updatedBan.EOSID
	}

	// Set rule ID if present
	if updatedRuleID.Valid {
		updatedBan.RuleID = &updatedRuleID.String
	}

	// Set rule name if present
	if updatedRuleTitle.Valid {
		updatedBan.RuleName = &updatedRuleTitle.String
	}

	// Set ban list information if present
	if updatedBanListID.Valid {
		updatedBan.BanListID = &updatedBanListID.String
	}
	if updatedBanListName.Valid {
		updatedBan.BanListName = &updatedBanListName.String
	}

	// Set evidence text if present
	if updatedEvidenceText.Valid {
		updatedBan.EvidenceText = &updatedEvidenceText.String
	}

	// Load evidence records
	updatedBan.Evidence, _ = s.loadBanEvidence(c.Request.Context(), updatedBan.ID)

	// Set expires_at and compute permanent flag
	if updatedBanExpiresAt.Valid {
		updatedBan.ExpiresAt = &updatedBanExpiresAt.Time
	}
	updatedBan.Permanent = updatedBan.ExpiresAt == nil

	// Create detailed audit log
	auditData := map[string]interface{}{
		"banId":        banId.String(),
		"steamId":      updatedBan.SteamID,
		"eosId":        updatedBan.EOSID,
		"oldReason":    currentBan.Reason,
		"newReason":    updatedBan.Reason,
		"oldExpiresAt": currentBan.ExpiresAt,
		"newExpiresAt": updatedBan.ExpiresAt,
		"oldBanListId": currentBan.BanListID,
		"newBanListId": updatedBan.BanListID,
		"oldRuleId":    currentBan.RuleID,
		"newRuleId":    updatedBan.RuleID,
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:ban:update", auditData)

	// Reload server config so the game server picks up the updated Bans.cfg.
	r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, server.Id)
	if _, err := r.ExecuteRaw("AdminReloadServerConfig"); err != nil {
		log.Warn().Err(err).Str("serverId", serverId.String()).Msg("Failed to reload server config after ban update")
	}

	responses.Success(c, "Ban updated successfully", &gin.H{
		"ban": updatedBan,
	})
}

// ServerBansCfg handles generating the ban config file for the server
func (s *Server) ServerBansCfg(c *gin.Context) {
	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	content, err := s.buildServerBansCfg(c.Request.Context(), serverId)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	// Set the content type and send the response
	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, content)
}

func (s *Server) buildServerBansCfg(ctx context.Context, serverId uuid.UUID) (string, error) {
	bans, err := s.getEffectiveServerBans(ctx, serverId)
	if err != nil {
		return "", err
	}
	return buildServerBansCfgContent(bans), nil
}

func (s *Server) getEffectiveServerBans(ctx context.Context, serverID uuid.UUID) ([]models.ServerBan, error) {
	return s.getEffectiveServerBansWithExecutor(ctx, s.Dependencies.DB, serverID)
}

func (s *Server) getEffectiveServerBansWithExecutor(ctx context.Context, executor dbpkg.Executor, serverID uuid.UUID) ([]models.ServerBan, error) {
	return core.GetServerBans(ctx, executor, serverID)
}

func collectServerBanIDs(bans []models.ServerBan) (steamIDs map[string]bool, eosIDs map[string]bool) {
	steamIDs = make(map[string]bool)
	eosIDs = make(map[string]bool)
	for _, ban := range bans {
		if ban.SteamID != "" {
			steamIDs[ban.SteamID] = true
		}
		if ban.EOSID != "" {
			eosIDs[utils.NormalizeEOSID(ban.EOSID)] = true
		}
	}
	return steamIDs, eosIDs
}

func filterServerBansByExcludedIDs(bans []models.ServerBan, excludedSteamIDs, excludedEOSIDs map[string]bool) []models.ServerBan {
	if len(excludedSteamIDs) == 0 && len(excludedEOSIDs) == 0 {
		return bans
	}

	filtered := make([]models.ServerBan, 0, len(bans))
	for _, ban := range bans {
		if ban.SteamID != "" && excludedSteamIDs[ban.SteamID] {
			continue
		}
		if ban.EOSID != "" && excludedEOSIDs[utils.NormalizeEOSID(ban.EOSID)] {
			continue
		}
		filtered = append(filtered, ban)
	}

	return filtered
}

func shouldPreserveExistingBansCfgEntry(entry models.CfgBanEntry, managedSteamIDs, managedEOSIDs, excludedSteamIDs, excludedEOSIDs map[string]bool) bool {
	if entry.Expired {
		return false
	}

	if entry.SteamID != "" && excludedSteamIDs[entry.SteamID] {
		return false
	}
	if entry.EOSID != "" && excludedEOSIDs[utils.NormalizeEOSID(entry.EOSID)] {
		return false
	}

	// Check if this entry is managed by Aegis (exists in DB). Managed entries are
	// already written from the DB query, so we skip them here to avoid duplicates.
	if entry.SteamID != "" {
		if managedSteamIDs[entry.SteamID] {
			return false
		}
	}

	if entry.EOSID != "" {
		normalizedEOSID := utils.NormalizeEOSID(entry.EOSID)
		if managedEOSIDs[normalizedEOSID] {
			return false
		}
	}

	return true
}

func buildServerBansCfgContent(bans []models.ServerBan) string {
	var banCfg strings.Builder
	for _, ban := range bans {
		bannedID := ban.SteamID
		if bannedID == "" {
			bannedID = utils.NormalizeEOSID(ban.EOSID)
		}
		if bannedID == "" {
			continue
		}

		adminInfo := ban.AdminName
		if adminInfo == "" {
			adminInfo = "System"
		}

		adminSteamID := ban.AdminSteamID
		if adminSteamID == "" {
			adminSteamID = "0"
		}

		expiryTimestamp := "0"
		if ban.ExpiresAt != nil {
			expiryTimestamp = strconv.FormatInt(ban.ExpiresAt.Unix(), 10)
		}

		reasonComment := ""
		if ban.Reason != "" {
			reasonComment = " //" + utils.SanitizeBanReason(ban.Reason)
		} else if ban.ExpiresAt == nil {
			reasonComment = " //Permanent ban"
		}

		banCfg.WriteString(fmt.Sprintf("%s [SteamID %s] Banned:%s:%s%s\n",
			adminInfo, adminSteamID, bannedID, expiryTimestamp, reasonComment))
	}

	return banCfg.String()
}

func buildMergedServerBansCfgContent(managedBans []models.ServerBan, existingContent string, existingContentErr error, excludedSteamIDs, excludedEOSIDs map[string]bool) (string, error) {
	content := buildServerBansCfgContent(managedBans)
	if existingContentErr != nil {
		return "", fmt.Errorf("failed to read existing Bans.cfg before sync: %w", existingContentErr)
	}
	if existingContent == "" {
		return content, nil
	}

	managedSteamIDs, managedEOSIDs := collectServerBanIDs(managedBans)
	entries, unparseableCount, parseErr := parseBansCfg(existingContent)
	if parseErr != nil {
		return "", fmt.Errorf("existing Bans.cfg too large to parse safely — aborting sync to avoid dropping bans: %w", parseErr)
	}
	if unparseableCount > 0 {
		return "", fmt.Errorf("existing Bans.cfg contains %d unparseable active lines — aborting sync to avoid dropping bans", unparseableCount)
	}

	var merged strings.Builder
	merged.WriteString(content)
	for _, entry := range entries {
		if shouldPreserveExistingBansCfgEntry(entry, managedSteamIDs, managedEOSIDs, excludedSteamIDs, excludedEOSIDs) {
			merged.WriteString(entry.RawLine)
			merged.WriteString("\n")
		}
	}

	return merged.String(), nil
}

// loadBanEvidence loads all evidence records for a given ban
func (s *Server) loadBanEvidence(ctx context.Context, banID string) ([]models.BanEvidence, error) {
	rows, err := s.Dependencies.DB.QueryContext(ctx, `
		SELECT id, ban_id, evidence_type, clickhouse_table, record_id, server_id, event_time, metadata, file_path, file_name, file_size, file_type, text_content, created_at, updated_at
		FROM ban_evidence
		WHERE ban_id = $1
		ORDER BY COALESCE(event_time, created_at) DESC
	`, banID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var evidence []models.BanEvidence
	for rows.Next() {
		var ev models.BanEvidence
		var metadataJSON []byte
		var clickhouseTable, recordID sql.NullString
		var eventTime sql.NullTime
		var filePath, fileName, fileType, textContent sql.NullString
		var fileSize sql.NullInt64

		err := rows.Scan(
			&ev.ID,
			&ev.BanID,
			&ev.EvidenceType,
			&clickhouseTable,
			&recordID,
			&ev.ServerID,
			&eventTime,
			&metadataJSON,
			&filePath,
			&fileName,
			&fileSize,
			&fileType,
			&textContent,
			&ev.CreatedAt,
			&ev.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Set nullable fields
		if clickhouseTable.Valid {
			ev.ClickhouseTable = &clickhouseTable.String
		}
		if recordID.Valid {
			ev.RecordID = &recordID.String
		}
		if eventTime.Valid {
			ev.EventTime = &eventTime.Time
		}
		if filePath.Valid {
			ev.FilePath = &filePath.String
		}
		if fileName.Valid {
			ev.FileName = &fileName.String
		}
		if fileSize.Valid {
			ev.FileSize = &fileSize.Int64
		}
		if fileType.Valid {
			ev.FileType = &fileType.String
		}
		if textContent.Valid {
			ev.TextContent = &textContent.String
		}

		// Fetch metadata from ClickHouse if not already stored (only for ClickHouse events)
		if ev.EvidenceType != "file_upload" && ev.EvidenceType != "text_paste" {
			if len(metadataJSON) == 0 || string(metadataJSON) == "{}" {
				if ev.ClickhouseTable != nil && ev.RecordID != nil {
					recordUUID, parseErr := uuid.Parse(*ev.RecordID)
					if parseErr != nil {
						log.Warn().Err(parseErr).
							Str("table", *ev.ClickhouseTable).
							Str("record_id", *ev.RecordID).
							Msg("Failed to parse record ID as UUID")
						ev.Metadata = make(map[string]interface{})
					} else {
						metadata, err := s.fetchEvidenceMetadataFromClickHouse(ctx, *ev.ClickhouseTable, recordUUID, ev.ServerID, ev.EvidenceType)
						if err != nil {
							log.Warn().Err(err).
								Str("table", *ev.ClickhouseTable).
								Str("record_id", *ev.RecordID).
								Msg("Failed to fetch evidence metadata from ClickHouse")
							// Continue with empty metadata if fetch fails
							ev.Metadata = make(map[string]interface{})
						} else {
							ev.Metadata = metadata
						}
					}
				} else {
					ev.Metadata = make(map[string]interface{})
				}
			} else {
				// Parse existing metadata JSON
				var metadata map[string]interface{}
				if err := json.Unmarshal(metadataJSON, &metadata); err == nil {
					ev.Metadata = metadata
				} else {
					ev.Metadata = make(map[string]interface{})
				}
			}
		} else {
			// For file/text evidence, parse metadata if present
			if len(metadataJSON) > 0 && string(metadataJSON) != "{}" {
				var metadata map[string]interface{}
				if err := json.Unmarshal(metadataJSON, &metadata); err == nil {
					ev.Metadata = metadata
				} else {
					ev.Metadata = make(map[string]interface{})
				}
			} else {
				ev.Metadata = make(map[string]interface{})
			}
		}

		evidence = append(evidence, ev)
	}

	return evidence, nil
}

// lookupPlayerNamesBatch looks up player names from ClickHouse for a batch of steam IDs
func (s *Server) lookupPlayerNamesBatch(ctx context.Context, steamIDs []string) map[string]string {
	result := make(map[string]string)

	if len(steamIDs) == 0 || s.Dependencies.Clickhouse == nil {
		return result
	}

	query := `
		SELECT
			steam,
			argMax(player_suffix, event_time) as player_name
		FROM squad_aegis.server_join_succeeded_events
		WHERE steam IN (?)
		GROUP BY steam
	`

	rows, err := s.Dependencies.Clickhouse.Query(ctx, query, steamIDs)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to lookup player names from ClickHouse")
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var steamID, playerName string
		if err := rows.Scan(&steamID, &playerName); err != nil {
			continue
		}
		if playerName != "" {
			result[steamID] = playerName
		}
	}

	return result
}

// fetchEvidenceMetadataFromClickHouse fetches the full event data from ClickHouse
func (s *Server) fetchEvidenceMetadataFromClickHouse(ctx context.Context, tableName string, recordID uuid.UUID, serverID uuid.UUID, evidenceType string) (map[string]interface{}, error) {
	if s.Dependencies.Clickhouse == nil {
		return nil, fmt.Errorf("ClickHouse client not available")
	}

	var query string
	var args []interface{}

	switch evidenceType {
	case "chat_message":
		// For chat messages, record_id is message_id (UUID)
		query = `
			SELECT
				message_id,
				sent_at as event_time,
				player_name,
				chat_type,
				message,
				steam_id,
				eos_id
			FROM squad_aegis.server_player_chat_messages
			WHERE server_id = ? AND message_id = ?
			LIMIT 1
		`
		args = []interface{}{serverID.String(), recordID.String()}

	case "player_connected":
		query = `
			SELECT
				id,
				event_time,
				player_controller,
				ip,
				steam,
				eos
			FROM squad_aegis.server_player_connected_events
			WHERE server_id = ? AND id = ?
			LIMIT 1
		`
		args = []interface{}{serverID.String(), recordID.String()}

	default:
		return nil, fmt.Errorf("unknown evidence type: %s", evidenceType)
	}

	rows, err := s.Dependencies.Clickhouse.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query ClickHouse: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("evidence record not found in ClickHouse")
	}

	metadata := make(map[string]interface{})

	switch evidenceType {
	case "player_died", "player_wounded":
		var id uuid.UUID
		var victimName, weapon, attackerController, attackerEos, attackerSteam string
		var eventTime string
		var teamkill uint8
		var damage float32

		err := rows.Scan(&id, &eventTime, &victimName, &weapon, &attackerController, &attackerEos, &attackerSteam, &teamkill, &damage)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		metadata["id"] = id.String()
		metadata["event_time"] = eventTime
		metadata["victim_name"] = victimName
		metadata["weapon"] = weapon
		metadata["attacker_player_controller"] = attackerController
		metadata["attacker_eos"] = attackerEos
		metadata["attacker_steam"] = attackerSteam
		metadata["teamkill"] = teamkill == 1
		metadata["damage"] = damage

	case "player_damaged":
		var id uuid.UUID
		var victimName, weapon, attackerController, attackerEos, attackerSteam string
		var eventTime string
		var damage float32

		err := rows.Scan(&id, &eventTime, &victimName, &weapon, &attackerController, &attackerEos, &attackerSteam, &damage)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		metadata["id"] = id.String()
		metadata["event_time"] = eventTime
		metadata["victim_name"] = victimName
		metadata["weapon"] = weapon
		metadata["attacker_controller"] = attackerController
		metadata["attacker_eos"] = attackerEos
		metadata["attacker_steam"] = attackerSteam
		metadata["damage"] = damage

	case "chat_message":
		var messageID uuid.UUID
		var playerName, chatType, message, eosID string
		var eventTime string
		var steamIDVal uint64

		err := rows.Scan(&messageID, &eventTime, &playerName, &chatType, &message, &steamIDVal, &eosID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		metadata["message_id"] = messageID.String()
		metadata["event_time"] = eventTime
		metadata["sent_at"] = eventTime
		metadata["player_name"] = playerName
		metadata["chat_type"] = chatType
		metadata["message"] = message
		metadata["steam_id"] = steamIDVal
		metadata["eos_id"] = eosID

	case "player_connected":
		var id uuid.UUID
		var playerController, ip, steam, eos string
		var eventTime string

		err := rows.Scan(&id, &eventTime, &playerController, &ip, &steam, &eos)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		metadata["id"] = id.String()
		metadata["event_time"] = eventTime
		metadata["player_controller"] = playerController
		metadata["ip"] = ip
		metadata["steam"] = steam
		metadata["eos"] = eos
	}

	return metadata, nil
}

// createBanEvidence creates evidence records for a ban
func (s *Server) createBanEvidence(ctx context.Context, banID string, serverID uuid.UUID, evidence []models.BanEvidenceCreateItem) error {
	if len(evidence) == 0 {
		return nil
	}

	// Start a transaction for batch insert
	tx, err := s.Dependencies.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO ban_evidence (id, ban_id, evidence_type, clickhouse_table, record_id, server_id, event_time, metadata, file_path, file_name, file_size, file_type, text_content, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	for _, ev := range evidence {
		// Convert metadata to JSON - always ensure we have valid JSON (empty object if nil)
		var metadataJSON []byte
		if len(ev.Metadata) > 0 {
			var marshalErr error
			metadataJSON, marshalErr = json.Marshal(ev.Metadata)
			if marshalErr != nil {
				log.Error().Err(marshalErr).Msg("Failed to marshal evidence metadata")
				metadataJSON = []byte("{}")
			}
		} else {
			// Ensure we always have valid JSON, even if metadata is nil or empty
			metadataJSON = []byte("{}")
		}

		// Handle event_time - use current time for file/text evidence if not provided
		var eventTime interface{}
		if ev.EventTime != nil {
			eventTime = *ev.EventTime
		} else if ev.EvidenceType == "file_upload" || ev.EvidenceType == "text_paste" {
			eventTime = now
		} else {
			eventTime = nil
		}

		_, err = stmt.ExecContext(ctx,
			uuid.New(),
			banID,
			ev.EvidenceType,
			ev.ClickhouseTable,
			ev.RecordID,
			serverID,
			eventTime,
			metadataJSON,
			ev.FilePath,
			ev.FileName,
			ev.FileSize,
			ev.FileType,
			ev.TextContent,
			now,
			now,
		)
		if err != nil {
			return fmt.Errorf("failed to insert evidence: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// getExistingEvidenceFiles returns a list of file paths for all file_upload evidence for a ban
func (s *Server) getExistingEvidenceFiles(ctx context.Context, banID string) ([]string, error) {
	rows, err := s.Dependencies.DB.QueryContext(ctx, `
		SELECT file_path
		FROM ban_evidence
		WHERE ban_id = $1 AND evidence_type = 'file_upload' AND file_path IS NOT NULL
	`, banID)
	if err != nil {
		return nil, fmt.Errorf("failed to query evidence files: %w", err)
	}
	defer rows.Close()

	var filePaths []string
	for rows.Next() {
		var filePath sql.NullString
		if err := rows.Scan(&filePath); err != nil {
			log.Warn().Err(err).Str("banId", banID).Msg("Failed to scan file path")
			continue
		}

		if filePath.Valid && filePath.String != "" {
			filePaths = append(filePaths, filePath.String)
		}
	}

	return filePaths, nil
}

// deleteSpecificEvidenceFiles deletes specific evidence files from storage
func (s *Server) deleteSpecificEvidenceFiles(ctx context.Context, banID string, filePaths []string) error {
	var deleteErrors []error
	for _, filePath := range filePaths {
		if filePath == "" {
			continue
		}

		// Delete the file from storage
		if err := s.Dependencies.Storage.Delete(ctx, filePath); err != nil {
			log.Warn().Err(err).
				Str("banId", banID).
				Str("filePath", filePath).
				Msg("Failed to delete evidence file from storage")
			deleteErrors = append(deleteErrors, fmt.Errorf("failed to delete file %s: %w", filePath, err))
			// Continue deleting other files even if one fails
		} else {
			log.Info().
				Str("banId", banID).
				Str("filePath", filePath).
				Msg("Deleted evidence file from storage")
		}
	}

	if len(deleteErrors) > 0 {
		return fmt.Errorf("failed to delete %d file(s): %v", len(deleteErrors), deleteErrors[0])
	}

	return nil
}

// deleteBanEvidenceFiles deletes all file evidence files from storage for a given ban
func (s *Server) deleteBanEvidenceFiles(ctx context.Context, banID string) error {
	filePaths, err := s.getBanEvidenceFilePaths(ctx, banID)
	if err != nil {
		return err
	}

	return s.deleteEvidenceFilesFromStorage(ctx, banID, filePaths)
}

func (s *Server) getBanEvidenceFilePaths(ctx context.Context, banID string) ([]string, error) {
	// Query for all file_upload evidence records for this ban
	rows, err := s.Dependencies.DB.QueryContext(ctx, `
		SELECT file_path
		FROM ban_evidence
		WHERE ban_id = $1 AND evidence_type = 'file_upload' AND file_path IS NOT NULL
	`, banID)
	if err != nil {
		return nil, fmt.Errorf("failed to query evidence files: %w", err)
	}
	defer rows.Close()

	var filePaths []string
	for rows.Next() {
		var filePath sql.NullString
		if err := rows.Scan(&filePath); err != nil {
			log.Warn().Err(err).Str("banId", banID).Msg("Failed to scan file path")
			continue
		}

		if !filePath.Valid || filePath.String == "" {
			continue
		}

		filePaths = append(filePaths, filePath.String)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate evidence files: %w", err)
	}

	return filePaths, nil
}

func (s *Server) deleteEvidenceFilesFromStorage(ctx context.Context, banID string, filePaths []string) error {
	var deleteErrors []error
	for _, filePath := range filePaths {
		if err := s.Dependencies.Storage.Delete(ctx, filePath); err != nil {
			log.Warn().Err(err).
				Str("banId", banID).
				Str("filePath", filePath).
				Msg("Failed to delete evidence file from storage")
			deleteErrors = append(deleteErrors, fmt.Errorf("failed to delete file %s: %w", filePath, err))
			// Continue deleting other files even if one fails
		} else {
			log.Info().
				Str("banId", banID).
				Str("filePath", filePath).
				Msg("Deleted evidence file from storage")
		}
	}

	if len(deleteErrors) > 0 {
		return fmt.Errorf("failed to delete %d file(s): %v", len(deleteErrors), deleteErrors[0])
	}

	return nil
}

// createBanEvidenceWithTx creates evidence records for a ban using an existing transaction
func (s *Server) createBanEvidenceWithTx(ctx context.Context, tx *sql.Tx, banID string, serverID uuid.UUID, evidence []models.BanEvidenceCreateItem) error {
	if len(evidence) == 0 {
		return nil
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO ban_evidence (id, ban_id, evidence_type, clickhouse_table, record_id, server_id, event_time, metadata, file_path, file_name, file_size, file_type, text_content, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	for _, ev := range evidence {
		// Convert metadata to JSON - always ensure we have valid JSON (empty object if nil)
		var metadataJSON []byte
		if len(ev.Metadata) > 0 {
			var marshalErr error
			metadataJSON, marshalErr = json.Marshal(ev.Metadata)
			if marshalErr != nil {
				log.Error().Err(marshalErr).Msg("Failed to marshal evidence metadata")
				metadataJSON = []byte("{}")
			}
		} else {
			// Ensure we always have valid JSON, even if metadata is nil or empty
			metadataJSON = []byte("{}")
		}

		// Handle event_time - use current time for file/text evidence if not provided
		var eventTime interface{}
		if ev.EventTime != nil {
			eventTime = *ev.EventTime
		} else if ev.EvidenceType == "file_upload" || ev.EvidenceType == "text_paste" {
			eventTime = now
		} else {
			eventTime = nil
		}

		_, err = stmt.ExecContext(ctx,
			uuid.New(),
			banID,
			ev.EvidenceType,
			ev.ClickhouseTable,
			ev.RecordID,
			serverID,
			eventTime,
			metadataJSON,
			ev.FilePath,
			ev.FileName,
			ev.FileSize,
			ev.FileType,
			ev.TextContent,
			now,
			now,
		)
		if err != nil {
			return fmt.Errorf("failed to insert evidence: %w", err)
		}
	}

	return nil
}
