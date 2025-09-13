package logwatcher_manager

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
)

// LogParser represents a log parser with a regex and a handler function
type LogParser struct {
	regex   *regexp.Regexp
	onMatch func([]string, uuid.UUID, *event_manager.EventManager, *EventStore)
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
				})

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

				// Store in event store based on win/loss status
				roundData := &RoundWinnerData{
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

				if args[6] == "won" {
					eventStore.StoreRoundWinner(roundData)
				} else {
					loserData := &RoundLoserData{
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
					eventStore.StoreRoundLoser(loserData)
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

				if _, exists := eventStore.GetPlayerData(args[6]); !exists {
					// Store minimal player data if we don't have full data yet
					playerData := &PlayerData{
						Controller: args[8],
					}
					eventStore.StorePlayerData(args[6], playerData)
				} else {
					// Update existing player data with controller info
					existingData, _ := eventStore.GetPlayerData(args[6])
					existingData.Controller = args[8]
					eventStore.StorePlayerData(args[6], existingData)
				}

				// Extra logic before publishing the event (similar to JavaScript)

				// Get victim by name
				if victim, exists := eventStore.GetPlayerInfoByName(args[3]); exists {
					eventManagerData.Victim = victim
				}

				// Get attacker by EOS ID
				if attacker, exists := eventStore.GetPlayerInfoByEOSID(args[6]); exists {
					eventManagerData.Attacker = attacker

					// Update attacker's playercontroller if missing
					if attacker.PlayerController == "" && args[8] != "" {
						// Update the PlayerData in the store
						if playerData, playerExists := eventStore.GetPlayerData(args[6]); playerExists {
							playerData.PlayerController = args[8]
							eventStore.StorePlayerData(args[6], playerData)
							// Refresh the attacker info
							if updatedAttacker, exists := eventStore.GetPlayerInfoByEOSID(args[6]); exists {
								eventManagerData.Attacker = updatedAttacker
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

				// Clear the name fields as they're now in victim/attacker objects
				eventManagerData.VictimName = ""
				eventManagerData.AttackerName = ""

				if eventManagerData.Teamkill {
					log.Info().Msg("Teamkill detected: " + fmt.Sprintf("%+v", eventManagerData))
				}

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

				// Get victim by name
				if victim, exists := eventStore.GetPlayerInfoByName(args[3]); exists {
					eventManagerData.Victim = victim
				}

				// Get attacker by EOS ID first, then by controller if not found
				if attacker, exists := eventStore.GetPlayerInfoByEOSID(args[6]); exists {
					eventManagerData.Attacker = attacker
				} else if attacker, exists := eventStore.GetPlayerInfoByController(args[5]); exists {
					eventManagerData.Attacker = attacker
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

				// Clear the victim name field as it's now in victim object
				eventManagerData.VictimName = ""

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

				// Create event manager data using the struct
				eventManagerData := &event_manager.LogJoinSucceededData{
					Time:         args[1],
					ChainID:      chainID,
					PlayerSuffix: args[3],
					EOSID:        player.EOSID,
					SteamID:      player.SteamID,
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
				sessionData := &SessionData{
					ChainID: args[2],
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
					VictimName:   args[6],
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

				// Get victim by name
				if victim, exists := eventStore.GetPlayerInfoByName(args[3]); exists {
					eventManagerData.Victim = victim
				}

				// Get attacker by EOS ID first, then by controller if not found
				if attacker, exists := eventStore.GetPlayerInfoByEOSID(args[6]); exists {
					eventManagerData.Attacker = attacker
				} else if attacker, exists := eventStore.GetPlayerInfoByController(args[5]); exists {
					eventManagerData.Attacker = attacker
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

				// Clear the victim name field as it's now in victim object
				eventManagerData.VictimName = ""

				eventManager.PublishEvent(serverID, eventManagerData, args[0])

				// Emit TEAMKILL event if this is a teamkill (similar to JavaScript logic)
				if eventManagerData.Teamkill {
					// You could publish a separate TEAMKILL event here if needed
					// eventManager.PublishEvent(serverID, eventManagerData, "TEAMKILL")
				}
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
				eventData := &WonData{
					Time:    args[1],
					ChainID: args[2],
					Winner:  &args[3],
					Layer:   args[4],
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
					eventData.WinnerData = &event_manager.RoundWinnerInfo{
						Time:       winnerData.Time,
						ChainID:    winnerData.ChainID,
						Team:       winnerData.Team,
						Subfaction: winnerData.Subfaction,
						Faction:    winnerData.Faction,
						Action:     winnerData.Action,
						Tickets:    winnerData.Tickets,
						Layer:      winnerData.Layer,
						Level:      winnerData.Level,
					}
				}

				if loserData, exists := eventStore.GetRoundLoser(true); exists {
					eventData.LoserData = &event_manager.RoundLoserInfo{
						Time:       loserData.Time,
						ChainID:    loserData.ChainID,
						Team:       loserData.Team,
						Subfaction: loserData.Subfaction,
						Faction:    loserData.Faction,
						Action:     loserData.Action,
						Tickets:    loserData.Tickets,
						Layer:      loserData.Layer,
						Level:      loserData.Level,
					}
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
					if wonData.Team != "" {
						eventManagerData.Team = wonData.Team
					}
					if wonData.Subfaction != "" {
						eventManagerData.Subfaction = wonData.Subfaction
					}
					if wonData.Faction != "" {
						eventManagerData.Faction = wonData.Faction
					}
					if wonData.Action != "" {
						eventManagerData.Action = wonData.Action
					}
					if wonData.Tickets != "" {
						eventManagerData.Tickets = wonData.Tickets
					}
					if wonData.Layer != "" {
						eventManagerData.Layer = wonData.Layer
					}
					if wonData.Level != "" {
						eventManagerData.Level = wonData.Level
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
	ProcessLogForEventsWithMetrics(logLine, serverID, parsers, eventManager, eventStore, nil)
}

// ProcessLogForEventsWithMetrics detects events based on regex, publishes them, and tracks metrics
func ProcessLogForEventsWithMetrics(logLine string, serverID uuid.UUID, parsers []LogParser, eventManager *event_manager.EventManager, eventStore *EventStore, metrics *LogParsingMetrics) {
	if metrics != nil {
		metrics.RecordLineProcessed()
	}

	for _, parser := range parsers {
		if matches := parser.regex.FindStringSubmatch(logLine); matches != nil {
			var processingTime time.Duration

			if metrics != nil {
				start := time.Now()
				parser.onMatch(matches, serverID, eventManager, eventStore)
				processingTime = time.Since(start)
				metrics.RecordMatchingLine(processingTime)
			} else {
				parser.onMatch(matches, serverID, eventManager, eventStore)
			}

			return // Only process the first match
		}
	}
}
