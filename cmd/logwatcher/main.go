package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/hpcloud/tail"
	"github.com/urfave/cli/v3"
	"google.golang.org/grpc"

	pb "go.codycody31.dev/squad-aegis/proto/logwatcher"
	"go.codycody31.dev/squad-aegis/shared/utils"
)

// Auth token (configurable via CLI)
var authToken string

// LogWatcherServer implements the LogWatcher service
type LogWatcherServer struct {
	pb.UnimplementedLogWatcherServer
	mu         sync.Mutex
	clients    map[pb.LogWatcher_StreamLogsServer]struct{}
	eventSubs  map[pb.LogWatcher_StreamEventsServer]struct{}
	logFile    string
	eventStore *EventStore
}

// LogParser represents a log parser with a regex and a handler function
type LogParser struct {
	regex   *regexp.Regexp
	onMatch func([]string, *LogWatcherServer)
}

// Global parsers for structured events
var logParsers = []LogParser{
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: ADMIN COMMAND: Message broadcasted <(.+)> from (.+)`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched ADMIN_BROADCAST event: ", args)
			// Build a JSON object with the event details.
			eventData := map[string]string{
				"time":    args[1],
				"chainID": args[2],
				"message": args[3],
				"from":    args[4],
			}

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "ADMIN_BROADCAST",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQDeployable::)?TakeDamage\(\): ([A-z0-9_]+)_C_[0-9]+: ([0-9.]+) damage attempt by causer ([A-z0-9_]+)_C_[0-9]+ instigator (.+) with damage type ([A-z0-9_]+)_C health remaining ([0-9.]+)`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched DEPLOYABLE_DAMAGED event: ", args)
			// Build a JSON object with the event details.
			eventData := map[string]string{
				"time":            args[1],
				"chainID":         args[2],
				"deployable":      args[3],
				"damage":          args[4],
				"weapon":          args[5],
				"playerSuffix":    args[6],
				"damageType":      args[7],
				"healthRemaining": args[8],
			}

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "DEPLOYABLE_DAMAGED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: PostLogin: NewPlayer: BP_PlayerController_C .+PersistentLevel\.([^\s]+) \(IP: ([\d.]+) \| Online IDs:([^)|]+)\)`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched PLAYER_CONNECTED event: ", args)

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
				// Store player data
				server.eventStore.mu.Lock()
				server.eventStore.joinRequests[args[2]] = player
				server.eventStore.players[playerID] = player

				// Handle reconnecting players
				delete(server.eventStore.disconnected, playerID)
				server.eventStore.mu.Unlock()
			}

			// Build event data
			eventData := map[string]interface{}{
				"raw":              args[0],
				"time":             args[1],
				"chainID":          args[2],
				"playercontroller": args[3],
				"ip":               args[4],
			}

			// Add all IDs to event data
			for platform, id := range ids {
				eventData[platform] = id
			}

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "PLAYER_CONNECTED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},

	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadGameEvents: Display: Team ([0-9]), (.*) \( ?(.*?) ?\) has (won|lost) the match with ([0-9]+) Tickets on layer (.*) \(level (.*)\)!`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched NEW_GAME event (tickets): ", args)

			// Build event data
			eventData := map[string]interface{}{
				"raw":        args[0],
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
			server.eventStore.mu.Lock()
			if args[6] == "won" {
				server.eventStore.session["ROUND_WINNER"] = eventData
			} else {
				server.eventStore.session["ROUND_LOSER"] = eventData
			}
			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "NEW_GAME",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogNet: UChannel::Close: Sending CloseBunch\. ChIndex == [0-9]+\. Name: \[UChannel\] ChIndex: [0-9]+, Closing: [0-9]+ \[UNetConnection\] RemoteAddr: ([\d.]+):[\d]+, Name: EOSIpNetConnection_[0-9]+, Driver: GameNetDriver EOSNetDriver_[0-9]+, IsServer: YES, PC: ([^ ]+PlayerController_C_[0-9]+), Owner: [^ ]+PlayerController_C_[0-9]+, UniqueId: RedpointEOS:([\d\w]+)`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched PLAYER_DISCONNECTED event: ", args)

			// Build event data
			eventData := map[string]interface{}{
				"raw":              args[0],
				"time":             args[1],
				"chainID":          args[2],
				"ip":               args[3],
				"playerController": args[4],
				"eosID":            args[5],
			}

			// Mark player as disconnected in the store
			eosID := args[5]
			server.eventStore.mu.Lock()
			if server.eventStore.disconnected == nil {
				server.eventStore.disconnected = make(map[string]map[string]interface{})
			}

			// Store the disconnection data
			server.eventStore.disconnected[eosID] = eventData
			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "PLAYER_DISCONNECTED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: Player:(.+) ActualDamage=([0-9.]+) from (.+) \(Online IDs:([^|]+)\| Player Controller ID: ([^ ]+)\)caused by ([A-z_0-9-]+)_C`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched PLAYER_DAMAGED event: ", args)

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
				"raw":                args[0],
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
			server.eventStore.mu.Lock()
			server.eventStore.session[victimName] = eventData

			// Update player data for attacker if EOS ID exists
			if eosID, ok := ids["eos"]; ok {
				// Initialize attacker data if it doesn't exist
				if _, exists := server.eventStore.players[eosID]; !exists {
					server.eventStore.players[eosID] = make(map[string]interface{})
				}
				server.eventStore.players[eosID]["controller"] = args[7]
			}
			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "PLAYER_DAMAGED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQSoldier::)?Die\(\): Player:(.+) KillingDamage=(?:-)*([0-9.]+) from ([A-z_0-9]+) \(Online IDs:([^)|]+)\| Contoller ID: ([\w\d]+)\) caused by ([A-z_0-9-]+)_C`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched PLAYER_DIED event: ", args)

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
			server.eventStore.mu.RLock()
			var existingData map[string]interface{}
			if sessionData, exists := server.eventStore.session[victimName]; exists {
				// Make a copy of existing data
				existingData = make(map[string]interface{})
				for k, v := range sessionData {
					existingData[k] = v
				}
			} else {
				existingData = make(map[string]interface{})
			}
			server.eventStore.mu.RUnlock()

			// Build event data, merging with existing session data
			eventData := existingData
			eventData["raw"] = args[0]
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

			// Get victim and attacker team information to check for teamkill
			var isTeamkill bool
			var victimTeamID, attackerTeamID string

			// Look up victim team ID
			server.eventStore.mu.RLock()
			if victimData, exists := server.eventStore.session[victimName]; exists {
				if teamID, hasTeam := victimData["teamID"].(string); hasTeam {
					victimTeamID = teamID
				}
			}

			// Look up attacker EOS ID from the event data
			var attackerEOSID string
			if eosID, hasEOS := eventData["attackerEos"].(string); hasEOS {
				attackerEOSID = eosID
			}

			// Look up attacker team ID if we have their EOS ID
			if attackerEOSID != "" {
				if attackerData, exists := server.eventStore.players[attackerEOSID]; exists {
					if teamID, hasTeam := attackerData["teamID"].(string); hasTeam {
						attackerTeamID = teamID
					}
				}
			}
			server.eventStore.mu.RUnlock()

			// Check for teamkill: same team but different players
			if victimTeamID != "" && attackerTeamID != "" && victimTeamID == attackerTeamID {
				// Ensure this isn't self-damage
				var victimEOSID string
				server.eventStore.mu.RLock()
				if victimData, exists := server.eventStore.session[victimName]; exists {
					if eosID, hasEOS := victimData["eosID"].(string); hasEOS {
						victimEOSID = eosID
					}
				}
				server.eventStore.mu.RUnlock()

				// If we have both EOSIDs and they're different, but teams are the same, it's a teamkill
				if victimEOSID != "" && attackerEOSID != "" && victimEOSID != attackerEOSID {
					isTeamkill = true
					eventData["teamkill"] = true
				}
			}

			// Update session data
			server.eventStore.mu.Lock()
			server.eventStore.session[victimName] = eventData
			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "PLAYER_DIED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)

			// If it's a teamkill, emit a separate TEAMKILL event
			if isTeamkill {
				teamkillData := &pb.EventEntry{
					Event: "TEAMKILL",
					Data:  string(jsonBytes),
				}
				server.broadcastEvent(teamkillData)
			}
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogNet: Join succeeded: (.+)`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched JOIN_SUCCEEDED event: ", args)

			// Convert chainID to number (stored as string in Go)
			chainID := args[2]

			// Fetch player data by chainID
			server.eventStore.mu.Lock()
			player, exists := server.eventStore.joinRequests[chainID]
			if !exists {
				log.Println("[ERROR] No join request found for chainID:", chainID)
				server.eventStore.mu.Unlock()
				return
			}

			// Join request processed, remove it
			delete(server.eventStore.joinRequests, chainID)

			// Create event data by combining player data with new data
			eventData := make(map[string]interface{})

			// Copy all player data to event data
			for k, v := range player {
				eventData[k] = v
			}

			// Add new fields
			eventData["raw"] = args[0]
			eventData["time"] = args[1]
			eventData["chainID"] = chainID
			eventData["playerSuffix"] = args[3]

			// Update player data with suffix
			player["playerSuffix"] = args[3]
			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "JOIN_SUCCEEDED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQPlayerController::)?OnPossess\(\): PC=(.+) \(Online IDs:([^)]+)\) Pawn=([A-z0-9_]+)_C`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched PLAYER_POSSESS event: ", args)

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
				"raw":              args[0],
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
			server.eventStore.mu.Lock()
			server.eventStore.session[playerSuffix] = map[string]interface{}{
				"chainID": args[2],
			}
			server.eventStore.mu.Unlock()

			// Add deprecated field for compatibility
			if steamID, ok := ids["steam"]; ok {
				eventData["pawn"] = steamID
			}

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "PLAYER_POSSESS",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: (.+) \(Online IDs:([^)]+)\) has revived (.+) \(Online IDs:([^)]+)\)\.`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched PLAYER_REVIVED event: ", args)

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

			// Get existing session data
			reviverName := args[3]
			server.eventStore.mu.RLock()
			var existingData map[string]interface{}
			if sessionData, exists := server.eventStore.session[reviverName]; exists {
				// Make a copy of existing data
				existingData = make(map[string]interface{})
				for k, v := range sessionData {
					existingData[k] = v
				}
			} else {
				existingData = make(map[string]interface{})
			}
			server.eventStore.mu.RUnlock()

			// Build event data, merging with existing session data
			eventData := existingData
			eventData["raw"] = args[0]
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

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "PLAYER_REVIVED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQSoldier::)?Wound\(\): Player:(.+) KillingDamage=(?:-)*([0-9.]+) from ([A-z_0-9]+) \(Online IDs:([^)|]+)\| Controller ID: ([\w\d]+)\) caused by ([A-z_0-9-]+)_C`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched PLAYER_WOUNDED event: ", args)

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
			server.eventStore.mu.RLock()
			var existingData map[string]interface{}
			if sessionData, exists := server.eventStore.session[victimName]; exists {
				// Make a copy of existing data
				existingData = make(map[string]interface{})
				for k, v := range sessionData {
					existingData[k] = v
				}
			} else {
				existingData = make(map[string]interface{})
			}
			server.eventStore.mu.RUnlock()

			// Build event data, merging with existing session data
			eventData := existingData
			eventData["raw"] = args[0]
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

			// Get victim and attacker team information to check for teamkill
			var isTeamkill bool
			var victimTeamID, attackerTeamID string

			// Look up victim team ID
			server.eventStore.mu.RLock()
			if victimData, exists := server.eventStore.session[victimName]; exists {
				if teamID, hasTeam := victimData["teamID"].(string); hasTeam {
					victimTeamID = teamID
				}
			}

			// Look up attacker EOS ID from the event data
			var attackerEOSID string
			if eosID, hasEOS := eventData["attackerEos"].(string); hasEOS {
				attackerEOSID = eosID
			}

			// Look up attacker team ID if we have their EOS ID
			if attackerEOSID != "" {
				if attackerData, exists := server.eventStore.players[attackerEOSID]; exists {
					if teamID, hasTeam := attackerData["teamID"].(string); hasTeam {
						attackerTeamID = teamID
					}
				}
			}
			server.eventStore.mu.RUnlock()

			// Check for teamkill: same team but different players
			if victimTeamID != "" && attackerTeamID != "" && victimTeamID == attackerTeamID {
				// Ensure this isn't self-damage
				var victimEOSID string
				server.eventStore.mu.RLock()
				if victimData, exists := server.eventStore.session[victimName]; exists {
					if eosID, hasEOS := victimData["eosID"].(string); hasEOS {
						victimEOSID = eosID
					}
				}
				server.eventStore.mu.RUnlock()

				// If we have both EOSIDs and they're different, but teams are the same, it's a teamkill
				if victimEOSID != "" && attackerEOSID != "" && victimEOSID != attackerEOSID {
					isTeamkill = true
					eventData["teamkill"] = true
				}
			}

			// Update session data
			server.eventStore.mu.Lock()
			server.eventStore.session[victimName] = eventData
			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "PLAYER_WOUNDED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)

			// If it's a teamkill, emit a separate TEAMKILL event
			if isTeamkill {
				teamkillData := &pb.EventEntry{
					Event: "TEAMKILL",
					Data:  string(jsonBytes),
				}
				server.broadcastEvent(teamkillData)
			}
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: USQGameState: Server Tick Rate: ([0-9.]+)`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched TICK_RATE event: ", args)
			// Build a JSON object with the event details.
			eventData := map[string]string{
				"time":     args[1],
				"chainID":  args[2],
				"tickRate": args[3],
			}

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "TICK_RATE",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQGameMode::)?DetermineMatchWinner\(\): (.+) won on (.+)`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched ROUND_ENDED event: ", args)

			// Build event data
			eventData := map[string]interface{}{
				"raw":     args[0],
				"time":    args[1],
				"chainID": args[2],
				"winner":  args[3],
				"layer":   args[4],
			}

			// Store in event store
			server.eventStore.mu.Lock()
			// Check if WON already exists
			_, wonExists := server.eventStore.session["WON"]
			if wonExists {
				// If WON exists, store with null winner
				nullWinnerData := make(map[string]interface{})
				for k, v := range eventData {
					nullWinnerData[k] = v
				}
				nullWinnerData["winner"] = nil
				server.eventStore.session["WON"] = nullWinnerData
			} else {
				// Otherwise, store original data
				server.eventStore.session["WON"] = eventData
			}
			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "ROUND_ENDED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogGameState: Match State Changed from InProgress to WaitingPostMatch`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched MATCH_STATE_CHANGE event (to WaitingPostMatch): ", args)

			// Get winner and loser data from event store
			server.eventStore.mu.Lock()

			// Initialize event data with time
			eventData := map[string]interface{}{
				"time": args[1],
			}

			// Add winner data if it exists
			if winnerData, exists := server.eventStore.session["ROUND_WINNER"]; exists {
				eventData["winner"] = winnerData
			} else {
				eventData["winner"] = nil
			}

			// Add loser data if it exists
			if loserData, exists := server.eventStore.session["ROUND_LOSER"]; exists {
				eventData["loser"] = loserData
			} else {
				eventData["loser"] = nil
			}

			// Clean up event store
			delete(server.eventStore.session, "ROUND_WINNER")
			delete(server.eventStore.session, "ROUND_LOSER")

			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "ROUND_ENDED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogWorld: Bringing World \/([A-z]+)\/(?:Maps\/)?([A-z0-9-]+)\/(?:.+\/)?([A-z0-9-]+)(?:\.[A-z0-9-]+)`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched NEW_GAME event (map loading): ", args)

			// Skip transition map
			if args[5] == "TransitionMap" {
				return
			}

			// Get WON data from event store if it exists
			server.eventStore.mu.Lock()

			// Initialize event data
			eventData := map[string]interface{}{
				"raw":            args[0],
				"time":           args[1],
				"chainID":        args[2],
				"dlc":            args[3],
				"mapClassname":   args[4],
				"layerClassname": args[5],
			}

			// Merge with WON data if it exists
			if wonData, exists := server.eventStore.session["WON"]; exists {
				for k, v := range wonData {
					eventData[k] = v
				}
				// Clean up WON data
				delete(server.eventStore.session, "WON")
			}

			// Clear the event store for the new game
			server.eventStore.session = make(map[string]map[string]interface{})
			server.eventStore.disconnected = make(map[string]map[string]interface{})
			// Note: We don't clear players or joinRequests as they persist across map changes

			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "NEW_GAME",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQPlayerController::)?SquadJoined\(\): Player:(.+) \(Online IDs:([^)]+)\) Joined Team ([0-9]) Squad ([0-9]+)`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched PLAYER_SQUAD_CHANGE event: ", args)

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
			teamID := args[5]
			squadID := args[6]

			eventData := map[string]interface{}{
				"raw":        args[0],
				"time":       args[1],
				"chainID":    args[2],
				"name":       playerName,
				"teamID":     teamID,
				"squadID":    squadID,
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
			server.eventStore.mu.Lock()

			// Create/update player session data
			if _, exists := server.eventStore.session[playerName]; !exists {
				server.eventStore.session[playerName] = make(map[string]interface{})
			}
			server.eventStore.session[playerName]["teamID"] = teamID
			server.eventStore.session[playerName]["squadID"] = squadID

			// If we have an EOS ID, also store team info by EOS ID
			if eosID, ok := ids["eos"]; ok {
				if _, exists := server.eventStore.players[eosID]; !exists {
					server.eventStore.players[eosID] = make(map[string]interface{})
				}
				server.eventStore.players[eosID]["teamID"] = teamID
				server.eventStore.players[eosID]["squadID"] = squadID
				server.eventStore.players[eosID]["name"] = playerName
			}

			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "PLAYER_SQUAD_CHANGE",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQPlayerController::)?TeamJoined\(\): Player:(.+) \(Online IDs:([^)]+)\) Is Now On Team ([0-9])`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched PLAYER_TEAM_CHANGE event: ", args)

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
			server.eventStore.mu.RLock()
			if playerData, exists := server.eventStore.session[playerName]; exists {
				if teamID, hasTeam := playerData["teamID"]; hasTeam {
					oldTeamID = teamID
				}
			}
			server.eventStore.mu.RUnlock()

			eventData := map[string]interface{}{
				"raw":       args[0],
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
			server.eventStore.mu.Lock()

			// Create/update player session data
			if _, exists := server.eventStore.session[playerName]; !exists {
				server.eventStore.session[playerName] = make(map[string]interface{})
			}
			server.eventStore.session[playerName]["teamID"] = newTeamID

			// If we have an EOS ID, also store team info by EOS ID
			if eosID, ok := ids["eos"]; ok {
				if _, exists := server.eventStore.players[eosID]; !exists {
					server.eventStore.players[eosID] = make(map[string]interface{})
				}
				server.eventStore.players[eosID]["teamID"] = newTeamID
				server.eventStore.players[eosID]["name"] = playerName
			}

			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "PLAYER_TEAM_CHANGE",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
}

