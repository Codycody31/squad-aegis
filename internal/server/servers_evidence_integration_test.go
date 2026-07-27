package server

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/models"
)

func TestGameplayEvidenceUsesStableHistoricalKey(t *testing.T) {
	client := serverTestClickHouseClient(t)
	ctx := context.Background()
	serverID := uuid.New()
	eventTime := time.Now().UTC().Truncate(time.Millisecond)

	if err := client.Exec(ctx, `
		INSERT INTO squad_aegis.server_player_connected_events (
			event_time, server_id, chain_id, player_controller, ip,
			steam, eos, epic, player_suffix
		) VALUES (?, ?, 'stable-evidence-chain', 'StableController', '192.0.2.10',
		          '76561198000000001', '', '', 'Stable Player')
	`, eventTime, serverID); err != nil {
		t.Fatalf("insert historical-style connected event: %v", err)
	}

	var firstKey, secondKey uuid.UUID
	for _, destination := range []*uuid.UUID{&firstKey, &secondKey} {
		if err := client.QueryRow(ctx, `
			SELECT evidence_key
			FROM squad_aegis.server_player_connected_events
			WHERE server_id = ? AND chain_id = 'stable-evidence-chain'
			LIMIT 1
		`, serverID).Scan(destination); err != nil {
			t.Fatalf("read deterministic evidence key: %v", err)
		}
	}
	if firstKey == uuid.Nil || firstKey != secondKey {
		t.Fatalf("evidence key was not stable: first=%s second=%s", firstKey, secondKey)
	}

	recordID := firstKey.String()
	evidence := []models.BanEvidence{{
		EvidenceType: "player_connected",
		RecordID:     &recordID,
		ServerID:     serverID,
		Metadata:     map[string]interface{}{"event_type": "PLAYER_CONNECTED"},
	}}
	api := &Server{Dependencies: &Dependencies{Clickhouse: client}}
	hydrated, err := api.hydrateBanEvidenceBatch(ctx, evidence)
	if err != nil {
		t.Fatalf("hydrate evidence by deterministic key: %v", err)
	}
	if len(hydrated) != 1 || hydrated[0].Metadata["player_controller"] != "StableController" {
		t.Fatalf("unexpected hydrated evidence: %#v", hydrated)
	}
}
