package discord_fob_hab_explosion_damage

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.codycody31.dev/squad-aegis/clients/logwatcher"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
)

func (e *DiscordFOBHabExplosionDamageExtension) handleFOBHabExplosionDamage(data interface{}) error {
	deployableDamaged, ok := data.(logwatcher.DeployableDamagedEvent)
	if !ok {
		return fmt.Errorf("invalid data type for deployable damaged")
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

	match := regexp.MustCompile(`/(?:FOBRadio|Hab)_/i`)
	if !match.MatchString(deployableDamaged.Deployable) {
		return nil
	}

	match = regexp.MustCompile(`/_Deployable_/i`)
	if !match.MatchString(deployableDamaged.Weapon) {
		return nil
	}

	sqRcon := squadRcon.NewSquadRcon(e.Deps.RconManager, e.Deps.Server.Id)
	players, err := sqRcon.GetServerPlayers()
	if err != nil {
		return err
	}
	onlinePlayers := players.OnlinePlayers

	var player *squadRcon.Player
	for _, p := range onlinePlayers {
		if strings.Contains(p.Name, deployableDamaged.PlayerSuffix) {
			player = &p
			break
		}
	}

	timeDamagedAt, err := time.Parse(time.RFC3339, deployableDamaged.Time)
	if err != nil {
		return err
	}

	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("FOB/HAB Explosion Damage: %s", player.Name),
		Color: color,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Player's Name",
				Value:  player.Name,
				Inline: true,
			},
			{
				Name:   "Player's Steam ID",
				Value:  fmt.Sprintf("[%s](https://steamcommunity.com/profiles/%s)", player.SteamId, player.SteamId),
				Inline: true,
			},
			{
				Name:   "Player's EosID",
				Value:  player.EosId,
				Inline: true,
			},
			{
				Name:   "Deployable",
				Value:  deployableDamaged.Deployable,
				Inline: false,
			},
			{
				Name:   "Weapon",
				Value:  deployableDamaged.Weapon,
				Inline: false,
			},
			{
				Name:   "Server",
				Value:  e.Deps.Server.Name,
				Inline: false,
			},
		},
		Timestamp: timeDamagedAt.Format(time.RFC3339),
	}

	// Send message to Discord
	session := e.discord.GetSession()
	_, err = session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{embed},
	})

	return err
}
