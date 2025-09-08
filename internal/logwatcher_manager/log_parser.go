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
				if args[4] != "RCON" {
					var steamID string
					steamIDStart := strings.Index(args[4], "steam: ")
					if steamIDStart != -1 {
						steamIDStart += len("steam: ")
						steamIDEnd := strings.Index(args[4][steamIDStart:], "]")
						if steamIDEnd != -1 {
							steamID = args[4][steamIDStart : steamIDStart+steamIDEnd]
						}
					}

					eventManager.PublishEvent(serverID, &event_manager.LogAdminBroadcastData{
						Time:    args[1],
						ChainID: strings.TrimSpace(args[2]),
						Message: args[3],
						From:    steamID,
					}, args[0])
				} else {
					eventManager.PublishEvent(serverID, &event_manager.LogAdminBroadcastData{
						Time:    args[1],
						ChainID: strings.TrimSpace(args[2]),
						Message: args[3],
						From:    args[4],
					}, args[0])
				}
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQDeployable::)?TakeDamage\(\): ([A-z0-9_]+)_C_[0-9]+: ([0-9.]+) damage attempt by causer ([A-z0-9_]+)_C_[0-9]+ instigator (.+) with damage type ([A-z0-9_]+)_C health remaining ([0-9.]+)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {
				eventData := &event_manager.LogDeployableDamagedData{
					Time:            args[1],
					ChainID:         strings.TrimSpace(args[2]),
					Deployable:      args[3],
					Damage:          args[4],
					Weapon:          args[5],
					PlayerSuffix:    args[6],
					DamageType:      args[7],
					HealthRemaining: args[8],
				}

				eventManager.PublishEvent(serverID, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: PostLogin: NewPlayer: BP_PlayerController_C .+PersistentLevel\.([^\s]+) \(IP: ([\d.]+) \| Online IDs:(?: EOS: ([^ )]+))?(?: steam: ([^ )]+))?\)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {
				// Build player data
				player := map[string]interface{}{
					"playercontroller": args[3],
					"ip":               args[4],
					"steam":            args[6],
					"eos":              args[5],
				}

				// Store player data in event store
				eventStore.StoreJoinRequest(strings.TrimSpace(args[2]), player)
				eventStore.StorePlayerData(args[6], player)

				// Handle reconnecting players
				eventStore.RemoveDisconnectedPlayer(args[6])

				// Create structured event data
				eventData := &event_manager.LogPlayerConnectedData{
					Time:             args[1],
					ChainID:          strings.TrimSpace(args[2]),
					PlayerController: args[3],
					IPAddress:        args[4],
					SteamID:          args[6],
					EOSID:            args[5],
				}

				eventManager.PublishEvent(serverID, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadGameEvents: Display: Team ([0-9]), (.*) \( ?(.*?) ?\) has (won|lost) the match with ([0-9]+) Tickets on layer (.*) \(level (.*)\)!`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {
				eventData := &event_manager.LogNewGameData{
					Time:       args[1],
					ChainID:    strings.TrimSpace(args[2]),
					Team:       args[3],
					Subfaction: args[4],
					Faction:    args[5],
					Action:     args[6],
					Tickets:    args[7],
					Layer:      args[8],
					Level:      args[9],
				}

				// Store in event store based on win/loss status (still using map for event store)
				legacyData := map[string]interface{}{
					"time":       args[1],
					"chainID":    strings.TrimSpace(args[2]),
					"team":       args[3],
					"subfaction": args[4],
					"faction":    args[5],
					"action":     args[6],
					"tickets":    args[7],
					"layer":      args[8],
					"level":      args[9],
				}

				if args[6] == "won" {
					eventStore.StoreRoundWinner(legacyData)
				} else {
					eventStore.StoreRoundLoser(legacyData)
				}

				eventManager.PublishEvent(serverID, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: Player:(.+) ActualDamage=([0-9.]+) from (.+) \(Online IDs:(?: EOS: ([^ )|]+))?(?: steam: ([^ )|]+))?\s*\|\s*Player Controller ID: ([^ )]+)\)caused by ([A-Za-z0-9_-]+)_C`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {
				// Skip if IDs are invalid
				if strings.Contains(args[6], "INVALID") {
					return
				}

				eventManagerData := &event_manager.LogPlayerDamagedData{
					Time:               args[1],
					ChainID:            strings.TrimSpace(args[2]),
					VictimName:         args[3],
					Damage:             args[4],
					AttackerName:       args[5],
					AttackerEOS:        args[6],
					AttackerSteam:      args[7],
					AttackerController: args[8],
					Weapon:             args[9],
				}

				// Build event data
				eventData := map[string]interface{}{
					"time":               args[1],
					"chainID":            args[2],
					"victimName":         args[3],
					"damage":             args[4],
					"attackerName":       args[5],
					"attackerEOS":        args[6],
					"attackerSteam":      args[7],
					"attackerController": args[8],
					"weapon":             args[9],
				}

				// Store session data for the victim
				victimName := args[3]
				eventStore.StoreSessionData(victimName, eventData)
				eventStore.StorePlayerData(args[6], map[string]interface{}{
					"controller": args[8],
				})

				eventManager.PublishEvent(serverID, eventManagerData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQSoldier::)?Die\(\): Player:(.+) KillingDamage=(?:-)*([0-9.]+) from ([A-Za-z0-9_]+) \(Online IDs:(?: EOS: ([^ )|]+))?(?: steam: ([^ )|]+))?\s*\| Contoller ID: ([\w\d]+)\) caused by ([A-Za-z0-9_-]+)_C`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {
				// Skip if IDs are invalid
				if strings.Contains(args[6], "INVALID") {
					return
				}

				// Get existing session data for this victim
				victimName := args[3]
				existingData, _ := eventStore.GetSessionData(victimName)
				if existingData == nil {
					existingData = make(map[string]interface{})
				}

				eventManagerData := &event_manager.LogPlayerDiedData{
					Time:                     args[1],
					WoundTime:                args[1],
					ChainID:                  strings.TrimSpace(args[2]),
					VictimName:               args[3],
					Damage:                   args[4],
					AttackerPlayerController: args[5],
					AttackerEOS:              args[6],
					AttackerSteam:            args[7],
					Weapon:                   args[9],
				}

				// Build event data, merging with existing session data
				eventData := existingData
				eventData["time"] = args[1]
				eventData["woundTime"] = args[1]
				eventData["chainID"] = args[2]
				eventData["victimName"] = args[3]
				eventData["damage"] = args[4]
				eventData["attackerPlayerController"] = args[5]
				eventData["weapon"] = args[9]

				isTeamkill := eventStore.CheckTeamkill(victimName, args[6])
				if isTeamkill {
					eventManagerData.Teamkill = true
					eventData["teamkill"] = true
				}

				// Update session data
				eventStore.StoreSessionData(victimName, eventData)

				eventManager.PublishEvent(serverID, eventManagerData, args[0])
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
					eventManager.PublishEvent(serverID, &event_manager.LogJoinSucceededData{
						Time:         args[1],
						ChainID:      chainID,
						PlayerSuffix: args[3],
					}, args[0])
					return
				}

				// Create event data by combining player data with new data
				eventData := make(map[string]interface{})
				eventManagerData := &event_manager.LogJoinSucceededData{
					Time:         args[1],
					ChainID:      chainID,
					PlayerSuffix: args[3],
				}

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
					eventManagerData.EOSID = eosID
					eventStore.StorePlayerData(eosID, player)
				} else if steamID, ok := player["steam"].(string); ok {
					eventManagerData.SteamID = steamID
					eventStore.StorePlayerData(steamID, player)
				}

				eventManager.PublishEvent(serverID, eventManagerData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQPlayerController::)?OnPossess\(\): PC=(.+) \(Online IDs:(?: EOS: ([^ )]+))?(?: steam: ([^ )]+))?\) Pawn=([A-Za-z0-9_]+)_C`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {
				eventManagerData := &event_manager.LogPlayerPossessData{
					Time:             args[1],
					ChainID:          args[2],
					PlayerSuffix:     args[3],
					PlayerEOS:        args[4],
					PlayerSteam:      args[5],
					PossessClassname: args[6],
				}

				// Store chainID in session data for the player suffix
				playerSuffix := args[3]
				sessionData := map[string]interface{}{
					"chainID": args[2],
				}
				eventStore.StoreSessionData(playerSuffix, sessionData)

				eventManager.PublishEvent(serverID, eventManagerData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(
				`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: (.+) ` +
					`\(Online IDs:(?: EOS: ([^ )]+))?(?: steam: ([^ )]+))?\) ` +
					`has revived (.+) ` +
					`\(Online IDs:(?: EOS: ([^ )]+))?(?: steam: ([^ )]+))?\)\.`,
			),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {
				eventManagerData := &event_manager.LogPlayerRevivedData{
					Time:         args[1],
					ChainID:      args[2],
					ReviverName:  args[3],
					VictimName:   args[5],
					ReviverEOS:   args[4],
					ReviverSteam: args[5],
					VictimEOS:    args[7],
					VictimSteam:  args[8],
				}

				eventManager.PublishEvent(serverID, eventManagerData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(
				`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQSoldier::)?Wound\(\): Player:(.+) ` +
					`KillingDamage=(?:-)*([0-9.]+) from ([A-Za-z0-9_]+) ` +
					`\(Online IDs:(?: EOS: ([^ )|]+))?(?: steam: ([^ )|]+))?\s*\| Controller ID: ([\w\d]+)\) ` +
					`caused by ([A-Za-z0-9_-]+)_C`,
			),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {
				// Skip if IDs are invalid
				if strings.Contains(args[6], "INVALID") {
					return
				}

				// Get existing session data for this victim
				victimName := args[3]
				existingData, _ := eventStore.GetSessionData(victimName)
				if existingData == nil {
					existingData = make(map[string]interface{})
				}

				eventManagerData := &event_manager.LogPlayerWoundedData{
					Time:                     args[1],
					ChainID:                  strings.TrimSpace(args[2]),
					VictimName:               args[3],
					Damage:                   args[4],
					AttackerPlayerController: args[5],
					AttackerEOS:              args[6],
					AttackerSteam:            args[7],
					Weapon:                   args[9],
				}

				// Build event data, merging with existing session data
				eventData := existingData
				eventData["time"] = args[1]
				eventData["chainID"] = args[2]
				eventData["victimName"] = args[3]
				eventData["damage"] = args[4]
				eventData["attackerPlayerController"] = args[5]
				eventData["attackerEOS"] = args[6]
				eventData["attackerSteam"] = args[7]
				eventData["weapon"] = args[9]

				isTeamkill := eventStore.CheckTeamkill(victimName, args[6])
				if isTeamkill {
					eventManagerData.Teamkill = true
					eventData["teamkill"] = true
				}

				// Update session data
				eventStore.StoreSessionData(victimName, eventData)

				eventManager.PublishEvent(serverID, eventManagerData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: USQGameState: Server Tick Rate: ([0-9.]+)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {
				eventManager.PublishEvent(serverID, &event_manager.LogTickRateData{
					Time:     args[1],
					ChainID:  args[2],
					TickRate: args[3],
				}, args[0])
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

				eventManager.PublishEvent(serverID, &event_manager.LogRoundEndedData{
					Time:    args[1],
					ChainID: args[2],
					Winner:  args[3],
					Layer:   args[4],
				}, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogGameState: Match State Changed from InProgress to WaitingPostMatch`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {
				eventData := &event_manager.LogRoundEndedData{
					Time: args[1],
				}

				if winnerData, exists := eventStore.GetRoundWinner(true); exists {
					eventData.WinnerData = winnerData
				}

				if loserData, exists := eventStore.GetRoundLoser(true); exists {
					eventData.LoserData = loserData
				}

				eventManager.PublishEvent(serverID, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogWorld: Bringing World \/([A-z]+)\/(?:Maps\/)?([A-z0-9-]+)\/(?:.+\/)?([A-z0-9-]+)(?:\.[A-z0-9-]+)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {

				// Skip transition map
				if args[5] == "TransitionMap" {
					return
				}

				eventManagerData := &event_manager.LogNewGameData{
					Time:           args[1],
					ChainID:        args[2],
					DLC:            args[3],
					MapClassname:   args[4],
					LayerClassname: args[5],
				}

				// Merge with WON data if it exists
				if wonData, exists := eventStore.GetWonData(); exists {
					for k, v := range wonData {
						switch k {
						case "team":
							if teamStr, ok := v.(string); ok {
								eventManagerData.Team = teamStr
							}
						case "subfaction":
							if subfactionStr, ok := v.(string); ok {
								eventManagerData.Subfaction = subfactionStr
							}
						case "faction":
							if factionStr, ok := v.(string); ok {
								eventManagerData.Faction = factionStr
							}
						case "action":
							if actionStr, ok := v.(string); ok {
								eventManagerData.Action = actionStr
							}
						case "tickets":
							if ticketsStr, ok := v.(string); ok {
								eventManagerData.Tickets = ticketsStr
							}
						case "layer":
							if layerStr, ok := v.(string); ok {
								eventManagerData.Layer = layerStr
							}
						case "level":
							if levelStr, ok := v.(string); ok {
								eventManagerData.Level = levelStr
							}
						}
					}
				}

				// Clear the event store for the new game
				eventStore.ClearNewGameData()

				eventManager.PublishEvent(serverID, eventManagerData, args[0])
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