// NewLogWatcherServer initializes the server
func NewLogWatcherServer(logFile string) *LogWatcherServer {
	log.Println("[DEBUG] Initializing LogWatcherServer with file:", logFile)
	server := &LogWatcherServer{
		clients:    make(map[pb.LogWatcher_StreamLogsServer]struct{}),
		eventSubs:  make(map[pb.LogWatcher_StreamEventsServer]struct{}),
		logFile:    logFile,
		eventStore: NewEventStore(),
	}

	// Start processing logs for events
	go server.processLogFile()

	return server
}

// Authenticate using a simple token
func validateToken(tokenString string) bool {
	if tokenString == authToken {
		log.Println("[DEBUG] Authentication successful")
		return true
	}
	log.Println("[DEBUG] Authentication failed")
	return false
}

// StreamLogs streams raw log lines to authenticated clients
func (s *LogWatcherServer) StreamLogs(req *pb.AuthRequest, stream pb.LogWatcher_StreamLogsServer) error {
	if !validateToken(req.Token) {
		return fmt.Errorf("unauthorized")
	}

	s.mu.Lock()
	s.clients[stream] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, stream)
		s.mu.Unlock()
	}()

	// Start tailing logs
	t, err := tail.TailFile(s.logFile, tail.Config{Follow: true, ReOpen: true, Poll: true})
	if err != nil {
		return err
	}

	for line := range t.Lines {
		stream.Send(&pb.LogEntry{Content: strings.TrimSpace(line.Text)})
	}

	return nil
}

