package server

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/file_upload"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	"go.codycody31.dev/squad-aegis/internal/shared/utils"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
)

// permanentThresholdYears defines how far in the future an expiry timestamp
// must be to be treated as a permanent ban. Timestamps like 9999999999 (year 2286)
// are used by Squad servers to represent permanent bans.
const permanentThresholdYears = 50

// maxBansCfgLines is the maximum number of lines accepted when parsing a Bans.cfg file.
const maxBansCfgLines = 50000

// maxBansCfgReadBytes is the maximum size in bytes when reading a Bans.cfg file (10 MB).
const maxBansCfgReadBytes = 10 * 1024 * 1024

// errBansCfgTooLarge is returned when a Bans.cfg file exceeds the size or line
// count safety limits. Callers must treat this as a hard error and abort any
// sync/import operation to avoid silently dropping bans.
var errBansCfgTooLarge = fmt.Errorf("Bans.cfg exceeds safety limits (max %d bytes / %d lines)", maxBansCfgReadBytes, maxBansCfgLines)

// parseBansCfg parses the content of a Squad Bans.cfg file into structured entries.
// Returns the parsed entries, the count of lines that could not be parsed, and an
// error if the file exceeds safety limits.
//
// Supported ID formats:
//   - Steam ID (numeric, e.g., 76561198000000001)
//   - EOS ID (32-character hex, e.g., 0002adb8a89b4d1d970a3cd1e4569092)
//
// Expected line format:
//
//	AdminName [SteamID X] Banned:<id>:<expiryTimestamp> //<reason>
//	N/A Banned:<id>:<expiryTimestamp> //<reason>
//	<id>:<expiryTimestamp> //<reason>
func parseBansCfg(content string) ([]models.CfgBanEntry, int, error) {
	if len(content) > maxBansCfgReadBytes {
		return nil, 0, errBansCfgTooLarge
	}
	lines := strings.Split(content, "\n")
	if len(lines) > maxBansCfgLines {
		return nil, 0, errBansCfgTooLarge
	}
	var entries []models.CfgBanEntry
	entryIndexes := make(map[string]int)
	unparseable := 0
	now := time.Now()
	permanentThreshold := now.AddDate(permanentThresholdYears, 0, 0).Unix()

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Support both the full Squad format ("... Banned:<id>:<expiry>") and
		// the legacy exported format ("<id>:<expiry>").
		banSpec := line
		if _, afterBanned, found := strings.Cut(line, "Banned:"); found {
			banSpec = afterBanned
		}

		// Extract reason from "//" suffix before parsing id:expiry
		reason := ""
		if commentIdx := strings.Index(banSpec, "//"); commentIdx >= 0 {
			reason = strings.TrimSpace(banSpec[commentIdx+2:])
			banSpec = banSpec[:commentIdx]
		}

		banSpec = strings.TrimSpace(banSpec)

		// Split into id and expiryTimestamp
		parts := strings.SplitN(banSpec, ":", 2)
		if len(parts) != 2 {
			unparseable++
			continue
		}

		idStr := strings.TrimSpace(parts[0])
		expiryStr := strings.TrimSpace(parts[1])

		if idStr == "" {
			unparseable++
			continue
		}

		// Classify the identifier as Steam ID or EOS ID
		steamID := ""
		eosID := ""
		if len(idStr) == 32 && utils.IsHex(idStr) {
			eosID = utils.NormalizeEOSID(idStr)
		} else if _, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			steamID = idStr
		} else {
			unparseable++
			continue
		}

		// Parse expiry timestamp
		expiryTimestamp, err := strconv.ParseInt(expiryStr, 10, 64)
		if err != nil {
			unparseable++
			continue
		}

		// Deduplicate by identifier (newest occurrence wins). Bans.cfg can
		// contain multiple lines for the same player as bans are updated over
		// time, and we need the latest line to drive import/sync decisions.
		dedupKey := steamID
		if dedupKey == "" {
			dedupKey = "eos:" + eosID
		}

		permanent := expiryTimestamp == 0 || expiryTimestamp >= permanentThreshold
		expired := !permanent && now.After(time.Unix(expiryTimestamp, 0))
		isAutoBan := strings.HasPrefix(reason, "Automatic ")

		entry := models.CfgBanEntry{
			SteamID:         steamID,
			EOSID:           eosID,
			ExpiryTimestamp: expiryTimestamp,
			Reason:          reason,
			Permanent:       permanent,
			Expired:         expired,
			IsAutoBan:       isAutoBan,
			RawLine:         line,
		}

		if idx, exists := entryIndexes[dedupKey]; exists {
			entries[idx] = entry
			continue
		}

		entryIndexes[dedupKey] = len(entries)
		entries = append(entries, entry)
	}

	return entries, unparseable, nil
}

