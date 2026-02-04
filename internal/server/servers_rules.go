package server

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

func (s *Server) listServerRules(c *gin.Context) {
	serverIdString := c.Param("serverId")
	serverID, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	query := `
		SELECT id, server_id, parent_id, display_order, title, description, created_at, updated_at
		FROM server_rules
		WHERE server_id = $1
		ORDER BY display_order ASC
	`
	rows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), query, serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get server rules"})
		return
	}
	defer rows.Close()

	var rules []models.ServerRule
	for rows.Next() {
		var rule models.ServerRule
		if err := rows.Scan(&rule.ID, &rule.ServerID, &rule.ParentID, &rule.DisplayOrder, &rule.Title, &rule.Description, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan server rule"})
			return
		}
		rules = append(rules, rule)
	}

	ruleIDs := []uuid.UUID{}
	for _, r := range rules {
		ruleIDs = append(ruleIDs, r.ID)
	}

	var actions []models.ServerRuleAction
	if len(ruleIDs) > 0 {
		query := `SELECT id, rule_id, violation_count, action_type, duration, message, created_at, updated_at FROM server_rule_actions WHERE rule_id = ANY($1)`
		actionRows, err := s.Dependencies.DB.QueryContext(c.Request.Context(), query, pq.Array(ruleIDs))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load rule actions"})
			return
		}
		defer actionRows.Close()

		for actionRows.Next() {
			var action models.ServerRuleAction
			if err := actionRows.Scan(&action.ID, &action.RuleID, &action.ViolationCount, &action.ActionType, &action.Duration, &action.Message, &action.CreatedAt, &action.UpdatedAt); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan rule action"})
				return
			}
			actions = append(actions, action)
		}
	}

	actionsByRuleID := make(map[uuid.UUID][]models.ServerRuleAction)
	for _, action := range actions {
		actionsByRuleID[action.RuleID] = append(actionsByRuleID[action.RuleID], action)
	}

	ruleMap := make(map[uuid.UUID]*models.ServerRule)
	for i := range rules {
		rule := &rules[i]
		if a, ok := actionsByRuleID[rule.ID]; ok {
			rule.Actions = a
		}
		ruleMap[rule.ID] = rule
	}

	var rootRules []*models.ServerRule
	for i := range rules {
		rule := &rules[i]
		if rule.ParentID != nil {
			if parent, ok := ruleMap[*rule.ParentID]; ok {
				parent.SubRules = append(parent.SubRules, *rule)
			}
		} else {
			rootRules = append(rootRules, rule)
		}
	}

	c.JSON(http.StatusOK, rootRules)
}

func (s *Server) createServerRule(c *gin.Context) {
	serverIdString := c.Param("serverId")
	serverID, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	var rule models.ServerRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rule data"})
		return
	}

	rule.ServerID = serverID

	query := `INSERT INTO server_rules (server_id, parent_id, display_order, title, description) VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at, updated_at`
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), query, rule.ServerID, rule.ParentID, rule.DisplayOrder, rule.Title, rule.Description).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create server rule"})
		return
	}

	c.JSON(http.StatusCreated, rule)
}

func (s *Server) updateServerRule(c *gin.Context) {
	serverIdString := c.Param("serverId")
	serverID, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	ruleIdString := c.Param("ruleId")
	ruleID, err := uuid.Parse(ruleIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid rule ID", &gin.H{"error": err.Error()})
		return
	}

	var existingRule models.ServerRule
	query := `SELECT id, server_id, parent_id, display_order, title, description, created_at, updated_at FROM server_rules WHERE id = $1 AND server_id = $2`
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), query, ruleID, serverID).Scan(
		&existingRule.ID, &existingRule.ServerID, &existingRule.ParentID, &existingRule.DisplayOrder,
		&existingRule.Title, &existingRule.Description, &existingRule.CreatedAt, &existingRule.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find rule"})
		return
	}

	var updateData models.ServerRule
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rule data"})
		return
	}

	// Update rule fields
	updateQuery := `UPDATE server_rules SET 
                    parent_id = $1, 
                    display_order = $2, 
                    title = $3, 
                    description = $4, 
                    updated_at = NOW() 
                  WHERE id = $5 AND server_id = $6
                  RETURNING updated_at`

	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), updateQuery,
		updateData.ParentID,
		updateData.DisplayOrder,
		updateData.Title,
		updateData.Description,
		ruleID,
		serverID).Scan(&existingRule.UpdatedAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update rule"})
		return
	}

	// Update our return structure with the new values
	existingRule.ParentID = updateData.ParentID
	existingRule.DisplayOrder = updateData.DisplayOrder
	existingRule.Title = updateData.Title
	existingRule.Description = updateData.Description

	c.JSON(http.StatusOK, existingRule)
}

