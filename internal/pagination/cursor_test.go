package pagination

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type testCursorValues struct {
	Timestamp time.Time `json:"timestamp"`
	ID        string    `json:"id"`
}

func TestCursorRoundTrip(t *testing.T) {
	t.Parallel()
	filters := MustFilterHash(struct {
		ServerID string `json:"server_id"`
		Search   string `json:"search"`
	}{ServerID: "server-1", Search: "kick"})
	want := testCursorValues{Timestamp: time.Date(2026, 7, 27, 1, 2, 3, 4, time.UTC), ID: "row-1"}

	cursor, err := Encode("audit.server", filters, want)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cursor, "=") {
		t.Fatalf("cursor must use unpadded base64url: %q", cursor)
	}

	var got testCursorValues
	if err := Decode(cursor, "audit.server", filters, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Timestamp.Equal(want.Timestamp) || got.ID != want.ID {
		t.Fatalf("decoded values = %#v, want %#v", got, want)
	}
}

func TestCursorValidation(t *testing.T) {
	t.Parallel()
	filterHash := MustFilterHash(map[string]string{"server_id": "server-1"})
	cursor, err := Encode("audit.server", filterHash, testCursorValues{ID: "row-1"})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		cursor string
		kind   string
		hash   string
		want   error
	}{
		{name: "invalid encoding", cursor: "not+base64", kind: "audit.server", hash: filterHash, want: ErrInvalidCursor},
		{name: "wrong endpoint", cursor: cursor, kind: "audit.global", hash: filterHash, want: ErrCursorKindMismatch},
		{name: "changed filters", cursor: cursor, kind: "audit.server", hash: MustFilterHash(map[string]string{"server_id": "server-2"}), want: ErrCursorFilterChanged},
		{name: "oversized", cursor: strings.Repeat("a", MaxEncodedCursorLen+1), kind: "audit.server", hash: filterHash, want: ErrCursorTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got testCursorValues
			err := Decode(tt.cursor, tt.kind, tt.hash, &got)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Decode() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestCursorRejectsUnknownEnvelopeFields(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(map[string]any{
		"v":           CursorVersion,
		"kind":        "audit.server",
		"filter_hash": "hash",
		"values":      map[string]string{"id": "row-1"},
		"extra":       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var values testCursorValues
	err = Decode(base64.RawURLEncoding.EncodeToString(raw), "audit.server", "hash", &values)
	if !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("Decode() error = %v, want invalid cursor", err)
	}
}

func TestCursorRejectsTrailingJSON(t *testing.T) {
	t.Parallel()
	filterHash := MustFilterHash(map[string]string{"server_id": "server-1"})
	cursor, err := Encode("audit.server", filterHash, testCursorValues{ID: "row-1"})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		t.Fatal(err)
	}
	decoded = append(decoded, []byte(` {"second":true}`)...)

	var values testCursorValues
	err = Decode(base64.RawURLEncoding.EncodeToString(decoded), "audit.server", filterHash, &values)
	if !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("Decode() error = %v, want invalid cursor", err)
	}
}

func TestTrimPage(t *testing.T) {
	t.Parallel()
	rows, more := TrimPage([]int{1, 2, 3}, 2)
	if !more || len(rows) != 2 || rows[0] != 1 || rows[1] != 2 {
		t.Fatalf("TrimPage() = %v, %v", rows, more)
	}
	rows, more = TrimPage([]int{1, 2}, 2)
	if more || len(rows) != 2 {
		t.Fatalf("TrimPage exact = %v, %v", rows, more)
	}
	rows, more = TrimPage([]int{}, 2)
	if more || len(rows) != 0 {
		t.Fatalf("TrimPage empty final page = %v, %v", rows, more)
	}
}
