package server_seeder_whitelist

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/whitelistprogress"
)

func TestLoadPlayerProgressMigratesLegacySeederState(t *testing.T) {
	legacyState, err := json.Marshal(map[string]*legacyPlayerProgressRecord{
		"76561198000000001": {
			SteamID:        "76561198000000001",
			Progress:       50,
			LastProgressed: time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC),
			TotalSeeded:    125,
			LastSeen:       time.Date(2026, 3, 21, 15, 30, 0, 0, time.UTC),
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

	plugin := &ServerSeederWhitelistPlugin{
		playerProgress: make(map[string]*PlayerProgressRecord),
	}

	err = plugin.Initialize(map[string]interface{}{
		"hours_to_whitelist": 6,
	}, &plugin_manager.PluginAPIs{
		DatabaseAPI: db,
		LogAPI:      fakeLogAPI{},
	})
	if err != nil {
		t.Fatalf("initialize plugin: %v", err)
	}

	record := plugin.playerProgress["76561198000000001"]
	if record == nil {
		t.Fatalf("expected migrated player record")
	}

	if got, want := record.QualifiedSeconds, int64(3*time.Hour/time.Second); got != want {
		t.Fatalf("qualified seconds = %d, want %d", got, want)
	}

	if got, want := record.LifetimeSeconds, int64(450*time.Minute/time.Second); got != want {
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

func TestUpdateConfigRemovesManagedRoleWhenThresholdIncreases(t *testing.T) {
	admins := &fakeAdminAPI{
		admins: []*plugin_manager.TemporaryAdminInfo{
			{
				SteamID:  "76561198000000002",
				RoleName: "seeder_whitelist",
			},
		},
	}

	plugin := &ServerSeederWhitelistPlugin{
		playerProgress: make(map[string]*PlayerProgressRecord),
	}

	err := plugin.Initialize(map[string]interface{}{
		"hours_to_whitelist":        6,
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

	plugin.playerProgress["76561198000000002"] = &PlayerProgressRecord{
		PlayerID:         "76561198000000002",
		QualifiedSeconds: whitelistprogress.RequiredSeconds(6),
		LifetimeSeconds:  whitelistprogress.RequiredSeconds(6),
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

	if len(admins.addCalls) != 0 {
		t.Fatalf("expected no admin add calls, got %d", len(admins.addCalls))
	}

	if len(admins.removeRoleCalls) != 1 {
		t.Fatalf("expected one role removal, got %d", len(admins.removeRoleCalls))
	}

	call := admins.removeRoleCalls[0]
	if call.playerID != "76561198000000002" {
		t.Fatalf("removed player ID = %q", call.playerID)
	}
	if call.roleName != "seeder_whitelist" {
		t.Fatalf("removed role = %q", call.roleName)
	}
}

func TestSendProgressToPlayerResolvesLegacyEOSRecordAcrossIdentifiers(t *testing.T) {
	rcon := &fakeRconAPI{}
	plugin := &ServerSeederWhitelistPlugin{
		playerProgress: make(map[string]*PlayerProgressRecord),
	}

	err := plugin.Initialize(map[string]interface{}{
		"hours_to_whitelist": 6,
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

	err = plugin.sendProgressToPlayer("76561198000000021", "76561198000000021", "ABCDEF0123456789ABCDEF0123456789")
	if err != nil {
		t.Fatalf("send progress: %v", err)
	}

	if len(rcon.warnings) != 1 {
		t.Fatalf("warning count = %d, want 1", len(rcon.warnings))
	}
	if strings.Contains(rcon.warnings[0].message, "No seeding progress found") {
		t.Fatalf("expected cross-identifier lookup to find progress, got %q", rcon.warnings[0].message)
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
