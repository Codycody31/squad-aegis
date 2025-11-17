package server

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// ServerEventsSearch handles searching for events in ClickHouse for evidence
func (s *Server) ServerEventsSearch(c *gin.Context) {
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

	// Get query parameters
	steamID := c.Query("steam_id")
	eventType := c.Query("event_type")
	limitStr := c.DefaultQuery("limit", "50")

	if steamID == "" {
		responses.BadRequest(c, "steam_id is required", nil)
		return
	}

	if eventType == "" {
		responses.BadRequest(c, "event_type is required", nil)
		return
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 50
	}

	// Convert Steam ID to uint64 for ClickHouse queries
	steamIDInt, err := strconv.ParseUint(steamID, 10, 64)
	if err != nil {
		responses.BadRequest(c, "Invalid Steam ID format", &gin.H{"error": "Steam ID must be a valid 64-bit integer"})
		return
	}

	// Map event type to ClickHouse table and build query
	var query string
	var tableName string

	switch eventType {
	case "player_died":
		tableName = "server_player_died_events"
		query = fmt.Sprintf(`
			SELECT 
				chain_id as record_id,
				event_time,
				victim_name,
				weapon,
				attacker_player_controller,
				attacker_eos,
				toString(attacker_steam) as attacker_steam,
				teamkill,
				damage
			FROM squad_aegis.%s
			WHERE server_id = ? AND attacker_steam = ?
			ORDER BY event_time DESC
			LIMIT ?
		`, tableName)

	case "player_wounded":
		tableName = "server_player_wounded_events"
		query = fmt.Sprintf(`
			SELECT 
				chain_id as record_id,
				event_time,
				victim_name,
				weapon,
				attacker_player_controller,
				attacker_eos,
				toString(attacker_steam) as attacker_steam,
				teamkill,
				damage
			FROM squad_aegis.%s
			WHERE server_id = ? AND attacker_steam = ?
			ORDER BY event_time DESC
			LIMIT ?
		`, tableName)

	case "player_damaged":
		tableName = "server_player_damaged_events"
		query = fmt.Sprintf(`
			SELECT 
				chain_id as record_id,
				event_time,
				victim_name,
				weapon,
				attacker_controller,
				attacker_eos,
				toString(attacker_steam) as attacker_steam,
				damage
			FROM squad_aegis.%s
			WHERE server_id = ? AND attacker_steam = ?
			ORDER BY event_time DESC
			LIMIT ?
		`, tableName)

	case "chat_message":
		tableName = "server_player_chat_messages"
		query = fmt.Sprintf(`
			SELECT 
				toString(message_id) as record_id,
				sent_at as event_time,
				player_name,
				chat_type,
				message,
				steam_id,
				eos_id
			FROM squad_aegis.%s
			WHERE server_id = ? AND steam_id = ?
			ORDER BY sent_at DESC
			LIMIT ?
		`, tableName)

	case "player_connected":
		tableName = "server_player_connected_events"
		query = fmt.Sprintf(`
			SELECT 
				chain_id as record_id,
				event_time,
				player_controller,
				ip,
				steam,
				eos
			FROM squad_aegis.%s
			WHERE server_id = ? AND steam = ?
			ORDER BY event_time DESC
			LIMIT ?
		`, tableName)

	default:
		responses.BadRequest(c, "Invalid event type", &gin.H{
			"error":        "event_type must be one of: player_died, player_wounded, player_damaged, chat_message, player_connected",
			"provided":     eventType,
			"valid_types": []string{"player_died", "player_wounded", "player_damaged", "chat_message", "player_connected"},
		})
		return
	}

	// Execute the query
	rows, err := s.Dependencies.Clickhouse.Query(c.Request.Context(), query, serverId.String(), steamIDInt, limit)
	if err != nil {
		responses.BadRequest(c, "Failed to query events", &gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	// Parse results based on event type
	events := []map[string]interface{}{}

	for rows.Next() {
		event := make(map[string]interface{})

		switch eventType {
		case "player_died", "player_wounded":
			var recordID, victimName, weapon, attackerController, attackerEos, attackerSteam string
			var eventTime string
			var teamkill uint8
			var damage float32

			err := rows.Scan(&recordID, &eventTime, &victimName, &weapon, &attackerController, &attackerEos, &attackerSteam, &teamkill, &damage)
			if err != nil {
				continue
			}

			event["event_id"] = recordID // For frontend compatibility
			event["record_id"] = recordID // Actual chain_id
			event["event_time"] = eventTime
			event["victim_name"] = victimName
			event["weapon"] = weapon
			event["attacker_player_controller"] = attackerController
			event["attacker_eos"] = attackerEos
			event["attacker_steam"] = attackerSteam
			event["teamkill"] = teamkill == 1
			event["damage"] = damage

		case "player_damaged":
			var recordID, victimName, weapon, attackerController, attackerEos, attackerSteam string
			var eventTime string
			var damage float32

			err := rows.Scan(&recordID, &eventTime, &victimName, &weapon, &attackerController, &attackerEos, &attackerSteam, &damage)
			if err != nil {
				continue
			}

			event["event_id"] = recordID // For frontend compatibility
			event["record_id"] = recordID // Actual chain_id
			event["event_time"] = eventTime
			event["victim_name"] = victimName
			event["weapon"] = weapon
			event["attacker_controller"] = attackerController
			event["attacker_eos"] = attackerEos
			event["attacker_steam"] = attackerSteam
			event["damage"] = damage

		case "chat_message":
			var recordID, playerName, chatType, message, eosID string
			var eventTime string
			var steamIDVal uint64

			err := rows.Scan(&recordID, &eventTime, &playerName, &chatType, &message, &steamIDVal, &eosID)
			if err != nil {
				continue
			}

			event["message_id"] = recordID // For frontend compatibility
			event["event_id"] = recordID    // Also include event_id
			event["record_id"] = recordID   // Actual message_id
			event["sent_at"] = eventTime
			event["event_time"] = eventTime // Also include event_time
			event["player_name"] = playerName
			event["chat_type"] = chatType
			event["message"] = message
			event["steam_id"] = steamIDVal
			event["eos_id"] = eosID

		case "player_connected":
			var recordID, playerController, ip, steam, eos string
			var eventTime string

			err := rows.Scan(&recordID, &eventTime, &playerController, &ip, &steam, &eos)
			if err != nil {
				continue
			}

			event["event_id"] = recordID // For frontend compatibility
			event["record_id"] = recordID // Actual chain_id
			event["event_time"] = eventTime
			event["player_controller"] = playerController
			event["ip"] = ip
			event["steam"] = steam
			event["eos"] = eos
		}

		events = append(events, event)
	}

	responses.Success(c, "Events fetched successfully", &gin.H{
		"events":     events,
		"count":      len(events),
		"event_type": eventType,
		"steam_id":   steamID,
	})
}