// StreamEvents streams structured events found in logs
func (s *LogWatcherServer) StreamEvents(req *pb.AuthRequest, stream pb.LogWatcher_StreamEventsServer) error {
	if !validateToken(req.Token) {
		return fmt.Errorf("unauthorized")
	}

	log.Println("[DEBUG] New StreamEvents subscriber")

	s.mu.Lock()
	s.eventSubs[stream] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.eventSubs, stream)
		s.mu.Unlock()
	}()

	// Keep stream open
	for {
		select {}
	}
}

// processLogFile continuously reads logs and processes events
func (s *LogWatcherServer) processLogFile() {
	// Start tailing logs
	t, err := tail.TailFile(s.logFile, tail.Config{Follow: true, ReOpen: true, Poll: true})
	if err != nil {
		return
	}

	for line := range t.Lines {
		s.processLogForEvents(line.Text)
	}
}

// processLogForEvents detects events based on regex and broadcasts them
func (s *LogWatcherServer) processLogForEvents(logLine string) {
	for _, parser := range logParsers {
		if matches := parser.regex.FindStringSubmatch(logLine); matches != nil {
			parser.onMatch(matches, s)
		}
	}
}

// broadcastEvent sends event data to all connected event stream clients
func (s *LogWatcherServer) broadcastEvent(event *pb.EventEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for stream := range s.eventSubs {
		stream.Send(event)
	}
}

