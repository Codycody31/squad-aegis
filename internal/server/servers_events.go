package server

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	"go.codycody31.dev/squad-aegis/internal/shared/utils"
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

	// Get query parameters — accept either steam_id or eos_id
	steamID := c.Query("steam_id")
	eosID := utils.NormalizeEOSID(c.Query("eos_id"))
	eventType := c.Query("event_type")
	limitStr := c.DefaultQuery("limit", "50")

	if steamID == "" && eosID == "" {
		responses.BadRequest(c, "steam_id or eos_id is required", nil)
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

	// Determine which identifier to filter on
	useSteamID := steamID != ""
	var steamIDInt uint64
	if useSteamID {
		parsed, parseErr := strconv.ParseUint(steamID, 10, 64)
		if parseErr != nil {
			responses.BadRequest(c, "Invalid Steam ID format", &gin.H{"error": "Steam ID must be a valid 64-bit integer"})
			return
		}
		steamIDInt = parsed
	}

	// Map event type to ClickHouse table and build query
	var query string
	var tableName string

	switch eventType {
	case "chat_message":
		tableName = "server_player_chat_messages"
		filterCol := "steam_id = ?"
		if !useSteamID {
			filterCol = "eos_id = ?"
		}
		query = fmt.Sprintf(`
			SELECT
				message_id,
				sent_at as event_time,
				player_name,
				chat_type,
				message,
				steam_id,
				eos_id
			FROM squad_aegis.%s
			WHERE server_id = ? AND %s
			ORDER BY sent_at DESC
			LIMIT ?
		`, tableName, filterCol)

	case "player_connected":
		tableName = "server_player_connected_events"
		filterCol := "pc.steam = ?"
		if !useSteamID {
			filterCol = "pc.eos = ?"
		}
		query = fmt.Sprintf(`
			SELECT
				pc.id,
				pc.event_time,
				pc.player_controller,
				pc.ip,
				pc.steam,
				pc.eos,
				pc.chain_id,
				argMin(js.player_suffix, abs(toUnixTimestamp(pc.event_time) - toUnixTimestamp(js.event_time))) as joined_name
			FROM squad_aegis.%s pc
			LEFT JOIN squad_aegis.server_join_succeeded_events js
				ON pc.server_id = js.server_id
				AND pc.chain_id = js.chain_id
				AND (
					(pc.steam IS NOT NULL AND pc.steam != '' AND pc.steam = js.steam)
					OR (pc.eos IS NOT NULL AND pc.eos != '' AND pc.eos = js.eos)
				)
				AND abs(toUnixTimestamp(pc.event_time) - toUnixTimestamp(js.event_time)) <= 10
			WHERE pc.server_id = ? AND %s
			GROUP BY pc.id, pc.event_time, pc.player_controller, pc.ip, pc.steam, pc.eos, pc.chain_id
			ORDER BY pc.event_time DESC
			LIMIT ?
		`, tableName, filterCol)

	default:
		responses.BadRequest(c, "Invalid event type", &gin.H{
			"error":       "event_type must be one of: chat_message, player_connected",
			"provided":    eventType,
			"valid_types": []string{"chat_message", "player_connected"},
		})
		return
	}

	// Build query args based on which identifier is being used
	var queryArgs []interface{}
	if useSteamID {
		if eventType == "player_connected" {
			// player_connected.steam is a string column
			queryArgs = []interface{}{serverId.String(), strconv.FormatUint(steamIDInt, 10), limit}
		} else {
			// chat_messages.steam_id is UInt64
			queryArgs = []interface{}{serverId.String(), steamIDInt, limit}
		}
	} else {
		queryArgs = []interface{}{serverId.String(), eosID, limit}
	}

	rows, err := s.Dependencies.Clickhouse.Query(c.Request.Context(), query, queryArgs...)
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
		case "chat_message":
			var messageID uuid.UUID
			var playerName, chatType, message, eosID string
			var eventTime string
			var steamIDVal uint64

			err := rows.Scan(&messageID, &eventTime, &playerName, &chatType, &message, &steamIDVal, &eosID)
			if err != nil {
				log.Error().Err(err).Msg("Failed to scan row")
				continue
			}

			recordID := messageID.String()
			event["message_id"] = recordID // For frontend compatibility
			event["event_id"] = recordID   // Also include event_id
			event["record_id"] = recordID  // Actual message_id UUID
			event["sent_at"] = eventTime
			event["event_time"] = eventTime // Also include event_time
			event["player_name"] = playerName
			event["chat_type"] = chatType
			event["message"] = message
			event["steam_id"] = strconv.FormatUint(steamIDVal, 10)
			event["eos_id"] = eosID

		case "player_connected":
			var id uuid.UUID
			var playerController, ip, steam, eos, chainID string
			var eventTime string
			var joinedName sql.NullString

			err := rows.Scan(&id, &eventTime, &playerController, &ip, &steam, &eos, &chainID, &joinedName)
			if err != nil {
				log.Error().Err(err).Msg("Failed to scan row")
				continue
			}

			recordID := id.String()
			event["event_id"] = recordID  // For frontend compatibility
			event["record_id"] = recordID // Actual id UUID
			event["event_time"] = eventTime
			event["player_controller"] = playerController
			event["ip"] = ip
			event["steam"] = steam
			event["eos"] = eos
			event["chain_id"] = chainID
			if joinedName.Valid {
				event["joined_name"] = joinedName.String
			}
		}

		events = append(events, event)
	}

	responseData := gin.H{
		"events":     events,
		"count":      len(events),
		"event_type": eventType,
	}
	if steamID != "" {
		responseData["steam_id"] = steamID
	}
	if eosID != "" {
		responseData["eos_id"] = eosID
	}

	responses.Success(c, "Events fetched successfully", &responseData)
}
