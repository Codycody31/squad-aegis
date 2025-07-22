package discord_admin_request

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/SquadGO/squad-rcon-go/v2/rconTypes"
	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
	"go.codycody31.dev/squad-aegis/internal/shared/utils"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
)

// handleChatMessage handles chat messages and looks for admin requests
func (e *DiscordAdminRequestExtension) handleChatMessage(data interface{}) error {
	rconMessage, ok := data.(rconTypes.Message)
	if !ok {
		return fmt.Errorf("invalid data type for chat message")
	}

	message, err := utils.ParseRconCommandMessage(rconMessage)
	if err != nil {
		return fmt.Errorf("failed to parse RCON command message: %w", err)
	}

	ignoreChats := plug_config_schema.GetArrayStringValue(e.Config, "ignore_chats")
	if slices.Contains(ignoreChats, message.ChatType) {
		return nil
	}

	if message.Command != "admin" {
		return nil
	}

	// Extract reason (everything after the command)
	reason := strings.TrimSpace(message.Args)

	if reason == "" {
		r := squadRcon.NewSquadRcon(e.Deps.RconManager, e.Deps.Server.Id)
		_, err := r.ExecuteRaw(fmt.Sprintf("AdminWarn %s Please specify what you would like help with when requesting an admin.", message.SteamID))
		if err != nil {
			log.Error().
				Err(err).
				Str("serverID", e.Deps.Server.Id.String()).
				Msg("Failed to notify player")
		}
		return nil
	}

	// Check if we can ping (cooldown)
	e.mu.Lock()
	pingDelay := 60000 // default cooldown: 60 seconds
	if delay, ok := e.Config["ping_delay"].(float64); ok {
		pingDelay = int(delay)
	}
	canPing := time.Since(e.lastPingTime).Milliseconds() > int64(pingDelay)
	if canPing {
		e.lastPingTime = time.Now()
	}
	e.mu.Unlock()

	r := squadRcon.NewSquadRcon(e.Deps.RconManager, e.Deps.Server.Id)

	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Select("u.steam_id").
		From("server_admins sa").
		Join("users u ON sa.user_id = u.id").
		Join("server_roles sr ON sa.server_role_id = sr.id").
		Where(squirrel.Eq{"sa.server_id": e.Deps.Server.Id}).
		Where(squirrel.Like{"sr.permissions": "%canseeadminchat%"}).
		ToSql()
	if err != nil {
		log.Error().
			Err(err).
			Str("serverID", e.Deps.Server.Id.String()).
			Msg("Failed to get server admins")
	}

	var steamIDs []int64
	rows, err := e.Deps.Database.QueryContext(context.Background(), sql, args...)
	if err != nil {
		log.Error().
			Err(err).
			Str("serverID", e.Deps.Server.Id.String()).
			Msg("Failed to get server admins")
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var steamID int64
		if err := rows.Scan(&steamID); err != nil {
			log.Error().
				Err(err).
				Str("serverID", e.Deps.Server.Id.String()).
				Msg("Failed to scan steam ID")
			return err
		}
		steamIDs = append(steamIDs, steamID)
	}

	if err := rows.Err(); err != nil {
		log.Error().
			Err(err).
			Str("serverID", e.Deps.Server.Id.String()).
			Msg("Error occurred during rows iteration")
		return err
	}

	team := 0
	squad := 0
	var onlineAdmins []int64

	players, err := r.GetServerPlayers()
	if err != nil {
		log.Error().
			Err(err).
			Str("serverID", e.Deps.Server.Id.String()).
			Msg("Failed to get server players")
	}
	for _, player := range players.OnlinePlayers {
		for _, steamID := range steamIDs {
			psid, err := strconv.ParseInt(player.SteamId, 10, 64)
			if err != nil {
				log.Error().
					Err(err).
					Str("serverID", e.Deps.Server.Id.String()).
					Msg("Failed to parse steam ID")
			}
			if steamID == psid {
				onlineAdmins = append(onlineAdmins, steamID)
			}
		}
		if player.SteamId == message.SteamID {
			team = player.TeamId
			squad = player.SquadId
			break
		}
	}

	// Format the Discord message
	err = e.sendDiscordMessage(message.PlayerName, message.SteamID, message.EosID, reason, team, squad, onlineAdmins, canPing)
	if err != nil {
		log.Error().
			Err(err).
			Str("serverID", e.Deps.Server.Id.String()).
			Msg("Failed to send Discord message")
	}

	userWarnMessage := "An admin has been notified. Please wait for us to get back to you."
	if e.Config["show_in_game_admins"] != nil {
		showInGameAdmins := e.Config["show_in_game_admins"].(bool)
		if len(onlineAdmins) == 0 && showInGameAdmins {
			userWarnMessage = "There are no in-game admins, however, an admin has been notified via Discord. Please wait for us to get back to you."
		} else if len(onlineAdmins) > 0 && showInGameAdmins {
			amountAdminsString := "are"
			if len(onlineAdmins) == 1 {
				amountAdminsString = "is"
			}
			amountAdminsStringPlural := "s"
			if len(onlineAdmins) == 1 {
				amountAdminsStringPlural = ""
			}

			userWarnMessage = fmt.Sprintf("There %s %d in-game admin%s. Please wait for us to get back to you.", amountAdminsString, len(onlineAdmins), amountAdminsStringPlural)
		}
	}

	_, err = r.ExecuteRaw(fmt.Sprintf("AdminWarn %s %s", message.SteamID, userWarnMessage))
	if err != nil {
		log.Error().
			Err(err).
			Str("serverID", e.Deps.Server.Id.String()).
			Msg("Failed to notify player")
	}

	if e.Config["warn_in_game_admins"] != nil {
		warnInGameAdmins := e.Config["warn_in_game_admins"].(bool)
		if warnInGameAdmins {
			for _, steamID := range steamIDs {
				r.ExecuteRaw(fmt.Sprintf("AdminWarn %d [%s] - %s", steamID, message.PlayerName, reason))
			}
		}
	}

	return nil
}