// readBansCfg reads the Bans.cfg file from the game server.
func (s *Server) readBansCfg(ctx context.Context, server *models.Server) (string, error) {
	if server.SquadGamePath == nil || *server.SquadGamePath == "" {
		return "", fmt.Errorf("SquadGame base path is not configured")
	}
	if server.LogSourceType == nil || *server.LogSourceType == "" {
		return "", fmt.Errorf("log source type is not configured")
	}

	bansCfgPath := buildBansCfgPath(*server.SquadGamePath, server.LogSourceType)

	switch *server.LogSourceType {
	case "sftp", "ftp":
		if server.LogHost == nil || server.LogUsername == nil || server.LogPassword == nil {
			return "", fmt.Errorf("missing SFTP/FTP credentials")
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
			return "", fmt.Errorf("failed to connect to server: %w", err)
		}
		defer uploader.Close()

		content, err := uploader.Read(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to read Bans.cfg: %w", err)
		}
		return content, nil

	case "local":
		f, err := os.Open(bansCfgPath)
		if err != nil {
			return "", fmt.Errorf("failed to open local Bans.cfg: %w", err)
		}
		defer f.Close()
		limited := io.LimitReader(f, int64(maxBansCfgReadBytes+1))
		data, err := io.ReadAll(limited)
		if err != nil {
			return "", fmt.Errorf("failed to read local Bans.cfg: %w", err)
		}
		if len(data) > maxBansCfgReadBytes {
			return "", errBansCfgTooLarge
		}
		return string(data), nil

	default:
		return "", fmt.Errorf("unsupported log source type: %s", *server.LogSourceType)
	}
}

// getExistingBanIDs returns sets of active Steam IDs and EOS IDs already enforced
// for this server, including subscribed ban-list entries.
func (s *Server) getExistingBanIDs(ctx context.Context, serverID uuid.UUID) (steamIDs map[string]bool, eosIDs map[string]bool, err error) {
	bans, err := s.getEffectiveServerBans(ctx, serverID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query existing bans: %w", err)
	}
	steamIDs, eosIDs = collectServerBanIDs(bans)
	return steamIDs, eosIDs, nil
}

// categorizeBans splits parsed Bans.cfg entries into new, existing, expired, and auto-ban categories.
// Auto-bans are always filtered out regardless of other criteria.
func categorizeBans(entries []models.CfgBanEntry, existingSteamIDs, existingEOSIDs map[string]bool) (newBans, existingBans, expiredBans, autoBans []models.CfgBanEntry) {
	for _, entry := range entries {
		if entry.IsAutoBan {
			autoBans = append(autoBans, entry)
		} else if entry.Expired {
			expiredBans = append(expiredBans, entry)
		} else if (entry.SteamID != "" && existingSteamIDs[entry.SteamID]) ||
			(entry.EOSID != "" && existingEOSIDs[entry.EOSID]) {
			existingBans = append(existingBans, entry)
		} else {
			newBans = append(newBans, entry)
		}
	}
	return
}

func validateBansCfgImport(unparseableCount int) error {
	if unparseableCount > 0 {
		return fmt.Errorf("Bans.cfg contains %d unparseable active lines; fix or remove them before importing", unparseableCount)
	}
	return nil
}

