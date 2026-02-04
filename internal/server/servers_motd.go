package server

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/file_upload"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/motd"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// getMOTDConfig retrieves or creates the MOTD configuration for a server
func (s *Server) getMOTDConfig(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	config, err := s.fetchOrCreateMOTDConfig(c.Request.Context(), serverID)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	// Check if FTP/SFTP credentials are available
	hasCredentials, credentialSource := s.checkMOTDCredentials(c.Request.Context(), serverID, config)

	responses.Success(c, "MOTD config retrieved", &gin.H{
		"config":            config,
		"has_credentials":   hasCredentials,
		"credential_source": credentialSource,
	})
}

// updateMOTDConfig updates the MOTD configuration for a server
func (s *Server) updateMOTDConfig(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	var req models.ServerMOTDConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responses.BadRequest(c, "Invalid request body", &gin.H{"error": err.Error()})
		return
	}

	// Ensure config exists
	config, err := s.fetchOrCreateMOTDConfig(c.Request.Context(), serverID)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	// Build update query dynamically
	query := `UPDATE server_motd_config SET updated_at = NOW()`
	args := []interface{}{}
	argIndex := 1

	if req.PrefixText != nil {
		query += fmt.Sprintf(", prefix_text = $%d", argIndex)
		args = append(args, *req.PrefixText)
		argIndex++
	}
	if req.SuffixText != nil {
		query += fmt.Sprintf(", suffix_text = $%d", argIndex)
		args = append(args, *req.SuffixText)
		argIndex++
	}
	if req.AutoGenerateFromRules != nil {
		query += fmt.Sprintf(", auto_generate_from_rules = $%d", argIndex)
		args = append(args, *req.AutoGenerateFromRules)
		argIndex++
	}
	if req.IncludeRuleDescriptions != nil {
		query += fmt.Sprintf(", include_rule_descriptions = $%d", argIndex)
		args = append(args, *req.IncludeRuleDescriptions)
		argIndex++
	}
	if req.UploadEnabled != nil {
		query += fmt.Sprintf(", upload_enabled = $%d", argIndex)
		args = append(args, *req.UploadEnabled)
		argIndex++
	}
	if req.AutoUploadOnChange != nil {
		query += fmt.Sprintf(", auto_upload_on_change = $%d", argIndex)
		args = append(args, *req.AutoUploadOnChange)
		argIndex++
	}
	if req.MOTDFilePath != nil {
		query += fmt.Sprintf(", motd_file_path = $%d", argIndex)
		args = append(args, *req.MOTDFilePath)
		argIndex++
	}
	if req.UseLogCredentials != nil {
		query += fmt.Sprintf(", use_log_credentials = $%d", argIndex)
		args = append(args, *req.UseLogCredentials)
		argIndex++
	}
	if req.UploadHost != nil {
		query += fmt.Sprintf(", upload_host = $%d", argIndex)
		args = append(args, *req.UploadHost)
		argIndex++
	}
	if req.UploadPort != nil {
		query += fmt.Sprintf(", upload_port = $%d", argIndex)
		args = append(args, *req.UploadPort)
		argIndex++
	}
	if req.UploadUsername != nil {
		query += fmt.Sprintf(", upload_username = $%d", argIndex)
		args = append(args, *req.UploadUsername)
		argIndex++
	}
	if req.UploadPassword != nil {
		query += fmt.Sprintf(", upload_password = $%d", argIndex)
		args = append(args, *req.UploadPassword)
		argIndex++
	}
	if req.UploadProtocol != nil {
		query += fmt.Sprintf(", upload_protocol = $%d", argIndex)
		args = append(args, *req.UploadProtocol)
		argIndex++
	}

	query += fmt.Sprintf(" WHERE id = $%d", argIndex)
	args = append(args, config.ID)

	_, err = s.Dependencies.DB.ExecContext(c.Request.Context(), query, args...)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to update MOTD config: %w", err), nil)
		return
	}

	// Fetch updated config
	updatedConfig, err := s.fetchOrCreateMOTDConfig(c.Request.Context(), serverID)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	// Trigger auto-upload if enabled
	if updatedConfig.AutoUploadOnChange && updatedConfig.UploadEnabled {
		go s.triggerMOTDUpload(context.Background(), serverID)
	}

	responses.Success(c, "MOTD config updated", &gin.H{"config": updatedConfig})
}