// sendDiscordMessage sends the admin request to Discord
func (e *DiscordAdminRequestExtension) sendDiscordMessage(playerName, steamID, eosID, reason string, team, squad int, onlineAdmins []int64, canPing bool) error {
	channelID, ok := e.Config["channel_id"].(string)
	if !ok || channelID == "" {
		return fmt.Errorf("channel_id not configured properly")
	}

	// Create the message content with pings if allowed
	content := ""
	if canPing {
		// Add @here if configured
		pingHere, _ := e.Config["ping_here"].(bool)
		if pingHere {
			content += "@here Admin Requested in " + e.Deps.Server.Name
		}

		// Add role pings if configured
		if pingGroups, ok := e.Config["ping_groups"].([]interface{}); ok {
			for _, group := range pingGroups {
				if groupID, ok := group.(string); ok {
					content += fmt.Sprintf("<@&%s> ", groupID)
				}
			}
		}
	}

	// Set embed color
	color := 16761867 // default: orange
	if colorVal, ok := e.Config["color"].(float64); ok {
		color = int(colorVal)
	}

	// Create embed
	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("Player **%s** has requested admin support!", playerName),
		Color: color,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Player",
				Value:  playerName,
				Inline: true,
			},
			{
				Name:   "Steam ID",
				Value:  fmt.Sprintf("[%s](https://steamcommunity.com/profiles/%s)", steamID, steamID),
				Inline: true,
			},
			{
				Name:   "Player's EosID",
				Value:  eosID,
				Inline: true,
			},
			{
				Name:   "Team & Squad",
				Value:  fmt.Sprintf("Team: %d, Squad: %d", team, squad),
				Inline: false,
			},
			{
				Name:   "Reason",
				Value:  reason,
				Inline: false,
			},
			{
				Name:   "Online Admins",
				Value:  fmt.Sprintf("%d", len(onlineAdmins)),
				Inline: false,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Send message to Discord
	session := e.discord.GetSession()
	_, err := session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: content,
		Embeds:  []*discordgo.MessageEmbed{embed},
	})

	return err
}
