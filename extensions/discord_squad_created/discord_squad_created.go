package discord_squad_created

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.codycody31.dev/squad-aegis/internal/rcon"
)

func (e *DiscordSquadCreatedExtension) handleSquadCreated(data interface{}) error {
	squadCreated, ok := data.(rcon.SquadCreated)
	if !ok {
		return fmt.Errorf("invalid data type for squad created")
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

	// Get use embed
	useEmbed, ok := e.Config["use_embed"].(bool)
	if !ok {
		useEmbed = true
	}

	if useEmbed {
		// Create embed
		embed := &discordgo.MessageEmbed{
			Title: "Squad Created",
			Color: color,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Player",
					Value:  squadCreated.PlayerName,
					Inline: true,
				},
				{
					Name:   "Team",
					Value:  squadCreated.TeamName,
					Inline: true,
				},
				{
					Name:   "Server",
					Value:  e.Deps.Server.Name,
					Inline: false,
				},
				{
					Name:   "Squad Number & Squad Name",
					Value:  fmt.Sprintf("%s : %s", squadCreated.SquadID, squadCreated.SquadName),
					Inline: false,
				},
			},
			Timestamp: time.Now().Format(time.RFC3339),
		}

		// Send message to Discord
		session := e.discord.GetSession()
		_, err := session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{embed},
		})

		return err
	} else {
		message := fmt.Sprintf("```Player: %s\n created Squad %s : %s\n on %s```", squadCreated.PlayerName, squadCreated.SquadID, squadCreated.SquadName, squadCreated.TeamName)

		// Send message to Discord
		session := e.discord.GetSession()
		_, err := session.ChannelMessageSend(channelID, message)
		return err
	}
}