// previewMOTD generates MOTD content without uploading
func (s *Server) previewMOTD(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	config, err := s.fetchOrCreateMOTDConfig(c.Request.Context(), serverID)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	rules, err := s.fetchServerRulesHierarchy(c.Request.Context(), serverID)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to fetch rules: %w", err), nil)
		return
	}

	generator := motd.NewGenerator()
	content := generator.GenerateMOTD(config, rules)
	rulesCount := generator.CountRules(rules)

	responses.Success(c, "MOTD preview generated", &gin.H{
		"content":      content,
		"rules_count":  rulesCount,
		"generated_at": time.Now().Format(time.RFC3339),
	})
}

// uploadMOTD generates and uploads MOTD to the game server
func (s *Server) uploadMOTD(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	config, err := s.fetchOrCreateMOTDConfig(c.Request.Context(), serverID)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	if !config.UploadEnabled {
		responses.BadRequest(c, "MOTD upload is not enabled for this server", nil)
		return
	}

	// Get upload configuration
	uploadConfig, err := s.getUploadConfig(c.Request.Context(), serverID, config)
	if err != nil {
		s.updateMOTDUploadError(c.Request.Context(), config.ID, err.Error())
		responses.BadRequest(c, "Upload not configured", &gin.H{"error": err.Error()})
		return
	}

	// Fetch rules and generate MOTD
	rules, err := s.fetchServerRulesHierarchy(c.Request.Context(), serverID)
	if err != nil {
		s.updateMOTDUploadError(c.Request.Context(), config.ID, err.Error())
		responses.InternalServerError(c, fmt.Errorf("failed to fetch rules: %w", err), nil)
		return
	}

	generator := motd.NewGenerator()
	content := generator.GenerateMOTD(config, rules)

	// Create uploader
	uploader, err := file_upload.NewUploader(uploadConfig)
	if err != nil {
		s.updateMOTDUploadError(c.Request.Context(), config.ID, err.Error())
		responses.InternalServerError(c, fmt.Errorf("failed to connect: %w", err), nil)
		return
	}
	defer uploader.Close()

	// Upload
	if err := uploader.Upload(c.Request.Context(), content); err != nil {
		s.updateMOTDUploadError(c.Request.Context(), config.ID, err.Error())
		responses.InternalServerError(c, fmt.Errorf("failed to upload: %w", err), nil)
		return
	}

	// Update success status
	s.updateMOTDUploadSuccess(c.Request.Context(), config.ID, content)

	responses.Success(c, "MOTD uploaded successfully", &gin.H{
		"uploaded_at": time.Now().Format(time.RFC3339),
	})
}

