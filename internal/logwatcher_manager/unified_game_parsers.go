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
				data := map[string]interface{}{
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
					eventStore.StoreRoundWinner(data)
				} else {
					eventStore.StoreRoundLoser(data)
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
				data := map[string]interface{}{
					"time":    args[1],
					"chainID": strings.TrimSpace(args[2]),
					"winner":  args[3],
					"layer":   args[4],
				}

				// Store WON data for correlation with NEW_GAME
				eventStore.StoreWonData(data)

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
					if winnerJSON, err := json.Marshal(winnerData); err == nil {
						unifiedEvent.WinnerData = string(winnerJSON)
						if winner, ok := winnerData["faction"].(string); ok {
							unifiedEvent.Winner = winner
						}
						if layer, ok := winnerData["layer"].(string); ok {
							unifiedEvent.Layer = layer
						}
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
					if team, ok := wonData["team"].(string); ok {
						unifiedEvent.Team = team
					}
					if subfaction, ok := wonData["subfaction"].(string); ok {
						unifiedEvent.Subfaction = subfaction
					}
					if faction, ok := wonData["faction"].(string); ok {
						unifiedEvent.Faction = faction
					}
					if action, ok := wonData["action"].(string); ok {
						unifiedEvent.Action = action
					}
					if tickets, ok := wonData["tickets"].(string); ok {
						unifiedEvent.Tickets = tickets
					}
					if layer, ok := wonData["layer"].(string); ok {
						unifiedEvent.Layer = layer
					}
					if level, ok := wonData["level"].(string); ok {
						unifiedEvent.Level = level
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
