package squadRcon

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/rcon_manager"
)

var (
	ErrNoNextMap = errors.New("next level is not defined")
	ErrNoMap     = errors.New("failed to get map")
)

type SquadRcon struct {
	// Replace direct Rcon reference with RconManager and serverID
	Manager  *rcon_manager.RconManager
	ServerID uuid.UUID
}

// Player represents a player in the game
type Player struct {
	Id              int    `json:"playerId"`
	EosId           string `json:"eosId"`
	SteamId         string `json:"steamId"`
	Name            string `json:"name"`
	TeamId          int    `json:"teamId"`
	SquadId         int    `json:"squadId"`
	SinceDisconnect string `json:"sinceDisconnect"`
	IsSquadLeader   bool   `json:"isSquadLeader"`
	Role            string `json:"role"`
}

// Squad represents a squad in the game
type Squad struct {
	ID      int      `json:"id"`
	TeamId  int      `json:"teamId"`
	Name    string   `json:"name"`
	Size    int      `json:"size"`
	Locked  bool     `json:"locked"`
	Leader  *Player  `json:"leader"`
	Players []Player `json:"players"`
}

// Team represents a team in the game
type Team struct {
	ID      int      `json:"id"`
	Name    string   `json:"name"`
	Squads  []Squad  `json:"squads"`
	Players []Player `json:"players"` // Unassigned players
}

type Map struct {
	Map      string   `json:"map"`
	Layer    string   `json:"layer"`
	Factions []string `json:"factions"`
}

type Layer struct {
	Name      string `json:"name"`
	Mod       string `json:"mod"`
	IsVanilla bool   `json:"isVanilla"`
}

// ServerInfo contains the server info and follows the A2S protocol
type ServerInfo struct {
	// Server information fields
	MaxPlayers          int    `json:"max_players"`
	GameMode            string `json:"game_mode"`
	MapName             string `json:"map_name"`
	SearchKeywords      string `json:"search_keywords"`
	GameVersion         string `json:"game_version"`
	LicensedServer      bool   `json:"licensed_server"`
	Playtime            int    `json:"playtime"`
	Flags               int    `json:"flags"`
	MatchHopper         string `json:"match_hopper"`
	MatchTimeout        int    `json:"match_timeout"`
	SessionTemplateName string `json:"session_template_name"`
	IsPasswordProtected bool   `json:"is_password_protected"`
	PlayerCount         int    `json:"player_count"`
	ServerName          string `json:"server_name"`

	// Tag fields
	TagLanguage    string `json:"tag_language"`
	TagPlaystyle   string `json:"tag_playstyle"`
	TagMapRotation string `json:"tag_map_rotation"`
	TagExperience  string `json:"tag_experience"`
	TagRules       string `json:"tag_rules"`

	// Mod information
	CurrentModLoadedCount int  `json:"current_mod_loaded_count"`
	AllModsWhitelisted    bool `json:"all_mods_whitelisted"`

	// Region and team information
	Region  string `json:"region"`
	TeamOne string `json:"team_one"`
	TeamTwo string `json:"team_two"`

	// Queue information
	PlayerReserveCount int `json:"player_reserve_count"`
	PublicQueueLimit   int `json:"public_queue_limit"`
	PublicQueue        int `json:"public_queue"`
	ReservedQueue      int `json:"reserved_queue"`
	BeaconPort         int `json:"beacon_port"`
}