// ServerBanImportPreview returns a preview of what would be imported from the server's Bans.cfg.
func (s *Server) ServerBanImportPreview(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIDStr := c.Param("serverId")
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverID, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Check if file access is configured
	cfgAvailable := server.SquadGamePath != nil && *server.SquadGamePath != "" &&
		server.LogSourceType != nil && *server.LogSourceType != ""

	if !cfgAvailable {
		responses.Success(c, "Import preview generated", &gin.H{
			"preview": models.BanImportPreview{
				CfgAvailable: false,
			},
		})
		return
	}

	cfgPath := buildBansCfgPath(*server.SquadGamePath, server.LogSourceType)

	content, err := s.readBansCfg(c.Request.Context(), server)
	if err != nil {
		responses.BadRequest(c, "Failed to read Bans.cfg", &gin.H{"error": err.Error()})
		return
	}

	entries, unparseableCount, parseErr := parseBansCfg(content)
	if parseErr != nil {
		responses.BadRequest(c, "Bans.cfg is too large to parse safely", &gin.H{"error": parseErr.Error()})
		return
	}

	existingSteamIDs, existingEOSIDs, err := s.getExistingBanIDs(c.Request.Context(), serverID)
	if err != nil {
		responses.BadRequest(c, "Failed to query existing bans", &gin.H{"error": err.Error()})
		return
	}

	newBans, existingBans, expiredBans, autoBans := categorizeBans(entries, existingSteamIDs, existingEOSIDs)

	responses.Success(c, "Import preview generated", &gin.H{
		"preview": models.BanImportPreview{
			CfgAvailable:     true,
			CfgPath:          cfgPath,
			NewBans:          newBans,
			ExistingBans:     existingBans,
			ExpiredBans:      expiredBans,
			AutoBans:         autoBans,
			UnparseableCount: unparseableCount,
		},
	})
}

