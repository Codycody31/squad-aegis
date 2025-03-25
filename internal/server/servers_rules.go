package server

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/core"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// ServerRule represents a rule in the server
type ServerRule struct {
	ID                string  `json:"id"`
	ServerID          string  `json:"serverId"`
	ParentID          *string `json:"parentId"`
	Name              string  `json:"name"`
	Description       string  `json:"description"`
	SuggestedDuration int     `json:"suggestedDuration"` // Duration in minutes, 0 for permanent
	OrderKey          string  `json:"orderKey"`          // For ordering rules (e.g., "1", "1.1", "1.1A")
}

// ServerRuleCreateRequest represents a request to create a rule
type ServerRuleCreateRequest struct {
	ParentID          *string `json:"parentId"`
	Name              string  `json:"name"`
	Description       string  `json:"description"`
	SuggestedDuration int     `json:"suggestedDuration"` // Duration in minutes, 0 for permanent
	OrderKey          string  `json:"orderKey"`          // For ordering rules
}

// ServerRuleUpdateRequest represents a request to update a rule
type ServerRuleUpdateRequest struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	SuggestedDuration int    `json:"suggestedDuration"` // Duration in minutes, 0 for permanent
	OrderKey          string `json:"orderKey"`          // For ordering rules
}

// ServerRuleBatchUpdate represents a request to batch update rules
type ServerRuleBatchUpdate struct {
	Updates []struct {
		ID       string `json:"id"`
		OrderKey string `json:"orderKey"`
	} `json:"updates"`
}