func MarshalServerInfo(serverInfo string) (ServerInfo, error) {
	// Create a map to hold the raw JSON data
	var rawData map[string]interface{}
	err := json.Unmarshal([]byte(serverInfo), &rawData)
	if err != nil {
		return ServerInfo{}, err
	}

	// Create a new ServerInfo struct to populate
	result := ServerInfo{}

	// Handle integer fields with _I suffix
	for key, value := range rawData {
		switch key {
		// Integer fields with _I suffix
		case "PLAYTIME_I", "MatchTimeout_d", "FLAGS_I", "PlayerCount_I",
			"CurrentModLoadedCount_I", "PlayerReserveCount_I", "PublicQueueLimit_I",
			"PublicQueue_I", "ReservedQueue_I", "BeaconPort_I":
			// Convert string to int
			if strValue, ok := value.(string); ok {
				intValue, err := strconv.Atoi(strValue)
				if err != nil {
					return ServerInfo{}, fmt.Errorf("failed to convert %s to integer: %w", key, err)
				}

				// Assign to the appropriate field
				switch key {
				case "PLAYTIME_I":
					result.Playtime = intValue
				case "FLAGS_I":
					result.Flags = intValue
				case "MatchTimeout_d":
					result.MatchTimeout = intValue
				case "PlayerCount_I":
					result.PlayerCount = intValue
				case "CurrentModLoadedCount_I":
					result.CurrentModLoadedCount = intValue
				case "PlayerReserveCount_I":
					result.PlayerReserveCount = intValue
				case "PublicQueueLimit_I":
					result.PublicQueueLimit = intValue
				case "PublicQueue_I":
					result.PublicQueue = intValue
				case "ReservedQueue_I":
					result.ReservedQueue = intValue
				case "BeaconPort_I":
					result.BeaconPort = intValue
				}
			} else if numValue, ok := value.(float64); ok {
				// Handle case where JSON unmarshaler already converted to number
				intValue := int(numValue)

				switch key {
				case "PLAYTIME_I":
					result.Playtime = intValue
				case "FLAGS_I":
					result.Flags = intValue
				case "MatchTimeout_d":
					result.MatchTimeout = intValue
				case "PlayerCount_I":
					result.PlayerCount = intValue
				case "CurrentModLoadedCount_I":
					result.CurrentModLoadedCount = intValue
				case "PlayerReserveCount_I":
					result.PlayerReserveCount = intValue
				case "PublicQueueLimit_I":
					result.PublicQueueLimit = intValue
				case "PublicQueue_I":
					result.PublicQueue = intValue
				case "ReservedQueue_I":
					result.ReservedQueue = intValue
				case "BeaconPort_I":
					result.BeaconPort = intValue
				}
			}

		// Boolean fields with _b suffix
		case "LICENSEDSERVER_b", "Password_b", "AllModsWhitelisted_b":
			if boolValue, ok := value.(bool); ok {
				switch key {
				case "LICENSEDSERVER_b":
					result.LicensedServer = boolValue
				case "Password_b":
					result.IsPasswordProtected = boolValue
				case "AllModsWhitelisted_b":
					result.AllModsWhitelisted = boolValue
				}
			} else if strValue, ok := value.(string); ok {
				// Handle boolean as string
				boolValue := strValue == "true" || strValue == "True" || strValue == "1"
				switch key {
				case "LICENSEDSERVER_b":
					result.LicensedServer = boolValue
				case "Password_b":
					result.IsPasswordProtected = boolValue
				case "AllModsWhitelisted_b":
					result.AllModsWhitelisted = boolValue
				}
			}

		// String fields with _s suffix
		case "GameMode_s":
			if strValue, ok := value.(string); ok {
				result.GameMode = strValue
			}
		case "MapName_s":
			if strValue, ok := value.(string); ok {
				result.MapName = strValue
			}
		case "SEARCHKEYWORDS_s":
			if strValue, ok := value.(string); ok {
				result.SearchKeywords = strValue
			}
		case "GameVersion_s":
			if strValue, ok := value.(string); ok {
				result.GameVersion = strValue
			}
		case "MATCHHOPPER_s":
			if strValue, ok := value.(string); ok {
				result.MatchHopper = strValue
			}
		case "SESSIONTEMPLATENAME_s":
			if strValue, ok := value.(string); ok {
				result.SessionTemplateName = strValue
			}
		case "ServerName_s":
			if strValue, ok := value.(string); ok {
				result.ServerName = strValue
			}
		case "TagLanguage_s":
			if strValue, ok := value.(string); ok {
				result.TagLanguage = strValue
			}
		case "TagPlaystyle_s":
			if strValue, ok := value.(string); ok {
				result.TagPlaystyle = strValue
			}
		case "TagMapRotation_s":
			if strValue, ok := value.(string); ok {
				result.TagMapRotation = strValue
			}
		case "TagExperience_s":
			if strValue, ok := value.(string); ok {
				result.TagExperience = strValue
			}
		case "TagRules_s":
			if strValue, ok := value.(string); ok {
				result.TagRules = strValue
			}
		case "Region_s":
			if strValue, ok := value.(string); ok {
				result.Region = strValue
			}
		case "TeamOne_s":
			if strValue, ok := value.(string); ok {
				result.TeamOne = strValue
			}
		case "TeamTwo_s":
			if strValue, ok := value.(string); ok {
				result.TeamTwo = strValue
			}

		// Handle MaxPlayers separately as it doesn't have a suffix
		case "MaxPlayers":
			if numValue, ok := value.(float64); ok {
				result.MaxPlayers = int(numValue)
			} else if strValue, ok := value.(string); ok {
				intValue, err := strconv.Atoi(strValue)
				if err != nil {
					return ServerInfo{}, fmt.Errorf("failed to convert MaxPlayers to integer: %w", err)
				}
				result.MaxPlayers = intValue
			}
		}
	}

	return result, nil
}