// ServerBanImportExecute imports bans from the server's Bans.cfg into the Aegis database.
func (s *Server) ServerBanImportExecute(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIDStr := c.Param("serverId")
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	var request models.BanImportRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	if !request.Confirm {
		responses.BadRequest(c, "Import must be confirmed", &gin.H{"error": "Set confirm to true to proceed"})
		return
	}

	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverID, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	content, err := s.readBansCfg(c.Request.Context(), server)
	if err != nil {
		responses.BadRequest(c, "Failed to read Bans.cfg", &gin.H{"error": err.Error()})
		return
	}

	entries, unparseableCount, parseErr := parseBansCfg(content)
	if parseErr != nil {
		responses.BadRequest(c, "Bans.cfg is too large to parse safely — import aborted to avoid dropping bans", &gin.H{"error": parseErr.Error()})
		return
	}
	if err := validateBansCfgImport(unparseableCount); err != nil {
		responses.BadRequest(c, "Bans.cfg contains unparseable active lines — import aborted to avoid dropping bans", &gin.H{"error": err.Error()})
		return
	}

	existingSteamIDs, existingEOSIDs, err := s.getExistingBanIDs(c.Request.Context(), serverID)
	if err != nil {
		responses.BadRequest(c, "Failed to query existing bans", &gin.H{"error": err.Error()})
		return
	}

	newBans, existingBans, expiredBans, autoBans := categorizeBans(entries, existingSteamIDs, existingEOSIDs)

	result := models.BanImportResult{
		SkippedCount: len(existingBans),
		ExpiredCount: len(expiredBans) + len(autoBans),
	}
	importedSteamIDs := map[string]bool(nil)
	importedEOSIDs := map[string]bool(nil)

	if len(newBans) == 0 {
		responses.Success(c, "No new bans to import", &gin.H{"result": result})
		return
	}

	// Import in a transaction
	ctx := c.Request.Context()
	tx, err := s.Dependencies.DB.BeginTx(ctx, nil)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	now := time.Now()
	for _, ban := range newBans {
		// Check for context cancellation (e.g. client disconnect, request timeout)
		// before each insert so we abort early rather than accumulating spurious
		// insert errors and attempting a commit that is guaranteed to fail.
		if ctxErr := ctx.Err(); ctxErr != nil {
			log.Warn().
				Err(ctxErr).
				Str("serverId", serverID.String()).
				Int("importedSoFar", result.ImportedCount).
				Msg("Context cancelled during ban import; rolling back transaction")
			responses.InternalServerError(c, ctxErr, &gin.H{"error": "Import aborted: request was cancelled or timed out"})
			return
		}

		// Compute expires_at directly from the parsed expiry timestamp.
		// No rounding needed — the exact original expiry is preserved.
		var expiresAt *time.Time
		if !ban.Permanent {
			t := time.Unix(ban.ExpiryTimestamp, 0)
			expiresAt = &t
		}

		reason := ban.Reason
		if reason == "" {
			reason = "Imported from Bans.cfg"
		}

		// Prepare nullable ID pointers
		var steamIDPtr *int64
		var eosIDPtr *string

		if ban.SteamID != "" {
			v, err := strconv.ParseInt(ban.SteamID, 10, 64)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("invalid Steam ID %s: %v", ban.SteamID, err))
				continue
			}
			steamIDPtr = &v
		}
		if ban.EOSID != "" {
			normalizedEOSID := utils.NormalizeEOSID(ban.EOSID)
			eosIDPtr = &normalizedEOSID
		}

		playerLabel := ban.SteamID
		if playerLabel == "" {
			playerLabel = ban.EOSID
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO server_bans (id, server_id, admin_id, steam_id, eos_id, reason, expires_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, uuid.New(), serverID, user.Id, steamIDPtr, eosIDPtr, reason, expiresAt, now, now)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to insert ban for %s: %v", playerLabel, err))
			continue
		}

		if ban.SteamID != "" {
			if importedSteamIDs == nil {
				importedSteamIDs = make(map[string]bool)
			}
			importedSteamIDs[ban.SteamID] = true
		}
		if ban.EOSID != "" {
			if importedEOSIDs == nil {
				importedEOSIDs = make(map[string]bool)
			}
			importedEOSIDs[utils.NormalizeEOSID(ban.EOSID)] = true
		}

		result.ImportedCount++
	}

	// Guard against a cancellation that occurred after the final insert but
	// before the commit; committing with a cancelled context would fail with a
	// confusing generic error.
	if ctxErr := ctx.Err(); ctxErr != nil {
		log.Warn().
			Err(ctxErr).
			Str("serverId", serverID.String()).
			Int("importedSoFar", result.ImportedCount).
			Msg("Context cancelled before ban import commit; rolling back transaction")
		responses.InternalServerError(c, ctxErr, &gin.H{"error": "Import aborted: request was cancelled or timed out"})
		return
	}

	if err := s.syncBansCfgWithExecutor(c.Request.Context(), tx, server); err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to sync Bans.cfg after import: %w", err), nil)
		return
	}

	if err := tx.Commit(); err != nil {
		if restoreErr := s.syncBansCfgWithExcludedIDs(c.Request.Context(), server, importedSteamIDs, importedEOSIDs); restoreErr != nil {
			log.Warn().Err(restoreErr).Str("serverId", serverID.String()).Msg("Failed to restore Bans.cfg after import commit error")
		}
		responses.InternalServerError(c, fmt.Errorf("failed to commit import after syncing Bans.cfg: %w", err), &gin.H{"error": "Failed to commit import"})
		return
	}

	// Reload server config so the game server picks up the updated Bans.cfg
	r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, server.Id)
	if _, err := r.ExecuteRaw("AdminReloadServerConfig"); err != nil {
		log.Warn().Err(err).Str("serverId", serverID.String()).Msg("Failed to reload server config after ban import")
	}

	// Audit log
	auditData := map[string]any{
		"importedCount": result.ImportedCount,
		"skippedCount":  result.SkippedCount,
		"expiredCount":  result.ExpiredCount,
		"errors":        result.Errors,
	}
	s.CreateAuditLog(c.Request.Context(), &serverID, &user.Id, "server:ban:import", auditData)

	responses.Success(c, "Bans imported successfully", &gin.H{"result": result})
}
