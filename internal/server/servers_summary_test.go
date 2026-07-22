package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestServerSummaryEntryJSONShape(t *testing.T) {
	t.Parallel()

	id := uuid.MustParse("11111111-2222-3333-4444-555555555555")

	t.Run("offline server omits live fields", func(t *testing.T) {
		t.Parallel()
		entry := ServerSummaryEntry{
			Id:        id,
			Name:      "Test",
			IpAddress: "203.0.113.10",
			GamePort:  7787,
			RconPort:  21114,
			BanCount:  3,
		}

		raw, err := json.Marshal(entry)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		s := string(raw)

		for _, absent := range []string{"players", "current_layer"} {
			if strings.Contains(s, absent) {
				t.Errorf("expected %q omitted for offline server, got %s", absent, s)
			}
		}
		for _, present := range []string{
			`"game_online":false`, `"rcon_online":false`, `"ban_count":3`,
		} {
			if !strings.Contains(s, present) {
				t.Errorf("expected %s in %s", present, s)
			}
		}
	})

	t.Run("online server includes live fields", func(t *testing.T) {
		t.Parallel()
		entry := ServerSummaryEntry{
			Id:           id,
			Name:         "Test",
			IpAddress:    "203.0.113.10",
			GamePort:     7787,
			RconPort:     21114,
			GameOnline:   true,
			RconOnline:   true,
			CurrentLayer: "Narva_AAS_v1",
			Players:      &ServerSummaryPlayers{Current: 12, Max: 100, Queue: 2},
		}

		raw, err := json.Marshal(entry)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		s := string(raw)

		for _, present := range []string{
			`"current":12`, `"max":100`, `"queue":2`, `"current_layer":"Narva_AAS_v1"`,
		} {
			if !strings.Contains(s, present) {
				t.Errorf("expected %s in %s", present, s)
			}
		}
	})
}