// PlayersData contains online and disconnected players
type PlayersData struct {
	OnlinePlayers       []Player `json:"onlinePlayers"`
	DisconnectedPlayers []Player `json:"disconnectedPlayers"`
}

// NewSquadRcon creates a new SquadRcon instance using RconManager
func NewSquadRcon(manager *rcon_manager.RconManager, serverID uuid.UUID) *SquadRcon {
	return &SquadRcon{
		Manager:  manager,
		ServerID: serverID,
	}
}

// NewSquadRconWithConnection creates a new SquadRcon instance and connects to the server using RconManager
func NewSquadRconWithConnection(manager *rcon_manager.RconManager, serverID uuid.UUID, host string, port int, password string) (*SquadRcon, error) {
	// Connect to the server using RconManager
	err := manager.ConnectToServer(serverID, host, port, password)
	if err != nil {
		return nil, err
	}

	return &SquadRcon{
		Manager:  manager,
		ServerID: serverID,
	}, nil
}

// BanPlayer bans a player from the server
func (s *SquadRcon) BanPlayer(steamId string, duration int, reason string) error {
	_, err := s.Manager.ExecuteCommand(s.ServerID, fmt.Sprintf("AdminBan %s %dd %s", steamId, duration, reason))
	return err
}

// GetServerPlayers gets the online and disconnected players from the server
func (s *SquadRcon) GetServerPlayers() (PlayersData, error) {
	playersResponse, err := s.Manager.ExecuteCommand(s.ServerID, "ListPlayers")
	if err != nil {
		return PlayersData{}, err
	}

	onlinePlayers := []Player{}
	disconnectedPlayers := []Player{}

	lines := strings.Split(playersResponse, "\n")
	for _, line := range lines {
		matchesOnline := regexp.MustCompile(`ID: ([0-9]+) \| Online IDs: EOS: (\w{32}) steam: (\d{17}) \| Name: (.+) \| Team ID: ([0-9]+) \| Squad ID: ([0-9]+|N\/A) \| Is Leader: (True|False) \| Role: (.*)`)
		matchesDisconnected := regexp.MustCompile(`^ID: (\d{1,}) \| Online IDs: EOS: (\w{32}) steam: (\d{17}) \| Since Disconnect: (\d{2,})m.(\d{2})s \| Name: (.*?)$`)

		matchOnline := matchesOnline.FindStringSubmatch(line)
		matchDisconnected := matchesDisconnected.FindStringSubmatch(line)

		if len(matchOnline) > 0 {
			playerId, _ := strconv.Atoi(matchOnline[1])
			teamId, _ := strconv.Atoi(matchOnline[5])
			squadId, _ := strconv.Atoi(matchOnline[6])

			player := Player{
				Id:            playerId,
				EosId:         matchOnline[2],
				SteamId:       matchOnline[3],
				Name:          matchOnline[4],
				TeamId:        teamId,
				SquadId:       squadId,
				IsSquadLeader: matchOnline[7] == "True",
				Role:          matchOnline[8],
			}

			onlinePlayers = append(onlinePlayers, player)
		} else if len(matchDisconnected) > 0 {
			playerId, _ := strconv.Atoi(matchDisconnected[1])

			player := Player{
				Id:              playerId,
				EosId:           matchDisconnected[2],
				SteamId:         matchDisconnected[3],
				SinceDisconnect: matchDisconnected[4] + "m" + matchDisconnected[5] + "s",
				Name:            matchDisconnected[6],
			}

			disconnectedPlayers = append(disconnectedPlayers, player)
		}
	}

	return PlayersData{
		OnlinePlayers:       onlinePlayers,
		DisconnectedPlayers: disconnectedPlayers,
	}, nil
}

