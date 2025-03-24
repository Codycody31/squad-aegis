package discord_killfeed

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.codycody31.dev/squad-aegis/clients/logwatcher"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
)

func (e *DiscordKillfeedExtension) handlePlayerWounded(data interface{}) error {
	playerWoundedEvent, ok := data.(logwatcher.PlayerWoundedEvent)
	if !ok {
		return fmt.Errorf("data is not a PlayerWoundedEvent")
	}

	// Get channel ID
	channelID, ok := e.Config["channel_id"].(string)
	if !ok || channelID == "" {
		return fmt.Errorf("channel_id not configured properly")
	}

	// Set embed color
	color := 16761867 // default: orange
	if colorVal, ok := e.Config["color"].(float64); ok {
		color = int(colorVal)
	}

	timeStr := playerWoundedEvent.Time
	timeProper, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return fmt.Errorf("failed to parse time: %v", err)
	}

	sqRcon := squadRcon.NewSquadRcon(e.Deps.RconManager, e.Deps.Server.Id)
	players, err := sqRcon.GetServerPlayers()
	if err != nil {
		return fmt.Errorf("failed to get server players: %v", err)
	}

	var attacker *squadRcon.Player
	var victim *squadRcon.Player
	for _, player := range players.OnlinePlayers {
		if player.SteamId == playerWoundedEvent.AttackerSteam {
			attacker = &player
		}
		if strings.Contains(player.Name, playerWoundedEvent.VictimName) {
			victim = &player
		}
	}

	// Create embed
	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("KillFeed: %s", attacker.Name),
		Color: color,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Attacker's Name",
				Value:  attacker.Name,
				Inline: true,
			},
			{
				Name:   "Attacker's Steam ID",
				Value:  fmt.Sprintf("[%s](https://steamcommunity.com/profiles/%s)", attacker.SteamId, attacker.SteamId),
				Inline: true,
			},
			{
				Name:   "Attacker's EosID",
				Value:  attacker.EosId,
				Inline: true,
			},
			{
				Name:   "Weapon",
				Value:  playerWoundedEvent.Weapon,
				Inline: false,
			},
			{
				Name:   "Victim's Name",
				Value:  playerWoundedEvent.VictimName,
				Inline: true,
			},
			{
				Name:   "Victim's Steam ID",
				Value:  fmt.Sprintf("[%s](https://steamcommunity.com/profiles/%s)", victim.SteamId, victim.SteamId),
				Inline: true,
			},
			{
				Name:   "Victim's EosID",
				Value:  victim.EosId,
				Inline: true,
			},
		},
		Timestamp: timeProper.Format(time.RFC3339),
	}

	if e.Config["disable_cbl"] != nil && e.Config["disable_cbl"].(bool) {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "CBL",
			Value:  "Disabled",
			Inline: false,
		})
	}

	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Server",
		Value:  e.Deps.Server.Name,
		Inline: false,
	})

	// Send message to Discord
	session := e.discord.GetSession()
	_, err = session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{embed},
	})

	return err
}
