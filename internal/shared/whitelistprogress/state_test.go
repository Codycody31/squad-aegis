package whitelistprogress

import (
	"errors"
	"testing"
	"time"
)

func TestParseStateRoundTrip(t *testing.T) {
	raw, err := MarshalPlayers(map[string]*PlayerRecord{
		"76561198000000021": {
			PlayerID:         "76561198000000021",
			QualifiedSeconds: 7200,
			LifetimeSeconds:  14400,
			LastEarnedAt:     time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
			LastSeenAt:       time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}

	state, err := ParseState(string(raw))
	if err != nil {
		t.Fatalf("parse state: %v", err)
	}

	if state.Version != CurrentVersion {
		t.Fatalf("version = %d, want %d", state.Version, CurrentVersion)
	}

	record := state.Players["76561198000000021"]
	if record == nil {
		t.Fatalf("expected parsed record")
	}

	if got, want := record.QualifiedSeconds, int64(7200); got != want {
		t.Fatalf("qualified seconds = %d, want %d", got, want)
	}
}

func TestParseStateRejectsLegacyPayload(t *testing.T) {
	legacy := `{"76561198000000022":{"steam_id":"76561198000000022","progress":50}}`

	_, err := ParseState(legacy)
	if !errors.Is(err, ErrUnknownFormat) {
		t.Fatalf("expected ErrUnknownFormat, got %v", err)
	}
}

func TestParseStateSupportsLegacyPlayerRecordIdentifier(t *testing.T) {
	state, err := ParseState(`{"version":1,"players":{"ABCDEF0123456789ABCDEF0123456789":{"steam_id":"ABCDEF0123456789ABCDEF0123456789","qualified_seconds":60,"lifetime_seconds":120}}}`)
	if err != nil {
		t.Fatalf("parse state: %v", err)
	}

	record := state.Players["abcdef0123456789abcdef0123456789"]
	if record == nil {
		t.Fatalf("expected normalized EOS record")
	}

	if got, want := record.PlayerID, "abcdef0123456789abcdef0123456789"; got != want {
		t.Fatalf("player ID = %q, want %q", got, want)
	}
}

func TestLegacyPercentToSeconds(t *testing.T) {
	if got, want := LegacyPercentToSeconds(50, 6), int64(3*time.Hour/time.Second); got != want {
		t.Fatalf("legacy conversion = %d, want %d", got, want)
	}
}
