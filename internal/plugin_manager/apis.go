package plugin_manager

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/clickhouse"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/rcon_manager"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
)

// eventDataToMap converts EventData to map[string]interface{} for plugins
func eventDataToMap(data event_manager.EventData) map[string]interface{} {
	if data == nil {
		return make(map[string]interface{})
	}

	// Marshal to JSON and back to get a map representation
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal event data to JSON")
		return make(map[string]interface{})
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal JSON to map")
		return make(map[string]interface{})
	}

	return result
}

// serverAPI implements ServerAPI interface
type serverAPI struct {
	serverID    uuid.UUID
	db          *sql.DB
	rconManager *rcon_manager.RconManager
}

func NewServerAPI(serverID uuid.UUID, db *sql.DB, rconManager *rcon_manager.RconManager) ServerAPI {
	return &serverAPI{
		serverID:    serverID,
		db:          db,
		rconManager: rconManager,
	}
}

func (api *serverAPI) GetServerID() uuid.UUID {
	return api.serverID
}

func (api *serverAPI) GetServerInfo() (*ServerInfo, error) {
	// Get basic server info from database
	query := `
		SELECT id, name, ip_address, game_port, rcon_port
		FROM servers 
		WHERE id = $1
	`

	var id uuid.UUID
	var name string
	var ipAddress string
	var gamePort int
	var rconPort int

	err := api.db.QueryRow(query, api.serverID).Scan(&id, &name, &ipAddress, &gamePort, &rconPort)
	if err != nil {
		return nil, fmt.Errorf("failed to get server info from database: %w", err)
	}

	// Create squad-rcon instance to get live server data
	squadRcon := squadRcon.NewSquadRcon(api.rconManager, api.serverID)

	// Get live server info via RCON
	liveInfo, err := squadRcon.GetServerInfo()
	if err != nil {
		// If RCON fails, return basic info from database
		return &ServerInfo{
			ID:          id,
			Name:        name,
			Host:        ipAddress,
			Port:        gamePort,
			MaxPlayers:  0,
			CurrentMap:  "Unknown",
			GameMode:    "Unknown",
			PlayerCount: 0,
			Status:      "offline",
		}, nil
	}

	// Get current map info
	currentMap, err := squadRcon.GetCurrentMap()
	if err != nil {
		currentMap.Layer = "Unknown"
	}

	return &ServerInfo{
		ID:          id,
		Name:        name,
		Host:        ipAddress,
		Port:        gamePort,
		MaxPlayers:  liveInfo.MaxPlayers,
		CurrentMap:  currentMap.Layer,
		GameMode:    liveInfo.GameMode,
		PlayerCount: liveInfo.PlayerCount,
		Status:      "online",
	}, nil
}

func (api *serverAPI) GetPlayers() ([]*PlayerInfo, error) {
	// Create squad-rcon instance to get live player data
	squadRconInstance := squadRcon.NewSquadRcon(api.rconManager, api.serverID)

	// Get live player data via RCON
	playersData, err := squadRconInstance.GetServerPlayers()
	if err != nil {
		return nil, fmt.Errorf("failed to get players via RCON: %w", err)
	}

	players := make([]*PlayerInfo, 0, len(playersData.OnlinePlayers))

	// Convert squad-rcon players to plugin API format
	for _, player := range playersData.OnlinePlayers {
		players = append(players, &PlayerInfo{
			ID:            fmt.Sprintf("%d", player.Id),
			Name:          player.Name,
			SteamID:       player.SteamId,
			EOSID:         player.EosId,
			TeamID:        player.TeamId,
			SquadID:       player.SquadId,
			IsSquadLeader: player.IsSquadLeader,
			IsAdmin:       false, // We'll determine this separately
			IsOnline:      true,
		})
	}

	return players, nil
}