func (s *Server) deleteServerRule(c *gin.Context) {
	serverIdString := c.Param("serverId")
	serverID, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	ruleIdString := c.Param("ruleId")
	ruleID, err := uuid.Parse(ruleIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid rule ID", &gin.H{"error": err.Error()})
		return
	}

	result, err := s.Dependencies.DB.ExecContext(c.Request.Context(), "DELETE FROM server_rules WHERE server_id = $1 AND id = $2", serverID, ruleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete rule"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check affected rows"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

// bulkUpdateServerRules updates multiple rules in a single transaction
func (s *Server) bulkUpdateServerRules(c *gin.Context) {
	serverIdString := c.Param("serverId")
	serverID, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	// Parse the request body which now includes rules and deleted_rule_ids
	var bulkRequest struct {
		Rules          []map[string]interface{} `json:"rules"`
		DeletedRuleIDs []string                 `json:"deleted_rule_ids"`
	}

	if err := c.ShouldBindJSON(&bulkRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data format: " + err.Error()})
		return
	}

	requestData := bulkRequest.Rules
	deletedRuleIDs := bulkRequest.DeletedRuleIDs

	// Convert the flexible data into our model, ignoring extra fields
	var requestRules []models.ServerRule
	for _, ruleData := range requestData {
		var rule models.ServerRule

		// Handle ID field
		if idStr, ok := ruleData["id"].(string); ok {
			if id, err := uuid.Parse(idStr); err == nil {
				rule.ID = id
			}
		}

		// Handle server_id field - always override with the correct server ID
		rule.ServerID = serverID

		// Handle parent_id field
		if parentIDStr, ok := ruleData["parent_id"].(string); ok && parentIDStr != "" {
			if parentID, err := uuid.Parse(parentIDStr); err == nil {
				rule.ParentID = &parentID
			}
		}

		// Handle display_order field
		if displayOrder, ok := ruleData["display_order"].(float64); ok {
			rule.DisplayOrder = int(displayOrder)
		}

		// Handle title field
		if title, ok := ruleData["title"].(string); ok {
			rule.Title = title
		}

		// Handle description field
		if description, ok := ruleData["description"].(string); ok {
			rule.Description = description
		}

		// Handle actions field
		if actionsData, ok := ruleData["actions"].([]interface{}); ok {
			for _, actionItem := range actionsData {
				if actionMap, ok := actionItem.(map[string]interface{}); ok {
					var action models.ServerRuleAction

					// Handle action ID
					if actionIDStr, ok := actionMap["id"].(string); ok {
						if actionID, err := uuid.Parse(actionIDStr); err == nil {
							action.ID = actionID
						}
					}

					// Handle rule_id
					if ruleIDStr, ok := actionMap["rule_id"].(string); ok {
						if ruleID, err := uuid.Parse(ruleIDStr); err == nil {
							action.RuleID = ruleID
						}
					}

					// Handle violation_count
					if violationCount, ok := actionMap["violation_count"].(float64); ok {
						action.ViolationCount = int(violationCount)
					}

					// Handle action_type
					if actionType, ok := actionMap["action_type"].(string); ok {
						action.ActionType = actionType
					}

					// Handle duration (support both duration_days and duration for backward compatibility)
					// Frontend sends duration_days, but we also check duration in case it's sent
					var durationDays *int
					if durationDaysVal, ok := actionMap["duration_days"].(float64); ok {
						d := int(durationDaysVal)
						durationDays = &d
					} else if durationVal, ok := actionMap["duration"].(float64); ok {
						d := int(durationVal)
						durationDays = &d
					} else if durationMinutesVal, ok := actionMap["duration_minutes"].(float64); ok {
						// Legacy: convert minutes to days (assuming it was stored incorrectly)
						d := int(durationMinutesVal) / (24 * 60)
						durationDays = &d
					}
					if durationDays != nil {
						action.Duration = durationDays
					}

					// Handle message
					if message, ok := actionMap["message"].(string); ok {
						action.Message = message
					}

					rule.Actions = append(rule.Actions, action)
				}
			}
		}

		// Ignore sub_rules field as it's handled through parent_id relationships

		requestRules = append(requestRules, rule)
	}

	// Start a transaction (even if only deleting rules)
	tx, err := s.Dependencies.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback() // Will be a no-op if tx.Commit() is called

	// Handle deletions first
	if len(deletedRuleIDs) > 0 {
		for _, ruleIDStr := range deletedRuleIDs {
			ruleID, err := uuid.Parse(ruleIDStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rule ID in deleted_rule_ids: " + err.Error()})
				return
			}

			// Delete the rule (cascade will handle actions and sub-rules if configured)
			_, err = tx.ExecContext(c.Request.Context(),
				"DELETE FROM server_rules WHERE id = $1 AND server_id = $2",
				ruleID, serverID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete rule: " + err.Error()})
				return
			}
		}
	}

	// Process all rules
	var updatedRules []*models.ServerRule
	for _, rule := range requestRules {
		if rule.ServerID != serverID {
			rule.ServerID = serverID // Ensure server ID is correct
		}

		var resultRule models.ServerRule
		// Check if rule exists and belongs to the server
		var isExistingRule bool = false

		if rule.ID != uuid.Nil {
			query := `SELECT id, server_id, created_at FROM server_rules WHERE id = $1 AND server_id = $2`
			var existingID, existingServerID uuid.UUID
			var createdAt sql.NullTime

			err = tx.QueryRowContext(c.Request.Context(), query, rule.ID, serverID).
				Scan(&existingID, &existingServerID, &createdAt)

			if err == nil {
				// Rule exists
				isExistingRule = true
				if createdAt.Valid {
					resultRule.CreatedAt = createdAt.Time
				}
			} else if err != sql.ErrNoRows {
				// Real error occurred
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify rule: " + err.Error()})
				return
			}
			// If err == sql.ErrNoRows, it's a new rule with client-generated ID
		}

		// Now handle create or update based on whether the rule exists
		if !isExistingRule {
			// New rule - either with nil ID or client-generated ID that doesn't exist
			query := `INSERT INTO server_rules (id, server_id, parent_id, display_order, title, description) 
                      VALUES ($1, $2, $3, $4, $5, $6) 
                      RETURNING id, created_at, updated_at`

			// Use the client-provided ID if it exists, otherwise generate a new one
			ruleID := rule.ID
			if ruleID == uuid.Nil {
				ruleID = uuid.New()
			}

			err = tx.QueryRowContext(c.Request.Context(), query,
				ruleID, rule.ServerID, rule.ParentID, rule.DisplayOrder, rule.Title, rule.Description).
				Scan(&resultRule.ID, &resultRule.CreatedAt, &resultRule.UpdatedAt)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create rule: " + err.Error()})
				return
			}

			// Copy other fields
			resultRule.ServerID = rule.ServerID
			resultRule.ParentID = rule.ParentID
			resultRule.DisplayOrder = rule.DisplayOrder
			resultRule.Title = rule.Title
			resultRule.Description = rule.Description
		} else {
			// Existing rule - update it

			// Update rule
			updateQuery := `UPDATE server_rules SET 
                parent_id = $1, 
                display_order = $2, 
                title = $3, 
                description = $4, 
                updated_at = NOW() 
              WHERE id = $5 AND server_id = $6
              RETURNING updated_at`

			err = tx.QueryRowContext(c.Request.Context(), updateQuery,
				rule.ParentID, rule.DisplayOrder, rule.Title, rule.Description, rule.ID, serverID).
				Scan(&resultRule.UpdatedAt)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update rule: " + err.Error()})
				return
			}

			// Copy fields
			resultRule.ID = rule.ID
			resultRule.ServerID = rule.ServerID
			resultRule.ParentID = rule.ParentID
			resultRule.DisplayOrder = rule.DisplayOrder
			resultRule.Title = rule.Title
			resultRule.Description = rule.Description
		}

		// Process actions for this rule
		if len(rule.Actions) > 0 {
			// Delete existing actions for this rule
			_, err = tx.ExecContext(c.Request.Context(),
				"DELETE FROM server_rule_actions WHERE rule_id = $1", resultRule.ID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear actions: " + err.Error()})
				return
			}

			// Insert new actions
			var resultActions []models.ServerRuleAction
			for _, action := range rule.Actions {
				action.RuleID = resultRule.ID // Ensure correct rule ID

				actionQuery := `INSERT INTO server_rule_actions 
					(rule_id, violation_count, action_type, duration, message) 
					VALUES ($1, $2, $3, $4, $5) 
					RETURNING id, created_at, updated_at`

				var resultAction models.ServerRuleAction
				err = tx.QueryRowContext(c.Request.Context(), actionQuery,
					action.RuleID, action.ViolationCount, action.ActionType,
					action.Duration, action.Message).
					Scan(&resultAction.ID, &resultAction.CreatedAt, &resultAction.UpdatedAt)

				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create action: " + err.Error()})
					return
				}

				// Copy other fields
				resultAction.RuleID = action.RuleID
				resultAction.ViolationCount = action.ViolationCount
				resultAction.ActionType = action.ActionType
				resultAction.Duration = action.Duration
				resultAction.Message = action.Message

				resultActions = append(resultActions, resultAction)
			}
			resultRule.Actions = resultActions
		}

		updatedRules = append(updatedRules, &resultRule)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
		return
	}

	// Trigger MOTD auto-upload if enabled
	motdAutoUploaded := s.TriggerMOTDUploadIfEnabled(c.Request.Context(), serverID)

	c.JSON(http.StatusOK, gin.H{
		"rules":              updatedRules,
		"motd_auto_uploaded": motdAutoUploaded,
	})
}
