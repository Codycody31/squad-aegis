package squad_leader_whitelist

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/whitelistprogress"
)

func TestLoadPlayerProgressMigratesLegacyLeadershipState(t *testing.T) {
	legacyState, err := json.Marshal(map[string]*legacyPlayerProgressRecord{
		"76561198000000011": {
			SteamID:         "76561198000000011",
			Progress:        25,
			LastProgressed:  time.Date(2026, 3, 18, 9, 0, 0, 0, time.UTC),
			TotalLeadership: 150,
			LastSeen:        time.Date(2026, 3, 19, 10, 45, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("marshal legacy state: %v", err)
	}

	db := &fakeDatabaseAPI{
		data: map[string]string{
			"player_progress": string(legacyState),
		},
	}

	plugin := &SquadLeaderWhitelistPlugin{
		playerProgress:     make(map[string]*PlayerProgressRecord),
		squadLeaderSession: make(map[string]*SquadLeaderSession),
	}

	err = plugin.Initialize(map[string]interface{}{
		"hours_to_whitelist": 8,
	}, &plugin_manager.PluginAPIs{
		DatabaseAPI: db,
		LogAPI:      fakeLogAPI{},
	})
	if err != nil {
		t.Fatalf("initialize plugin: %v", err)
	}

	record := plugin.playerProgress["76561198000000011"]
	if record == nil {
		t.Fatalf("expected migrated player record")
	}

	if got, want := record.QualifiedSeconds, int64(2*time.Hour/time.Second); got != want {
		t.Fatalf("qualified seconds = %d, want %d", got, want)
	}

	if got, want := record.LifetimeSeconds, int64(12*time.Hour/time.Second); got != want {
		t.Fatalf("lifetime seconds = %d, want %d", got, want)
	}

	migratedRaw, ok := db.data["player_progress"]
	if !ok {
		t.Fatalf("expected migrated state to be persisted")
	}

	state, err := whitelistprogress.ParseState(migratedRaw)
	if err != nil {
		t.Fatalf("parse migrated state: %v", err)
	}

	if state.Version != whitelistprogress.CurrentVersion {
		t.Fatalf("state version = %d, want %d", state.Version, whitelistprogress.CurrentVersion)
	}
}

func TestUpdateConfigAddsManagedRoleWhenThresholdDecreases(t *testing.T) {
	admins := &fakeAdminAPI{}

	plugin := &SquadLeaderWhitelistPlugin{
		playerProgress:     make(map[string]*PlayerProgressRecord),
		squadLeaderSession: make(map[string]*SquadLeaderSession),
	}

	err := plugin.Initialize(map[string]interface{}{
		"hours_to_whitelist":        10,
		"auto_add_temporary_admins": true,
	}, &plugin_manager.PluginAPIs{
		DatabaseAPI: &fakeDatabaseAPI{data: map[string]string{}},
		AdminAPI:    admins,
		RconAPI:     &fakeRconAPI{},
		LogAPI:      fakeLogAPI{},
	})
	if err != nil {
		t.Fatalf("initialize plugin: %v", err)
	}

	plugin.playerProgress["76561198000000012"] = &PlayerProgressRecord{
		PlayerID:         "76561198000000012",
		QualifiedSeconds: whitelistprogress.RequiredSeconds(8),
		LifetimeSeconds:  whitelistprogress.RequiredSeconds(8),
		LastEarnedAt:     time.Now(),
		LastSeenAt:       time.Now(),
	}
	plugin.status = plugin_manager.PluginStatusRunning
	plugin.adminSyncTicker = time.NewTicker(time.Hour)
	defer func() {
		if plugin.adminSyncTicker != nil {
			plugin.adminSyncTicker.Stop()
		}
	}()

	err = plugin.UpdateConfig(map[string]interface{}{
		"hours_to_whitelist":        8,
		"auto_add_temporary_admins": true,
	})
	if err != nil {
		t.Fatalf("update config: %v", err)
	}

	if len(admins.removeRoleCalls) != 0 {
		t.Fatalf("expected no role removals, got %d", len(admins.removeRoleCalls))
	}

	if len(admins.addCalls) != 1 {
		t.Fatalf("expected one admin add call, got %d", len(admins.addCalls))
	}

	call := admins.addCalls[0]
	if call.playerID != "76561198000000012" {
		t.Fatalf("added player ID = %q", call.playerID)
	}
	if call.roleName != "squad_leader_whitelist" {
		t.Fatalf("added role = %q", call.roleName)
	}
	if call.expiresAt == nil {
		t.Fatalf("expected expiring whitelist role")
	}
}

func TestSendProgressToPlayerFindsProgressAndSessionAcrossIdentifiers(t *testing.T) {
	rcon := &fakeRconAPI{}
	plugin := &SquadLeaderWhitelistPlugin{
		playerProgress:     make(map[string]*PlayerProgressRecord),
		squadLeaderSession: make(map[string]*SquadLeaderSession),
	}

	err := plugin.Initialize(map[string]interface{}{
		"hours_to_whitelist": 8,
	}, &plugin_manager.PluginAPIs{
		DatabaseAPI: &fakeDatabaseAPI{data: map[string]string{}},
		RconAPI:     rcon,
		LogAPI:      fakeLogAPI{},
	})
	if err != nil {
		t.Fatalf("initialize plugin: %v", err)
	}

	plugin.playerProgress["abcdef0123456789abcdef0123456789"] = &PlayerProgressRecord{
		PlayerID:         "abcdef0123456789abcdef0123456789",
		EOSID:            "abcdef0123456789abcdef0123456789",
		QualifiedSeconds: int64(time.Hour / time.Second),
		LifetimeSeconds:  int64(2 * time.Hour / time.Second),
		LastEarnedAt:     time.Now(),
		LastSeenAt:       time.Now(),
	}
	plugin.squadLeaderSession["abcdef0123456789abcdef0123456789"] = &SquadLeaderSession{
		PlayerID:  "abcdef0123456789abcdef0123456789",
		EOSID:     "abcdef0123456789abcdef0123456789",
		StartTime: time.Now().Add(-time.Hour),
		LastCheck: time.Now(),
		SquadSize: 6,
		SquadName: "Alpha",
		Unlocked:  true,
	}

	err = plugin.sendProgressToPlayer("76561198000000021", "76561198000000021", "ABCDEF0123456789ABCDEF0123456789")
	if err != nil {
		t.Fatalf("send progress: %v", err)
	}

	if len(rcon.warnings) != 1 {
		t.Fatalf("warning count = %d, want 1", len(rcon.warnings))
	}
	if strings.Contains(rcon.warnings[0].message, "No squad leadership progress found") {
		t.Fatalf("expected cross-identifier lookup to find progress, got %q", rcon.warnings[0].message)
	}
	if !strings.Contains(rcon.warnings[0].message, "Currently leading squad: Alpha") {
		t.Fatalf("expected active session in message, got %q", rcon.warnings[0].message)
	}
}

type fakeDatabaseAPI struct {
	data map[string]string
}

func (f *fakeDatabaseAPI) GetPluginData(key string) (string, error) {
	value, ok := f.data[key]
	if !ok {
		return "", errors.New("key not found")
	}
	return value, nil
}

func (f *fakeDatabaseAPI) SetPluginData(key string, value string) error {
	if f.data == nil {
		f.data = make(map[string]string)
	}
	f.data[key] = value
	return nil
}

func (f *fakeDatabaseAPI) DeletePluginData(key string) error {
	delete(f.data, key)
	return nil
}

type fakeAdminAPI struct {
	admins          []*plugin_manager.TemporaryAdminInfo
	addCalls        []addAdminCall
	removeRoleCalls []removeRoleCall
}

type addAdminCall struct {
	playerID  string
	roleName  string
	notes     string
	expiresAt *time.Time
}

type removeRoleCall struct {
	playerID string
	roleName string
	notes    string
}

func (f *fakeAdminAPI) AddTemporaryAdmin(playerID string, roleName string, notes string, expiresAt *time.Time) error {
	f.addCalls = append(f.addCalls, addAdminCall{
		playerID:  playerID,
		roleName:  roleName,
		notes:     notes,
		expiresAt: expiresAt,
	})
	return nil
}

func (f *fakeAdminAPI) RemoveTemporaryAdmin(playerID string, notes string) error {
	return nil
}

func (f *fakeAdminAPI) RemoveTemporaryAdminRole(playerID string, roleName string, notes string) error {
	f.removeRoleCalls = append(f.removeRoleCalls, removeRoleCall{
		playerID: playerID,
		roleName: roleName,
		notes:    notes,
	})
	return nil
}

func (f *fakeAdminAPI) GetPlayerAdminStatus(playerID string) (*plugin_manager.PlayerAdminStatus, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeAdminAPI) ListTemporaryAdmins() ([]*plugin_manager.TemporaryAdminInfo, error) {
	return f.admins, nil
}

type fakeRconAPI struct {
	commands []string
	warnings []warningCall
}

type warningCall struct {
	playerID string
	message  string
}

func (f *fakeRconAPI) SendCommand(command string) (string, error) {
	f.commands = append(f.commands, command)
	return "", nil
}

func (f *fakeRconAPI) Broadcast(message string) error {
	return nil
}

func (f *fakeRconAPI) SendWarningToPlayer(playerID string, message string) error {
	f.warnings = append(f.warnings, warningCall{
		playerID: playerID,
		message:  message,
	})
	return nil
}

func (f *fakeRconAPI) KickPlayer(playerID string, reason string) error {
	return nil
}

func (f *fakeRconAPI) BanPlayer(playerID string, reason string, duration time.Duration) error {
	return nil
}

func (f *fakeRconAPI) BanWithEvidence(playerID string, reason string, duration time.Duration, eventID string, eventType string) (string, error) {
	return "", nil
}

func (f *fakeRconAPI) WarnPlayerWithRule(playerID string, message string, ruleID *string) error {
	return nil
}

func (f *fakeRconAPI) KickPlayerWithRule(playerID string, reason string, ruleID *string) error {
	return nil
}

func (f *fakeRconAPI) BanPlayerWithRule(playerID string, reason string, duration time.Duration, ruleID *string) error {
	return nil
}

func (f *fakeRconAPI) BanWithEvidenceAndRule(playerID string, reason string, duration time.Duration, eventID string, eventType string, ruleID *string) (string, error) {
	return "", nil
}

func (f *fakeRconAPI) BanWithEvidenceAndRuleAndMetadata(playerID string, reason string, duration time.Duration, eventID string, eventType string, ruleID *string, metadata map[string]interface{}) (string, error) {
	return "", nil
}

func (f *fakeRconAPI) RemovePlayerFromSquad(playerID string) error {
	return nil
}

func (f *fakeRconAPI) RemovePlayerFromSquadById(playerID string) error {
	return nil
}

type fakeLogAPI struct{}

func (fakeLogAPI) Info(message string, fields map[string]interface{})             {}
func (fakeLogAPI) Warn(message string, fields map[string]interface{})             {}
func (fakeLogAPI) Error(message string, err error, fields map[string]interface{}) {}
func (fakeLogAPI) Debug(message string, fields map[string]interface{})            {}