func (s *SquadRcon) GetServerSquads() ([]Squad, []string, error) {
	squadsResponse, err := s.Manager.ExecuteCommand(s.ServerID, "ListSquads")
	if err != nil {
		return []Squad{}, []string{}, err
	}

	squads := []Squad{}
	teamNames := []string{}
	var currentTeamID int

	// First pass: Extract teams and squads
	for _, line := range strings.Split(squadsResponse, "\n") {
		// Match team information
		matchesTeam := regexp.MustCompile(`^Team ID: ([1|2]) \((.*)\)`)
		matchTeam := matchesTeam.FindStringSubmatch(line)

		if len(matchTeam) > 0 {
			teamId, _ := strconv.Atoi(matchTeam[1])
			currentTeamID = teamId
			teamNames = append(teamNames, matchTeam[2])
			continue
		}

		// Match squad information
		matchesSquad := regexp.MustCompile(`^ID: (\d{1,}) \| Name: (.*?) \| Size: (\d) \| Locked: (True|False)`)
		matchSquad := matchesSquad.FindStringSubmatch(line)

		if len(matchSquad) > 0 {
			squadId, _ := strconv.Atoi(matchSquad[1])
			size, _ := strconv.Atoi(matchSquad[3])

			squad := Squad{
				ID:      squadId,
				TeamId:  currentTeamID,
				Name:    matchSquad[2],
				Size:    size,
				Locked:  matchSquad[4] == "True",
				Players: []Player{},
			}

			squads = append(squads, squad)
		}
	}

	return squads, teamNames, nil
}

func (s *SquadRcon) GetCurrentMap() (Map, error) {
	currentMap, err := s.Manager.ExecuteCommand(s.ServerID, "ShowCurrentMap")
	if err != nil {
		return Map{}, err
	}

	matchesMap := regexp.MustCompile(`^Current level is (.*?), layer is (.*?), factions (.*?) (.*?)$`)
	matches := matchesMap.FindStringSubmatch(currentMap)
	if len(matches) > 0 {
		return Map{
			Map:      matches[1],
			Layer:    matches[2],
			Factions: []string{matches[3], matches[4]},
		}, nil
	}

	return Map{}, ErrNoMap
}

func (s *SquadRcon) GetNextMap() (Map, error) {
	nextMap, err := s.Manager.ExecuteCommand(s.ServerID, "ShowNextMap")
	if err != nil {
		if nextMap == "Next level is not defined" {
			return Map{}, ErrNoNextMap
		}

		return Map{}, err
	}

	matchesMap := regexp.MustCompile(`^Next level is (.*?), layer is (.*?), factions (.*?) (.*?)$`)
	matches := matchesMap.FindStringSubmatch(nextMap)
	if len(matches) > 0 {
		return Map{
			Map:      matches[1],
			Layer:    matches[2],
			Factions: []string{matches[3], matches[4]},
		}, nil
	}

	return Map{}, ErrNoMap
}