func (api *serverAPI) GetAdmins() ([]*AdminInfo, error) {
	// FIXME: will only grab admins that have a user account, not ones that are just defined by the steam id
	query := `
		SELECT sa.id, u.name, u.steam_id, sr.name as role
		FROM server_admins sa
		LEFT JOIN users u ON sa.user_id = u.id
		LEFT JOIN server_roles sr ON sa.server_role_id = sr.id
		WHERE sa.server_id = $1
	`

	rows, err := api.db.Query(query, api.serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to query admins: %w", err)
	}
	defer rows.Close()

	var admins []*AdminInfo
	adminSteamIDs := make(map[string]*AdminInfo)

	for rows.Next() {
		var admin AdminInfo
		var steamID sql.NullInt64
		var name sql.NullString
		var role sql.NullString

		err := rows.Scan(&admin.ID, &name, &steamID, &role)
		if err != nil {
			return nil, fmt.Errorf("failed to scan admin row: %w", err)
		}

		if name.Valid {
			admin.Name = name.String
		} else {
			admin.Name = "Unknown"
		}

		if steamID.Valid {
			admin.SteamID = fmt.Sprintf("%d", steamID.Int64)
		}

		if role.Valid {
			admin.Role = role.String
		} else {
			admin.Role = "Admin"
		}

		admin.IsOnline = false // Default to offline
		admins = append(admins, &admin)

		if admin.SteamID != "" {
			adminSteamIDs[admin.SteamID] = &admin
		}
	}

	// Get online players to determine which admins are online
	players, err := api.GetPlayers()
	if err == nil {
		for _, player := range players {
			if admin, exists := adminSteamIDs[player.SteamID]; exists {
				admin.IsOnline = true
			}
		}
	}

	return admins, nil
}

// adminAPI implements AdminAPI interface
type adminAPI struct {
	serverID         uuid.UUID
	db               *sql.DB
	rconManager      *rcon_manager.RconManager
	pluginInstanceID uuid.UUID
}

func NewAdminAPI(serverID uuid.UUID, db *sql.DB, rconManager *rcon_manager.RconManager, pluginInstanceID uuid.UUID) AdminAPI {
	return &adminAPI{
		serverID:         serverID,
		db:               db,
		rconManager:      rconManager,
		pluginInstanceID: pluginInstanceID,
	}
}

