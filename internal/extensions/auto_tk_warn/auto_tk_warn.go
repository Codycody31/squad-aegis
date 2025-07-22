package auto_tk_warn

import (
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
)

// TeamkillEvent represents a teamkill event from the logwatcher
type TeamkillEvent struct {
	AttackerEosID string `json:"attackerEos"`
	AttackerName  string `json:"attackerName"`
	VictimEosID   string `json:"victimEos"`
	VictimName    string `json:"victimName"`
	Weapon        string `json:"weapon"`
	Time          string `json:"time"`
}

// handleTeamkill handles teamkill events from the logwatcher
func (e *AutoTKWarnExtension) handleTeamkill(data interface{}) error {
	// The data is a map with Event and Data fields
	eventMap, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid event data format")
	}

	// Extract the event data JSON string
	eventDataStr, ok := eventMap["Data"].(string)
	if !ok {
		return fmt.Errorf("event data is not a string")
	}

	// Parse the JSON data into a map
	var eventData map[string]interface{}
	if err := json.Unmarshal([]byte(eventDataStr), &eventData); err != nil {
		return fmt.Errorf("failed to parse event data: %w", err)
	}

	// Extract attacker and victim information
	attackerEosID, _ := eventData["attackerEos"].(string)
	attackerName, _ := eventData["attackerName"].(string)
	victimEosID, _ := eventData["victimEos"].(string)
	victimName, _ := eventData["victimName"].(string)
	weapon, _ := eventData["weapon"].(string)

	// If we're missing essential information, log and return
	if attackerEosID == "" || victimEosID == "" {
		log.Warn().
			Str("extension", "auto_tk_warn").
			Msg("Teamkill event missing attacker or victim ID")
		return nil
	}

	// Get configuration
	attackerMessage := plug_config_schema.GetStringValue(e.Config, "attacker_message")
	victimMessage := plug_config_schema.GetStringValue(e.Config, "victim_message")

	// Log the teamkill
	log.Info().
		Str("extension", "auto_tk_warn").
		Str("attacker", attackerName).
		Str("attackerEosID", attackerEosID).
		Str("victim", victimName).
		Str("victimEosID", victimEosID).
		Str("weapon", weapon).
		Msg("Teamkill detected")

	// Create RCON client
	r := squadRcon.NewSquadRcon(e.Deps.RconManager, e.Deps.Server.Id)

	// Send warning to attacker if configured
	if attackerMessage != "" {
		log.Debug().
			Str("extension", "auto_tk_warn").
			Str("player", attackerName).
			Str("message", attackerMessage).
			Msg("Sending warning to teamkiller")

		// Execute the warn command
		_, err := r.ExecuteRaw(fmt.Sprintf("AdminWarn %s %s", attackerEosID, attackerMessage))
		if err != nil {
			log.Error().
				Str("extension", "auto_tk_warn").
				Str("player", attackerName).
				Err(err).
				Msg("Failed to warn teamkiller")
		}
	}

	// Send warning to victim if configured
	if victimMessage != "" {
		log.Debug().
			Str("extension", "auto_tk_warn").
			Str("player", victimName).
			Str("message", victimMessage).
			Msg("Sending message to teamkill victim")

		// Execute the warn command
		_, err := r.ExecuteRaw(fmt.Sprintf("AdminWarn %s %s", victimEosID, victimMessage))
		if err != nil {
			log.Error().
				Str("extension", "auto_tk_warn").
				Str("player", victimName).
				Err(err).
				Msg("Failed to warn teamkill victim")
		}
	}

	return nil
}
