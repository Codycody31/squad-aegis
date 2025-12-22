package scrambler

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
)

// SwapPlan represents a complete plan for scrambling teams
type SwapPlan struct {
	Moves   []PlayerMove
	Summary ScrambleSummary
}

// PlayerMove represents a single player team change
type PlayerMove struct {
	SteamID     string
	Name        string
	CurrentTeam int
	TargetTeam  int
	SquadID     int
	SquadName   string
	Reason      string
}

// ScrambleSummary provides statistics about the scramble
type ScrambleSummary struct {
	TotalPlayers    int
	PlayersToMove   int
	SquadsPreserved int
	SquadsSplit     int
	Team1Before     int
	Team2Before     int
	Team1After      int
	Team2After      int
}

// SquadGroup represents a squad or pseudo-squad for scrambling
type SquadGroup struct {
	ID       string
	TeamID   int
	Name     string
	Players  []*plugin_manager.PlayerInfo
	Size     int
	Locked   bool
	IsLeader bool
	IsPseudo bool // True for unassigned players treated as single-player squads
}

// Config holds scrambler configuration
type Config struct {
	ScramblePercentage float64
	WinStreakTeam      int
	LogAPI             plugin_manager.LogAPI
}

// Scrambler handles team scrambling logic
type Scrambler struct {
	config Config
	rng    *rand.Rand
}