// testMOTDConnection tests the FTP/SFTP connection
func (s *Server) testMOTDConnection(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	config, err := s.fetchOrCreateMOTDConfig(c.Request.Context(), serverID)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	uploadConfig, err := s.getUploadConfig(c.Request.Context(), serverID, config)
	if err != nil {
		responses.BadRequest(c, "Upload not configured", &gin.H{"error": err.Error()})
		return
	}

	uploader, err := file_upload.NewUploader(uploadConfig)
	if err != nil {
		responses.Success(c, "Connection failed", &gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	defer uploader.Close()

	if err := uploader.TestConnection(c.Request.Context()); err != nil {
		responses.Success(c, "Connection test failed", &gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	responses.Success(c, "Connection successful", &gin.H{
		"success": true,
		"message": fmt.Sprintf("Successfully connected to %s://%s:%d", uploadConfig.Protocol, uploadConfig.Host, uploadConfig.Port),
	})
}

// Helper functions

func (s *Server) fetchOrCreateMOTDConfig(ctx context.Context, serverID uuid.UUID) (*models.ServerMOTDConfig, error) {
	var config models.ServerMOTDConfig

	query := `
		SELECT id, server_id, prefix_text, suffix_text, auto_generate_from_rules, include_rule_descriptions,
		       upload_enabled, auto_upload_on_change, motd_file_path, use_log_credentials, upload_host, upload_port,
		       upload_username, upload_password, upload_protocol, last_uploaded_at, last_upload_error,
		       last_generated_content, created_at, updated_at
		FROM server_motd_config
		WHERE server_id = $1
	`

	err := s.Dependencies.DB.QueryRowContext(ctx, query, serverID).Scan(
		&config.ID, &config.ServerID, &config.PrefixText, &config.SuffixText,
		&config.AutoGenerateFromRules, &config.IncludeRuleDescriptions,
		&config.UploadEnabled, &config.AutoUploadOnChange, &config.MOTDFilePath,
		&config.UseLogCredentials, &config.UploadHost, &config.UploadPort,
		&config.UploadUsername, &config.UploadPassword, &config.UploadProtocol,
		&config.LastUploadedAt, &config.LastUploadError, &config.LastGeneratedContent,
		&config.CreatedAt, &config.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Create default config using INSERT ... ON CONFLICT to handle race conditions
		config = models.ServerMOTDConfig{
			ID:                      uuid.New(),
			ServerID:                serverID,
			PrefixText:              "",
			SuffixText:              "",
			AutoGenerateFromRules:   true,
			IncludeRuleDescriptions: true,
			UploadEnabled:           false,
			AutoUploadOnChange:      false,
			MOTDFilePath:            "/SquadGame/ServerConfig/MOTD.cfg",
			UseLogCredentials:       true,
			CreatedAt:               time.Now(),
			UpdatedAt:               time.Now(),
		}

		// Use upsert to handle race conditions - if another request already created the row,
		// just return the existing one
		upsertQuery := `
			INSERT INTO server_motd_config (id, server_id, prefix_text, suffix_text, auto_generate_from_rules,
			    include_rule_descriptions, upload_enabled, auto_upload_on_change, motd_file_path, use_log_credentials,
			    created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			ON CONFLICT (server_id) DO UPDATE SET updated_at = server_motd_config.updated_at
			RETURNING id, server_id, prefix_text, suffix_text, auto_generate_from_rules, include_rule_descriptions,
			    upload_enabled, auto_upload_on_change, motd_file_path, use_log_credentials, upload_host, upload_port,
			    upload_username, upload_password, upload_protocol, last_uploaded_at, last_upload_error,
			    last_generated_content, created_at, updated_at
		`

		err = s.Dependencies.DB.QueryRowContext(ctx, upsertQuery,
			config.ID, config.ServerID, config.PrefixText, config.SuffixText,
			config.AutoGenerateFromRules, config.IncludeRuleDescriptions,
			config.UploadEnabled, config.AutoUploadOnChange, config.MOTDFilePath,
			config.UseLogCredentials, config.CreatedAt, config.UpdatedAt,
		).Scan(
			&config.ID, &config.ServerID, &config.PrefixText, &config.SuffixText,
			&config.AutoGenerateFromRules, &config.IncludeRuleDescriptions,
			&config.UploadEnabled, &config.AutoUploadOnChange, &config.MOTDFilePath,
			&config.UseLogCredentials, &config.UploadHost, &config.UploadPort,
			&config.UploadUsername, &config.UploadPassword, &config.UploadProtocol,
			&config.LastUploadedAt, &config.LastUploadError, &config.LastGeneratedContent,
			&config.CreatedAt, &config.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create MOTD config: %w", err)
		}

		return &config, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch MOTD config: %w", err)
	}

	return &config, nil
}

func (s *Server) fetchServerRulesHierarchy(ctx context.Context, serverID uuid.UUID) ([]models.ServerRule, error) {
	query := `
		SELECT id, server_id, parent_id, display_order, title, description, created_at, updated_at
		FROM server_rules
		WHERE server_id = $1
		ORDER BY display_order ASC
	`

	rows, err := s.Dependencies.DB.QueryContext(ctx, query, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.ServerRule
	for rows.Next() {
		var rule models.ServerRule
		if err := rows.Scan(&rule.ID, &rule.ServerID, &rule.ParentID, &rule.DisplayOrder,
			&rule.Title, &rule.Description, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	// Load actions for all rules
	if len(rules) > 0 {
		ruleIDs := make([]uuid.UUID, len(rules))
		for i, r := range rules {
			ruleIDs[i] = r.ID
		}

		actionsQuery := `SELECT id, rule_id, violation_count, action_type, duration, message, created_at, updated_at
		                 FROM server_rule_actions WHERE rule_id = ANY($1)`
		actionRows, err := s.Dependencies.DB.QueryContext(ctx, actionsQuery, pq.Array(ruleIDs))
		if err != nil {
			return nil, err
		}
		defer actionRows.Close()

		actionsByRuleID := make(map[uuid.UUID][]models.ServerRuleAction)
		for actionRows.Next() {
			var action models.ServerRuleAction
			if err := actionRows.Scan(&action.ID, &action.RuleID, &action.ViolationCount, &action.ActionType,
				&action.Duration, &action.Message, &action.CreatedAt, &action.UpdatedAt); err != nil {
				return nil, err
			}
			actionsByRuleID[action.RuleID] = append(actionsByRuleID[action.RuleID], action)
		}

		for i := range rules {
			if actions, ok := actionsByRuleID[rules[i].ID]; ok {
				rules[i].Actions = actions
			}
		}
	}

	// Build hierarchy: group rules by parent ID, then recursively build tree
	childrenByParent := make(map[uuid.UUID][]models.ServerRule)
	var rootRules []models.ServerRule

	for _, rule := range rules {
		rule.SubRules = nil // Reset SubRules
		if rule.ParentID != nil {
			childrenByParent[*rule.ParentID] = append(childrenByParent[*rule.ParentID], rule)
		} else {
			rootRules = append(rootRules, rule)
		}
	}

	// Recursively attach children
	var attachChildren func(rules []models.ServerRule) []models.ServerRule
	attachChildren = func(rules []models.ServerRule) []models.ServerRule {
		for i := range rules {
			if children, ok := childrenByParent[rules[i].ID]; ok {
				rules[i].SubRules = attachChildren(children)
			}
		}
		return rules
	}

	rootRules = attachChildren(rootRules)

	return rootRules, nil
}

func (s *Server) getUploadConfig(ctx context.Context, serverID uuid.UUID, config *models.ServerMOTDConfig) (file_upload.UploadConfig, error) {
	var uploadConfig file_upload.UploadConfig

	if config.UseLogCredentials {
		// Get server's log credentials
		server, err := core.GetServerById(ctx, s.Dependencies.DB, serverID, nil)
		if err != nil {
			return uploadConfig, fmt.Errorf("failed to get server: %w", err)
		}

		if server.LogSourceType == nil || (*server.LogSourceType != "sftp" && *server.LogSourceType != "ftp") {
			return uploadConfig, fmt.Errorf("server does not have FTP/SFTP log configuration")
		}

		if server.LogHost == nil || server.LogPort == nil || server.LogUsername == nil || server.LogPassword == nil {
			return uploadConfig, fmt.Errorf("server log credentials are incomplete")
		}

		uploadConfig = file_upload.UploadConfig{
			Protocol: *server.LogSourceType,
			Host:     *server.LogHost,
			Port:     *server.LogPort,
			Username: *server.LogUsername,
			Password: *server.LogPassword,
			FilePath: config.MOTDFilePath,
		}
	} else {
		// Use custom credentials
		if config.UploadProtocol == nil || config.UploadHost == nil || config.UploadPort == nil ||
			config.UploadUsername == nil || config.UploadPassword == nil ||
			*config.UploadProtocol == "" || *config.UploadHost == "" ||
			*config.UploadUsername == "" || *config.UploadPassword == "" {
			return uploadConfig, fmt.Errorf("custom upload credentials are incomplete")
		}

		uploadConfig = file_upload.UploadConfig{
			Protocol: *config.UploadProtocol,
			Host:     *config.UploadHost,
			Port:     *config.UploadPort,
			Username: *config.UploadUsername,
			Password: *config.UploadPassword,
			FilePath: config.MOTDFilePath,
		}
	}

	return uploadConfig, nil
}

func (s *Server) checkMOTDCredentials(ctx context.Context, serverID uuid.UUID, config *models.ServerMOTDConfig) (bool, string) {
	if config.UseLogCredentials {
		server, err := core.GetServerById(ctx, s.Dependencies.DB, serverID, nil)
		if err != nil {
			return false, ""
		}

		if server.LogSourceType != nil && (*server.LogSourceType == "sftp" || *server.LogSourceType == "ftp") &&
			server.LogHost != nil && server.LogPort != nil && server.LogUsername != nil && server.LogPassword != nil {
			return true, "log_credentials"
		}
		return false, ""
	}

	if config.UploadProtocol != nil && config.UploadHost != nil && config.UploadPort != nil &&
		config.UploadUsername != nil && config.UploadPassword != nil &&
		*config.UploadProtocol != "" && *config.UploadHost != "" &&
		*config.UploadUsername != "" && *config.UploadPassword != "" {
		return true, "custom_credentials"
	}

	return false, ""
}

func (s *Server) updateMOTDUploadError(ctx context.Context, configID uuid.UUID, errorMsg string) {
	query := `UPDATE server_motd_config SET last_upload_error = $1, updated_at = NOW() WHERE id = $2`
	s.Dependencies.DB.ExecContext(ctx, query, errorMsg, configID)
}

func (s *Server) updateMOTDUploadSuccess(ctx context.Context, configID uuid.UUID, content string) {
	query := `UPDATE server_motd_config SET last_uploaded_at = NOW(), last_upload_error = NULL,
	          last_generated_content = $1, updated_at = NOW() WHERE id = $2`
	s.Dependencies.DB.ExecContext(ctx, query, content, configID)
}

// triggerMOTDUpload triggers an MOTD upload in the background
func (s *Server) triggerMOTDUpload(ctx context.Context, serverID uuid.UUID) {
	config, err := s.fetchOrCreateMOTDConfig(ctx, serverID)
	if err != nil || !config.UploadEnabled {
		return
	}

	uploadConfig, err := s.getUploadConfig(ctx, serverID, config)
	if err != nil {
		s.updateMOTDUploadError(ctx, config.ID, err.Error())
		return
	}

	rules, err := s.fetchServerRulesHierarchy(ctx, serverID)
	if err != nil {
		s.updateMOTDUploadError(ctx, config.ID, err.Error())
		return
	}

	generator := motd.NewGenerator()
	content := generator.GenerateMOTD(config, rules)

	uploader, err := file_upload.NewUploader(uploadConfig)
	if err != nil {
		s.updateMOTDUploadError(ctx, config.ID, err.Error())
		return
	}
	defer uploader.Close()

	if err := uploader.Upload(ctx, content); err != nil {
		s.updateMOTDUploadError(ctx, config.ID, err.Error())
		return
	}

	s.updateMOTDUploadSuccess(ctx, config.ID, content)
}

// TriggerMOTDUploadIfEnabled is exported for use by rules bulk update.
// Returns true if upload was triggered, false otherwise.
func (s *Server) TriggerMOTDUploadIfEnabled(ctx context.Context, serverID uuid.UUID) bool {
	config, err := s.fetchOrCreateMOTDConfig(ctx, serverID)
	if err != nil {
		return false
	}

	if config.AutoUploadOnChange && config.UploadEnabled {
		go s.triggerMOTDUpload(context.Background(), serverID)
		return true
	}
	return false
}
