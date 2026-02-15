package server

import (
	"context"
	"fmt"
	"math"
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
)

// parseBansCfg parses the content of a Squad Bans.cfg file into structured entries.
// Returns the parsed entries and the count of lines that could not be parsed.
//
// Expected format:
//
//	AdminName [SteamID X] Banned:<steamId>:<expiryTimestamp> //<reason>
func parseBansCfg(content string) ([]models.CfgBanEntry, int) {
	lines := strings.Split(content, "\n")
	var entries []models.CfgBanEntry
	seen := make(map[string]bool)
	unparseable := 0
	now := time.Now()

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Find "Banned:" in the line
		_, afterBanned, found := strings.Cut(line, "Banned:")
		if !found {
			unparseable++
			continue
		}

		// Extract reason from "//" suffix before parsing steamId:expiry
		reason := ""
		if commentIdx := strings.Index(afterBanned, "//"); commentIdx >= 0 {
			reason = strings.TrimSpace(afterBanned[commentIdx+2:])
			afterBanned = afterBanned[:commentIdx]
		}

		afterBanned = strings.TrimSpace(afterBanned)

		// Split into steamId and expiryTimestamp
		parts := strings.SplitN(afterBanned, ":", 2)
		if len(parts) != 2 {
			unparseable++
			continue
		}

		steamID := strings.TrimSpace(parts[0])
		expiryStr := strings.TrimSpace(parts[1])

		if steamID == "" {
			unparseable++
			continue
		}

		// Validate steamID is numeric
		if _, err := strconv.ParseInt(steamID, 10, 64); err != nil {
			unparseable++
			continue
		}

		// Parse expiry timestamp
		expiryTimestamp, err := strconv.ParseInt(expiryStr, 10, 64)
		if err != nil {
			unparseable++
			continue
		}

		// Deduplicate by SteamID (first occurrence wins)
		if seen[steamID] {
			continue
		}
		seen[steamID] = true

		permanent := expiryTimestamp == 0
		expired := !permanent && now.After(time.Unix(expiryTimestamp, 0))

		entries = append(entries, models.CfgBanEntry{
			SteamID:         steamID,
			ExpiryTimestamp: expiryTimestamp,
			Reason:          reason,
			Permanent:       permanent,
			Expired:         expired,
			RawLine:         line,
		})
	}

	return entries, unparseable
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
		data, err := os.ReadFile(bansCfgPath)
		if err != nil {
			return "", fmt.Errorf("failed to read local Bans.cfg: %w", err)
		}
		return string(data), nil

	default:
		return "", fmt.Errorf("unsupported log source type: %s", *server.LogSourceType)
	}
}

// getExistingBanSteamIDs returns a set of SteamIDs that already have active bans for this server.
func (s *Server) getExistingBanSteamIDs(ctx context.Context, serverID uuid.UUID) (map[string]bool, error) {
	rows, err := s.Dependencies.DB.QueryContext(ctx, `
		SELECT steam_id FROM server_bans WHERE server_id = $1
	`, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing bans: %w", err)
	}
	defer rows.Close()

	existing := make(map[string]bool)
	for rows.Next() {
		var steamIDInt int64
		if err := rows.Scan(&steamIDInt); err != nil {
			return nil, fmt.Errorf("failed to scan steam ID: %w", err)
		}
		existing[strconv.FormatInt(steamIDInt, 10)] = true
	}
	return existing, nil
}

// categorizeBans splits parsed Bans.cfg entries into new, existing, and expired categories.
func categorizeBans(entries []models.CfgBanEntry, existingSteamIDs map[string]bool) (newBans, existingBans, expiredBans []models.CfgBanEntry) {
	for _, entry := range entries {
		if entry.Expired {
			expiredBans = append(expiredBans, entry)
		} else if existingSteamIDs[entry.SteamID] {
			existingBans = append(existingBans, entry)
		} else {
			newBans = append(newBans, entry)
		}
	}
	return
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

	entries, unparseableCount := parseBansCfg(content)

	existingSteamIDs, err := s.getExistingBanSteamIDs(c.Request.Context(), serverID)
	if err != nil {
		responses.BadRequest(c, "Failed to query existing bans", &gin.H{"error": err.Error()})
		return
	}

	newBans, existingBans, expiredBans := categorizeBans(entries, existingSteamIDs)

	responses.Success(c, "Import preview generated", &gin.H{
		"preview": models.BanImportPreview{
			CfgAvailable:     true,
			CfgPath:          cfgPath,
			NewBans:          newBans,
			ExistingBans:     existingBans,
			ExpiredBans:      expiredBans,
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

	entries, _ := parseBansCfg(content)

	existingSteamIDs, err := s.getExistingBanSteamIDs(c.Request.Context(), serverID)
	if err != nil {
		responses.BadRequest(c, "Failed to query existing bans", &gin.H{"error": err.Error()})
		return
	}

	newBans, existingBans, expiredBans := categorizeBans(entries, existingSteamIDs)

	result := models.BanImportResult{
		SkippedCount: len(existingBans),
		ExpiredCount: len(expiredBans),
	}

	if len(newBans) == 0 {
		responses.Success(c, "No new bans to import", &gin.H{"result": result})
		return
	}

	// Import in a transaction
	tx, err := s.Dependencies.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	now := time.Now()
	for _, ban := range newBans {
		// Calculate duration: remaining days until expiry, or 0 for permanent
		duration := 0
		if !ban.Permanent {
			remaining := time.Unix(ban.ExpiryTimestamp, 0).Sub(now)
			duration = max(int(math.Ceil(remaining.Hours()/24)), 1)
		}

		reason := ban.Reason
		if reason == "" {
			reason = "Imported from Bans.cfg"
		}

		steamIDInt, err := strconv.ParseInt(ban.SteamID, 10, 64)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("invalid Steam ID %s: %v", ban.SteamID, err))
			continue
		}

		_, err = tx.ExecContext(c.Request.Context(), `
			INSERT INTO server_bans (id, server_id, admin_id, steam_id, reason, duration, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, uuid.New(), serverID, user.Id, steamIDInt, reason, duration, now, now)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to insert ban for %s: %v", ban.SteamID, err))
			continue
		}

		result.ImportedCount++
	}

	if err := tx.Commit(); err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to commit import"})
		return
	}

	// Sync Bans.cfg to reflect the merged state (DB is source of truth)
	if err := s.syncBansCfg(c.Request.Context(), server); err != nil {
		log.Warn().Err(err).Str("serverId", serverID.String()).Msg("Failed to sync Bans.cfg after import")
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
