package logwatcher_manager

import (
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/player_tracker"
	"go.codycody31.dev/squad-aegis/internal/shared/utils"
)

// LogParser represents a log parser with a regex and a handler function
type LogParser struct {
	regex   *regexp.Regexp
	onMatch func([]string, uuid.UUID, *event_manager.EventManager, EventStoreInterface, *player_tracker.PlayerTracker)
}

// LogParsingMetrics tracks parsing performance metrics
type LogParsingMetrics struct {
	mu                      sync.RWMutex
	startTime               time.Time
	totalLines              int64
	matchingLines           int64
	totalMatchingLatency    time.Duration
	lastMinuteLines         []time.Time
	lastMinuteMatchingLines []time.Time
}

// ProcessLogForEvents detects events based on regex and publishes them
func ProcessLogForEvents(logLine string, serverID uuid.UUID, parsers []LogParser, eventManager *event_manager.EventManager, eventStore EventStoreInterface, playerTracker *player_tracker.PlayerTracker) {
	ProcessLogForEventsWithMetrics(logLine, serverID, parsers, eventManager, eventStore, playerTracker, nil)
}

// NewLogParsingMetrics creates a new metrics tracker
func NewLogParsingMetrics() *LogParsingMetrics {
	return &LogParsingMetrics{
		startTime:               time.Now(),
		lastMinuteLines:         make([]time.Time, 0),
		lastMinuteMatchingLines: make([]time.Time, 0),
	}
}

// RecordLineProcessed records that a line was processed
func (m *LogParsingMetrics) RecordLineProcessed() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.totalLines++
	m.lastMinuteLines = append(m.lastMinuteLines, now)
	m.cleanupOldEntries(now)
}

// RecordMatchingLine records that a line matched and the time it took to process
func (m *LogParsingMetrics) RecordMatchingLine(processingTime time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.matchingLines++
	m.totalMatchingLatency += processingTime
	m.lastMinuteMatchingLines = append(m.lastMinuteMatchingLines, now)
	m.cleanupOldEntries(now)
}

// cleanupOldEntries removes entries older than 1 minute
func (m *LogParsingMetrics) cleanupOldEntries(now time.Time) {
	oneMinuteAgo := now.Add(-time.Minute)

	// Clean up regular lines
	newLines := make([]time.Time, 0, len(m.lastMinuteLines))
	for _, t := range m.lastMinuteLines {
		if t.After(oneMinuteAgo) {
			newLines = append(newLines, t)
		}
	}
	m.lastMinuteLines = newLines

	// Clean up matching lines
	newMatchingLines := make([]time.Time, 0, len(m.lastMinuteMatchingLines))
	for _, t := range m.lastMinuteMatchingLines {
		if t.After(oneMinuteAgo) {
			newMatchingLines = append(newMatchingLines, t)
		}
	}
	m.lastMinuteMatchingLines = newMatchingLines
}

// GetMetrics returns current metrics
func (m *LogParsingMetrics) GetMetrics() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.cleanupOldEntries(now)

	linesPerMinute := float64(len(m.lastMinuteLines))
	matchingLinesPerMinute := float64(len(m.lastMinuteMatchingLines))

	var averageMatchingLatency float64
	if m.matchingLines > 0 {
		averageMatchingLatency = float64(m.totalMatchingLatency.Nanoseconds()) / float64(m.matchingLines) / 1000000 // Convert to milliseconds
	}

	return map[string]interface{}{
		"linesPerMinute":         linesPerMinute,
		"matchingLinesPerMinute": matchingLinesPerMinute,
		"matchingLatency":        averageMatchingLatency, // in milliseconds
		"totalLines":             m.totalLines,
		"totalMatchingLines":     m.matchingLines,
		"uptime":                 time.Since(m.startTime).Seconds(),
	}
}

