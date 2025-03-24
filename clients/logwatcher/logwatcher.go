package logwatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/proto/logwatcher"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ParsedEvent represents an event with its original data and parsed event data.
type ParsedEvent struct {
	Original *logwatcher.EventEntry // The original event from the server
	Data     any                    // Parsed event data (if available)
}

// AdminBroadcastEvent is the dedicated struct for ADMIN_BROADCAST events.
type AdminBroadcastEvent struct {
	Time    string `json:"time"`
	ChainID string `json:"chainID"`
	Message string `json:"message"`
	From    string `json:"from"`
}

// DeployableDamagedEvent is the dedicated struct for DEPLOYABLE_DAMAGED events.
type DeployableDamagedEvent struct {
	Time            string `json:"time"`
	ChainID         string `json:"chainID"`
	Deployable      string `json:"deployable"`
	Damage          string `json:"damage"`
	Weapon          string `json:"weapon"`
	PlayerSuffix    string `json:"playerSuffix"`
	DamageType      string `json:"damageType"`
	HealthRemaining string `json:"healthRemaining"`
}

// TickRateEvent is the dedicated struct for TICK_RATE events.
type TickRateEvent struct {
	Time     string `json:"time"`
	ChainID  string `json:"chainID"`
	TickRate string `json:"tickRate"`
}

// PlayerConnectedEvent is the dedicated struct for PLAYER_CONNECTED events.
type PlayerConnectedEvent struct {
	Time             string `json:"time"`
	ChainID          string `json:"chainID"`
	PlayerController string `json:"playercontroller"`
	IP               string `json:"ip"`
	Steam            string `json:"steam,omitempty"`
	EOS              string `json:"eos,omitempty"`
}

// NewGameEvent is the dedicated struct for NEW_GAME events.
type NewGameEvent struct {
	Time           string `json:"time"`
	ChainID        string `json:"chainID"`
	Team           string `json:"team,omitempty"`
	Subfaction     string `json:"subfaction,omitempty"`
	Faction        string `json:"faction,omitempty"`
	Action         string `json:"action,omitempty"`
	Tickets        string `json:"tickets,omitempty"`
	Layer          string `json:"layer,omitempty"`
	Level          string `json:"level,omitempty"`
	DLC            string `json:"dlc,omitempty"`
	MapClassname   string `json:"mapClassname,omitempty"`
	LayerClassname string `json:"layerClassname,omitempty"`
}

// PlayerDisconnectedEvent is the dedicated struct for PLAYER_DISCONNECTED events.
type PlayerDisconnectedEvent struct {
	Time             string `json:"time"`
	ChainID          string `json:"chainID"`
	IP               string `json:"ip"`
	PlayerController string `json:"playerController"`
	EOSID            string `json:"eosID"`
}

// PlayerDamagedEvent is the dedicated struct for PLAYER_DAMAGED events.
type PlayerDamagedEvent struct {
	Time               string `json:"time"`
	ChainID            string `json:"chainID"`
	VictimName         string `json:"victimName"`
	Damage             string `json:"damage"`
	AttackerName       string `json:"attackerName"`
	AttackerController string `json:"attackerController"`
	Weapon             string `json:"weapon"`
	AttackerEOS        string `json:"attackerEos,omitempty"`
	AttackerSteam      string `json:"attackerSteam,omitempty"`
}

// PlayerDiedEvent is the dedicated struct for PLAYER_DIED events.
type PlayerDiedEvent struct {
	Time                     string `json:"time"`
	WoundTime                string `json:"woundTime"`
	ChainID                  string `json:"chainID"`
	VictimName               string `json:"victimName"`
	Damage                   string `json:"damage"`
	AttackerPlayerController string `json:"attackerPlayerController"`
	Weapon                   string `json:"weapon"`
	AttackerEOS              string `json:"attackerEos,omitempty"`
	AttackerSteam            string `json:"attackerSteam,omitempty"`
	Teamkill                 bool   `json:"teamkill,omitempty"`
}

