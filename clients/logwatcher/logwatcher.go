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

type TickRateEvent struct {
	Time     string `json:"time"`
	ChainID  string `json:"chainID"`
	TickRate string `json:"tickRate"`
}

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