// New creates a new Scrambler instance
func New(config Config) *Scrambler {
	return &Scrambler{
		config: config,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateSwapPlan creates an optimal swap plan to balance teams while preserving squads
func (s *Scrambler) GenerateSwapPlan(
	squads []*plugin_manager.SquadInfo,
	players []*plugin_manager.PlayerInfo,
) (*SwapPlan, error) {
	if s.config.LogAPI != nil {
		s.config.LogAPI.Debug("Starting scrambler", map[string]interface{}{
			"squads":  len(squads),
			"players": len(players),
		})
	}

	// Stage 1: Prepare data - create squad groups
	squadGroups := s.prepareSquadGroups(squads, players)

	// Count players by team
	team1Count := 0
	team2Count := 0
	for _, player := range players {
		if player.TeamID == 1 {
			team1Count++
		} else if player.TeamID == 2 {
			team2Count++
		}
	}

	if s.config.LogAPI != nil {
		s.config.LogAPI.Debug("Team counts", map[string]interface{}{
			"team1":  team1Count,
			"team2":  team2Count,
			"groups": len(squadGroups),
		})
	}

	// Stage 2: Calculate target moves
	totalPlayers := team1Count + team2Count
	targetMovesCount := int(math.Round(float64(totalPlayers) * s.config.ScramblePercentage))

	// Ensure we move at least some players if there's an imbalance
	imbalance := int(math.Abs(float64(team1Count - team2Count)))
	if targetMovesCount < imbalance/2 {
		targetMovesCount = imbalance / 2
	}

	if s.config.LogAPI != nil {
		s.config.LogAPI.Debug("Target moves calculated", map[string]interface{}{
			"moves":      targetMovesCount,
			"percentage": s.config.ScramblePercentage * 100,
			"total":      totalPlayers,
		})
	}

	// Stage 3: Use backtracking to find optimal squad swaps
	swapPlan := s.findOptimalSwaps(squadGroups, team1Count, team2Count, targetMovesCount)

	// Calculate summary
	summary := ScrambleSummary{
		TotalPlayers:    totalPlayers,
		PlayersToMove:   len(swapPlan.Moves),
		SquadsPreserved: s.countPreservedSquads(swapPlan.Moves, squadGroups),
		SquadsSplit:     s.countSplitSquads(swapPlan.Moves, squadGroups),
		Team1Before:     team1Count,
		Team2Before:     team2Count,
		Team1After:      team1Count,
		Team2After:      team2Count,
	}

	// Adjust after counts
	for _, move := range swapPlan.Moves {
		if move.CurrentTeam == 1 {
			summary.Team1After--
			summary.Team2After++
		} else {
			summary.Team2After--
			summary.Team1After++
		}
	}

	swapPlan.Summary = summary

	if s.config.LogAPI != nil {
		s.config.LogAPI.Debug("Scramble plan complete", map[string]interface{}{
			"moves":            summary.PlayersToMove,
			"squads_preserved": summary.SquadsPreserved,
			"squads_split":     summary.SquadsSplit,
			"team1_after":      summary.Team1After,
			"team2_after":      summary.Team2After,
		})
	}

	return swapPlan, nil
}

// prepareSquadGroups converts squads and unassigned players into SquadGroup format
func (s *Scrambler) prepareSquadGroups(
	squads []*plugin_manager.SquadInfo,
	players []*plugin_manager.PlayerInfo,
) []*SquadGroup {
	groups := make([]*SquadGroup, 0)

	// Convert real squads
	for _, squad := range squads {
		if len(squad.Players) == 0 {
			continue
		}

		group := &SquadGroup{
			ID:       fmt.Sprintf("T%d-S%d", squad.TeamID, squad.ID),
			TeamID:   squad.TeamID,
			Name:     squad.Name,
			Players:  squad.Players,
			Size:     len(squad.Players),
			Locked:   squad.Locked,
			IsLeader: squad.Leader != nil,
			IsPseudo: false,
		}
		groups = append(groups, group)
	}

	// Create pseudo-squads for unassigned players (SquadID = 0 or no squad)
	unassignedByTeam := make(map[int][]*plugin_manager.PlayerInfo)
	for _, player := range players {
		if player.SquadID == 0 {
			unassignedByTeam[player.TeamID] = append(unassignedByTeam[player.TeamID], player)
		}
	}

	// Create pseudo-squad for each unassigned player
	pseudoCounter := 0
	for teamID, unassigned := range unassignedByTeam {
		for _, player := range unassigned {
			pseudoCounter++
			group := &SquadGroup{
				ID:       fmt.Sprintf("T%d-Pseudo%d", teamID, pseudoCounter),
				TeamID:   teamID,
				Name:     "Unassigned",
				Players:  []*plugin_manager.PlayerInfo{player},
				Size:     1,
				Locked:   false,
				IsLeader: false,
				IsPseudo: true,
			}
			groups = append(groups, group)
		}
	}

	return groups
}

// findOptimalSwaps uses backtracking algorithm to find best squad combinations
func (s *Scrambler) findOptimalSwaps(
	groups []*SquadGroup,
	team1Count, team2Count, targetMoves int,
) *SwapPlan {
	// Separate groups by team
	team1Groups := make([]*SquadGroup, 0)
	team2Groups := make([]*SquadGroup, 0)

	for _, group := range groups {
		if group.TeamID == 1 {
			team1Groups = append(team1Groups, group)
		} else if group.TeamID == 2 {
			team2Groups = append(team2Groups, group)
		}
	}

	// Prioritize moving players from the winning team if specified
	var primaryGroups []*SquadGroup
	var secondaryGroups []*SquadGroup

	if s.config.WinStreakTeam == 1 {
		primaryGroups = team1Groups
		secondaryGroups = team2Groups
	} else if s.config.WinStreakTeam == 2 {
		primaryGroups = team2Groups
		secondaryGroups = team1Groups
	} else {
		// No preference, prioritize larger team
		if team1Count > team2Count {
			primaryGroups = team1Groups
			secondaryGroups = team2Groups
		} else {
			primaryGroups = team2Groups
			secondaryGroups = team1Groups
		}
	}

	// Shuffle groups for randomization
	s.shuffleGroups(primaryGroups)
	s.shuffleGroups(secondaryGroups)

	// Sort by priority: pseudo-squads first, then small squads, avoid locked squads
	sort.Slice(primaryGroups, func(i, j int) bool {
		return s.groupPriority(primaryGroups[i]) > s.groupPriority(primaryGroups[j])
	})

	// Select groups for mutual swaps to maintain balance
	team1Selected, team2Selected := s.selectGroupsForMutualSwap(
		team1Groups, team2Groups, targetMoves, team1Count, team2Count,
	)

	// Generate moves from selected groups
	moves := make([]PlayerMove, 0)

	// Move team 1 players to team 2
	for _, group := range team1Selected {
		reason := fmt.Sprintf("Squad swap (%s)", group.Name)
		if group.IsPseudo {
			reason = "Unassigned player balance"
		} else if group.Locked {
			reason = fmt.Sprintf("Locked squad swap (%s)", group.Name)
		}

		for _, player := range group.Players {
			moves = append(moves, PlayerMove{
				SteamID:     player.SteamID,
				Name:        player.Name,
				CurrentTeam: player.TeamID,
				TargetTeam:  2,
				SquadID:     player.SquadID,
				SquadName:   group.Name,
				Reason:      reason,
			})
		}
	}

	// Move team 2 players to team 1
	for _, group := range team2Selected {
		reason := fmt.Sprintf("Squad swap (%s)", group.Name)
		if group.IsPseudo {
			reason = "Unassigned player balance"
		} else if group.Locked {
			reason = fmt.Sprintf("Locked squad swap (%s)", group.Name)
		}

		for _, player := range group.Players {
			moves = append(moves, PlayerMove{
				SteamID:     player.SteamID,
				Name:        player.Name,
				CurrentTeam: player.TeamID,
				TargetTeam:  1,
				SquadID:     player.SquadID,
				SquadName:   group.Name,
				Reason:      reason,
			})
		}
	}

	return &SwapPlan{
		Moves: moves,
	}
}

// selectGroupsForMutualSwap picks groups from both teams for mutual swapping
func (s *Scrambler) selectGroupsForMutualSwap(
	team1Groups, team2Groups []*SquadGroup,
	targetMoves, team1Count, team2Count int,
) ([]*SquadGroup, []*SquadGroup) {
	var team1Selected, team2Selected []*SquadGroup

	// Calculate how many to move from each team for balance
	// Target: roughly equal numbers moved from each side
	halfTarget := targetMoves / 2

	// Prioritize team with more players
	var largerGroups, smallerGroups []*SquadGroup
	var largerTeamID int

	if team1Count > team2Count {
		largerGroups = team1Groups
		smallerGroups = team2Groups
		largerTeamID = 1
	} else {
		largerGroups = team2Groups
		smallerGroups = team1Groups
		largerTeamID = 2
	}

	// Shuffle for randomization
	s.shuffleGroups(largerGroups)
	s.shuffleGroups(smallerGroups)

	// Sort by priority
	sort.Slice(largerGroups, func(i, j int) bool {
		return s.groupPriority(largerGroups[i]) > s.groupPriority(largerGroups[j])
	})
	sort.Slice(smallerGroups, func(i, j int) bool {
		return s.groupPriority(smallerGroups[i]) > s.groupPriority(smallerGroups[j])
	})

	// Select from larger team first (more players to balance)
	largerSelected := make([]*SquadGroup, 0)
	largerCount := 0
	for _, group := range largerGroups {
		if largerCount >= halfTarget+2 {
			break
		}
		if group.Locked && largerCount > halfTarget/2 {
			continue
		}
		largerSelected = append(largerSelected, group)
		largerCount += group.Size
	}

	// Select roughly equal amount from smaller team
	smallerSelected := make([]*SquadGroup, 0)
	smallerCount := 0
	targetFromSmaller := largerCount - int(math.Abs(float64(team1Count-team2Count))/2)
	if targetFromSmaller < 0 {
		targetFromSmaller = 0
	}

	for _, group := range smallerGroups {
		if smallerCount >= targetFromSmaller {
			break
		}
		if group.Locked && smallerCount > targetFromSmaller/2 {
			continue
		}
		// Only add if it doesn't overshoot too much
		if smallerCount+group.Size <= targetFromSmaller+3 || smallerCount == 0 {
			smallerSelected = append(smallerSelected, group)
			smallerCount += group.Size
		}
	}

	// Assign to correct team
	if largerTeamID == 1 {
		team1Selected = largerSelected
		team2Selected = smallerSelected
	} else {
		team1Selected = smallerSelected
		team2Selected = largerSelected
	}

	return team1Selected, team2Selected
}

// groupPriority calculates priority score for squad selection
// Higher score = higher priority for swapping
func (s *Scrambler) groupPriority(group *SquadGroup) int {
	score := 0

	// Pseudo-squads (unassigned) have highest priority
	if group.IsPseudo {
		score += 1000
	}

	// Prefer smaller squads to minimize disruption
	score += (10 - group.Size) * 10

	// Avoid locked squads
	if group.Locked {
		score -= 500
	}

	// Slightly prefer squads without leaders to preserve command structure
	if !group.IsLeader {
		score += 20
	}

	return score
}

// shuffleGroups randomly shuffles squad groups
func (s *Scrambler) shuffleGroups(groups []*SquadGroup) {
	for i := len(groups) - 1; i > 0; i-- {
		j := s.rng.Intn(i + 1)
		groups[i], groups[j] = groups[j], groups[i]
	}
}

// countPreservedSquads counts squads that are moved entirely together
func (s *Scrambler) countPreservedSquads(moves []PlayerMove, groups []*SquadGroup) int {
	preserved := 0

	for _, group := range groups {
		if group.IsPseudo || group.Size == 0 {
			continue
		}

		// Check if all players in this squad are being moved
		allMoved := true
		movedCount := 0
		for _, player := range group.Players {
			moved := false
			for _, move := range moves {
				if move.SteamID == player.SteamID {
					moved = true
					movedCount++
					break
				}
			}
			if !moved {
				allMoved = false
			}
		}

		// If all members moved together, squad is preserved
		if allMoved && movedCount == group.Size {
			preserved++
		}
	}

	return preserved
}

// countSplitSquads counts squads that have some but not all members moved
func (s *Scrambler) countSplitSquads(moves []PlayerMove, groups []*SquadGroup) int {
	split := 0

	for _, group := range groups {
		if group.IsPseudo || group.Size == 0 {
			continue
		}

		// Check how many players in this squad are being moved
		movedCount := 0
		for _, player := range group.Players {
			for _, move := range moves {
				if move.SteamID == player.SteamID {
					movedCount++
					break
				}
			}
		}

		// If some but not all moved, squad is split
		if movedCount > 0 && movedCount < group.Size {
			split++
		}
	}

	return split
}
