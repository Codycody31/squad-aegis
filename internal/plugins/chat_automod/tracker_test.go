package chat_automod

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type fakeDatabaseAPI struct {
	data map[string]string
}

func (f *fakeDatabaseAPI) GetPluginData(key string) (string, error) {
	value, ok := f.data[key]
	if !ok {
		return "", errors.New("not found")
	}
	return value, nil
}

func (f *fakeDatabaseAPI) SetPluginData(key string, value string) error {
	f.data[key] = value
	return nil
}

func (f *fakeDatabaseAPI) DeletePluginData(key string) error {
	delete(f.data, key)
	return nil
}

func TestViolationTrackerSupportsSteamOnlyEOSOnlyAndDualIDs(t *testing.T) {
	const steamID = "76561198000000042"
	const eosID = "abcdef0123456789abcdef0123456789"

	t.Run("steam only", func(t *testing.T) {
		db := &fakeDatabaseAPI{data: make(map[string]string)}
		tracker := NewViolationTracker(db, 30)

		err := tracker.RecordViolation(steamID, steamID, "", "evt-steam", CategoryCustom, "warn", "steam only")
		if err != nil {
			t.Fatalf("RecordViolation() error = %v", err)
		}

		count, err := tracker.GetActiveViolationCount(steamID, steamID, "")
		if err != nil {
			t.Fatalf("GetActiveViolationCount() error = %v", err)
		}
		if count != 1 {
			t.Fatalf("GetActiveViolationCount() = %d, want 1", count)
		}

		if _, ok := db.data["violations:"+steamID]; !ok {
			t.Fatalf("expected steam storage key to be written")
		}
	})

	t.Run("eos only", func(t *testing.T) {
		db := &fakeDatabaseAPI{data: make(map[string]string)}
		tracker := NewViolationTracker(db, 30)

		err := tracker.RecordViolation(eosID, "", eosID, "evt-eos", CategoryCustom, "warn", "eos only")
		if err != nil {
			t.Fatalf("RecordViolation() error = %v", err)
		}

		count, err := tracker.GetActiveViolationCount(eosID, "", eosID)
		if err != nil {
			t.Fatalf("GetActiveViolationCount() error = %v", err)
		}
		if count != 1 {
			t.Fatalf("GetActiveViolationCount() = %d, want 1", count)
		}

		if _, ok := db.data["violations:"+eosID]; !ok {
			t.Fatalf("expected eos storage key to be written")
		}
	})

	t.Run("dual id lookup finds legacy eos record and backfills aliases", func(t *testing.T) {
		db := &fakeDatabaseAPI{data: make(map[string]string)}
		tracker := NewViolationTracker(db, 30)

		legacyRecord, err := json.Marshal(&ViolationRecord{
			SteamID: eosID,
			Violations: []Violation{
				{
					Timestamp:   time.Now(),
					EventID:     "evt-legacy",
					Category:    CategoryCustom,
					ActionTaken: "warn",
				},
			},
		})
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		db.data["violations:"+eosID] = string(legacyRecord)

		count, err := tracker.GetActiveViolationCount(steamID, steamID, eosID)
		if err != nil {
			t.Fatalf("GetActiveViolationCount() error = %v", err)
		}
		if count != 1 {
			t.Fatalf("GetActiveViolationCount() = %d, want 1", count)
		}

		record, err := tracker.GetViolationRecord(steamID, steamID, eosID)
		if err != nil {
			t.Fatalf("GetViolationRecord() error = %v", err)
		}
		if record.PlayerID != steamID {
			t.Fatalf("record.PlayerID = %q, want %q", record.PlayerID, steamID)
		}
		if record.SteamID != steamID {
			t.Fatalf("record.SteamID = %q, want %q", record.SteamID, steamID)
		}
		if record.EOSID != eosID {
			t.Fatalf("record.EOSID = %q, want %q", record.EOSID, eosID)
		}

		err = tracker.RecordViolation(steamID, steamID, eosID, "evt-dual", CategoryCustom, "warn", "dual ids")
		if err != nil {
			t.Fatalf("RecordViolation() error = %v", err)
		}

		if _, ok := db.data["violations:"+steamID]; !ok {
			t.Fatalf("expected steam storage key to be written after dual-id update")
		}
		if _, ok := db.data["violations:"+eosID]; !ok {
			t.Fatalf("expected eos storage key to remain written after dual-id update")
		}
	})
}
