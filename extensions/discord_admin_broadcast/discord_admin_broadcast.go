package discord_admin_broadcast

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.codycody31.dev/squad-aegis/clients/logwatcher"
)

func (e *DiscordAdminBroadcastExtension) handleAdminBroadcast(data interface{}) error {
	message := data.(logwatcher.AdminBroadcastEvent)

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

	timeStr := message.Time
	timeProper, err := time.Parse(time.RFC3339, timeStr)

	// Create embed
	embed := &discordgo.MessageEmbed{
		Title: "Admin Broadcast",
		Color: color,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Message",
				Value:  message.Message,
				Inline: false,
			},
			{
				Name:   "Server",
				Value:  e.Deps.Server.Name,
				Inline: false,
			},
		},
		Timestamp: timeProper.Format(time.RFC3339),
	}

	// Send message to Discord
	session := e.discord.GetSession()
	_, err = session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{embed},
	})

	return err
}
