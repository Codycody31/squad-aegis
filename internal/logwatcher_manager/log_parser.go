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
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: PostLogin: NewPlayer: BP_PlayerController_C .+PersistentLevel\.([^\s]+) \(IP: ([\d.]+) \| Online IDs:(?: EOS: ([^ )]+))?(?: steam: ([^ )]+))?\)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore EventStoreInterface, playerTracker *player_tracker.PlayerTracker) {
				var playerSuffix string
				if playerTracker != nil {
					if args[5] != "" {
						if playerInfo, ok := playerTracker.GetPlayerByEOSID(args[5]); ok {
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
					SteamID:          args[6],
					EOSID:            args[5],
				}

				// Store player data in event store
				eventStore.StoreJoinRequest(strings.TrimSpace(args[2]), player)
				eventStore.StorePlayerData(args[6], &PlayerData{
					PlayerController: args[3],
					IP:               args[4],
					SteamID:          args[6],
					EOSID:            args[5],
					PlayerSuffix:     playerSuffix,
				})

				// Update player tracker with PlayerController data
				if playerTracker != nil && args[5] != "" {
					playerTracker.UpdatePlayerFromLog(args[5], "", args[3], "", "")
				}

				// Create structured event data
				eventData := &event_manager.LogPlayerConnectedData{
					Time:             args[1],
					ChainID:          strings.TrimSpace(args[2]),
					PlayerController: args[3],
					IPAddress:        args[4],
					PlayerSuffix:     playerSuffix,
					SteamID:          args[6],
					EOSID:            args[5],
				}

				eventManager.PublishEvent(serverID, eventData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: Player:(.+) ActualDamage=([0-9.]+) from (.+) \(Online IDs:(?: EOS: ([^ )|]+))?(?: steam: ([^ )|]+))?\s*\|\s*Player Controller ID: ([^ )]+)\)caused by ([A-Za-z0-9_-]+)_C`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore EventStoreInterface, playerTracker *player_tracker.PlayerTracker) {
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

				// Store session data for the victim
				sessionData := &SessionData{
					ChainID:            args[2],
					VictimName:         args[3],
					Damage:             args[4],
					AttackerName:       args[5],
					AttackerEOS:        args[6],
					AttackerSteam:      args[7],
					AttackerController: args[8],
					Weapon:             args[9],
				}

				// Store session data for the victim
				eventStore.StoreSessionData(args[3], sessionData)

				attacker, exists := eventStore.GetPlayerData(args[6])
				if !exists {
					eventStore.StorePlayerData(args[6], &PlayerData{
						Controller: args[5],
					})
				} else {
					attacker.Controller = args[5]
					eventStore.StorePlayerData(args[6], attacker)
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
					attacker, attackerExists := playerTracker.GetPlayerByEOSID(args[6])
					if !attackerExists {
						attacker, attackerExists = playerTracker.GetPlayerByController(args[8])
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
						if attacker.PlayerController == "" && args[8] != "" {
							// Update the PlayerData in the store
							if playerData, playerExists := eventStore.GetPlayerData(args[6]); playerExists {
								playerData.PlayerController = args[8]
								eventStore.StorePlayerData(args[6], playerData)
								// Refresh the attacker info from PlayerTracker
								if updatedAttacker, exists := playerTracker.GetPlayerByEOSID(args[6]); exists {
									// Convert player_tracker.PlayerInfo to event_manager.PlayerInfo
									eventManagerData.Attacker = &event_manager.PlayerInfo{
										PlayerController: updatedAttacker.PlayerController,
										IP:               "", // Not available in PlayerTracker
										SteamID:          updatedAttacker.SteamID,
										EOSID:            updatedAttacker.EOSID,
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
					attackerEOSID := args[6]

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
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQSoldier::)?Die\(\): Player:(.+) KillingDamage=(?:-)*([0-9.]+) from ([A-Za-z0-9_]+) \(Online IDs:(?: EOS: ([^ )|]+))?(?: steam: ([^ )|]+))?\s*\| Contoller ID: ([\w\d]+)\) caused by ([A-Za-z0-9_-]+)_C`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore EventStoreInterface, playerTracker *player_tracker.PlayerTracker) {
				// Skip if IDs are invalid
				if strings.Contains(args[6], "INVALID") {
					return
				}

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
					AttackerPlayerController: args[5],
					AttackerEOS:              args[6],
					AttackerSteam:            args[7],
					Weapon:                   args[9],
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
					AttackerController: args[5],
					Weapon:             args[9],
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
					attacker, exists := playerTracker.GetPlayerByEOSID(args[6])
					if !exists {
						attacker, exists = playerTracker.GetPlayerByController(args[5])
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
					attackerEOSID := args[6]

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
					SteamID:      player.SteamID,
					IPAddress:    player.IP,
				}

				// Update player data with suffix and store it
				playerData := &PlayerData{
					PlayerController: player.PlayerController,
					IP:               player.IP,
					SteamID:          player.SteamID,
					EOSID:            player.EOSID,
					PlayerSuffix:     args[3], // Update with suffix from join succeeded
				}

				// Store updated player data using EOS ID as primary key, fallback to Steam ID
				if player.EOSID != "" {
					eventStore.StorePlayerData(player.EOSID, playerData)
				} else if player.SteamID != "" {
					eventStore.StorePlayerData(player.SteamID, playerData)
				}

				// Update player tracker with PlayerSuffix data (eosID, steamID, name, playerController, playerSuffix)
				if playerTracker != nil && player.EOSID != "" {
					playerTracker.UpdatePlayerFromLog(player.EOSID, player.SteamID, "", player.PlayerController, args[3])
				}

				eventManager.PublishEvent(serverID, eventManagerData, args[0])
			},
		},
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQPlayerController::)?OnPossess\(\): PC=(.+) \(Online IDs:(?: EOS: ([^ )]+))?(?: steam: ([^ )]+))?\) Pawn=([A-Za-z0-9_]+)_C`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore EventStoreInterface, playerTracker *player_tracker.PlayerTracker) {
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
				sessionData := &SessionData{
					ChainID: args[2],
				}
				eventStore.StoreSessionData(playerSuffix, sessionData)

				if playerTracker != nil && args[4] != "" {
					playerTracker.UpdatePlayerFromLog(args[4], args[5], "", args[6], args[3])
				}

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
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore EventStoreInterface, playerTracker *player_tracker.PlayerTracker) {
				eventManagerData := &event_manager.LogPlayerRevivedData{
					Time:         args[1],
					ChainID:      args[2],
					ReviverName:  args[3],
					VictimName:   args[6],
					ReviverEOS:   args[4],
					ReviverSteam: args[5],
					VictimEOS:    args[7],
					VictimSteam:  args[8],
				}

				// Get reviver by EOS ID
				if reviver, exists := eventStore.GetPlayerInfoByEOSID(args[4]); exists {
					eventManagerData.Reviver = reviver
				}

				// Get victim by EOS ID
				if victim, exists := eventStore.GetPlayerInfoByEOSID(args[7]); exists {
					eventManagerData.Victim = victim
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
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore EventStoreInterface, playerTracker *player_tracker.PlayerTracker) {
				// Skip if IDs are invalid
				if strings.Contains(args[6], "INVALID") {
					return
				}

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
					AttackerPlayerController: args[5],
					AttackerEOS:              args[6],
					AttackerSteam:            args[7],
					Weapon:                   args[9],
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
					AttackerController: args[5],
					Weapon:             args[9],
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
					attacker, exists := playerTracker.GetPlayerByEOSID(args[6])
					if !exists {
						attacker, exists = playerTracker.GetPlayerByController(args[5])
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
					attackerEOSID := args[6]

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