// JoinSucceededEvent is the dedicated struct for JOIN_SUCCEEDED events.
type JoinSucceededEvent struct {
	Time         string `json:"time"`
	ChainID      string `json:"chainID"`
	PlayerSuffix string `json:"playerSuffix"`
	IP           string `json:"ip,omitempty"`
	Steam        string `json:"steam,omitempty"`
	EOS          string `json:"eos,omitempty"`
}

// PlayerPossessEvent is the dedicated struct for PLAYER_POSSESS events.
type PlayerPossessEvent struct {
	Time             string `json:"time"`
	ChainID          string `json:"chainID"`
	PlayerSuffix     string `json:"playerSuffix"`
	PossessClassname string `json:"possessClassname"`
	PlayerEOS        string `json:"playerEos,omitempty"`
	PlayerSteam      string `json:"playerSteam,omitempty"`
}

// PlayerRevivedEvent is the dedicated struct for PLAYER_REVIVED events.
type PlayerRevivedEvent struct {
	Time         string `json:"time"`
	ChainID      string `json:"chainID"`
	ReviverName  string `json:"reviverName"`
	VictimName   string `json:"victimName"`
	ReviverEOS   string `json:"reviverEos,omitempty"`
	ReviverSteam string `json:"reviverSteam,omitempty"`
	VictimEOS    string `json:"victimEos,omitempty"`
	VictimSteam  string `json:"victimSteam,omitempty"`
}

// PlayerWoundedEvent is the dedicated struct for PLAYER_WOUNDED events.
type PlayerWoundedEvent struct {
	Time                     string `json:"time"`
	ChainID                  string `json:"chainID"`
	VictimName               string `json:"victimName"`
	Damage                   string `json:"damage"`
	AttackerPlayerController string `json:"attackerPlayerController"`
	Weapon                   string `json:"weapon"`
	AttackerEOS              string `json:"attackerEos,omitempty"`
	AttackerSteam            string `json:"attackerSteam,omitempty"`
	Teamkill                 bool   `json:"teamkill,omitempty"`
}

// RoundEndedEvent is the dedicated struct for ROUND_ENDED events.
type RoundEndedEvent struct {
	Time       string                 `json:"time"`
	ChainID    string                 `json:"chainID,omitempty"`
	Winner     string                 `json:"winner,omitempty"`
	Layer      string                 `json:"layer,omitempty"`
	WinnerData map[string]interface{} `json:"winner,omitempty"`
	LoserData  map[string]interface{} `json:"loser,omitempty"`
}

// PlayerSquadChangeEvent is the dedicated struct for PLAYER_SQUAD_CHANGE events.
type PlayerSquadChangeEvent struct {
	Time        string `json:"time"`
	ChainID     string `json:"chainID"`
	Name        string `json:"name"`
	TeamID      string `json:"teamID"`
	SquadID     string `json:"squadID"`
	OldTeamID   string `json:"oldTeamID,omitempty"`
	OldSquadID  string `json:"oldSquadID,omitempty"`
	PlayerEOS   string `json:"playerEos,omitempty"`
	PlayerSteam string `json:"playerSteam,omitempty"`
}

// PlayerTeamChangeEvent is the dedicated struct for PLAYER_TEAM_CHANGE events.
type PlayerTeamChangeEvent struct {
	Time        string `json:"time"`
	ChainID     string `json:"chainID"`
	Name        string `json:"name"`
	NewTeamID   string `json:"newTeamID"`
	OldTeamID   string `json:"oldTeamID,omitempty"`
	PlayerEOS   string `json:"playerEos,omitempty"`
	PlayerSteam string `json:"playerSteam,omitempty"`
}

// TeamkillEvent shares the same structure as PlayerWoundedEvent or PlayerDiedEvent with teamkill=true.
// Using PlayerDiedEvent as it has more fields.
type TeamkillEvent PlayerDiedEvent

