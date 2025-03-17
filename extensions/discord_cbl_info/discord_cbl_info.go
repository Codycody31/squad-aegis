package discord_cbl_info

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bwmarrin/discordgo"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
)

// CBLSteamUser represents a player's data from the Community Ban List
type CBLSteamUser struct {
	ID                            string  `json:"id"`
	Name                          string  `json:"name"`
	AvatarFull                    string  `json:"avatarFull"`
	ReputationPoints              int     `json:"reputationPoints"`
	ReputationPointsMonthChange   int     `json:"reputationPointsMonthChange"`
	RiskRating                    int     `json:"riskRating"`
	ReputationRank                int     `json:"reputationRank"`
	LastRefreshedInfo             string  `json:"lastRefreshedInfo"`
	LastRefreshedReputationPoints string  `json:"lastRefreshedReputationPoints"`
	LastRefreshedReputationRank   string  `json:"lastRefreshedReputationRank"`
	ActiveBans                    CBLBans `json:"activeBans"`
	ExpiredBans                   CBLBans `json:"expiredBans"`
}

// CBLBans represents the bans a player has
type CBLBans struct {
	Edges []CBLBanEdge `json:"edges"`
}

// CBLBanEdge represents a ban edge in the GraphQL response
type CBLBanEdge struct {
	Cursor string     `json:"cursor"`
	Node   CBLBanNode `json:"node"`
}

// CBLBanNode represents a ban node in the GraphQL response
type CBLBanNode struct {
	ID string `json:"id"`
}

// GraphQLResponse represents the response from the CBL GraphQL API
type GraphQLResponse struct {
	Data struct {
		SteamUser CBLSteamUser `json:"steamUser"`
	} `json:"data"`
}

// PlayerConnectedData represents a player connection event
type PlayerConnectedData struct {
	Player   squadRcon.Player `json:"player"`
	Time     time.Time        `json:"time"`
	ServerID string           `json:"serverId"`
}

func (e *DiscordCBLInfoExtension) handlePlayerConnected(data interface{}) error {
	connData, ok := data.(PlayerConnectedData)
	if !ok {
		return fmt.Errorf("invalid data type for PLAYER_CONNECTED event: %T", data)
	}

	steamID := connData.Player.SteamId
	playerName := connData.Player.Name

	// Skip if steam ID is empty
	if steamID == "" {
		return nil
	}

	// Query the CBL GraphQL API
	cblUser, err := e.fetchCBLData(steamID)
	if err != nil {
		return fmt.Errorf("failed to fetch CBL data for player %s (Steam ID: %s): %w", playerName, steamID, err)
	}

	// If no data found for the user or reputation is below threshold, return
	if cblUser == nil {
		return nil
	}

	// Get threshold from config
	threshold := 6 // Default threshold
	if thresholdVal, ok := e.Config["threshold"].(float64); ok {
		threshold = int(thresholdVal)
	}

	if cblUser.ReputationPoints < threshold {
		return nil
	}

	// Get channel ID from config
	channelID, ok := e.Config["channel_id"].(string)
	if !ok || channelID == "" {
		return fmt.Errorf("channel_id not configured properly")
	}

	// Set embed color
	color := 16761867 // default: orange
	if colorVal, ok := e.Config["color"].(float64); ok {
		color = int(colorVal)
	}

	// Create fields for the embed
	fields := []*discordgo.MessageEmbedField{
		{
			Name:   "Player Name",
			Value:  playerName,
			Inline: false,
		},
		{
			Name:   "Steam ID",
			Value:  fmt.Sprintf("[%s](https://steamcommunity.com/profiles/%s)", steamID, steamID),
			Inline: true,
		},
		{
			Name:   "CBL Profile",
			Value:  fmt.Sprintf("[View on CBL](https://communitybanlist.com/search/%s)", steamID),
			Inline: true,
		},
		{
			Name:   "Reputation Points",
			Value:  fmt.Sprintf("%d", cblUser.ReputationPoints),
			Inline: true,
		},
		{
			Name:   "Risk Rating",
			Value:  fmt.Sprintf("%d/10", cblUser.RiskRating),
			Inline: true,
		},
		{
			Name:   "Reputation Rank",
			Value:  fmt.Sprintf("#%d", cblUser.ReputationRank),
			Inline: true,
		},
		{
			Name:   "Active Bans",
			Value:  fmt.Sprintf("%d", len(cblUser.ActiveBans.Edges)),
			Inline: true,
		},
		{
			Name:   "Expired Bans",
			Value:  fmt.Sprintf("%d", len(cblUser.ExpiredBans.Edges)),
			Inline: true,
		},
	}

	// Create embed
	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("%s is a potentially harmful player on %s", playerName, e.Deps.Server.Name),
		Author: &discordgo.MessageEmbedAuthor{
			Name:    "Community Ban List",
			URL:     "https://communitybanlist.com/",
			IconURL: "https://communitybanlist.com/static/media/cbl-logo.caf6584e.png",
		},
		Description: fmt.Sprintf(
			"[%s](https://communitybanlist.com/search/%s) has %d reputation points on the Community Ban List and is therefore a potentially harmful player.",
			playerName, steamID, cblUser.ReputationPoints,
		),
		Color:     color,
		Fields:    fields,
		Timestamp: time.Now().Format(time.RFC3339),
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: cblUser.AvatarFull,
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Powered by Squad Aegis and the Community Ban List",
		},
	}

	// Send message to Discord
	session := e.discord.GetSession()
	_, err = session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{embed},
	})

	return err
}

// fetchCBLData fetches player data from the Community Ban List API
func (e *DiscordCBLInfoExtension) fetchCBLData(steamID string) (*CBLSteamUser, error) {
	query := `
		query Search($id: String!) {
			steamUser(id: $id) {
				id
				name
				avatarFull
				reputationPoints
				reputationPointsMonthChange
				riskRating
				reputationRank
				lastRefreshedInfo
				lastRefreshedReputationPoints
				lastRefreshedReputationRank
				activeBans: bans(orderBy: "created", orderDirection: DESC, expired: false) {
					edges {
						cursor
						node {
							id
						}
					}
				}
				expiredBans: bans(orderBy: "created", orderDirection: DESC, expired: true) {
					edges {
						cursor
						node {
							id
						}
					}
				}
			}
		}
	`

	// Create the request body
	requestBody := map[string]interface{}{
		"query":     query,
		"variables": map[string]string{"id": steamID},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", "https://communitybanlist.com/graphql", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse the response
	var graphQLResp GraphQLResponse
	if err := json.Unmarshal(body, &graphQLResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// If no user data returned, return nil
	if graphQLResp.Data.SteamUser.ID == "" {
		return nil, nil
	}

	return &graphQLResp.Data.SteamUser, nil
}
