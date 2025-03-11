package squadRcon

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.codycody31.dev/squad-aegis/internal/rcon"
)

var (
	ErrNoNextMap = errors.New("next level is not defined")
	ErrNoMap     = errors.New("failed to get map")
)

type SquadRcon struct {
	rcon *rcon.Rcon
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

// PlayersData contains online and disconnected players
type PlayersData struct {
	OnlinePlayers       []Player `json:"onlinePlayers"`
	DisconnectedPlayers []Player `json:"disconnectedPlayers"`
}

func NewSquadRcon(rconConfig rcon.RconConfig) (*SquadRcon, error) {
	rcon, err := rcon.NewRcon(rconConfig)
	if err != nil {
		return nil, err
	}

	return &SquadRcon{rcon: rcon}, nil
}

// BanPlayer bans a player from the server
func (s *SquadRcon) BanPlayer(steamId string, duration int, reason string) error {
	_, err := s.rcon.Execute(fmt.Sprintf("AdminBan %s %dd %s", steamId, duration, reason))
	return err
}

// GetServerPlayers gets the online and disconnected players from the server
func (s *SquadRcon) GetServerPlayers() (PlayersData, error) {
	playersResponse, err := s.rcon.Execute("ListPlayers")
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

func (s *SquadRcon) GetServerSquads() ([]Squad, error) {
	squadsResponse, err := s.rcon.Execute("ListSquads")
	if err != nil {
		return []Squad{}, err
	}

	squads := []Squad{}
	var currentTeamID int

	// First pass: Extract teams and squads
	for _, line := range strings.Split(squadsResponse, "\n") {
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

	return squads, nil
}

func (s *SquadRcon) GetCurrentMap() (Map, error) {
	currentMap, err := s.rcon.Execute("ShowCurrentMap")
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
	nextMap, err := s.rcon.Execute("ShowNextMap")
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
	availableLayers, err := s.rcon.Execute("ListLayers")
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

// ParseTeamsAndSquads builds the teams/squads structure from parsed squads and players data
func ParseTeamsAndSquads(squads []Squad, players PlayersData) ([]Team, error) {
	teams := []Team{}

	// First pass: Extract teams from squads
	teamMap := make(map[int]*Team)
	for _, squad := range squads {
		// Create team if it doesn't exist
		if _, exists := teamMap[squad.TeamId]; !exists {
			team := Team{
				ID:      squad.TeamId,
				Name:    getTeamName(squad.TeamId), // Helper function to get team name
				Squads:  []Squad{},
				Players: []Player{},
			}
			teams = append(teams, team)
			teamMap[squad.TeamId] = &teams[len(teams)-1]
		}
	}

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
		if team, ok := teamMap[squad.TeamId]; ok {
			team.Squads = append(team.Squads, squad)
		}
	}

	// Assign unassigned players to teams
	for _, player := range players.OnlinePlayers {
		// Only process players that are not in a squad (SquadId is 0 or N/A)
		if player.SquadId <= 0 {
			if team, ok := teamMap[player.TeamId]; ok {
				team.Players = append(team.Players, player)
			}
		}
	}

	return teams, nil
}

// Helper function to get team name based on ID
func getTeamName(teamId int) string {
	switch teamId {
	case 1:
		return "Team 1"
	case 2:
		return "Team 2"
	default:
		return "Unknown Team"
	}
}

// GetTeamsAndSquads combines the ListPlayers and ListSquads commands to build a complete team structure
func (s *SquadRcon) GetTeamsAndSquads() ([]Team, error) {
	// Get squads
	squads, err := s.GetServerSquads()
	if err != nil {
		return []Team{}, err
	}

	// Get players
	players, err := s.GetServerPlayers()
	if err != nil {
		return []Team{}, err
	}

	// Parse and combine the data
	return ParseTeamsAndSquads(squads, players)
}

// ParseTeamsFromRawSquadsResponse parses the raw squads response and extracts teams
// This is kept for backward compatibility or if needed to parse directly from raw response
func ParseTeamsFromRawSquadsResponse(squadsResponse string) []Team {
	teams := []Team{}
	teamMap := make(map[int]*Team)

	// Extract teams
	for _, line := range strings.Split(squadsResponse, "\n") {
		// Match team information
		matchesTeam := regexp.MustCompile(`^Team ID: ([1|2]) \((.*)\)`)
		matchTeam := matchesTeam.FindStringSubmatch(line)

		if len(matchTeam) > 0 {
			teamId, _ := strconv.Atoi(matchTeam[1])

			team := Team{
				ID:      teamId,
				Name:    matchTeam[2],
				Squads:  []Squad{},
				Players: []Player{},
			}

			teams = append(teams, team)
			teamMap[teamId] = &teams[len(teams)-1]
		}
	}

	return teams
}

func (s *SquadRcon) Close() {
	s.rcon.Close()
}