// ServerRulesList handles listing all rules for a server
func (s *Server) ServerRulesList(c *gin.Context) {
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

	// Query the database for rules
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), `
		SELECT id, server_id, parent_id, name, description, suggested_duration, order_key
		FROM server_rules
		WHERE server_id = $1
		ORDER BY order_key ASC
	`, serverId)
	if err != nil {
		responses.BadRequest(c, "Failed to query rules", &gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	rules := []ServerRule{}
	for rows.Next() {
		var rule ServerRule
		var parentId sql.NullString
		err := rows.Scan(
			&rule.ID,
			&rule.ServerID,
			&parentId,
			&rule.Name,
			&rule.Description,
			&rule.SuggestedDuration,
			&rule.OrderKey,
		)
		if err != nil {
			responses.BadRequest(c, "Failed to scan rule", &gin.H{"error": err.Error()})
			return
		}

		if parentId.Valid {
			rule.ParentID = &parentId.String
		}

		rules = append(rules, rule)
	}

	responses.Success(c, "Rules fetched successfully", &gin.H{
		"rules": rules,
	})
}

// ServerRulesAdd handles adding a new rule
func (s *Server) ServerRulesAdd(c *gin.Context) {
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

	var request ServerRuleCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Validate request
	if request.Name == "" {
		responses.BadRequest(c, "Rule name is required", &gin.H{"error": "Rule name is required"})
		return
	}

	if request.SuggestedDuration < 0 {
		responses.BadRequest(c, "Suggested duration must be a positive integer", &gin.H{"error": "Suggested duration must be a positive integer"})
		return
	}

	if request.OrderKey == "" {
		responses.BadRequest(c, "Order key is required", &gin.H{"error": "Order key is required"})
		return
	}

	// If parent ID is provided, verify it exists and belongs to this server
	if request.ParentID != nil {
		parentId, err := uuid.Parse(*request.ParentID)
		if err != nil {
			responses.BadRequest(c, "Invalid parent rule ID", &gin.H{"error": err.Error()})
			return
		}

		var count int
		err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
			SELECT COUNT(*) FROM server_rules
			WHERE id = $1 AND server_id = $2
		`, parentId, serverId).Scan(&count)

		if err != nil {
			responses.BadRequest(c, "Failed to verify parent rule", &gin.H{"error": err.Error()})
			return
		}

		if count == 0 {
			responses.BadRequest(c, "Parent rule not found", &gin.H{"error": "Parent rule not found"})
			return
		}
	}

	// Insert the rule into the database
	var ruleID string
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		INSERT INTO server_rules (server_id, parent_id, name, description, suggested_duration, order_key)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, serverId, request.ParentID, request.Name, request.Description, request.SuggestedDuration, request.OrderKey).Scan(&ruleID)

	if err != nil {
		responses.BadRequest(c, "Failed to create rule", &gin.H{"error": err.Error()})
		return
	}

	// Create audit log
	auditData := map[string]interface{}{
		"ruleId":            ruleID,
		"name":              request.Name,
		"description":       request.Description,
		"suggestedDuration": request.SuggestedDuration,
		"orderKey":          request.OrderKey,
	}
	if request.ParentID != nil {
		auditData["parentId"] = *request.ParentID
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:rule:create", auditData)

	responses.Success(c, "Rule created successfully", &gin.H{
		"ruleId": ruleID,
	})
}

// ServerRulesUpdate handles updating a rule
func (s *Server) ServerRulesUpdate(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	ruleIdString := c.Param("ruleId")
	ruleId, err := uuid.Parse(ruleIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid rule ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	var request ServerRuleUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Validate request
	if request.Name == "" {
		responses.BadRequest(c, "Rule name is required", &gin.H{"error": "Rule name is required"})
		return
	}

	if request.SuggestedDuration < 0 {
		responses.BadRequest(c, "Suggested duration must be a positive integer", &gin.H{"error": "Suggested duration must be a positive integer"})
		return
	}

	if request.OrderKey == "" {
		responses.BadRequest(c, "Order key is required", &gin.H{"error": "Order key is required"})
		return
	}

	// Update the rule
	result, err := s.Dependencies.DB.ExecContext(c.Request.Context(), `
		UPDATE server_rules
		SET name = $1, description = $2, suggested_duration = $3, order_key = $4
		WHERE id = $5 AND server_id = $6
	`, request.Name, request.Description, request.SuggestedDuration, request.OrderKey, ruleId, serverId)

	if err != nil {
		responses.BadRequest(c, "Failed to update rule", &gin.H{"error": err.Error()})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		responses.BadRequest(c, "Failed to get rows affected", &gin.H{"error": err.Error()})
		return
	}

	if rowsAffected == 0 {
		responses.BadRequest(c, "Rule not found", &gin.H{"error": "Rule not found"})
		return
	}

	// Create audit log
	auditData := map[string]interface{}{
		"ruleId":            ruleId.String(),
		"name":              request.Name,
		"description":       request.Description,
		"suggestedDuration": request.SuggestedDuration,
		"orderKey":          request.OrderKey,
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:rule:update", auditData)

	responses.Success(c, "Rule updated successfully", nil)
}

// ServerRulesDelete handles deleting a rule
func (s *Server) ServerRulesDelete(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	ruleIdString := c.Param("ruleId")
	ruleId, err := uuid.Parse(ruleIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid rule ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Get rule details before deletion for audit log
	var name string
	var description string
	var suggestedDuration int
	var orderKey string
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT name, description, suggested_duration, order_key FROM server_rules
		WHERE id = $1 AND server_id = $2
	`, ruleId, serverId).Scan(&name, &description, &suggestedDuration, &orderKey)

	if err != nil && err != sql.ErrNoRows {
		responses.BadRequest(c, "Failed to get rule details", &gin.H{"error": err.Error()})
		return
	}

	// Delete the rule
	result, err := s.Dependencies.DB.ExecContext(c.Request.Context(), `
		DELETE FROM server_rules
		WHERE id = $1 AND server_id = $2
	`, ruleId, serverId)

	if err != nil {
		responses.BadRequest(c, "Failed to delete rule", &gin.H{"error": err.Error()})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		responses.BadRequest(c, "Failed to get rows affected", &gin.H{"error": err.Error()})
		return
	}

	if rowsAffected == 0 {
		responses.BadRequest(c, "Rule not found", &gin.H{"error": "Rule not found"})
		return
	}

	// Create audit log
	auditData := map[string]interface{}{
		"ruleId":            ruleId.String(),
		"name":              name,
		"description":       description,
		"suggestedDuration": suggestedDuration,
		"orderKey":          orderKey,
	}

	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:rule:delete", auditData)

	responses.Success(c, "Rule deleted successfully", nil)
}

// ServerRulesBatchUpdate handles batch updating rules
func (s *Server) ServerRulesBatchUpdate(c *gin.Context) {
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

	var request ServerRuleBatchUpdate
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Start a transaction
	tx, err := s.Dependencies.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		responses.BadRequest(c, "Failed to start transaction", &gin.H{"error": err.Error()})
		return
	}
	defer tx.Rollback()

	// Update each rule
	for _, update := range request.Updates {
		ruleId, err := uuid.Parse(update.ID)
		if err != nil {
			responses.BadRequest(c, "Invalid rule ID", &gin.H{"error": err.Error()})
			return
		}

		result, err := tx.ExecContext(c.Request.Context(), `
			UPDATE server_rules
			SET order_key = $1
			WHERE id = $2 AND server_id = $3
		`, update.OrderKey, ruleId, serverId)

		if err != nil {
			responses.BadRequest(c, "Failed to update rule", &gin.H{"error": err.Error()})
			return
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			responses.BadRequest(c, "Failed to get rows affected", &gin.H{"error": err.Error()})
			return
		}

		if rowsAffected == 0 {
			responses.BadRequest(c, "Rule not found", &gin.H{"error": "Rule not found"})
			return
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		responses.BadRequest(c, "Failed to commit transaction", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Rules updated successfully", nil)
}