// GetAvailableMaps gets the available maps from the server
func (s *SquadRcon) GetAvailableLayers() ([]Layer, error) {
	availableLayers, err := s.Manager.ExecuteCommand(s.ServerID, "ListLayers")
	if err != nil {
		return []Layer{}, err
	}

	availableLayers = strings.Replace(availableLayers, "List of available layers :\n", "", 1)

	layers := []Layer{}
	for _, line := range strings.Split(availableLayers, "\n") {
		if strings.Contains(line, "(") {
			mod := strings.Split(line, "(")[1]
			mod = strings.Trim(mod, ")")
			layers = append(layers, Layer{
				Name:      strings.Split(line, "(")[0],
				Mod:       mod,
				IsVanilla: false,
			})
		} else {
			layers = append(layers, Layer{
				Name:      line,
				Mod:       "",
				IsVanilla: true,
			})
		}
	}
	return layers, nil
}

// GetServerInfo gets the server info from the server
func (s *SquadRcon) GetServerInfo() (ServerInfo, error) {
	serverInfo, err := s.Manager.ExecuteCommand(s.ServerID, "ShowServerInfo")
	if err != nil {
		return ServerInfo{}, err
	}

	return MarshalServerInfo(serverInfo)
}

// ParseTeamsAndSquads builds the teams/squads structure from parsed squads and players data
func ParseTeamsAndSquads(squads []Squad, teamNames []string, players PlayersData) ([]Team, error) {
	teams := []Team{}
	teamMap := make(map[int]*Team)

	teams = append(teams, Team{
		ID:      1,
		Name:    teamNames[0],
		Squads:  []Squad{},
		Players: []Player{},
	})
	teamMap[0] = &teams[0]

	teams = append(teams, Team{
		ID:      2,
		Name:    teamNames[1],
		Squads:  []Squad{},
		Players: []Player{},
	})
	teamMap[1] = &teams[1]

	// Second pass: Assign players to squads and identify squad leaders
	squadMap := make(map[int]*Squad)
	for i := range squads {
		squadMap[squads[i].ID] = &squads[i]
	}

	for _, player := range players.OnlinePlayers {
		// Skip players with invalid squad IDs
		if player.SquadId <= 0 {
			continue
		}

		// Find the squad for this player
		if squad, ok := squadMap[player.SquadId]; ok {
			// Add player to squad
			squad.Players = append(squad.Players, player)

			// Set squad leader if applicable
			if player.IsSquadLeader {
				squad.Leader = &player
			}
		}
	}

	// Third pass: Assign squads to teams and unassigned players to teams
	// Assign squads to teams
	for _, squad := range squads {
		if team, ok := teamMap[squad.TeamId-1]; ok {
			team.Squads = append(team.Squads, squad)
		}
	}

	// Assign unassigned players to teams
	for _, player := range players.OnlinePlayers {
		// Only process players that are not in a squad (SquadId is 0 or N/A)
		if player.SquadId <= 0 {
			if team, ok := teamMap[player.TeamId-1]; ok {
				team.Players = append(team.Players, player)
			}
		}
	}

	return teams, nil
}

// GetTeamsAndSquads combines the ListPlayers and ListSquads commands to build a complete team structure
func (s *SquadRcon) GetTeamsAndSquads() ([]Team, error) {
	// Get squads
	squads, teamNames, err := s.GetServerSquads()
	if err != nil {
		return []Team{}, err
	}

	// Get players
	players, err := s.GetServerPlayers()
	if err != nil {
		return []Team{}, err
	}

	// Parse and combine the data
	return ParseTeamsAndSquads(squads, teamNames, players)
}

// Close method is no longer needed as RconManager handles connections
// But we'll keep a no-op version for API compatibility
func (s *SquadRcon) Close() {
	// No-op - connection management is handled by RconManager
}

// ExecuteRaw allows executing raw RCON commands directly
func (s *SquadRcon) ExecuteRaw(command string) (string, error) {
	return s.Manager.ExecuteCommand(s.ServerID, command)
}
