package server

import (
	"context"
	"database/sql"
	"encoding/json"
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
		SELECT sb.id, sb.server_id, sb.admin_id, u.username, sb.steam_id, sb.reason, sb.duration, sb.rule_id, sr.title as rule_title,  sb.ban_list_id, bl.name as ban_list_name, sb.evidence_text, sb.created_at, sb.updated_at
		FROM server_bans sb
		JOIN users u ON sb.admin_id = u.id
		LEFT JOIN ban_lists bl ON sb.ban_list_id = bl.id
		LEFT JOIN server_rules sr ON sb.rule_id = sr.id
		WHERE sb.server_id = $1
		ORDER BY sb.created_at DESC
	`, serverId)
	if err != nil {
		responses.BadRequest(c, "Failed to query bans", &gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	bans := []models.ServerBan{}
	steamIDs := []string{}
	for rows.Next() {
		var ban models.ServerBan
		var steamIDInt int64
		var ruleID sql.NullString
		var ruleTitle sql.NullString
		var banListID sql.NullString
		var banListName sql.NullString
		var evidenceText sql.NullString
		err := rows.Scan(
			&ban.ID,
			&ban.ServerID,
			&ban.AdminID,
			&ban.AdminName,
			&steamIDInt,
			&ban.Reason,
			&ban.Duration,
			&ruleID,
			&ruleTitle,
			&banListID,
			&banListName,
			&evidenceText,
			&ban.CreatedAt,
			&ban.UpdatedAt,
		)
		if err != nil {
			responses.BadRequest(c, "Failed to scan ban", &gin.H{"error": err.Error()})
			return
		}

		// Convert steamID from int64 to string
		ban.SteamID = strconv.FormatInt(steamIDInt, 10)

		// Collect steam IDs for batch lookup
		steamIDs = append(steamIDs, ban.SteamID)

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

		// Calculate if ban is permanent and expiry date
		ban.Permanent = ban.Duration == 0
		if !ban.Permanent {
			ban.ExpiresAt = ban.CreatedAt.Add(time.Duration(ban.Duration) * 24 * time.Hour)
		}

		// Load evidence records for this ban
		ban.Evidence, _ = s.loadBanEvidence(c.Request.Context(), ban.ID)

		bans = append(bans, ban)
	}

	// Batch lookup player names from ClickHouse
	playerNames := s.lookupPlayerNamesBatch(c.Request.Context(), steamIDs)

	// Assign player names to bans
	for i := range bans {
		if name, ok := playerNames[bans[i].SteamID]; ok {
			bans[i].Name = name
		} else {
			// Fallback to steam ID if no name found
			bans[i].Name = bans[i].SteamID
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

	// Insert the ban into the database (using steam_id directly)
	var banID uuid.UUID = uuid.New()
	now := time.Now()

	query := `
		INSERT INTO server_bans (id, server_id, admin_id, steam_id, reason, duration, evidence_text, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`
	args := []interface{}{banID, serverId, user.Id, steamID, request.Reason, request.Duration, request.EvidenceText, now, now}

	// Add rule_id and ban_list_id if provided
	if request.RuleID != nil && *request.RuleID != "" {
		ruleUUID, err := uuid.Parse(*request.RuleID)
		if err != nil {
			responses.BadRequest(c, "Invalid rule ID format", &gin.H{"error": err.Error()})
			return
		}

		if request.BanListID != nil && *request.BanListID != "" {
			banListUUID, err := uuid.Parse(*request.BanListID)
			if err != nil {
				responses.BadRequest(c, "Invalid ban list ID format", &gin.H{"error": err.Error()})
				return
			}
			query = `
				INSERT INTO server_bans (id, server_id, admin_id, steam_id, reason, duration, rule_id, ban_list_id, evidence_text, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
				RETURNING id
			`
			args = append(args[:6], ruleUUID, banListUUID, request.EvidenceText, now, now)
		} else {
			query = `
				INSERT INTO server_bans (id, server_id, admin_id, steam_id, reason, duration, rule_id, evidence_text, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
				RETURNING id
			`
			args = append(args[:6], ruleUUID, request.EvidenceText, now, now)
		}
	} else if request.BanListID != nil && *request.BanListID != "" {
		banListUUID, err := uuid.Parse(*request.BanListID)
		if err != nil {
			responses.BadRequest(c, "Invalid ban list ID format", &gin.H{"error": err.Error()})
			return
		}
		query = `
			INSERT INTO server_bans (id, server_id, admin_id, steam_id, reason, duration, ban_list_id, evidence_text, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING id
		`
		args = append(args[:6], banListUUID, request.EvidenceText, now, now)
	}

	var returnedBanID string
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), query, args...).Scan(&returnedBanID)
	if err != nil {
		responses.BadRequest(c, "Failed to create ban", &gin.H{"error": err.Error()})
		return
	}

	// Insert evidence records if provided
	if len(request.Evidence) > 0 {
		err = s.createBanEvidence(c.Request.Context(), banID.String(), serverId, request.Evidence)
		if err != nil {
			log.Error().Err(err).Str("banId", banID.String()).Msg("Failed to create ban evidence")
			// Don't fail the entire ban creation, just log the error
		}
	}

	// Also apply the ban via RCON if the server is online
	if server != nil {
		r := squadRcon.NewSquadRcon(s.Dependencies.RconManager, server.Id)
		// Removed as we update the remote ban list, no the one on the server itself
		// err = r.BanPlayer(request.SteamID, request.Duration, request.Reason)
		// if err != nil {
		// 	log.Error().Err(err).Str("steamId", request.SteamID).Str("serverId", server.Id.String()).Msg("Failed to apply ban via RCON")
		// }

		err = r.KickPlayer(request.SteamID, request.Reason)
		if err != nil {
			log.Error().Err(err).Str("steamId", request.SteamID).Str("serverId", server.Id.String()).Msg("Failed to kick player via RCON")
		}
	}

	// Create detailed audit log
	auditData := map[string]interface{}{
		"banId":         banID.String(),
		"steamId":       request.SteamID,
		"reason":        request.Reason,
		"duration":      request.Duration,
		"evidenceCount": len(request.Evidence),
	}

	// Add expiry information if not permanent
	if request.Duration > 0 {
		expiresAt := time.Now().Add(time.Duration(request.Duration) * 24 * time.Hour)
		auditData["expiresAt"] = expiresAt.Format(time.RFC3339)
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:ban:create", auditData)

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

	// Get the ban details first (to get the Steam ID for RCON unban)
	var steamIDInt int64
	var reason string
	var duration int
	var adminId uuid.UUID

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT sb.steam_id, sb.reason, sb.duration, sb.admin_id
		FROM server_bans sb
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

	// Delete evidence files from storage before deleting the ban
	// (ban deletion will cascade delete evidence records)
	if err := s.deleteBanEvidenceFiles(c.Request.Context(), banId.String()); err != nil {
		log.Warn().Err(err).Str("banId", banId.String()).Msg("Failed to delete some evidence files from storage, continuing with ban deletion")
		// Continue with ban deletion even if file deletion fails
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

	// Get the current ban details first
	var currentBan models.ServerBan
	var steamIDInt int64
	var ruleID sql.NullString
	var ruleTitle sql.NullString
	var banListID sql.NullString
	var banListName sql.NullString
	var evidenceText sql.NullString

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT sb.id, sb.server_id, sb.admin_id, u.username, sb.steam_id, sb.reason, sb.duration, sb.rule_id, sr.title as rule_title,  sb.ban_list_id, bl.name as ban_list_name, sb.evidence_text, sb.created_at, sb.updated_at
		FROM server_bans sb
		JOIN users u ON sb.admin_id = u.id
		LEFT JOIN ban_lists bl ON sb.ban_list_id = bl.id
		LEFT JOIN server_rules sr ON sb.rule_id = sr.id
		WHERE sb.id = $1 AND sb.server_id = $2
	`, banId, serverId).Scan(
		&currentBan.ID,
		&currentBan.ServerID,
		&currentBan.AdminID,
		&currentBan.AdminName,
		&steamIDInt,
		&currentBan.Reason,
		&currentBan.Duration,
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
			responses.BadRequest(c, "Failed to get ban details", &gin.H{"error": err.Error()})
		}
		return
	}

	// Convert steamID from int64 to string
	currentBan.SteamID = strconv.FormatInt(steamIDInt, 10)
	currentBan.Name = currentBan.SteamID

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

	// Calculate if ban is permanent and expiry date
	currentBan.Permanent = currentBan.Duration == 0
	if !currentBan.Permanent {
		currentBan.ExpiresAt = currentBan.CreatedAt.Add(time.Duration(currentBan.Duration) * 24 * time.Hour)
	}

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
		if *request.Duration < 0 {
			responses.BadRequest(c, "Duration must be a positive integer", &gin.H{"error": "Duration must be a positive integer"})
			return
		}
		updateFields = append(updateFields, fmt.Sprintf("duration = $%d", argIndex))
		updateArgs = append(updateArgs, *request.Duration)
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

	// Execute the update
	result, err := s.Dependencies.DB.ExecContext(c.Request.Context(), query, updateArgs...)
	if err != nil {
		responses.BadRequest(c, "Failed to update ban", &gin.H{"error": err.Error()})
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

	// Update evidence records if provided (including empty array to clear evidence)
	if request.Evidence != nil {
		// First, get the list of existing evidence files to determine which ones to delete
		existingFiles, err := s.getExistingEvidenceFiles(c.Request.Context(), banId.String())
		if err != nil {
			log.Error().Err(err).Str("banId", banId.String()).Msg("Failed to query existing evidence files")
			responses.BadRequest(c, "Failed to update evidence", &gin.H{"error": "Failed to query existing evidence"})
			return
		}

		// Build a set of file paths from the new evidence that should be kept
		newFilePaths := make(map[string]bool)
		for _, ev := range *request.Evidence {
			if ev.EvidenceType == "file_upload" && ev.FilePath != nil && *ev.FilePath != "" {
				newFilePaths[*ev.FilePath] = true
			}
		}

		// Determine which files need to be deleted (files that exist but are NOT in the new evidence)
		filesToDelete := []string{}
		for _, existingFile := range existingFiles {
			if !newFilePaths[existingFile] {
				filesToDelete = append(filesToDelete, existingFile)
			}
		}

		// Delete only the files that are being removed
		if len(filesToDelete) > 0 {
			if err := s.deleteSpecificEvidenceFiles(c.Request.Context(), banId.String(), filesToDelete); err != nil {
				log.Warn().Err(err).Str("banId", banId.String()).Msg("Failed to delete some evidence files from storage, continuing with database update")
				// Continue with update even if file deletion fails
			}
		}

		// Use a transaction to ensure atomicity - if insert fails, rollback the delete
		tx, txErr := s.Dependencies.DB.BeginTx(c.Request.Context(), nil)
		if txErr != nil {
			log.Error().Err(txErr).Str("banId", banId.String()).Msg("Failed to begin transaction for evidence update")
			responses.BadRequest(c, "Failed to update evidence", &gin.H{"error": "Failed to start evidence update transaction"})
			return
		}
		defer tx.Rollback()

		// Delete existing evidence records from database
		_, delErr := tx.ExecContext(c.Request.Context(), `
			DELETE FROM ban_evidence WHERE ban_id = $1
		`, banId)
		if delErr != nil {
			log.Error().Err(delErr).Str("banId", banId.String()).Msg("Failed to delete old ban evidence")
			tx.Rollback()
			responses.BadRequest(c, "Failed to update evidence", &gin.H{"error": "Failed to delete existing evidence"})
			return
		}

		// Insert new evidence (if any)
		if len(*request.Evidence) > 0 {
			createErr := s.createBanEvidenceWithTx(c.Request.Context(), tx, banId.String(), serverId, *request.Evidence)
			if createErr != nil {
				log.Error().Err(createErr).Str("banId", banId.String()).Msg("Failed to create updated ban evidence")
				tx.Rollback()
				responses.BadRequest(c, "Failed to update evidence", &gin.H{"error": createErr.Error()})
				return
			}
		}

		// Commit the transaction
		if commitErr := tx.Commit(); commitErr != nil {
			log.Error().Err(commitErr).Str("banId", banId.String()).Msg("Failed to commit evidence update transaction")
			responses.BadRequest(c, "Failed to update evidence", &gin.H{"error": "Failed to commit evidence update"})
			return
		}
	}

	// Get updated ban details for response and RCON
	var updatedBan models.ServerBan
	var updatedSteamIDInt int64
	var updatedRuleID sql.NullString
	var updatedRuleTitle sql.NullString
	var updatedBanListID sql.NullString
	var updatedBanListName sql.NullString
	var updatedEvidenceText sql.NullString

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT sb.id, sb.server_id, sb.admin_id, u.username, sb.steam_id, sb.reason, sb.duration, sb.rule_id, sr.title as rule_title,  sb.ban_list_id, bl.name as ban_list_name, sb.evidence_text, sb.created_at, sb.updated_at
		FROM server_bans sb
		JOIN users u ON sb.admin_id = u.id
		LEFT JOIN ban_lists bl ON sb.ban_list_id = bl.id
		LEFT JOIN server_rules sr ON sb.rule_id = sr.id
		WHERE sb.id = $1
	`, banId).Scan(
		&updatedBan.ID,
		&updatedBan.ServerID,
		&updatedBan.AdminID,
		&updatedBan.AdminName,
		&updatedSteamIDInt,
		&updatedBan.Reason,
		&updatedBan.Duration,
		&updatedRuleID,
		&updatedRuleTitle,
		&updatedBanListID,
		&updatedBanListName,
		&updatedEvidenceText,
		&updatedBan.CreatedAt,
		&updatedBan.UpdatedAt,
	)
	if err != nil {
		responses.BadRequest(c, "Failed to get updated ban details", &gin.H{"error": err.Error()})
		return
	}

	// Convert steamID from int64 to string
	updatedBan.SteamID = strconv.FormatInt(updatedSteamIDInt, 10)
	updatedBan.Name = updatedBan.SteamID

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

	// Calculate if ban is permanent and expiry date
	updatedBan.Permanent = updatedBan.Duration == 0
	if !updatedBan.Permanent {
		updatedBan.ExpiresAt = updatedBan.CreatedAt.Add(time.Duration(updatedBan.Duration) * 24 * time.Hour)
	}

	// Create detailed audit log
	auditData := map[string]interface{}{
		"banId":        banId.String(),
		"steamId":      updatedBan.SteamID,
		"oldReason":    currentBan.Reason,
		"newReason":    updatedBan.Reason,
		"oldDuration":  currentBan.Duration,
		"newDuration":  updatedBan.Duration,
		"oldBanListId": currentBan.BanListID,
		"newBanListId": updatedBan.BanListID,
		"oldRuleId":    currentBan.RuleID,
		"newRuleId":    updatedBan.RuleID,
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:ban:update", auditData)

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

	// Query the database for active bans
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
		SELECT sb.steam_id, sb.reason, sb.duration, sb.created_at, sb.admin_id, u.username, u.steam_id
		FROM server_bans sb
		LEFT JOIN users u ON sb.admin_id = u.id
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
		var adminID sql.NullString
		var adminUsername sql.NullString
		var adminSteamIDInt sql.NullInt64
		err := rows.Scan(&steamIDInt, &reason, &duration, &createdAt, &adminID, &adminUsername, &adminSteamIDInt)
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

		// Format the ban entry in official Squad format
		steamIDStr := strconv.FormatInt(steamIDInt, 10)

		// Build admin info
		adminInfo := "System"
		adminSteamID := "0"
		if adminUsername.Valid && adminUsername.String != "" {
			adminInfo = adminUsername.String
		}
		if adminSteamIDInt.Valid && adminSteamIDInt.Int64 > 0 {
			adminSteamID = strconv.FormatInt(adminSteamIDInt.Int64, 10)
		}

		var expiryTimestamp string
		if duration == 0 {
			expiryTimestamp = "0"
		} else {
			expiryTimestamp = strconv.FormatInt(unixTimeOfExpiry.Unix(), 10)
		}

		// Build the reason comment
		reasonComment := ""
		if reason != "" {
			reasonComment = " //" + reason
		} else if duration == 0 {
			reasonComment = " //Permanent ban"
		}

		banCfg.WriteString(fmt.Sprintf("%s [SteamID %s] Banned:%s:%s%s\n",
			adminInfo, adminSteamID, steamIDStr, expiryTimestamp, reasonComment))
	}

	// Set the content type and send the response
	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, banCfg.String())
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
	// Query for all file_upload evidence records for this ban
	rows, err := s.Dependencies.DB.QueryContext(ctx, `
		SELECT file_path
		FROM ban_evidence
		WHERE ban_id = $1 AND evidence_type = 'file_upload' AND file_path IS NOT NULL
	`, banID)
	if err != nil {
		return fmt.Errorf("failed to query evidence files: %w", err)
	}
	defer rows.Close()

	var deleteErrors []error
	for rows.Next() {
		var filePath sql.NullString
		if err := rows.Scan(&filePath); err != nil {
			log.Warn().Err(err).Str("banId", banID).Msg("Failed to scan file path")
			continue
		}

		if !filePath.Valid || filePath.String == "" {
			continue
		}

		// Delete the file from storage
		if err := s.Dependencies.Storage.Delete(ctx, filePath.String); err != nil {
			log.Warn().Err(err).
				Str("banId", banID).
				Str("filePath", filePath.String).
				Msg("Failed to delete evidence file from storage")
			deleteErrors = append(deleteErrors, fmt.Errorf("failed to delete file %s: %w", filePath.String, err))
			// Continue deleting other files even if one fails
		} else {
			log.Info().
				Str("banId", banID).
				Str("filePath", filePath.String).
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