// GetLogParsers returns all log parsers for Squad logs
func GetLogParsers() []LogParser {
	return []LogParser{
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: ADMIN COMMAND: Message broadcasted <(.+)> from (.+)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore EventStoreInterface, playerTracker *player_tracker.PlayerTracker) {
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
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore EventStoreInterface, playerTracker *player_tracker.PlayerTracker) {
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
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: PostLogin: NewPlayer: BP_PlayerController_C .+PersistentLevel\.([^\s]+) \(IP: ([\d.]+) \| Online IDs:([^)]*)\)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore EventStoreInterface, playerTracker *player_tracker.PlayerTracker) {
				onlineIDs := utils.ParseOnlineIDs(args[5])
				var playerSuffix string
				playerID := onlineIDs.EOSID
				if playerID == "" {
					playerID = onlineIDs.SteamID
				}
				if playerID == "" {
					playerID = onlineIDs.EpicID
				}
				if playerTracker != nil {
					if playerID != "" {
						if playerInfo, ok := playerTracker.GetPlayerByIdentifier(playerID); ok {
							playerSuffix = playerInfo.PlayerSuffix
							if playerSuffix == "" {
								playerSuffix = playerInfo.Name
							}
						}
					}
					if playerSuffix == "" && args[3] != "" {
						if playerInfo, ok := playerTracker.GetPlayerByController(args[3]); ok {
							playerSuffix = playerInfo.PlayerSuffix
							if playerSuffix == "" {
								playerSuffix = playerInfo.Name
							}
						}
					}
				}

				// Build player data
				player := &JoinRequestData{
					PlayerController: args[3],
					IP:               args[4],
					SteamID:          onlineIDs.SteamID,
					EOSID:            onlineIDs.EOSID,
					EpicID:           onlineIDs.EpicID,
				}

				// Store player data in event store
				eventStore.StoreJoinRequest(strings.TrimSpace(args[2]), player)
				eventStore.StorePlayerData(playerID, &PlayerData{
					PlayerController: args[3],
					IP:               args[4],
					SteamID:          onlineIDs.SteamID,
					EOSID:            onlineIDs.EOSID,
					EpicID:           onlineIDs.EpicID,
					PlayerSuffix:     playerSuffix,
				})

				// Update player tracker with PlayerController data
				if playerTracker != nil {
					playerTracker.UpdatePlayerFromLog(onlineIDs.EOSID, onlineIDs.SteamID, onlineIDs.EpicID, "", args[3], "")
				}

				// Create structured event data
				eventData := &event_manager.LogPlayerConnectedData{
					Time:             args[1],
					ChainID:          strings.TrimSpace(args[2]),
					PlayerController: args[3],
					IPAddress:        args[4],
					PlayerSuffix:     playerSuffix,
					SteamID:          onlineIDs.SteamID,
					EOSID:            onlineIDs.EOSID,
					EpicID:           onlineIDs.EpicID,
				}

				eventManager.PublishEvent(serverID, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: Player:(.+) ActualDamage=([0-9.]+) from (.+) \(Online IDs:(.*?)\s*\|\s*Player Controller ID: ([^ )]+)\)caused by ([A-Za-z0-9_-]+)_C`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore EventStoreInterface, playerTracker *player_tracker.PlayerTracker) {
				onlineIDs := utils.ParseOnlineIDs(args[6])

				eventManagerData := &event_manager.LogPlayerDamagedData{
					Time:               args[1],
					ChainID:            strings.TrimSpace(args[2]),
					VictimName:         args[3],
					Damage:             args[4],
					AttackerName:       args[5],
					AttackerEOS:        onlineIDs.EOSID,
					AttackerSteam:      onlineIDs.SteamID,
					AttackerController: args[7],
					Weapon:             args[8],
				}

				// Store session data for the victim
				sessionData := &SessionData{
					ChainID:            args[2],
					VictimName:         args[3],
					Damage:             args[4],
					AttackerName:       args[5],
					AttackerEOS:        onlineIDs.EOSID,
					AttackerSteam:      onlineIDs.SteamID,
					AttackerController: args[7],
					Weapon:             args[8],
				}

				// Store session data for the victim
				eventStore.StoreSessionData(args[3], sessionData)

				attackerPlayerID := onlineIDs.EOSID
				if attackerPlayerID == "" {
					attackerPlayerID = onlineIDs.SteamID
				}
				if attackerPlayerID == "" {
					attackerPlayerID = onlineIDs.EpicID
				}

				attacker, exists := eventStore.GetPlayerData(attackerPlayerID)
				if !exists {
					eventStore.StorePlayerData(attackerPlayerID, &PlayerData{
						SteamID:    onlineIDs.SteamID,
						EOSID:      onlineIDs.EOSID,
						EpicID:     onlineIDs.EpicID,
						Controller: args[7],
					})
				} else {
					attacker.Controller = args[7]
					eventStore.StorePlayerData(attackerPlayerID, attacker)
				}

				// Extra logic before publishing the event (similar to JavaScript)

				// Get victim by name (try RCON name first, then log suffix)
				if playerTracker != nil {
					victim, victimFound := playerTracker.GetPlayerByName(args[3])
					if !victimFound {
						victim, victimFound = playerTracker.GetPlayerByPlayerSuffix(args[3])
					}
					if victimFound {
						// Populate explicit victim fields
						eventManagerData.Victim = &event_manager.PlayerInfo{
							PlayerController: victim.PlayerController,
							IP:               "", // Not available in PlayerTracker
							SteamID:          victim.SteamID,
							EOSID:            victim.EOSID,
							EpicID:           victim.EpicID,
							PlayerSuffix:     victim.PlayerSuffix,
							Controller:       victim.PlayerController,
							TeamID:           victim.TeamID,
							SquadID:          victim.SquadID,
						}
						eventManagerData.VictimEOS = victim.EOSID
						eventManagerData.VictimSteam = victim.SteamID
						eventManagerData.VictimTeam = victim.TeamID
						eventManagerData.VictimSquad = victim.SquadID
						eventManagerData.VictimName = victim.PlayerSuffix

						if strings.TrimSpace(eventManagerData.VictimName) == "" {
							eventManagerData.VictimName = victim.Name
							eventManagerData.Victim.PlayerSuffix = victim.Name
						}
					}

					// Get attacker by EOS ID first, then by controller, then by suffix if not found
					attacker, attackerExists := playerTracker.GetPlayerByIdentifier(attackerPlayerID)
					if !attackerExists {
						attacker, attackerExists = playerTracker.GetPlayerByController(args[7])
					}
					if !attackerExists && args[5] != "" {
						attacker, attackerExists = playerTracker.GetPlayerByPlayerSuffix(args[5])
					}
					if attackerExists {
						// Convert player_tracker.PlayerInfo to event_manager.PlayerInfo
						eventManagerData.Attacker = &event_manager.PlayerInfo{
							PlayerController: attacker.PlayerController,
							IP:               "", // Not available in PlayerTracker
							SteamID:          attacker.SteamID,
							EOSID:            attacker.EOSID,
							EpicID:           attacker.EpicID,
							PlayerSuffix:     attacker.PlayerSuffix,
							Controller:       attacker.PlayerController,
							TeamID:           attacker.TeamID,
							SquadID:          attacker.SquadID,
						}
						// Populate explicit attacker fields
						eventManagerData.AttackerTeam = attacker.TeamID
						eventManagerData.AttackerSquad = attacker.SquadID
						eventManagerData.AttackerName = attacker.PlayerSuffix

						if strings.TrimSpace(eventManagerData.AttackerName) == "" {
							eventManagerData.AttackerName = attacker.Name
							eventManagerData.Attacker.PlayerSuffix = attacker.Name
						}

						// Update attacker's playercontroller if missing
						if attacker.PlayerController == "" && args[7] != "" {
							// Update the PlayerData in the store
							if playerData, playerExists := eventStore.GetPlayerData(attackerPlayerID); playerExists {
								playerData.PlayerController = args[7]
								eventStore.StorePlayerData(attackerPlayerID, playerData)
								// Refresh the attacker info from PlayerTracker
								if updatedAttacker, exists := playerTracker.GetPlayerByIdentifier(attackerPlayerID); exists {
									// Convert player_tracker.PlayerInfo to event_manager.PlayerInfo
									eventManagerData.Attacker = &event_manager.PlayerInfo{
										PlayerController: updatedAttacker.PlayerController,
										IP:               "", // Not available in PlayerTracker
										SteamID:          updatedAttacker.SteamID,
										EOSID:            updatedAttacker.EOSID,
										EpicID:           updatedAttacker.EpicID,
										PlayerSuffix:     updatedAttacker.PlayerSuffix,
										Controller:       updatedAttacker.PlayerController,
										TeamID:           updatedAttacker.TeamID,
										SquadID:          updatedAttacker.SquadID,
									}
									// Update explicit attacker fields
									eventManagerData.AttackerTeam = updatedAttacker.TeamID
									eventManagerData.AttackerSquad = updatedAttacker.SquadID
									eventManagerData.AttackerName = updatedAttacker.PlayerSuffix
								}
							}
						}
					}
				}

				// Check for teamkill if we have both victim and attacker data
				if eventManagerData.Victim != nil && eventManagerData.Attacker != nil {
					victimTeamID := eventManagerData.Victim.TeamID
					attackerTeamID := eventManagerData.Attacker.TeamID
					victimEOSID := eventManagerData.Victim.EOSID
					attackerEOSID := onlineIDs.EOSID

					if victimTeamID != "" && attackerTeamID != "" && victimTeamID == attackerTeamID {
						if victimEOSID != "" && victimEOSID != attackerEOSID {
							eventManagerData.Teamkill = true
						}
					}
				}

				eventManager.PublishEvent(serverID, eventManagerData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQSoldier::)?Die\(\): Player:(.+) KillingDamage=(?:-)*([0-9.]+) from ([A-Za-z0-9_]+) \(Online IDs:(.*?)\s*\| Contoller ID: ([\w\d]+)\) caused by ([A-Za-z0-9_-]+)_C`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore EventStoreInterface, playerTracker *player_tracker.PlayerTracker) {
				onlineIDs := utils.ParseOnlineIDs(args[6])

				// Get existing session data for this victim
				victimName := args[3]
				existingData, _ := eventStore.GetSessionData(victimName)
				if existingData == nil {
					existingData = &SessionData{}
				}

				eventManagerData := &event_manager.LogPlayerDiedData{
					Time:                     args[1],
					WoundTime:                args[1],
					ChainID:                  strings.TrimSpace(args[2]),
					VictimName:               args[3],
					Damage:                   args[4],
					AttackerPlayerController: args[7],
					AttackerEOS:              onlineIDs.EOSID,
					AttackerSteam:            onlineIDs.SteamID,
					Weapon:                   args[8],
				}

				// Build session data, merging with existing session data
				sessionData := &SessionData{
					ChainID:            existingData.ChainID,
					Time:               args[1],
					WoundTime:          args[1],
					VictimName:         args[3],
					Damage:             args[4],
					AttackerName:       existingData.AttackerName,
					AttackerEOS:        existingData.AttackerEOS,
					AttackerSteam:      existingData.AttackerSteam,
					AttackerController: args[7],
					Weapon:             args[8],
					TeamID:             existingData.TeamID,
					EOSID:              existingData.EOSID,
				}

				// Update session data
				eventStore.StoreSessionData(victimName, sessionData)

				// Extra logic before publishing the event (similar to JavaScript)

				// Get victim by name using PlayerTracker (try RCON name first, then log suffix)
				if playerTracker != nil {
					victim, exists := playerTracker.GetPlayerByName(args[3])
					if !exists {
						victim, exists = playerTracker.GetPlayerByPlayerSuffix(args[3])
					}
					if exists {
						// Convert player_tracker.PlayerInfo to event_manager.PlayerInfo
						eventManagerData.Victim = &event_manager.PlayerInfo{
							PlayerController: victim.PlayerController,
							IP:               "", // Not available in PlayerTracker
							SteamID:          victim.SteamID,
							EOSID:            victim.EOSID,
							EpicID:           victim.EpicID,
							PlayerSuffix:     victim.PlayerSuffix,
							Controller:       victim.PlayerController,
							TeamID:           victim.TeamID,
							SquadID:          victim.SquadID,
						}
						// Populate explicit victim fields
						eventManagerData.VictimEOS = victim.EOSID
						eventManagerData.VictimSteam = victim.SteamID
						eventManagerData.VictimTeam = victim.TeamID
						eventManagerData.VictimSquad = victim.SquadID
						eventManagerData.VictimName = victim.PlayerSuffix

						if strings.TrimSpace(eventManagerData.VictimName) == "" {
							eventManagerData.VictimName = victim.Name
							eventManagerData.Victim.PlayerSuffix = victim.Name
						}
					}

					// Get attacker by EOS ID first, then by controller, then by suffix if not found
					attackerPlayerID := onlineIDs.EOSID
					if attackerPlayerID == "" {
						attackerPlayerID = onlineIDs.SteamID
					}
					if attackerPlayerID == "" {
						attackerPlayerID = onlineIDs.EpicID
					}
					attacker, exists := playerTracker.GetPlayerByIdentifier(attackerPlayerID)
					if !exists {
						attacker, exists = playerTracker.GetPlayerByController(args[7])
					}
					if !exists && existingData.AttackerName != "" {
						attacker, exists = playerTracker.GetPlayerByPlayerSuffix(existingData.AttackerName)
					}
					if exists {
						// Convert player_tracker.PlayerInfo to event_manager.PlayerInfo
						eventManagerData.Attacker = &event_manager.PlayerInfo{
							PlayerController: attacker.PlayerController,
							IP:               "", // Not available in PlayerTracker
							SteamID:          attacker.SteamID,
							EOSID:            attacker.EOSID,
							EpicID:           attacker.EpicID,
							PlayerSuffix:     attacker.PlayerSuffix,
							Controller:       attacker.PlayerController,
							TeamID:           attacker.TeamID,
							SquadID:          attacker.SquadID,
						}

						// Populate explicit attacker fields
						eventManagerData.AttackerEOS = utils.ReturnOldIfEmpty(eventManagerData.AttackerEOS, attacker.EOSID)
						eventManagerData.AttackerSteam = utils.ReturnOldIfEmpty(eventManagerData.AttackerSteam, attacker.SteamID)
						eventManagerData.AttackerTeam = attacker.TeamID
						eventManagerData.AttackerSquad = attacker.SquadID
						eventManagerData.AttackerName = attacker.PlayerSuffix

						// if AttackerName is empty, try attacker.Name
						if strings.TrimSpace(eventManagerData.AttackerName) == "" {
							eventManagerData.AttackerName = attacker.Name
							eventManagerData.Attacker.PlayerSuffix = attacker.Name
						}
					}
				}

				// Check for teamkill if we have both victim and attacker data
				if eventManagerData.Victim != nil && eventManagerData.Attacker != nil {
					victimTeamID := eventManagerData.Victim.TeamID
					attackerTeamID := eventManagerData.Attacker.TeamID
					victimEOSID := eventManagerData.Victim.EOSID
					attackerEOSID := onlineIDs.EOSID

					if victimTeamID != "" && attackerTeamID != "" && victimTeamID == attackerTeamID {
						if victimEOSID != "" && victimEOSID != attackerEOSID {
							eventManagerData.Teamkill = true
						}
					}
				}

				eventManager.PublishEvent(serverID, eventManagerData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogNet: Join succeeded: (.+)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore EventStoreInterface, playerTracker *player_tracker.PlayerTracker) {

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

				// Create event manager data using the struct
				eventManagerData := &event_manager.LogJoinSucceededData{
					Time:         args[1],
					ChainID:      chainID,
					PlayerSuffix: args[3],
					EOSID:        player.EOSID,
					EpicID:       player.EpicID,
					SteamID:      player.SteamID,
					IPAddress:    player.IP,
				}

				// Update player data with suffix and store it
				playerData := &PlayerData{
					PlayerController: player.PlayerController,
					IP:               player.IP,
					SteamID:          player.SteamID,
					EOSID:            player.EOSID,
					EpicID:           player.EpicID,
					PlayerSuffix:     args[3], // Update with suffix from join succeeded
				}

				storeID := player.EOSID
				if storeID == "" {
					storeID = player.SteamID
				}
				if storeID == "" {
					storeID = player.EpicID
				}
				eventStore.StorePlayerData(storeID, playerData)

				// Update player tracker with PlayerSuffix data (eosID, steamID, name, playerController, playerSuffix)
				if playerTracker != nil {
					playerTracker.UpdatePlayerFromLog(player.EOSID, player.SteamID, player.EpicID, "", player.PlayerController, args[3])
				}

				eventManager.PublishEvent(serverID, eventManagerData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQPlayerController::)?OnPossess\(\): PC=(.+) \(Online IDs:([^)]*)\) Pawn=([A-Za-z0-9_]+)_C`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore EventStoreInterface, playerTracker *player_tracker.PlayerTracker) {
				onlineIDs := utils.ParseOnlineIDs(args[4])
				eventManagerData := &event_manager.LogPlayerPossessData{
					Time:             args[1],
					ChainID:          args[2],
					PlayerSuffix:     args[3],
					PlayerEOS:        onlineIDs.EOSID,
					PlayerSteam:      onlineIDs.SteamID,
					PlayerEpic:       onlineIDs.EpicID,
					PossessClassname: args[5],
				}

				// Store chainID in session data for the player suffix
				playerSuffix := args[3]
				sessionData := &SessionData{
					ChainID: args[2],
				}
				eventStore.StoreSessionData(playerSuffix, sessionData)

				if playerTracker != nil {
					playerTracker.UpdatePlayerFromLog(onlineIDs.EOSID, onlineIDs.SteamID, onlineIDs.EpicID, "", args[5], args[3])
				}

				eventManager.PublishEvent(serverID, eventManagerData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(
				`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: (.+) ` +
					`\(Online IDs:([^)]*)\) ` +
					`has revived (.+) ` +
					`\(Online IDs:([^)]*)\)\.`,
			),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore EventStoreInterface, playerTracker *player_tracker.PlayerTracker) {
				reviverIDs := utils.ParseOnlineIDs(args[4])
				victimIDs := utils.ParseOnlineIDs(args[6])
				eventManagerData := &event_manager.LogPlayerRevivedData{
					Time:         args[1],
					ChainID:      args[2],
					ReviverName:  args[3],
					VictimName:   args[5],
					ReviverEOS:   reviverIDs.EOSID,
					ReviverSteam: reviverIDs.SteamID,
					VictimEOS:    victimIDs.EOSID,
					VictimSteam:  victimIDs.SteamID,
				}

				reviverID := reviverIDs.EOSID
				if reviverID == "" {
					reviverID = reviverIDs.SteamID
				}
				if reviverID == "" {
					reviverID = reviverIDs.EpicID
				}
				victimID := victimIDs.EOSID
				if victimID == "" {
					victimID = victimIDs.SteamID
				}
				if victimID == "" {
					victimID = victimIDs.EpicID
				}

				if reviver, exists := eventStore.GetPlayerInfoByIdentifier(reviverID); exists {
					eventManagerData.Reviver = reviver
				}

				if victim, exists := eventStore.GetPlayerInfoByIdentifier(victimID); exists {
					eventManagerData.Victim = victim
				}

				eventManager.PublishEvent(serverID, eventManagerData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(
				`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQSoldier::)?Wound\(\): Player:(.+) ` +
					`KillingDamage=(?:-)*([0-9.]+) from ([A-Za-z0-9_]+) ` +
					`\(Online IDs:(.*?)\s*\| Controller ID: ([\w\d]+)\) ` +
					`caused by ([A-Za-z0-9_-]+)_C`,
			),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore EventStoreInterface, playerTracker *player_tracker.PlayerTracker) {
				onlineIDs := utils.ParseOnlineIDs(args[6])

				// Get existing session data for this victim
				victimName := args[3]
				existingData, _ := eventStore.GetSessionData(victimName)
				if existingData == nil {
					existingData = &SessionData{}
				}

				eventManagerData := &event_manager.LogPlayerWoundedData{
					Time:                     args[1],
					ChainID:                  strings.TrimSpace(args[2]),
					VictimName:               args[3],
					Damage:                   args[4],
					AttackerPlayerController: args[7],
					AttackerEOS:              onlineIDs.EOSID,
					AttackerSteam:            onlineIDs.SteamID,
					Weapon:                   args[8],
				}

				// Build session data, merging with existing session data
				sessionData := &SessionData{
					ChainID:            existingData.ChainID,
					Time:               args[1],
					WoundTime:          existingData.WoundTime,
					VictimName:         args[3],
					Damage:             args[4],
					AttackerName:       existingData.AttackerName,
					AttackerEOS:        existingData.AttackerEOS,
					AttackerSteam:      existingData.AttackerSteam,
					AttackerController: args[7],
					Weapon:             args[8],
					TeamID:             existingData.TeamID,
					EOSID:              existingData.EOSID,
				}

				// Update session data
				eventStore.StoreSessionData(victimName, sessionData)

				// Extra logic before publishing the event (similar to JavaScript)

				// Get victim by name using PlayerTracker (try RCON name first, then log suffix)
				if playerTracker != nil {
					victim, exists := playerTracker.GetPlayerByName(args[3])
					if !exists {
						victim, exists = playerTracker.GetPlayerByPlayerSuffix(args[3])
					}
					if exists {
						// Convert player_tracker.PlayerInfo to event_manager.PlayerInfo
						eventManagerData.Victim = &event_manager.PlayerInfo{
							PlayerController: victim.PlayerController,
							IP:               "", // Not available in PlayerTracker
							SteamID:          victim.SteamID,
							EOSID:            victim.EOSID,
							EpicID:           victim.EpicID,
							PlayerSuffix:     victim.PlayerSuffix,
							Controller:       victim.PlayerController,
							TeamID:           victim.TeamID,
							SquadID:          victim.SquadID,
						}
						// Populate explicit victim fields
						eventManagerData.VictimEOS = victim.EOSID
						eventManagerData.VictimSteam = victim.SteamID
						eventManagerData.VictimTeam = victim.TeamID
						eventManagerData.VictimSquad = victim.SquadID
						eventManagerData.VictimName = victim.PlayerSuffix

						if strings.TrimSpace(eventManagerData.VictimName) == "" {
							eventManagerData.VictimName = victim.Name
							eventManagerData.Victim.PlayerSuffix = victim.Name
						}
					}

					// Get attacker by EOS ID first, then by controller, then by suffix if not found
					attackerPlayerID := onlineIDs.EOSID
					if attackerPlayerID == "" {
						attackerPlayerID = onlineIDs.SteamID
					}
					if attackerPlayerID == "" {
						attackerPlayerID = onlineIDs.EpicID
					}
					attacker, exists := playerTracker.GetPlayerByIdentifier(attackerPlayerID)
					if !exists {
						attacker, exists = playerTracker.GetPlayerByController(args[7])
					}
					if !exists && existingData.AttackerName != "" {
						attacker, exists = playerTracker.GetPlayerByPlayerSuffix(existingData.AttackerName)
					}
					if exists {
						// Convert player_tracker.PlayerInfo to event_manager.PlayerInfo
						eventManagerData.Attacker = &event_manager.PlayerInfo{
							PlayerController: attacker.PlayerController,
							IP:               "", // Not available in PlayerTracker
							SteamID:          attacker.SteamID,
							EOSID:            attacker.EOSID,
							EpicID:           attacker.EpicID,
							PlayerSuffix:     attacker.PlayerSuffix,
							Controller:       attacker.PlayerController,
							TeamID:           attacker.TeamID,
							SquadID:          attacker.SquadID,
						}

						// Populate explicit attacker fields
						eventManagerData.AttackerEOS = utils.ReturnOldIfEmpty(eventManagerData.AttackerEOS, attacker.EOSID)
						eventManagerData.AttackerSteam = utils.ReturnOldIfEmpty(eventManagerData.AttackerSteam, attacker.SteamID)
						eventManagerData.AttackerTeam = attacker.TeamID
						eventManagerData.AttackerSquad = attacker.SquadID
						eventManagerData.AttackerName = attacker.PlayerSuffix

						// if AttackerName is empty, try attacker.Name
						if strings.TrimSpace(eventManagerData.AttackerName) == "" {
							eventManagerData.AttackerName = attacker.Name
							eventManagerData.Attacker.PlayerSuffix = attacker.Name
						}
					}
				}

				// Check for teamkill if we have both victim and attacker data
				if eventManagerData.Victim != nil && eventManagerData.Attacker != nil {
					victimTeamID := eventManagerData.Victim.TeamID
					attackerTeamID := eventManagerData.Attacker.TeamID
					victimEOSID := eventManagerData.Victim.EOSID
					attackerEOSID := onlineIDs.EOSID

					if victimTeamID != "" && attackerTeamID != "" && victimTeamID == attackerTeamID {
						if victimEOSID != "" && victimEOSID != attackerEOSID {
							eventManagerData.Teamkill = true
						}
					}
				}

				eventManager.PublishEvent(serverID, eventManagerData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: USQGameState: Server Tick Rate: ([0-9.]+)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore EventStoreInterface, playerTracker *player_tracker.PlayerTracker) {
				eventManager.PublishEvent(serverID, &event_manager.LogTickRateData{
					Time:     args[1],
					ChainID:  args[2],
					TickRate: args[3],
				}, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogNet: UChannel::Close: Sending CloseBunch\. ChIndex == [0-9]+\. Name: \[UChannel\] ChIndex: [0-9]+, Closing: [0-9]+ \[UNetConnection\] RemoteAddr: ([\d.]+):[\d]+, Name: RedpointEOSIpNetConnection_[0-9]+, Driver: Name:GameNetDriver Def:GameNetDriver RedpointEOSNetDriver_[0-9]+, IsServer: YES, PC: ([^ ]+PlayerController_C_[0-9]+), Owner: [^ ]+PlayerController_C_[0-9]+, UniqueId: RedpointEOS:([\da-f]+)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore EventStoreInterface, playerTracker *player_tracker.PlayerTracker) {
				player, ok := eventStore.GetPlayerData(args[5])
				if !ok {
					eventManager.PublishEvent(serverID, &event_manager.LogPlayerDisconnectedData{
						Time:             args[1],
						ChainID:          strings.TrimSpace(args[2]),
						IP:               args[3],
						PlayerController: args[4],
						EOSID:            args[5],
					}, args[0])
				} else {
					err := eventStore.RemovePlayerData(player.EOSID)
					if err != nil {
						log.Error().Err(err).Msg("Failed to remove player data by EOS ID on disconnect")
					}
					err = eventStore.RemovePlayerData(player.SteamID)
					if err != nil {
						log.Error().Err(err).Msg("Failed to remove player data by Steam ID on disconnect")
					}
					eventManager.PublishEvent(serverID, &event_manager.LogPlayerDisconnectedData{
						Time:             args[1],
						ChainID:          strings.TrimSpace(args[2]),
						IP:               args[3],
						PlayerController: args[4],
						PlayerSuffix:     player.PlayerSuffix,
						SteamID:          player.SteamID,
						TeamID:           player.TeamID,
						EOSID:            args[5],
						EpicID:           player.EpicID,
					}, args[0])
				}
			},
		},
	}
}

// ProcessLogForEventsWithMetrics detects events based on regex, publishes them, and tracks metrics
func ProcessLogForEventsWithMetrics(logLine string, serverID uuid.UUID, parsers []LogParser, eventManager *event_manager.EventManager, eventStore EventStoreInterface, playerTracker *player_tracker.PlayerTracker, metrics *LogParsingMetrics) {
	if metrics != nil {
		metrics.RecordLineProcessed()
	}

	for _, parser := range parsers {
		if matches := parser.regex.FindStringSubmatch(logLine); matches != nil {
			var processingTime time.Duration

			if metrics != nil {
				start := time.Now()
				parser.onMatch(matches, serverID, eventManager, eventStore, playerTracker)
				processingTime = time.Since(start)
				metrics.RecordMatchingLine(processingTime)
			} else {
				parser.onMatch(matches, serverID, eventManager, eventStore, playerTracker)
			}

			return // Only process the first match
		}
	}
}
