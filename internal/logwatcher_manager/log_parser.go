package logwatcher_manager

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
)

// LogParser represents a log parser with a regex and a handler function
type LogParser struct {
	regex   *regexp.Regexp
	onMatch func([]string, uuid.UUID, *event_manager.EventManager, *EventStore)
}

// GetLogParsers returns all log parsers for Squad logs
func GetLogParsers() []LogParser {
	return []LogParser{
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: ADMIN COMMAND: Message broadcasted <(.+)> from (.+)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {

				eventData := map[string]interface{}{
					"time":    args[1],
					"chainID": args[2],
					"message": args[3],
					"from":    args[4],
				}

				eventManager.PublishEvent(serverID, event_manager.EventTypeLogAdminBroadcast, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQDeployable::)?TakeDamage\(\): ([A-z0-9_]+)_C_[0-9]+: ([0-9.]+) damage attempt by causer ([A-z0-9_]+)_C_[0-9]+ instigator (.+) with damage type ([A-z0-9_]+)_C health remaining ([0-9.]+)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {

				eventData := map[string]interface{}{
					"time":            args[1],
					"chainID":         args[2],
					"deployable":      args[3],
					"damage":          args[4],
					"weapon":          args[5],
					"playerSuffix":    args[6],
					"damageType":      args[7],
					"healthRemaining": args[8],
				}

				eventManager.PublishEvent(serverID, event_manager.EventTypeLogDeployableDamaged, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: PostLogin: NewPlayer: BP_PlayerController_C .+PersistentLevel\.([^\s]+) \(IP: ([\d.]+) \| Online IDs:([^)|]+)\)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {

				// Parse online IDs from the log
				idsString := args[5]
				ids := make(map[string]string)

				// Split IDs by commas and extract platform:id pairs
				idPairs := strings.Split(idsString, ",")
				for _, pair := range idPairs {
					pair = strings.TrimSpace(pair)
					parts := strings.Split(pair, ":")
					if len(parts) == 2 {
						platform := strings.ToLower(strings.TrimSpace(parts[0]))
						id := strings.TrimSpace(parts[1])
						ids[platform] = id
					}
				}

				// Build player data
				player := map[string]interface{}{
					"playercontroller": args[3],
					"ip":               args[4],
				}

				// Add all IDs to player data
				for platform, id := range ids {
					player[platform] = id
				}

				// Get EOS ID if available, otherwise use Steam ID as fallback
				playerID := ""
				if eosID, ok := ids["eos"]; ok {
					playerID = eosID
				} else if steamID, ok := ids["steam"]; ok {
					playerID = steamID
				}

				if playerID != "" {
					// Store player data in event store
					eventStore.StoreJoinRequest(args[2], player)
					eventStore.StorePlayerData(playerID, player)

					// Handle reconnecting players
					eventStore.RemoveDisconnectedPlayer(playerID)
				}

				// Build event data
				eventData := map[string]interface{}{
					"time":             args[1],
					"chainID":          args[2],
					"playercontroller": args[3],
					"ip":               args[4],
				}

				// Add all IDs to event data
				for platform, id := range ids {
					eventData[platform] = id
				}

				eventManager.PublishEvent(serverID, event_manager.EventTypeLogPlayerConnected, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadGameEvents: Display: Team ([0-9]), (.*) \( ?(.*?) ?\) has (won|lost) the match with ([0-9]+) Tickets on layer (.*) \(level (.*)\)!`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {

				eventData := map[string]interface{}{
					"time":       args[1],
					"chainID":    args[2],
					"team":       args[3],
					"subfaction": args[4],
					"faction":    args[5],
					"action":     args[6],
					"tickets":    args[7],
					"layer":      args[8],
					"level":      args[9],
				}

				// Store in event store based on win/loss status
				if args[6] == "won" {
					eventStore.StoreRoundWinner(eventData)
				} else {
					eventStore.StoreRoundLoser(eventData)
				}

				eventManager.PublishEvent(serverID, event_manager.EventTypeLogNewGame, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogNet: UChannel::Close: Sending CloseBunch\. ChIndex == [0-9]+\. Name: \[UChannel\] ChIndex: [0-9]+, Closing: [0-9]+ \[UNetConnection\] RemoteAddr: ([\d.]+):[\d]+, Name: EOSIpNetConnection_[0-9]+, Driver: GameNetDriver EOSNetDriver_[0-9]+, IsServer: YES, PC: ([^ ]+PlayerController_C_[0-9]+), Owner: [^ ]+PlayerController_C_[0-9]+, UniqueId: RedpointEOS:([\d\w]+)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {

				eventData := map[string]interface{}{
					"time":             args[1],
					"chainID":          args[2],
					"ip":               args[3],
					"playerController": args[4],
					"eosID":            args[5],
				}

				// Mark player as disconnected in the store
				eosID := args[5]
				eventStore.StoreDisconnectedPlayer(eosID, eventData)

				eventManager.PublishEvent(serverID, event_manager.EventTypeLogPlayerDisconnected, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: Player:(.+) ActualDamage=([0-9.]+) from (.+) \(Online IDs:([^|]+)\| Player Controller ID: ([^ ]+)\)caused by ([A-z_0-9-]+)_C`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {

				// Skip if IDs are invalid
				if strings.Contains(args[6], "INVALID") {
					return
				}

				// Parse online IDs from the log
				idsString := args[6]
				ids := make(map[string]string)

				// Split IDs by commas and extract platform:id pairs
				idPairs := strings.Split(idsString, ",")
				for _, pair := range idPairs {
					pair = strings.TrimSpace(pair)
					parts := strings.Split(pair, ":")
					if len(parts) == 2 {
						platform := strings.ToLower(strings.TrimSpace(parts[0]))
						id := strings.TrimSpace(parts[1])
						ids[platform] = id
					}
				}

				// Build event data
				eventData := map[string]interface{}{
					"time":               args[1],
					"chainID":            args[2],
					"victimName":         args[3],
					"damage":             args[4],
					"attackerName":       args[5],
					"attackerController": args[7],
					"weapon":             args[8],
				}

				// Add all attacker IDs to event data with capitalized platform name
				for platform, id := range ids {
					// Capitalize first letter of platform for key name
					platformKey := "attacker"
					if len(platform) > 0 {
						platformKey += strings.ToUpper(platform[:1])
						if len(platform) > 1 {
							platformKey += platform[1:]
						}
					}
					eventData[platformKey] = id
				}

				// Store session data for the victim
				victimName := args[3]
				eventStore.StoreSessionData(victimName, eventData)

				// Update player data for attacker if EOS ID exists
				if eosID, ok := ids["eos"]; ok {
					attackerData := map[string]interface{}{
						"controller": args[7],
					}
					eventStore.StorePlayerData(eosID, attackerData)
				}

				eventManager.PublishEvent(serverID, event_manager.EventTypeLogPlayerDamaged, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQSoldier::)?Die\(\): Player:(.+) KillingDamage=(?:-)*([0-9.]+) from ([A-z_0-9]+) \(Online IDs:([^)|]+)\| Contoller ID: ([\w\d]+)\) caused by ([A-z_0-9-]+)_C`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {

				// Skip if IDs are invalid
				if strings.Contains(args[6], "INVALID") {
					return
				}

				// Parse online IDs from the log
				idsString := args[6]
				ids := make(map[string]string)

				// Split IDs by commas and extract platform:id pairs
				idPairs := strings.Split(idsString, ",")
				for _, pair := range idPairs {
					pair = strings.TrimSpace(pair)
					parts := strings.Split(pair, ":")
					if len(parts) == 2 {
						platform := strings.ToLower(strings.TrimSpace(parts[0]))
						id := strings.TrimSpace(parts[1])
						ids[platform] = id
					}
				}

				// Get existing session data for this victim
				victimName := args[3]
				existingData, _ := eventStore.GetSessionData(victimName)
				if existingData == nil {
					existingData = make(map[string]interface{})
				}

				// Build event data, merging with existing session data
				eventData := existingData
				eventData["time"] = args[1]
				eventData["woundTime"] = args[1]
				eventData["chainID"] = args[2]
				eventData["victimName"] = args[3]
				eventData["damage"] = args[4]
				eventData["attackerPlayerController"] = args[5]
				eventData["weapon"] = args[8]

				// Add all attacker IDs to event data with capitalized platform name
				for platform, id := range ids {
					// Capitalize first letter of platform for key name
					platformKey := "attacker"
					if len(platform) > 0 {
						platformKey += strings.ToUpper(platform[:1])
						if len(platform) > 1 {
							platformKey += platform[1:]
						}
					}
					eventData[platformKey] = id
				}

				// Check for teamkill using EventStore
				var attackerEOSID string
				if eosID, hasEOS := eventData["attackerEos"].(string); hasEOS {
					attackerEOSID = eosID
				}

				isTeamkill := eventStore.CheckTeamkill(victimName, attackerEOSID)
				if isTeamkill {
					eventData["teamkill"] = true
				}

				// Update session data
				eventStore.StoreSessionData(victimName, eventData)

				eventManager.PublishEvent(serverID, event_manager.EventTypeLogPlayerDied, eventData, args[0])

				// If it's a teamkill, emit a separate TEAMKILL event
				if isTeamkill {
					eventManager.PublishEvent(serverID, event_manager.EventTypeLogTeamkill, eventData, args[0])
				}
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogNet: Join succeeded: (.+)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {

				// Convert chainID to string
				chainID := args[2]

				// Fetch player data by chainID from join requests
				player, exists := eventStore.GetJoinRequest(chainID)
				if !exists {
					// If no join request found, create basic event data
					eventData := map[string]interface{}{
						"time":         args[1],
						"chainID":      chainID,
						"playerSuffix": args[3],
					}
					eventManager.PublishEvent(serverID, event_manager.EventTypeLogJoinSucceeded, eventData, args[0])
					return
				}

				// Create event data by combining player data with new data
				eventData := make(map[string]interface{})

				// Copy all player data to event data
				for k, v := range player {
					eventData[k] = v
				}

				// Add new fields
				eventData["time"] = args[1]
				eventData["chainID"] = chainID
				eventData["playerSuffix"] = args[3]

				// Update player data with suffix
				player["playerSuffix"] = args[3]
				// Store updated player data back if we have a player ID
				if eosID, ok := player["eos"].(string); ok {
					eventStore.StorePlayerData(eosID, player)
				} else if steamID, ok := player["steam"].(string); ok {
					eventStore.StorePlayerData(steamID, player)
				}

				eventManager.PublishEvent(serverID, event_manager.EventTypeLogJoinSucceeded, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQPlayerController::)?OnPossess\(\): PC=(.+) \(Online IDs:([^)]+)\) Pawn=([A-z0-9_]+)_C`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {

				// Parse online IDs from the log
				idsString := args[4]
				ids := make(map[string]string)

				// Split IDs by commas and extract platform:id pairs
				idPairs := strings.Split(idsString, ",")
				for _, pair := range idPairs {
					pair = strings.TrimSpace(pair)
					parts := strings.Split(pair, ":")
					if len(parts) == 2 {
						platform := strings.ToLower(strings.TrimSpace(parts[0]))
						id := strings.TrimSpace(parts[1])
						ids[platform] = id
					}
				}

				// Build event data
				eventData := map[string]interface{}{
					"time":             args[1],
					"chainID":          args[2],
					"playerSuffix":     args[3],
					"possessClassname": args[5],
				}

				// Add all player IDs to event data with capitalized platform name
				for platform, id := range ids {
					// Capitalize first letter of platform for key name
					platformKey := "player"
					if len(platform) > 0 {
						platformKey += strings.ToUpper(platform[:1])
						if len(platform) > 1 {
							platformKey += platform[1:]
						}
					}
					eventData[platformKey] = id
				}

				// Store chainID in session data for the player suffix
				playerSuffix := args[3]
				sessionData := map[string]interface{}{
					"chainID": args[2],
				}
				eventStore.StoreSessionData(playerSuffix, sessionData)

				// Add deprecated field for compatibility
				if steamID, ok := ids["steam"]; ok {
					eventData["pawn"] = steamID
				}

				eventManager.PublishEvent(serverID, event_manager.EventTypeLogPlayerPossess, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: (.+) \(Online IDs:([^)]+)\) has revived (.+) \(Online IDs:([^)]+)\)\.`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {

				// Parse reviver IDs
				reviverIdsString := args[4]
				reviverIds := make(map[string]string)
				idPairs := strings.Split(reviverIdsString, ",")
				for _, pair := range idPairs {
					pair = strings.TrimSpace(pair)
					parts := strings.Split(pair, ":")
					if len(parts) == 2 {
						platform := strings.ToLower(strings.TrimSpace(parts[0]))
						id := strings.TrimSpace(parts[1])
						reviverIds[platform] = id
					}
				}

				// Parse victim IDs
				victimIdsString := args[6]
				victimIds := make(map[string]string)
				idPairs = strings.Split(victimIdsString, ",")
				for _, pair := range idPairs {
					pair = strings.TrimSpace(pair)
					parts := strings.Split(pair, ":")
					if len(parts) == 2 {
						platform := strings.ToLower(strings.TrimSpace(parts[0]))
						id := strings.TrimSpace(parts[1])
						victimIds[platform] = id
					}
				}

				// Get existing session data for reviver
				reviverName := args[3]
				existingData, _ := eventStore.GetSessionData(reviverName)
				if existingData == nil {
					existingData = make(map[string]interface{})
				}

				// Build event data, merging with existing session data
				eventData := existingData
				eventData["time"] = args[1]
				eventData["chainID"] = args[2]
				eventData["reviverName"] = args[3]
				eventData["victimName"] = args[5]

				// Add all reviver IDs to event data with capitalized platform name
				for platform, id := range reviverIds {
					// Capitalize first letter of platform for key name
					platformKey := "reviver"
					if len(platform) > 0 {
						platformKey += strings.ToUpper(platform[:1])
						if len(platform) > 1 {
							platformKey += platform[1:]
						}
					}
					eventData[platformKey] = id
				}

				// Add all victim IDs to event data with capitalized platform name
				for platform, id := range victimIds {
					// Capitalize first letter of platform for key name
					platformKey := "victim"
					if len(platform) > 0 {
						platformKey += strings.ToUpper(platform[:1])
						if len(platform) > 1 {
							platformKey += platform[1:]
						}
					}
					eventData[platformKey] = id
				}

				eventManager.PublishEvent(serverID, event_manager.EventTypeLogPlayerRevived, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQSoldier::)?Wound\(\): Player:(.+) KillingDamage=(?:-)*([0-9.]+) from ([A-z_0-9]+) \(Online IDs:([^)|]+)\| Controller ID: ([\w\d]+)\) caused by ([A-z_0-9-]+)_C`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {

				// Skip if IDs are invalid
				if strings.Contains(args[6], "INVALID") {
					return
				}

				// Parse online IDs from the log
				idsString := args[6]
				ids := make(map[string]string)

				// Split IDs by commas and extract platform:id pairs
				idPairs := strings.Split(idsString, ",")
				for _, pair := range idPairs {
					pair = strings.TrimSpace(pair)
					parts := strings.Split(pair, ":")
					if len(parts) == 2 {
						platform := strings.ToLower(strings.TrimSpace(parts[0]))
						id := strings.TrimSpace(parts[1])
						ids[platform] = id
					}
				}

				// Get existing session data for this victim
				victimName := args[3]
				existingData, _ := eventStore.GetSessionData(victimName)
				if existingData == nil {
					existingData = make(map[string]interface{})
				}

				// Build event data, merging with existing session data
				eventData := existingData
				eventData["time"] = args[1]
				eventData["chainID"] = args[2]
				eventData["victimName"] = args[3]
				eventData["damage"] = args[4]
				eventData["attackerPlayerController"] = args[5]
				eventData["weapon"] = args[8]

				// Add all attacker IDs to event data with capitalized platform name
				for platform, id := range ids {
					// Capitalize first letter of platform for key name
					platformKey := "attacker"
					if len(platform) > 0 {
						platformKey += strings.ToUpper(platform[:1])
						if len(platform) > 1 {
							platformKey += platform[1:]
						}
					}
					eventData[platformKey] = id
				}

				// Check for teamkill using EventStore
				var attackerEOSID string
				if eosID, hasEOS := eventData["attackerEos"].(string); hasEOS {
					attackerEOSID = eosID
				}

				isTeamkill := eventStore.CheckTeamkill(victimName, attackerEOSID)
				if isTeamkill {
					eventData["teamkill"] = true
				}

				// Update session data
				eventStore.StoreSessionData(victimName, eventData)

				eventManager.PublishEvent(serverID, event_manager.EventTypeLogPlayerWounded, eventData, args[0])

				// If it's a teamkill, emit a separate TEAMKILL event
				if isTeamkill {
					eventManager.PublishEvent(serverID, event_manager.EventTypeLogTeamkill, eventData, args[0])
				}
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: USQGameState: Server Tick Rate: ([0-9.]+)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {
				eventData := map[string]interface{}{
					"time":     args[1],
					"chainID":  args[2],
					"tickRate": args[3],
				}

				eventManager.PublishEvent(serverID, event_manager.EventTypeLogTickRate, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQGameMode::)?DetermineMatchWinner\(\): (.+) won on (.+)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {
				eventData := map[string]interface{}{
					"time":    args[1],
					"chainID": args[2],
					"winner":  args[3],
					"layer":   args[4],
				}

				// Store WON data in event store for new game correlation
				eventStore.StoreWonData(eventData)

				eventManager.PublishEvent(serverID, event_manager.EventTypeLogRoundEnded, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogGameState: Match State Changed from InProgress to WaitingPostMatch`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {

				// Initialize event data with time
				eventData := map[string]interface{}{
					"time": args[1],
				}

				// Add winner data if it exists
				if winnerData, exists := eventStore.GetRoundWinner(true); exists {
					eventData["winner"] = winnerData
				} else {
					eventData["winner"] = nil
				}

				// Add loser data if it exists
				if loserData, exists := eventStore.GetRoundLoser(true); exists {
					eventData["loser"] = loserData
				} else {
					eventData["loser"] = nil
				}

				eventManager.PublishEvent(serverID, event_manager.EventTypeLogRoundEnded, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogWorld: Bringing World \/([A-z]+)\/(?:Maps\/)?([A-z0-9-]+)\/(?:.+\/)?([A-z0-9-]+)(?:\.[A-z0-9-]+)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {

				// Skip transition map
				if args[5] == "TransitionMap" {
					return
				}

				// Initialize event data
				eventData := map[string]interface{}{
					"time":           args[1],
					"chainID":        args[2],
					"dlc":            args[3],
					"mapClassname":   args[4],
					"layerClassname": args[5],
				}

				// Merge with WON data if it exists
				if wonData, exists := eventStore.GetWonData(); exists {
					for k, v := range wonData {
						eventData[k] = v
					}
				}

				// Clear the event store for the new game
				eventStore.ClearNewGameData()

				eventManager.PublishEvent(serverID, event_manager.EventTypeLogNewGame, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQPlayerController::)?SquadJoined\(\): Player:(.+) \(Online IDs:([^)]+)\) Joined Team ([0-9]) Squad ([0-9]+)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {

				// Parse online IDs from the log
				idsString := args[4]
				ids := make(map[string]string)

				// Split IDs by commas and extract platform:id pairs
				idPairs := strings.Split(idsString, ",")
				for _, pair := range idPairs {
					pair = strings.TrimSpace(pair)
					parts := strings.Split(pair, ":")
					if len(parts) == 2 {
						platform := strings.ToLower(strings.TrimSpace(parts[0]))
						id := strings.TrimSpace(parts[1])
						ids[platform] = id
					}
				}

				// Build event data
				eventData := map[string]interface{}{
					"time":       args[1],
					"chainID":    args[2],
					"name":       args[3],
					"teamID":     args[5],
					"squadID":    args[6],
					"oldTeamID":  nil, // We don't track previous team in this implementation
					"oldSquadID": nil, // We don't track previous squad in this implementation
				}

				// Add all player IDs to event data with capitalized platform name
				for platform, id := range ids {
					// Capitalize first letter of platform for key name
					platformKey := "player"
					if len(platform) > 0 {
						platformKey += strings.ToUpper(platform[:1])
						if len(platform) > 1 {
							platformKey += platform[1:]
						}
					}
					eventData[platformKey] = id
				}

				// Store player information including team ID in session data
				playerName := args[3]
				teamID := args[5]
				squadID := args[6]

				// Create/update player session data
				sessionData := map[string]interface{}{
					"teamID":  teamID,
					"squadID": squadID,
				}
				eventStore.StoreSessionData(playerName, sessionData)

				// If we have an EOS ID, also store team info by EOS ID
				if eosID, ok := ids["eos"]; ok {
					playerData := map[string]interface{}{
						"teamID":  teamID,
						"squadID": squadID,
						"name":    playerName,
					}
					eventStore.StorePlayerData(eosID, playerData)
				}

				eventManager.PublishEvent(serverID, event_manager.EventTypeLogPlayerSquadChange, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQPlayerController::)?TeamJoined\(\): Player:(.+) \(Online IDs:([^)]+)\) Is Now On Team ([0-9])`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {

				// Parse online IDs from the log
				idsString := args[4]
				ids := make(map[string]string)

				// Split IDs by commas and extract platform:id pairs
				idPairs := strings.Split(idsString, ",")
				for _, pair := range idPairs {
					pair = strings.TrimSpace(pair)
					parts := strings.Split(pair, ":")
					if len(parts) == 2 {
						platform := strings.ToLower(strings.TrimSpace(parts[0]))
						id := strings.TrimSpace(parts[1])
						ids[platform] = id
					}
				}

				// Build event data
				playerName := args[3]
				newTeamID := args[5]

				// Get old team ID if available
				var oldTeamID interface{} = nil
				if playerData, exists := eventStore.GetSessionData(playerName); exists {
					if teamID, hasTeam := playerData["teamID"]; hasTeam {
						oldTeamID = teamID
					}
				}

				eventData := map[string]interface{}{
					"time":      args[1],
					"chainID":   args[2],
					"name":      playerName,
					"newTeamID": newTeamID,
					"oldTeamID": oldTeamID,
				}

				// Add all player IDs to event data with capitalized platform name
				for platform, id := range ids {
					// Capitalize first letter of platform for key name
					platformKey := "player"
					if len(platform) > 0 {
						platformKey += strings.ToUpper(platform[:1])
						if len(platform) > 1 {
							platformKey += platform[1:]
						}
					}
					eventData[platformKey] = id
				}

				// Store player information including team ID in session data
				sessionData := map[string]interface{}{
					"teamID": newTeamID,
				}
				eventStore.StoreSessionData(playerName, sessionData)

				// If we have an EOS ID, also store team info by EOS ID
				if eosID, ok := ids["eos"]; ok {
					playerData := map[string]interface{}{
						"teamID": newTeamID,
						"name":   playerName,
					}
					eventStore.StorePlayerData(eosID, playerData)
				}

				eventManager.PublishEvent(serverID, event_manager.EventTypeLogPlayerTeamChange, eventData, args[0])
			},
		},
	}
}

// processLogForEvents detects events based on regex and publishes them
func ProcessLogForEvents(logLine string, serverID uuid.UUID, parsers []LogParser, eventManager *event_manager.EventManager, eventStore *EventStore) {
	for _, parser := range parsers {
		if matches := parser.regex.FindStringSubmatch(logLine); matches != nil {
			parser.onMatch(matches, serverID, eventManager, eventStore)
		}
	}
}
