package ban_enforcer

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/rcon_manager"
)

// BanEnforcer watches for player connections and kicks banned players.
type BanEnforcer struct {
	db           *sql.DB
	eventManager *event_manager.EventManager
	rconManager  *rcon_manager.RconManager
	subscriber   *event_manager.EventSubscriber
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewBanEnforcer creates a new BanEnforcer instance.
func NewBanEnforcer(ctx context.Context, db *sql.DB, eventManager *event_manager.EventManager, rconManager *rcon_manager.RconManager) *BanEnforcer {
	ctx, cancel := context.WithCancel(ctx)
	return &BanEnforcer{
		db:           db,
		eventManager: eventManager,
		rconManager:  rconManager,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start subscribes to player connection events and begins processing.
func (b *BanEnforcer) Start() {
	log.Info().Msg("Starting ban enforcer")

	filter := event_manager.EventFilter{
		Types: []event_manager.EventType{event_manager.EventTypeLogPlayerConnected},
	}
	b.subscriber = b.eventManager.Subscribe(filter, nil, 500)

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		b.processLoop()
	}()
}

// Stop unsubscribes from events and waits for processing to finish.
func (b *BanEnforcer) Stop() {
	log.Info().Msg("Stopping ban enforcer")

	if b.subscriber != nil {
		b.eventManager.Unsubscribe(b.subscriber.ID)
	}

	b.cancel()
	b.wg.Wait()
}

func (b *BanEnforcer) processLoop() {
	eventChan := b.subscriber.Channel
	for {
		select {
		case <-b.ctx.Done():
			return
		case event, ok := <-eventChan:
			if !ok {
				return
			}
			b.handlePlayerConnected(event)
		}
	}
}

func (b *BanEnforcer) handlePlayerConnected(event event_manager.Event) {
	data, ok := event.Data.(*event_manager.LogPlayerConnectedData)
	if !ok {
		return
	}

	steamID := data.SteamID
	if steamID == "" {
		return
	}

	serverID := event.ServerID

	// Check for an active ban on this server (including subscribed ban lists)
	ban, err := core.GetActiveBanForServer(b.ctx, b.db, serverID, steamID)
	if err != nil {
		// sql.ErrNoRows means no active ban - this is the normal case
		if err == sql.ErrNoRows {
			return
		}
		log.Error().Err(err).Str("steamId", steamID).Str("serverId", serverID.String()).Msg("Failed to check active ban")
		return
	}

	// Player has an active ban - kick them
	reason := ban.Reason
	if reason == "" {
		reason = "You are banned from this server"
	}

	kickCmd := fmt.Sprintf("AdminKick %s %s", steamID, reason)
	_, err = b.rconManager.ExecuteCommand(serverID, kickCmd)
	if err != nil {
		log.Error().Err(err).
			Str("steamId", steamID).
			Str("serverId", serverID.String()).
			Str("banId", ban.ID).
			Msg("Failed to kick banned player")
		return
	}

	log.Info().
		Str("steamId", steamID).
		Str("serverId", serverID.String()).
		Str("banId", ban.ID).
		Bool("permanent", ban.Permanent).
		Str("reason", reason).
		Msg("Kicked banned player on connection (aegis enforcement)")
}