// StartServer runs the gRPC server
func StartServer(ctx context.Context, c *cli.Command) error {
	logFile := c.String("log-file")
	port := c.String("port")
	authToken = c.String("auth-token")

	server := NewLogWatcherServer(logFile)

	// Start gRPC server
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterLogWatcherServer(grpcServer, server)

	log.Printf("[INFO] LogWatcher gRPC server listening on :%s", port)
	return grpcServer.Serve(lis)
}

func main() {
	ctx := utils.WithContextSigtermCallback(context.Background(), func() {
		log.Println("[INFO] Received SIGTERM, shutting down")
	})

	app := &cli.Command{
		Name:  "logwatcher",
		Usage: "Watches a file and streams changes via gRPC",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Sources:  cli.EnvVars("LOGWATCHER_LOG_FILE"),
				Name:     "log-file",
				Usage:    "Path to the log file to watch",
				Required: true,
			},
			&cli.StringFlag{
				Sources:  cli.EnvVars("LOGWATCHER_PORT"),
				Name:     "port",
				Usage:    "Port to run the gRPC server on",
				Value:    "31135",
				Required: false,
			},
			&cli.StringFlag{
				Sources:  cli.EnvVars("LOGWATCHER_AUTH_TOKEN"),
				Name:     "auth-token",
				Usage:    "Simple auth token for authentication",
				Required: true,
			},
		},
		Action: StartServer,
	}

	if err := app.Run(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}
