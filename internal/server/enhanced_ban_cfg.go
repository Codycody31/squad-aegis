package server

import (
	"bufio"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	"go.codycody31.dev/squad-aegis/internal/shared/utils"
)

// ServerBansCfgEnhanced handles generating the enhanced ban config file for the server
// This supports both local bans and remote ban list integration
func (s *Server) ServerBansCfgEnhanced(c *gin.Context) {
	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	// Check query parameters
	oursOnly := c.Query("ours") == "true"
	includeRemote := c.Query("remote") != "false" // Default to true

	var banCfg strings.Builder
	now := time.Now()

	if oursOnly {
		// Only return server-specific bans (for pushing to CBL)
		err = s.generateServerSpecificBans(c, serverId, &banCfg, now)
	} else {
		// Return all bans (server + subscribed ban lists + remote sources)
		err = s.generateAllBans(c, serverId, &banCfg, now, includeRemote)
	}

	if err != nil {
		responses.BadRequest(c, "Failed to generate ban config", &gin.H{"error": err.Error()})
		return
	}

	// Set the content type and send the response
	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, banCfg.String())
}

func (s *Server) generateServerSpecificBans(c *gin.Context, serverId uuid.UUID, banCfg *strings.Builder, now time.Time) error {
	// Query only direct server bans (not from ban lists)
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
		SELECT sb.steam_id, sb.eos_id, sb.reason, sb.duration, sb.created_at, sb.admin_id, u.username, u.steam_id
		FROM server_bans sb
		LEFT JOIN users u ON sb.admin_id = u.id
		WHERE sb.server_id = $1 AND sb.ban_list_id IS NULL
	`, serverId)
	if err != nil {
		return err
	}
	defer rows.Close()

	return s.processBanRows(rows, banCfg, now)
}

func (s *Server) generateAllBans(c *gin.Context, serverId uuid.UUID, banCfg *strings.Builder, now time.Time, includeRemote bool) error {
	// Get all active bans using the enhanced core function
	bans, err := core.GetServerBans(c.Request.Context(), s.Dependencies.DB, serverId)
	if err != nil {
		return err
	}

	// Process database bans
	for _, ban := range bans {
		// Determine the player identifier for this ban
		bannedID := ban.SteamID
		if bannedID == "" {
			bannedID = ban.EOSID
		}
		if bannedID == "" {
			continue
		}

		// Check if this Steam ID is in the ignore list (only for Steam IDs)
		if ban.SteamID != "" {
			isIgnored, err := core.IsIgnoredSteamID(c.Request.Context(), s.Dependencies.DB, ban.SteamID)
			if err != nil {
				log.Warn().Err(err).Str("steam_id", ban.SteamID).Msg("Failed to check if Steam ID is ignored, including ban anyway")
			} else if isIgnored {
				continue
			}
		}

		var expiryTimestamp string
		if ban.Duration == 0 {
			expiryTimestamp = "0"
		} else {
			expiryTime := ban.CreatedAt.Add(time.Duration(ban.Duration) * (time.Hour * 24))
			expiryTimestamp = strconv.FormatInt(expiryTime.Unix(), 10)
		}

		// Build the reason comment
		reasonComment := ""
		if ban.Reason != "" {
			reasonComment = " //" + utils.SanitizeBanReason(ban.Reason)
		} else if ban.Duration == 0 {
			reasonComment = " //Permanent ban"
		}

		banCfg.WriteString(fmt.Sprintf("%s:%s%s\n",
			bannedID, expiryTimestamp, reasonComment))
	}

	// Include remote ban sources if requested
	if includeRemote {
		err = s.appendRemoteBans(c, banCfg, now)
		if err != nil {
			// Log error but don't fail the entire request
			log.Warn().Err(err).Msg("Failed to fetch remote bans")
		}
	}

	return nil
}

func (s *Server) processBanRows(rows *sql.Rows, banCfg *strings.Builder, now time.Time) error {
	for rows.Next() {
		var steamIDInt sql.NullInt64
		var eosIDStr sql.NullString
		var reason string
		var duration int
		var createdAt time.Time
		var adminID sql.NullString
		var adminUsername sql.NullString
		var adminSteamIDInt sql.NullInt64

		err := rows.Scan(&steamIDInt, &eosIDStr, &reason, &duration, &createdAt, &adminID, &adminUsername, &adminSteamIDInt)
		if err != nil {
			return err
		}

		// Determine the banned player ID (prefer Steam ID, fall back to EOS ID)
		var bannedID string
		if steamIDInt.Valid {
			bannedID = strconv.FormatInt(steamIDInt.Int64, 10)
		} else if eosIDStr.Valid {
			bannedID = utils.NormalizeEOSID(eosIDStr.String)
		} else {
			continue
		}

		var expiryTimestamp string
		if duration == 0 {
			expiryTimestamp = "0"
		} else {
			expiryTime := createdAt.Add(time.Duration(duration) * (time.Hour * 24))
			expiryTimestamp = strconv.FormatInt(expiryTime.Unix(), 10)
		}

		// Build the reason comment
		reasonComment := ""
		if reason != "" {
			reasonComment = " //" + utils.SanitizeBanReason(reason)
		} else if duration == 0 {
			reasonComment = " //Permanent ban"
		}

		banCfg.WriteString(fmt.Sprintf("%s:%s%s\n",
			bannedID, expiryTimestamp, reasonComment))
	}
	return nil
}

func (s *Server) appendRemoteBans(c *gin.Context, banCfg *strings.Builder, now time.Time) error {
	// Get enabled remote ban sources
	sources, err := core.GetRemoteBanSources(c.Request.Context(), s.Dependencies.DB)
	if err != nil {
		return err
	}

	for _, source := range sources {
		if !source.SyncEnabled {
			continue
		}

		err = s.fetchAndProcessRemoteBans(c, source.URL, banCfg, now)
		if err != nil {
			// Log error but continue with other sources
			log.Warn().Err(err).Str("url", source.URL).Msg("Failed to fetch remote ban source")
			continue
		}
	}

	return nil
}

func (s *Server) fetchAndProcessRemoteBans(c *gin.Context, url string, banCfg *strings.Builder, now time.Time) error {
	// Validate URL to prevent SSRF
	if err := utils.ValidateRemoteURL(url); err != nil {
		return fmt.Errorf("remote ban source URL validation failed: %w", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	// Determine format based on content type or URL
	contentType := resp.Header.Get("Content-Type")

	if strings.Contains(contentType, "text/csv") || strings.HasSuffix(url, ".csv") {
		return s.processCSVBans(c, resp.Body, banCfg, now)
	} else {
		// Assume text format (steamid:timestamp)
		return s.processTextBans(c, resp.Body, banCfg, now)
	}
}

func (s *Server) processCSVBans(c *gin.Context, body io.Reader, banCfg *strings.Builder, now time.Time) error {
	reader := csv.NewReader(body)

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if len(record) < 1 {
			continue
		}

		steamID := strings.TrimSpace(record[0])
		if steamID == "" || steamID == "steam_id" { // Skip header
			continue
		}

		// Check if this Steam ID is in the ignore list
		isIgnored, err := core.IsIgnoredSteamID(c.Request.Context(), s.Dependencies.DB, steamID)
		if err != nil {
			// Log error but continue processing
			log.Warn().Err(err).Str("steam_id", steamID).Msg("Failed to check if Steam ID is ignored, including ban anyway")
		} else if isIgnored {
			// Skip this ban if it's in the ignore list
			continue
		}

		// For CSV format, we assume permanent bans unless specified otherwise
		expiry := "0"
		if len(record) > 3 && record[3] != "" {
			if record[3] == "0" {
				expiry = "0"
			} else {
				// If there's an expiry timestamp
				if expiryTime, err := time.Parse(time.RFC3339, record[3]); err == nil {
					if now.After(expiryTime) {
						continue // Skip expired bans
					}
					expiry = strconv.FormatInt(expiryTime.Unix(), 10)
				}
			}
		}

		// Remote bans don't have admin info, so use "Remote" as admin name
		banCfg.WriteString(fmt.Sprintf("Remote [SteamID 0] Banned:%s:%s //From remote source\n", steamID, expiry))
	}

	return nil
}

func (s *Server) processTextBans(c *gin.Context, body io.Reader, banCfg *strings.Builder, now time.Time) error {
	scanner := bufio.NewScanner(body)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		steamID := strings.TrimSpace(parts[0])
		expiryStr := strings.TrimSpace(parts[1])

		if steamID == "" {
			continue
		}

		// Check if this Steam ID is in the ignore list
		isIgnored, err := core.IsIgnoredSteamID(c.Request.Context(), s.Dependencies.DB, steamID)
		if err != nil {
			// Log error but continue processing
			log.Warn().Err(err).Str("steam_id", steamID).Msg("Failed to check if Steam ID is ignored, including ban anyway")
		} else if isIgnored {
			// Skip this ban if it's in the ignore list
			continue
		}

		// Check if ban is expired
		if expiryStr != "0" {
			if expiryTime, err := strconv.ParseInt(expiryStr, 10, 64); err == nil {
				if now.After(time.Unix(expiryTime, 0)) {
					continue // Skip expired bans
				}
			}
		} else {
			expiryStr = "0"
		}

		// Remote bans don't have admin info, so use "Remote" as admin name
		banCfg.WriteString(fmt.Sprintf("Remote [SteamID 0] Banned:%s:%s //From remote source\n", steamID, expiryStr))
	}

	return scanner.Err()
}

// BanListCfg handles generating a ban config for a specific ban list
func (s *Server) BanListCfg(c *gin.Context) {
	banListIdString := c.Param("banListId")
	banListId, err := uuid.Parse(banListIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid ban list ID", &gin.H{"error": err.Error()})
		return
	}

	// Query bans from the specific ban list
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
		SELECT sb.steam_id, sb.eos_id, sb.reason, sb.duration, sb.created_at, sb.admin_id, u.username, u.steam_id
		FROM server_bans sb
		LEFT JOIN users u ON sb.admin_id = u.id
		WHERE sb.ban_list_id = $1
	`, banListId)
	if err != nil {
		responses.BadRequest(c, "Failed to query bans", &gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var banCfg strings.Builder
	now := time.Now()

	err = s.processBanRows(rows, &banCfg, now)
	if err != nil {
		responses.BadRequest(c, "Failed to process bans", &gin.H{"error": err.Error()})
		return
	}

	// Set the content type and send the response
	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, banCfg.String())
}
