package utils

import (
	"testing"
	"time"
)

func TestParseBanDuration(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantNil    bool
		wantErr    bool
		checkDelta time.Duration // approximate expected duration from now (0 = skip delta check)
	}{
		{name: "empty string is permanent", input: "", wantNil: true},
		{name: "zero is permanent", input: "0", wantNil: true},
		{name: "permanent keyword", input: "permanent", wantNil: true},
		{name: "PERMANENT keyword", input: "PERMANENT", wantNil: true},
		{name: "bare number days", input: "7", checkDelta: 7 * 24 * time.Hour},
		{name: "days suffix", input: "7d", checkDelta: 7 * 24 * time.Hour},
		{name: "hours suffix", input: "2h", checkDelta: 2 * time.Hour},
		{name: "minutes suffix", input: "30m", checkDelta: 30 * time.Minute},
		{name: "negative number is permanent", input: "-1", wantNil: true},
		{name: "zero days suffix", input: "0d", wantNil: true},
		{name: "invalid format", input: "abc", wantErr: true},
		{name: "invalid unit", input: "5x", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			result, err := ParseBanDuration(tt.input)
			after := time.Now()

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantNil {
				if result != nil {
					t.Fatalf("expected nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Fatalf("expected non-nil result")
			}

			if tt.checkDelta > 0 {
				expectedEarliest := before.Add(tt.checkDelta)
				expectedLatest := after.Add(tt.checkDelta)

				if result.Before(expectedEarliest.Add(-time.Second)) || result.After(expectedLatest.Add(time.Second)) {
					t.Fatalf("result %v not in expected range [%v, %v]", result, expectedEarliest, expectedLatest)
				}
			}
		})
	}

	// Test months separately since AddDate uses calendar months
	t.Run("months suffix", func(t *testing.T) {
		before := time.Now()
		result, err := ParseBanDuration("1M")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatalf("expected non-nil result")
		}
		// Should be roughly 28-31 days in the future
		delta := result.Sub(before)
		if delta < 27*24*time.Hour || delta > 32*24*time.Hour {
			t.Fatalf("1M should be ~28-31 days, got %v", delta)
		}
	})
}
