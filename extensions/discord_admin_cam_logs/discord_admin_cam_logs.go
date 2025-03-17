package discord_admin_cam_logs

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.codycody31.dev/squad-aegis/internal/rcon"
)

func (e *DiscordAdminCamLogsExtension) handleAdminCamPossessed(data interface{}) error {
	message := data.(rcon.PosAdminCam)

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

	// Create embed
	embed := &discordgo.MessageEmbed{
		Title: "Admin Entered Admin Camera",
		Color: color,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Admin's Name",
				Value:  message.AdminName,
				Inline: false,
			},
			{
				Name:   "Admin's Steam ID",
				Value:  fmt.Sprintf("[%s](https://steamcommunity.com/profiles/%s)", message.SteamID, message.SteamID),
				Inline: true,
			},
			{
				Name:   "Admin's EosID",
				Value:  message.EosID,
				Inline: true,
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
}

func (e *DiscordAdminCamLogsExtension) handleAdminCamUnpossessed(data interface{}) error {
	message := data.(rcon.UnposAdminCam)

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

	// Create embed
	embed := &discordgo.MessageEmbed{
		Title: "Admin Left Admin Camera",
		Color: color,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Admin's Name",
				Value:  message.AdminName,
				Inline: false,
			},
			{
				Name:   "Admin's Steam ID",
				Value:  fmt.Sprintf("[%s](https://steamcommunity.com/profiles/%s)", message.SteamID, message.SteamID),
				Inline: true,
			},
			{
				Name:   "Admin's EosID",
				Value:  message.EosID,
				Inline: true,
			},
			// TODO: Add time in admin camera
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Send message to Discord
	session := e.discord.GetSession()
	_, err := session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{embed},
	})

	return err
}
