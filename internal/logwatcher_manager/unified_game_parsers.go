package logwatcher_manager

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
)

// GetUnifiedGameEventParsers returns consolidated log parsers for game-related events
func GetUnifiedGameEventParsers() []LogParser {
	return []LogParser{
		// Match when tickets appear in the log - store winner/loser data
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadGameEvents: Display: Team ([0-9]), (.*) \( ?(.*?) ?\) has (won|lost) the match with ([0-9]+) Tickets on layer (.*) \(level (.*)\)!`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {
				if args[6] == "won" {
					eventStore.StoreRoundWinner(&RoundWinnerData{
						Time:       args[1],
						ChainID:    strings.TrimSpace(args[2]),
						Team:       args[3],
						Subfaction: args[4],
						Faction:    args[5],
						Action:     args[6],
						Tickets:    args[7],
						Layer:      args[8],
						Level:      args[9],
					})
				} else {
					eventStore.StoreRoundLoser(&RoundLoserData{
						Time:       args[1],
						ChainID:    strings.TrimSpace(args[2]),
						Team:       args[3],
						Subfaction: args[4],
						Faction:    args[5],
						Action:     args[6],
						Tickets:    args[7],
						Layer:      args[8],
						Level:      args[9],
					})
				}

				// Create unified event for individual ticket events
				unifiedEvent := &event_manager.LogGameEventUnifiedData{
					Time:       args[1],
					ChainID:    strings.TrimSpace(args[2]),
					EventType:  "TICKET_UPDATE",
					Team:       args[3],
					Subfaction: args[4],
					Faction:    args[5],
					Action:     args[6],
					Tickets:    args[7],
					Layer:      args[8],
					Level:      args[9],
					RawLog:     args[0],
				}

				eventManager.PublishEvent(serverID, unifiedEvent, args[0])
			},
		},
		// Match when game determines match winner
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQGameMode::)?DetermineMatchWinner\(\): (.+) won on (.+)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {
				// Store WON data for correlation with NEW_GAME
				eventStore.StoreWonData(&WonData{
					Time:    args[1],
					ChainID: strings.TrimSpace(args[2]),
					Winner:  &args[3],
					Layer:   args[4],
				})

				// Create unified event for match winner
				unifiedEvent := &event_manager.LogGameEventUnifiedData{
					Time:      args[1],
					ChainID:   strings.TrimSpace(args[2]),
					EventType: "MATCH_WINNER",
					Winner:    args[3],
					Layer:     args[4],
					RawLog:    args[0],
				}

				eventManager.PublishEvent(serverID, unifiedEvent, args[0])
			},
		},
		// Match when game state changes to post-match (score board)
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogGameState: Match State Changed from InProgress to WaitingPostMatch`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {
				unifiedEvent := &event_manager.LogGameEventUnifiedData{
					Time:      args[1],
					ChainID:   strings.TrimSpace(args[2]),
					EventType: "ROUND_ENDED",
					FromState: "InProgress",
					ToState:   "WaitingPostMatch",
					RawLog:    args[0],
				}

				// Add winner/loser data if available
				if winnerData, exists := eventStore.GetRoundWinner(true); exists {
					unifiedEvent.Winner = winnerData.Faction
					unifiedEvent.Layer = winnerData.Layer
					if winnerJSON, err := json.Marshal(winnerData); err == nil {
						unifiedEvent.WinnerData = string(winnerJSON)
					}
				}

				if loserData, exists := eventStore.GetRoundLoser(true); exists {
					if loserJSON, err := json.Marshal(loserData); err == nil {
						unifiedEvent.LoserData = string(loserJSON)
					}
				}

				eventManager.PublishEvent(serverID, unifiedEvent, args[0])
			},
		},
		// Match when bringing world (new game/map change)
		{
			regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogWorld: Bringing World \/([A-z0-9]+)\/(?:Maps\/)?([A-z0-9-]+)\/(?:.+\/)?([A-z0-9-]+)(?:\.[A-z0-9-]+)`),
			onMatch: func(args []string, serverID uuid.UUID, eventManager *event_manager.EventManager, eventStore *EventStore) {
				// Skip transition map
				if args[5] == "TransitionMap" {
					return
				}

				unifiedEvent := &event_manager.LogGameEventUnifiedData{
					Time:           args[1],
					ChainID:        strings.TrimSpace(args[2]),
					EventType:      "NEW_GAME",
					DLC:            args[3],
					MapClassname:   args[4],
					LayerClassname: args[5],
					RawLog:         args[0],
				}

				// Merge with existing WON data if available
				if wonData, exists := eventStore.GetWonData(); exists {
					if wonData.Team != "" {
						unifiedEvent.Team = wonData.Team
					}
					if wonData.Subfaction != "" {
						unifiedEvent.Subfaction = wonData.Subfaction
					}
					if wonData.Faction != "" {
						unifiedEvent.Faction = wonData.Faction
					}
					if wonData.Action != "" {
						unifiedEvent.Action = wonData.Action
					}
					if wonData.Tickets != "" {
						unifiedEvent.Tickets = wonData.Tickets
					}
					if wonData.Level != "" {
						unifiedEvent.Level = wonData.Level
					}
					if wonData.Layer != "" {
						unifiedEvent.Layer = wonData.Layer
					}

					// Store as metadata for completeness
					if metadataJSON, err := json.Marshal(wonData); err == nil {
						unifiedEvent.Metadata = string(metadataJSON)
					}
				}

				// Clear the event store for the new game
				eventStore.ClearNewGameData()

				eventManager.PublishEvent(serverID, unifiedEvent, args[0])
			},
		},
	}
}

// GetOptimizedLogParsers returns all log parsers including the unified game event parsers
func GetOptimizedLogParsers() []LogParser {
	// Get existing parsers but exclude the game-related ones
	allParsers := GetLogParsers()
	optimizedParsers := make([]LogParser, 0)

	// Filter out the old game-related parsers by checking their regex patterns
	gameEventPatterns := []string{
		`LogSquadGameEvents: Display: Team`,
		`DetermineMatchWinner`,
		`Match State Changed from InProgress to WaitingPostMatch`,
		`LogWorld: Bringing World`,
	}

	for _, parser := range allParsers {
		isGameEvent := false
		for _, pattern := range gameEventPatterns {
			if strings.Contains(parser.regex.String(), pattern) {
				isGameEvent = true
				break
			}
		}
		if !isGameEvent {
			optimizedParsers = append(optimizedParsers, parser)
		}
	}

	// Add the unified game event parsers
	optimizedParsers = append(optimizedParsers, GetUnifiedGameEventParsers()...)

	return optimizedParsers
}
