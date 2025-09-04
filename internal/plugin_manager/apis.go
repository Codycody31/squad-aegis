package plugin_manager

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/rcon_manager"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
)

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
			ID:       fmt.Sprintf("%d", player.Id),
			Name:     player.Name,
			SteamID:  player.SteamId,
			EOSID:    player.EosId,
			TeamID:   player.TeamId,
			SquadID:  player.SquadId,
			IsAdmin:  false, // We'll determine this separately
			IsOnline: true,
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

func (api *databaseAPI) GetPluginData(pluginInstanceID uuid.UUID, key string) (string, error) {
	if pluginInstanceID != api.instanceID {
		return "", fmt.Errorf("access denied: can only access own plugin data")
	}

	query := `SELECT value FROM plugin_data WHERE plugin_instance_id = $1 AND key = $2`

	var value string
	err := api.db.QueryRow(query, pluginInstanceID, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("key not found")
	}
	if err != nil {
		return "", fmt.Errorf("failed to get plugin data: %w", err)
	}

	return value, nil
}

func (api *databaseAPI) SetPluginData(pluginInstanceID uuid.UUID, key string, value string) error {
	if pluginInstanceID != api.instanceID {
		return fmt.Errorf("access denied: can only modify own plugin data")
	}

	query := `
		INSERT INTO plugin_data (plugin_instance_id, key, value, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (plugin_instance_id, key)
		DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`

	_, err := api.db.Exec(query, pluginInstanceID, key, value)
	if err != nil {
		return fmt.Errorf("failed to set plugin data: %w", err)
	}

	return nil
}

func (api *databaseAPI) DeletePluginData(pluginInstanceID uuid.UUID, key string) error {
	if pluginInstanceID != api.instanceID {
		return fmt.Errorf("access denied: can only delete own plugin data")
	}

	query := `DELETE FROM plugin_data WHERE plugin_instance_id = $1 AND key = $2`

	_, err := api.db.Exec(query, pluginInstanceID, key)
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

// List of allowed RCON commands for plugins
var allowedCommands = map[string]bool{
	"AdminBroadcast":         true,
	"AdminDemotePlayer":      true,
	"AdminPromotePlayer":     true,
	"AdminKick":              true,
	"AdminKickById":          true,
	"AdminBan":               true,
	"AdminBanById":           true,
	"AdminListPlayers":       true,
	"AdminListAdmins":        true,
	"ShowCurrentMap":         true,
	"ShowNextMap":            true,
	"AdminCurrentMap":        true,
	"AdminNextMap":           true,
	"AdminSetNextMap":        true,
	"AdminChangeMap":         true,
	"AdminSetMaxNumPlayers":  true,
	"AdminSetServerPassword": true,
	"AdminPause":             true,
	"AdminUnpause":           true,
	"AdminRestartMatch":      true,
	"AdminEndMatch":          true,
}

func (api *rconAPI) SendCommand(command string) (string, error) {
	// Extract command name (first word)
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	commandName := parts[0]
	if !allowedCommands[commandName] {
		return "", fmt.Errorf("command %s is not allowed for plugins", commandName)
	}

	// Execute command via RCON manager
	response, err := api.rconManager.ExecuteCommand(api.serverID, command)
	if err != nil {
		return "", fmt.Errorf("failed to execute RCON command: %w", err)
	}

	return response, nil
}

func (api *rconAPI) SendMessage(message string) error {
	command := fmt.Sprintf("AdminBroadcast %s", message)
	_, err := api.rconManager.ExecuteCommand(api.serverID, command)
	if err != nil {
		return fmt.Errorf("failed to send broadcast message: %w", err)
	}
	return nil
}

func (api *rconAPI) SendMessageToPlayer(playerID string, message string) error {
	command := fmt.Sprintf("AdminWarn \"%s\" %s", playerID, message)
	_, err := api.rconManager.ExecuteCommand(api.serverID, command)
	if err != nil {
		return fmt.Errorf("failed to send message to player: %w", err)
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
	pluginEventType := event_manager.EventType(fmt.Sprintf("PLUGIN_%s", strings.ToUpper(eventType)))

	api.eventManager.PublishEvent(api.serverID, pluginEventType, data, raw)
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
	serverID   uuid.UUID
	instanceID uuid.UUID
}

func NewLogAPI(serverID, instanceID uuid.UUID) LogAPI {
	return &logAPI{
		serverID:   serverID,
		instanceID: instanceID,
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
}

func (api *logAPI) Warn(message string, fields map[string]interface{}) {
	logger := log.Warn().
		Str("serverID", api.serverID.String()).
		Str("pluginInstanceID", api.instanceID.String())

	for key, value := range fields {
		logger = logger.Interface(key, value)
	}

	logger.Msg(message)
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
}

func (api *logAPI) Debug(message string, fields map[string]interface{}) {
	logger := log.Debug().
		Str("serverID", api.serverID.String()).
		Str("pluginInstanceID", api.instanceID.String())

	for key, value := range fields {
		logger = logger.Interface(key, value)
	}

	logger.Msg(message)
}
