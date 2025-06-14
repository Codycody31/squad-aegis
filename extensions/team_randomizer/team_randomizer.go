package team_randomizer

import (
	"fmt"
	"math/rand"
	"strconv"

	"github.com/SquadGO/squad-rcon-go/v2/rconTypes"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
	"go.codycody31.dev/squad-aegis/shared/utils"
)

func (e *TeamRandomizerExtension) handleTeamRandomizationRequest(data interface{}) error {
	rconMessage, ok := data.(rconTypes.Message)
	if !ok {
		return fmt.Errorf("invalid data type for chat message")
	}

	message, err := utils.ParseRconCommandMessage(rconMessage)
	if err != nil {
		return fmt.Errorf("failed to parse RCON command message: %w", err)
	}

	if message.ChatType != "ChatAdmin" {
		return nil
	}

	if message.Command != "randomize" {
		return nil
	}

	r := squadRcon.NewSquadRcon(e.Deps.RconManager, e.Deps.Server.Id)
	serverPlayers, err := r.GetServerPlayers()
	if err != nil {
		return fmt.Errorf("failed to get server players: %w", err)
	}

	_, err = r.ExecuteRaw(fmt.Sprintf("AdminWarn %s Team Randomizer started", message.SteamID))
	if err != nil {
		return fmt.Errorf("failed to send admin warn: %w", err)
	}

	players := serverPlayers.OnlinePlayers

	// Shuffle players
	currentIndex := len(players)
	var temporaryValue interface{}
	var randomIndex int

	for currentIndex != 0 {
		randomIndex = rand.Intn(currentIndex)
		currentIndex--

		temporaryValue = players[currentIndex]
		players[currentIndex] = players[randomIndex]
		players[randomIndex] = temporaryValue.(squadRcon.Player)
	}

	team := "1"

	for _, player := range players {
		teamId, err := strconv.Atoi(team)
		if err != nil {
			return fmt.Errorf("failed to convert team ID to int: %w", err)
		}

		if player.TeamId != teamId {
			_, err := r.ExecuteRaw(fmt.Sprintf("AdminForceTeamChange %s", player.EosId))
			if err != nil {
				return fmt.Errorf("failed to force team change for player %s: %w", player.EosId, err)
			}
		}
		team = toggleTeam(team)
	}

	return nil
}

func toggleTeam(currentTeam string) string {
	if currentTeam == "1" {
		return "2"
	}
	return "1"
}
