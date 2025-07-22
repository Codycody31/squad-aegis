package discord_chat

import (
	"fmt"
	"slices"
	"time"

	"github.com/SquadGO/squad-rcon-go/v2/rconTypes"
	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
)

// handleChatMessage handles chat messages and looks for admin requests
func (e *DiscordChatExtension) handleChatMessage(data interface{}) error {
	message, ok := data.(rconTypes.Message)
	if !ok {
		return fmt.Errorf("invalid data type for chat message")
	}

	ignoreChats := plug_config_schema.GetArrayStringValue(e.Config, "ignore_chats")
	if slices.Contains(ignoreChats, message.ChatType) {
		return nil
	}

	team := 0
	squad := 0

	r := squadRcon.NewSquadRcon(e.Deps.RconManager, e.Deps.Server.Id)
	players, err := r.GetServerPlayers()
	if err != nil {
		log.Error().
			Err(err).
			Str("serverID", e.Deps.Server.Id.String()).
			Msg("Failed to get server players")
	}
	for _, player := range players.OnlinePlayers {
		if player.SteamId == message.SteamID {
			team = player.TeamId
			squad = player.SquadId
			break
		}
	}

	// Send message to Discord
	return e.sendDiscordMessage(message, team, squad)
}

// sendDiscordMessage sends the chat message to Discord
func (e *DiscordChatExtension) sendDiscordMessage(message rconTypes.Message, team, squad int) error {
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
		Title: message.ChatType,
		Color: color,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Player",
				Value:  message.PlayerName,
				Inline: false,
			},
			{
				Name:   "Steam ID",
				Value:  fmt.Sprintf("[%s](https://steamcommunity.com/profiles/%s)", message.SteamID, message.SteamID),
				Inline: true,
			},
			{
				Name:   "Player's EosID",
				Value:  message.EosID,
				Inline: true,
			},
			{
				Name:   "Team & Squad",
				Value:  fmt.Sprintf("Team: %d, Squad: %d", team, squad),
				Inline: false,
			},
			{
				Name:   "Server",
				Value:  e.Deps.Server.Name,
				Inline: false,
			},
			{
				Name:   "Message",
				Value:  message.Message,
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
}