// EventStreamer handles event streaming from the server.
type EventStreamer struct {
	addr    string
	token   string
	eventCh chan *ParsedEvent
	conn    *grpc.ClientConn
	client  logwatcher.LogWatcherClient
}

// NewEventStreamer initializes the event streamer.
func NewEventStreamer(addr, token string) *EventStreamer {
	return &EventStreamer{
		addr:    addr,
		token:   token,
		eventCh: make(chan *ParsedEvent, 50), // Buffered channel for parsed events.
	}
}

// Start connects to the server and begins streaming events.
func (es *EventStreamer) Start(ctx context.Context) error {
	conn, err := grpc.DialContext(ctx, es.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	es.conn = conn
	es.client = logwatcher.NewLogWatcherClient(conn)

	go es.streamEvents(ctx)
	return nil
}

// streamEvents streams events from the server, parses them into dedicated structs, and emits them to a channel.
func (es *EventStreamer) streamEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			stream, err := es.client.StreamEvents(ctx, &logwatcher.AuthRequest{Token: es.token})
			if err != nil {
				log.Error().Err(err).Msg("Failed to open event stream, retrying in 2 seconds...")
				time.Sleep(2 * time.Second)
				continue
			}

			// Continuously receive streamed events.
			for {
				event, err := stream.Recv()
				if err != nil {
					log.Error().Err(err).Msg("Error receiving event from stream, reconnecting...")
					time.Sleep(2 * time.Second)
					break // Reconnect on error.
				}

				var parsed any
				switch event.Event {
				case "ADMIN_BROADCAST":
					var adminEvent AdminBroadcastEvent
					if err := json.Unmarshal([]byte(event.Data), &adminEvent); err != nil {
						log.Error().Err(err).Msg("Failed to parse ADMIN_BROADCAST event data")
						parsed = nil
					} else {
						parsed = adminEvent
					}
				case "TICK_RATE":
					var tickRateEvent TickRateEvent
					if err := json.Unmarshal([]byte(event.Data), &tickRateEvent); err != nil {
						log.Error().Err(err).Msg("Failed to parse TICK_RATE event data")
						parsed = nil
					} else {
						parsed = tickRateEvent
					}
				case "DEPLOYABLE_DAMAGED":
					var deployableEvent DeployableDamagedEvent
					if err := json.Unmarshal([]byte(event.Data), &deployableEvent); err != nil {
						log.Error().Err(err).Msg("Failed to parse DEPLOYABLE_DAMAGED event data")
						parsed = nil
					} else {
						parsed = deployableEvent
					}
				case "PLAYER_CONNECTED":
					var playerEvent PlayerConnectedEvent
					if err := json.Unmarshal([]byte(event.Data), &playerEvent); err != nil {
						log.Error().Err(err).Msg("Failed to parse PLAYER_CONNECTED event data")
						parsed = nil
					} else {
						parsed = playerEvent
					}
				case "NEW_GAME":
					var gameEvent NewGameEvent
					if err := json.Unmarshal([]byte(event.Data), &gameEvent); err != nil {
						log.Error().Err(err).Msg("Failed to parse NEW_GAME event data")
						parsed = nil
					} else {
						parsed = gameEvent
					}
				case "PLAYER_DISCONNECTED":
					var disconnectEvent PlayerDisconnectedEvent
					if err := json.Unmarshal([]byte(event.Data), &disconnectEvent); err != nil {
						log.Error().Err(err).Msg("Failed to parse PLAYER_DISCONNECTED event data")
						parsed = nil
					} else {
						parsed = disconnectEvent
					}
				case "PLAYER_DAMAGED":
					var damageEvent PlayerDamagedEvent
					if err := json.Unmarshal([]byte(event.Data), &damageEvent); err != nil {
						log.Error().Err(err).Msg("Failed to parse PLAYER_DAMAGED event data")
						parsed = nil
					} else {
						parsed = damageEvent
					}
				case "PLAYER_DIED":
					var deathEvent PlayerDiedEvent
					if err := json.Unmarshal([]byte(event.Data), &deathEvent); err != nil {
						log.Error().Err(err).Msg("Failed to parse PLAYER_DIED event data")
						parsed = nil
					} else {
						parsed = deathEvent
					}
				case "JOIN_SUCCEEDED":
					var joinEvent JoinSucceededEvent
					if err := json.Unmarshal([]byte(event.Data), &joinEvent); err != nil {
						log.Error().Err(err).Msg("Failed to parse JOIN_SUCCEEDED event data")
						parsed = nil
					} else {
						parsed = joinEvent
					}
				case "PLAYER_POSSESS":
					var possessEvent PlayerPossessEvent
					if err := json.Unmarshal([]byte(event.Data), &possessEvent); err != nil {
						log.Error().Err(err).Msg("Failed to parse PLAYER_POSSESS event data")
						parsed = nil
					} else {
						parsed = possessEvent
					}
				case "PLAYER_REVIVED":
					var reviveEvent PlayerRevivedEvent
					if err := json.Unmarshal([]byte(event.Data), &reviveEvent); err != nil {
						log.Error().Err(err).Msg("Failed to parse PLAYER_REVIVED event data")
						parsed = nil
					} else {
						parsed = reviveEvent
					}
				case "PLAYER_WOUNDED":
					var woundEvent PlayerWoundedEvent
					if err := json.Unmarshal([]byte(event.Data), &woundEvent); err != nil {
						log.Error().Err(err).Msg("Failed to parse PLAYER_WOUNDED event data")
						parsed = nil
					} else {
						parsed = woundEvent
					}
				case "ROUND_ENDED":
					var roundEvent RoundEndedEvent
					if err := json.Unmarshal([]byte(event.Data), &roundEvent); err != nil {
						log.Error().Err(err).Msg("Failed to parse ROUND_ENDED event data")
						parsed = nil
					} else {
						parsed = roundEvent
					}
				case "PLAYER_SQUAD_CHANGE":
					var squadEvent PlayerSquadChangeEvent
					if err := json.Unmarshal([]byte(event.Data), &squadEvent); err != nil {
						log.Error().Err(err).Msg("Failed to parse PLAYER_SQUAD_CHANGE event data")
						parsed = nil
					} else {
						parsed = squadEvent
					}
				case "PLAYER_TEAM_CHANGE":
					var teamEvent PlayerTeamChangeEvent
					if err := json.Unmarshal([]byte(event.Data), &teamEvent); err != nil {
						log.Error().Err(err).Msg("Failed to parse PLAYER_TEAM_CHANGE event data")
						parsed = nil
					} else {
						parsed = teamEvent
					}
				case "TEAMKILL":
					var teamkillEvent TeamkillEvent
					if err := json.Unmarshal([]byte(event.Data), &teamkillEvent); err != nil {
						log.Error().Err(err).Msg("Failed to parse TEAMKILL event data")
						parsed = nil
					} else {
						parsed = teamkillEvent
					}
				default:
					log.Warn().Msgf("Unknown event type received: %s", event.Event)
					parsed = event.Data // Fallback to raw data.
				}

				parsedEvent := &ParsedEvent{
					Original: event,
					Data:     parsed,
				}

				es.eventCh <- parsedEvent
			}
		}
	}
}

// GetEvents returns the channel for consuming parsed events.
func (es *EventStreamer) GetEvents() <-chan *ParsedEvent {
	return es.eventCh
}

// Stop closes the event channel and connection.
func (es *EventStreamer) Stop() {
	log.Info().Msg("Stopping EventStreamer")
	close(es.eventCh)
	if es.conn != nil {
		es.conn.Close()
	}
}