func (api *adminAPI) AddTemporaryAdmin(steamID string, roleName string, notes string, expiresAt *time.Time) error {
	// First, get or create the role for this server
	var roleID uuid.UUID
	err := api.db.QueryRow(`
		SELECT id FROM server_roles 
		WHERE server_id = $1 AND name = $2
	`, api.serverID, roleName).Scan(&roleID)

	if err == sql.ErrNoRows {
		// Role doesn't exist, create it with basic permissions
		roleID = uuid.New()
		_, err = api.db.Exec(`
			INSERT INTO server_roles (id, server_id, name, permissions, created_at)
			VALUES ($1, $2, $3, $4, NOW())
		`, roleID, api.serverID, roleName, "reserve")
		if err != nil {
			return fmt.Errorf("failed to create role %s: %w", roleName, err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to query role %s: %w", roleName, err)
	}

	// Parse steamID to int64 for database storage
	steamIDInt, err := strconv.ParseInt(steamID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid steam ID format: %w", err)
	}

	// Check if admin record already exists
	var existingID uuid.UUID
	err = api.db.QueryRow(`
		SELECT id FROM server_admins 
		WHERE server_id = $1 AND steam_id = $2 AND server_role_id = $3
	`, api.serverID, steamIDInt, roleID).Scan(&existingID)

	if err == sql.ErrNoRows {
		// Create new admin record
		adminID := uuid.New()
		_, err = api.db.Exec(`
			INSERT INTO server_admins (id, server_id, steam_id, server_role_id, expires_at, notes, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW())
		`, adminID, api.serverID, steamIDInt, roleID, expiresAt, notes)
		if err != nil {
			return fmt.Errorf("failed to create admin record: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check existing admin record: %w", err)
	} else {
		// Update existing admin record
		_, err = api.db.Exec(`
			UPDATE server_admins 
			SET expires_at = $1, notes = $2
			WHERE id = $3
		`, expiresAt, notes, existingID)
		if err != nil {
			return fmt.Errorf("failed to update admin record: %w", err)
		}
	}

	// Also add via RCON for immediate effect
	rconCommand := fmt.Sprintf("AdminAdd %s %s", steamID, roleName)
	if _, err := api.rconManager.ExecuteCommand(api.serverID, rconCommand); err != nil {
		// Log but don't fail - database record is created
		log.Debug().Err(err).
			Str("steam_id", steamID).
			Str("role", roleName).
			Msg("RCON AdminAdd command failed, relying on database record only")
	}

	return nil
}

func (api *adminAPI) RemoveTemporaryAdmin(steamID string, notes string) error {
	// Parse steamID to int64 for database query
	steamIDInt, err := strconv.ParseInt(steamID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid steam ID format: %w", err)
	}

	// Find and remove admin records for this steam ID
	rows, err := api.db.Query(`
		SELECT id FROM server_admins 
		WHERE server_id = $1 AND steam_id = $2
	`, api.serverID, steamIDInt)
	if err != nil {
		return fmt.Errorf("failed to query admin records: %w", err)
	}
	defer rows.Close()

	var adminIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed to scan admin ID: %w", err)
		}
		adminIDs = append(adminIDs, id)
	}

	if len(adminIDs) == 0 {
		return fmt.Errorf("no admin records found for steam ID %s", steamID)
	}

	// Remove admin records
	for _, adminID := range adminIDs {
		_, err = api.db.Exec(`DELETE FROM server_admins WHERE id = $1`, adminID)
		if err != nil {
			return fmt.Errorf("failed to remove admin record %s: %w", adminID, err)
		}
	}

	// Also remove via RCON for immediate effect
	rconCommand := fmt.Sprintf("AdminRemove %s", steamID)
	if _, err := api.rconManager.ExecuteCommand(api.serverID, rconCommand); err != nil {
		// Log but don't fail - database record is removed
		log.Debug().Err(err).
			Str("steam_id", steamID).
			Msg("RCON AdminRemove command failed, relying on database record removal only")
	}

	return nil
}

func (api *adminAPI) GetPlayerAdminStatus(steamID string) (*PlayerAdminStatus, error) {
	// Parse steamID to int64 for database query
	steamIDInt, err := strconv.ParseInt(steamID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid steam ID format: %w", err)
	}

	// Query admin roles for this player
	rows, err := api.db.Query(`
		SELECT sa.id, sr.name, sa.notes, sa.expires_at, sa.created_at
		FROM server_admins sa
		JOIN server_roles sr ON sa.server_role_id = sr.id
		WHERE sa.server_id = $1 AND sa.steam_id = $2
	`, api.serverID, steamIDInt)
	if err != nil {
		return nil, fmt.Errorf("failed to query admin status: %w", err)
	}
	defer rows.Close()

	var roles []*PlayerAdminRole
	hasExpiring := false

	for rows.Next() {
		var role PlayerAdminRole
		var expiresAt sql.NullTime
		var notes sql.NullString
		var createdAt time.Time

		err := rows.Scan(&role.ID, &role.RoleName, &notes, &expiresAt, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan admin role: %w", err)
		}

		if notes.Valid {
			role.Notes = notes.String
		}

		if expiresAt.Valid {
			role.ExpiresAt = &expiresAt.Time
			role.IsExpired = expiresAt.Time.Before(time.Now())
			if !role.IsExpired {
				hasExpiring = true
			}
		}

		roles = append(roles, &role)
	}

	status := &PlayerAdminStatus{
		SteamID:     steamID,
		IsAdmin:     len(roles) > 0,
		Roles:       roles,
		HasExpiring: hasExpiring,
	}

	return status, nil
}

func (api *adminAPI) ListTemporaryAdmins() ([]*TemporaryAdminInfo, error) {
	// Query all temporary admins for this server with notes containing plugin information
	rows, err := api.db.Query(`
		SELECT sa.id, sa.steam_id, sr.name as role_name, sa.notes, sa.expires_at, sa.created_at
		FROM server_admins sa
		JOIN server_roles sr ON sa.server_role_id = sr.id
		WHERE sa.server_id = $1 AND sa.notes IS NOT NULL AND sa.notes LIKE '%Plugin:%'
		ORDER BY sa.created_at DESC
	`, api.serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to query temporary admins: %w", err)
	}
	defer rows.Close()

	var admins []*TemporaryAdminInfo
	for rows.Next() {
		var admin TemporaryAdminInfo
		var steamIDInt int64
		var notes sql.NullString
		var expiresAt sql.NullTime

		err := rows.Scan(&admin.ID, &steamIDInt, &admin.RoleName, &notes, &expiresAt, &admin.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan temporary admin: %w", err)
		}

		admin.SteamID = fmt.Sprintf("%d", steamIDInt)

		if notes.Valid {
			admin.Notes = notes.String
		}

		if expiresAt.Valid {
			admin.ExpiresAt = &expiresAt.Time
			admin.IsExpired = expiresAt.Time.Before(time.Now())
		}

		admins = append(admins, &admin)
	}

	return admins, nil
}

// databaseAPI implements DatabaseAPI interface
type databaseAPI struct {
	instanceID uuid.UUID
	db         *sql.DB
}

func NewDatabaseAPI(instanceID uuid.UUID, db *sql.DB) DatabaseAPI {
	return &databaseAPI{
		instanceID: instanceID,
		db:         db,
	}
}

func (api *databaseAPI) ExecuteQuery(query string, args ...interface{}) (*sql.Rows, error) {
	// Only allow SELECT queries for security
	trimmedQuery := strings.TrimSpace(strings.ToUpper(query))
	if !strings.HasPrefix(trimmedQuery, "SELECT") {
		return nil, fmt.Errorf("only SELECT queries are allowed")
	}

	// Prevent certain dangerous operations
	if strings.Contains(trimmedQuery, "DROP") ||
		strings.Contains(trimmedQuery, "DELETE") ||
		strings.Contains(trimmedQuery, "INSERT") ||
		strings.Contains(trimmedQuery, "UPDATE") ||
		strings.Contains(trimmedQuery, "ALTER") ||
		strings.Contains(trimmedQuery, "CREATE") {
		return nil, fmt.Errorf("query contains prohibited operations")
	}

	return api.db.Query(query, args...)
}

func (api *databaseAPI) GetPluginData(key string) (string, error) {
	query := `SELECT value FROM plugin_data WHERE plugin_instance_id = $1 AND key = $2`

	var value string
	err := api.db.QueryRow(query, api.instanceID, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("key not found")
	}
	if err != nil {
		return "", fmt.Errorf("failed to get plugin data: %w", err)
	}

	return value, nil
}

func (api *databaseAPI) SetPluginData(key string, value string) error {
	query := `
		INSERT INTO plugin_data (plugin_instance_id, key, value, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (plugin_instance_id, key)
		DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`

	_, err := api.db.Exec(query, api.instanceID, key, value)
	if err != nil {
		return fmt.Errorf("failed to set plugin data: %w", err)
	}

	return nil
}

func (api *databaseAPI) DeletePluginData(key string) error {
	query := `DELETE FROM plugin_data WHERE plugin_instance_id = $1 AND key = $2`

	_, err := api.db.Exec(query, api.instanceID, key)
	if err != nil {
		return fmt.Errorf("failed to delete plugin data: %w", err)
	}

	return nil
}

// rconAPI implements RconAPI interface
type rconAPI struct {
	serverID    uuid.UUID
	rconManager *rcon_manager.RconManager
}

func NewRconAPI(serverID uuid.UUID, rconManager *rcon_manager.RconManager) RconAPI {
	return &rconAPI{
		serverID:    serverID,
		rconManager: rconManager,
	}
}

func (api *rconAPI) SendCommand(command string) (string, error) {
	// Extract command name (first word)
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	// Execute command via RCON manager
	response, err := api.rconManager.ExecuteCommand(api.serverID, command)
	if err != nil {
		return "", fmt.Errorf("failed to execute RCON command: %w", err)
	}

	return response, nil
}

func (api *rconAPI) Broadcast(message string) error {
	command := fmt.Sprintf("AdminBroadcast %s", message)
	_, err := api.rconManager.ExecuteCommand(api.serverID, command)
	if err != nil {
		return fmt.Errorf("failed to send broadcast message: %w", err)
	}
	return nil
}

func (api *rconAPI) SendWarningToPlayer(playerID string, message string) error {
	command := fmt.Sprintf("AdminWarn \"%s\" %s", playerID, message)
	_, err := api.rconManager.ExecuteCommand(api.serverID, command)
	if err != nil {
		return fmt.Errorf("failed to send warning to player: %w", err)
	}
	return nil
}

func (api *rconAPI) KickPlayer(playerID string, reason string) error {
	command := fmt.Sprintf("AdminKick \"%s\" %s", playerID, reason)
	_, err := api.rconManager.ExecuteCommand(api.serverID, command)
	if err != nil {
		return fmt.Errorf("failed to kick player: %w", err)
	}
	return nil
}

func (api *rconAPI) BanPlayer(playerID string, reason string, duration time.Duration) error {
	// Convert duration to days for Squad's ban system
	durationDays := int(duration.Hours() / 24)
	if durationDays < 1 {
		durationDays = 1 // Minimum 1 day
	}

	command := fmt.Sprintf("AdminBan \"%s\" %dd %s", playerID, durationDays, reason)
	_, err := api.rconManager.ExecuteCommand(api.serverID, command)
	if err != nil {
		return fmt.Errorf("failed to ban player: %w", err)
	}

	// FIXME: Need to extend logic here to support updating the DB with ban info

	return nil
}

// eventAPI implements EventAPI interface
type eventAPI struct {
	serverID     uuid.UUID
	eventManager *event_manager.EventManager
}

func NewEventAPI(serverID uuid.UUID, eventManager *event_manager.EventManager) EventAPI {
	return &eventAPI{
		serverID:     serverID,
		eventManager: eventManager,
	}
}

func (api *eventAPI) PublishEvent(eventType string, data map[string]interface{}, raw string) error {
	// Create custom event type for plugin events
	// FIXME: Implement publishing custom events from plugins
	// pluginEventType := event_manager.EventType(fmt.Sprintf("PLUGIN_%s", strings.ToUpper(eventType)))

	// api.eventManager.PublishEventLegacy(api.serverID, pluginEventType, data, raw)
	return nil
}

func (api *eventAPI) SubscribeToEvents(eventTypes []string, handler func(*PluginEvent)) error {
	// Convert string event types to EventType
	var filterTypes []event_manager.EventType
	for _, eventType := range eventTypes {
		filterTypes = append(filterTypes, event_manager.EventType(eventType))
	}

	// Create filter
	filter := event_manager.EventFilter{
		Types: filterTypes,
	}

	// Subscribe to events
	subscriber := api.eventManager.Subscribe(filter, &api.serverID, 100)

	// Start goroutine to handle events
	go func() {
		for event := range subscriber.Channel {
			rawString := ""
			if event.RawData != nil {
				if str, ok := event.RawData.(string); ok {
					rawString = str
				}
			}

			pluginEvent := &PluginEvent{
				ID:        event.ID,
				ServerID:  event.ServerID,
				Source:    EventSourceSystem,
				Type:      string(event.Type),
				Data:      event.Data,
				Raw:       rawString,
				Timestamp: event.Timestamp,
			}
			handler(pluginEvent)
		}
	}()

	return nil
}

// connectorAPI implements ConnectorAPI interface
type connectorAPI struct {
	pluginManager *PluginManager
}

func NewConnectorAPI(pluginManager *PluginManager) ConnectorAPI {
	return &connectorAPI{
		pluginManager: pluginManager,
	}
}

func (api *connectorAPI) GetConnector(connectorID string) (interface{}, error) {
	return api.pluginManager.GetConnectorAPI(connectorID)
}

func (api *connectorAPI) ListConnectors() []string {
	return api.pluginManager.ListAvailableConnectors()
}

// logAPI implements LogAPI interface
type logAPI struct {
	serverID         uuid.UUID
	instanceID       uuid.UUID
	clickhouseClient *clickhouse.Client
	db               *sql.DB
}

func NewLogAPI(serverID, instanceID uuid.UUID, clickhouseClient *clickhouse.Client, db *sql.DB) LogAPI {
	return &logAPI{
		serverID:         serverID,
		instanceID:       instanceID,
		clickhouseClient: clickhouseClient,
		db:               db,
	}
}

// Helper function to write log to ClickHouse
func (api *logAPI) writeToClickHouse(level, message string, errorMsg *string, fields map[string]interface{}) {
	// Marshal fields to JSON
	fieldsJSON := "{}"
	if len(fields) > 0 {
		if jsonBytes, err := json.Marshal(fields); err == nil {
			fieldsJSON = string(jsonBytes)
		}
	}

	// Insert into ClickHouse
	insertQuery := `
		INSERT INTO squad_aegis.plugin_logs (
			timestamp, server_id, plugin_instance_id, 
			level, message, error_message, fields
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := api.clickhouseClient.Exec(ctx, insertQuery,
		time.Now().UTC(),
		api.serverID,
		api.instanceID,
		level,
		message,
		errorMsg,
		fieldsJSON,
	); err != nil {
		log.Error().Err(err).Msg("Failed to write plugin log to ClickHouse")
	}
}

func (api *logAPI) Info(message string, fields map[string]interface{}) {
	logger := log.Info().
		Str("serverID", api.serverID.String()).
		Str("pluginInstanceID", api.instanceID.String())

	for key, value := range fields {
		logger = logger.Interface(key, value)
	}

	logger.Msg(message)

	// Also write to ClickHouse
	api.writeToClickHouse("info", message, nil, fields)
}

func (api *logAPI) Warn(message string, fields map[string]interface{}) {
	logger := log.Warn().
		Str("serverID", api.serverID.String()).
		Str("pluginInstanceID", api.instanceID.String())

	for key, value := range fields {
		logger = logger.Interface(key, value)
	}

	logger.Msg(message)

	// Also write to ClickHouse
	api.writeToClickHouse("warn", message, nil, fields)
}

func (api *logAPI) Error(message string, err error, fields map[string]interface{}) {
	logger := log.Error().
		Str("serverID", api.serverID.String()).
		Str("pluginInstanceID", api.instanceID.String())

	if err != nil {
		logger = logger.Err(err)
	}

	for key, value := range fields {
		logger = logger.Interface(key, value)
	}

	logger.Msg(message)

	// Also write to ClickHouse
	var errorMsg *string
	if err != nil {
		errStr := err.Error()
		errorMsg = &errStr
	}
	api.writeToClickHouse("error", message, errorMsg, fields)
}

func (api *logAPI) Debug(message string, fields map[string]interface{}) {
	logger := log.Debug().
		Str("serverID", api.serverID.String()).
		Str("pluginInstanceID", api.instanceID.String())

	for key, value := range fields {
		logger = logger.Interface(key, value)
	}

	logger.Msg(message)

	// Also write to ClickHouse
	api.writeToClickHouse("debug", message, nil, fields)
}
