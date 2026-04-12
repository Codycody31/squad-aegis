package plugin_manager

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

type testSQLDriver struct {
	queryContext func(query string, args []driver.NamedValue) (driver.Rows, error)
	execContext  func(query string, args []driver.NamedValue) (driver.Result, error)
}

func (d *testSQLDriver) Open(string) (driver.Conn, error) {
	return &testSQLConn{driver: d}, nil
}

type testSQLConn struct {
	driver *testSQLDriver
}

func (c *testSQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("prepare is not implemented in testSQLConn")
}

func (c *testSQLConn) Close() error {
	return nil
}

func (c *testSQLConn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("transactions are not implemented in testSQLConn")
}

func (c *testSQLConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if c.driver.queryContext == nil {
		return nil, fmt.Errorf("unexpected query: %s", query)
	}

	return c.driver.queryContext(query, args)
}

func (c *testSQLConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if c.driver.execContext == nil {
		return nil, fmt.Errorf("unexpected exec: %s", query)
	}

	return c.driver.execContext(query, args)
}

func (c *testSQLConn) CheckNamedValue(*driver.NamedValue) error {
	return nil
}

type testSQLRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *testSQLRows) Columns() []string {
	return r.columns
}

func (r *testSQLRows) Close() error {
	return nil
}

func (r *testSQLRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}

	for i := range dest {
		dest[i] = nil
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

var testSQLDriverSeq uint64

func openTestSQLDB(t *testing.T, driverImpl *testSQLDriver) *sql.DB {
	t.Helper()

	driverName := fmt.Sprintf("plugin-manager-test-%d", atomic.AddUint64(&testSQLDriverSeq, 1))
	sql.Register(driverName, driverImpl)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

func TestLoadInstalledPluginPackagesActivatesPendingRestartPackageOnStartup(t *testing.T) {
	requireLinuxNativePlugins(t)

	runtimeDir := t.TempDir()
	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.App.InContainer = false
		cfg.Plugins.NativeEnabled = true
		cfg.Plugins.RuntimeDir = runtimeDir
		cfg.Plugins.AllowUnsafeSideload = true
	})

	pluginSubdir := filepath.Join(runtimeDir, "com.example.pending", "1.0.0")
	if err := os.MkdirAll(pluginSubdir, 0o750); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	runtimePath := filepath.Join(pluginSubdir, "com.example.pending.so")
	libraryBytes := []byte("fake-runtime-library")
	if err := os.WriteFile(runtimePath, libraryBytes, 0o640); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(libraryBytes))

	manifest := testManifest("com.example.pending")
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	now := time.Now().UTC()
	var saveCalls int

	db := openTestSQLDB(t, &testSQLDriver{
		queryContext: func(query string, _ []driver.NamedValue) (driver.Rows, error) {
			if !strings.Contains(query, "FROM plugin_packages") {
				return nil, fmt.Errorf("unexpected query: %s", query)
			}

			return &testSQLRows{
				columns: []string{
					"plugin_id",
					"name",
					"description",
					"version",
					"source",
					"distribution",
					"official",
					"install_state",
					"runtime_path",
					"manifest_json",
					"signature_verified",
					"unsafe",
					"checksum",
					"min_host_api_version",
					"required_capabilities",
					"target_os",
					"target_arch",
					"last_error",
					"created_at",
					"updated_at",
					"manifest_signature",
					"manifest_public_key",
				},
				values: [][]driver.Value{{
					"com.example.pending",
					manifest.Name,
					manifest.Description,
					manifest.Version,
					string(PluginSourceNative),
					string(PluginDistributionSideload),
					false,
					string(PluginInstallStatePendingRestart),
					runtimePath,
					manifestRaw,
					true,
					false,
					checksum,
					int64(NativePluginHostAPIVersion),
					[]byte("[]"),
					manifest.Targets[0].TargetOS,
					manifest.Targets[0].TargetArch,
					"Restart Aegis to activate the updated native plugin package",
					now,
					now,
					"",
					"",
				}},
			}, nil
		},
		execContext: func(query string, args []driver.NamedValue) (driver.Result, error) {
			if !strings.Contains(query, "INSERT INTO plugin_packages") {
				return nil, fmt.Errorf("unexpected exec: %s", query)
			}

			saveCalls++
			if got, want := fmt.Sprint(args[7].Value), string(PluginInstallStateReady); got != want {
				t.Fatalf("persisted install state = %q, want %q", got, want)
			}
			if got, want := fmt.Sprint(args[17].Value), ""; got != want {
				t.Fatalf("persisted last error = %q, want empty string", got)
			}

			return driver.RowsAffected(1), nil
		},
	})

	previousLoader := nativePluginVerifiedLoader
	nativePluginVerifiedLoader = func(gotPath, gotChecksum string, gotManifest PluginPackageManifest, gotTarget PluginPackageTarget) (PluginDefinition, error) {
		if gotPath != runtimePath {
			t.Fatalf("runtimePath = %q, want %q", gotPath, runtimePath)
		}
		if gotChecksum != checksum {
			t.Fatalf("checksum = %q, want %q", gotChecksum, checksum)
		}
		if gotManifest.PluginID != "com.example.pending" {
			t.Fatalf("manifest.PluginID = %q, want com.example.pending", gotManifest.PluginID)
		}

		return PluginDefinition{
			ID:             gotManifest.PluginID,
			Name:           gotManifest.Name,
			Source:         PluginSourceNative,
			CreateInstance: func() Plugin { return &noopPlugin{} },
		}, nil
	}
	t.Cleanup(func() {
		nativePluginVerifiedLoader = previousLoader
	})

	pm := &PluginManager{
		db:                  db,
		registry:            NewPluginRegistry(),
		nativePackages:      make(map[string]*InstalledPluginPackage),
		loadedNativePlugins: make(map[string]string),
	}

	if err := pm.loadInstalledPluginPackages(); err != nil {
		t.Fatalf("loadInstalledPluginPackages() error = %v", err)
	}

	if saveCalls != 1 {
		t.Fatalf("savePluginPackageToDatabase() calls = %d, want 1", saveCalls)
	}

	pkg := pm.getNativePackage("com.example.pending")
	if pkg == nil {
		t.Fatal("getNativePackage() returned nil")
	}
	if got, want := pkg.InstallState, PluginInstallStateReady; got != want {
		t.Fatalf("pkg.InstallState = %q, want %q", got, want)
	}
	if pkg.LastError != "" {
		t.Fatalf("pkg.LastError = %q, want empty string", pkg.LastError)
	}

	definition, err := pm.registry.GetPlugin("com.example.pending")
	if err != nil {
		t.Fatalf("registry.GetPlugin() error = %v", err)
	}
	if got, want := definition.InstallState, PluginInstallStateReady; got != want {
		t.Fatalf("definition.InstallState = %q, want %q", got, want)
	}

	loadedVersion, ok := pm.getLoadedNativePluginVersion("com.example.pending")
	if !ok {
		t.Fatal("getLoadedNativePluginVersion() ok = false, want true")
	}
	if got, want := loadedVersion, manifest.Version; got != want {
		t.Fatalf("loaded version = %q, want %q", got, want)
	}
}

func TestLoadPluginsFromDatabaseKeepsUnavailableInstancesVisible(t *testing.T) {
	serverID := uuid.New()
	instanceID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Minute)
	updatedAt := createdAt.Add(30 * time.Second)

	var updateCalls int
	var deleteCalls int

	db := openTestSQLDB(t, &testSQLDriver{
		queryContext: func(query string, _ []driver.NamedValue) (driver.Rows, error) {
			if !strings.Contains(query, "FROM plugin_instances") {
				return nil, fmt.Errorf("unexpected query: %s", query)
			}

			return &testSQLRows{
				columns: []string{
					"id",
					"server_id",
					"plugin_id",
					"notes",
					"config",
					"enabled",
					"log_level",
					"created_at",
					"updated_at",
				},
				values: [][]driver.Value{{
					instanceID.String(),
					serverID.String(),
					"com.example.missing",
					"persisted instance",
					[]byte(`{"token":"super-secret"}`),
					true,
					"info",
					createdAt,
					updatedAt,
				}},
			}, nil
		},
		execContext: func(query string, _ []driver.NamedValue) (driver.Result, error) {
			switch {
			case strings.Contains(query, "UPDATE plugin_instances"):
				updateCalls++
			case strings.Contains(query, "DELETE FROM plugin_data"):
				return driver.RowsAffected(0), nil
			case strings.Contains(query, "DELETE FROM plugin_instances"):
				deleteCalls++
			default:
				return nil, fmt.Errorf("unexpected exec: %s", query)
			}

			return driver.RowsAffected(1), nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	pm := &PluginManager{
		db:       db,
		registry: NewPluginRegistry(),
		plugins:  make(map[uuid.UUID]map[uuid.UUID]*PluginInstance),
		nativePackages: map[string]*InstalledPluginPackage{
			"com.example.missing": {
				PluginID:     "com.example.missing",
				Name:         "Broken Native Plugin",
				Source:       PluginSourceNative,
				Distribution: PluginDistributionSideload,
				InstallState: PluginInstallStateError,
				LastError:    "failed to load native plugin",
			},
		},
		ctx: ctx,
	}

	if err := pm.loadPluginsFromDatabase(); err != nil {
		t.Fatalf("loadPluginsFromDatabase() error = %v", err)
	}

	instance, err := pm.GetPluginInstance(serverID, instanceID)
	if err != nil {
		t.Fatalf("GetPluginInstance() error = %v", err)
	}
	if got, want := instance.PluginName, "Broken Native Plugin"; got != want {
		t.Fatalf("instance.PluginName = %q, want %q", got, want)
	}
	if got, want := instance.Status, PluginStatusError; got != want {
		t.Fatalf("instance.Status = %q, want %q", got, want)
	}
	if got, want := instance.InstallState, PluginInstallStateError; got != want {
		t.Fatalf("instance.InstallState = %q, want %q", got, want)
	}
	if got, want := instance.LastError, "failed to load native plugin"; got != want {
		t.Fatalf("instance.LastError = %q, want %q", got, want)
	}
	if len(instance.Config) != 0 {
		t.Fatalf("len(instance.Config) = %d, want 0 masked fields", len(instance.Config))
	}

	stored := pm.plugins[serverID][instanceID]
	if stored == nil {
		t.Fatal("stored plugin instance = nil")
	}
	if stored.Plugin != nil {
		t.Fatal("stored.Plugin != nil, want unavailable runtime")
	}
	if got, want := stored.Config["token"], "super-secret"; got != want {
		t.Fatalf("stored.Config[token] = %v, want %q", got, want)
	}

	if err := pm.DisablePluginInstance(serverID, instanceID); err != nil {
		t.Fatalf("DisablePluginInstance() error = %v", err)
	}
	if updateCalls != 1 {
		t.Fatalf("update calls = %d, want 1", updateCalls)
	}

	if err := pm.DeletePluginInstance(serverID, instanceID); err != nil {
		t.Fatalf("DeletePluginInstance() error = %v", err)
	}
	if deleteCalls != 1 {
		t.Fatalf("delete calls = %d, want 1", deleteCalls)
	}
	if len(pm.GetPluginInstances(serverID)) != 0 {
		t.Fatalf("len(GetPluginInstances()) = %d, want 0 after delete", len(pm.GetPluginInstances(serverID)))
	}
}

func TestDeleteInstalledPluginPackageRejectsPersistedDatabaseInstances(t *testing.T) {
	db := openTestSQLDB(t, &testSQLDriver{
		queryContext: func(query string, _ []driver.NamedValue) (driver.Rows, error) {
			if !strings.Contains(query, "SELECT EXISTS") {
				return nil, fmt.Errorf("unexpected query: %s", query)
			}

			return &testSQLRows{
				columns: []string{"exists"},
				values:  [][]driver.Value{{true}},
			}, nil
		},
		execContext: func(query string, _ []driver.NamedValue) (driver.Result, error) {
			return nil, fmt.Errorf("unexpected exec: %s", query)
		},
	})

	pm := &PluginManager{
		db: db,
		nativePackages: map[string]*InstalledPluginPackage{
			"com.example.plugin": {
				PluginID: "com.example.plugin",
			},
		},
	}

	err := pm.DeleteInstalledPluginPackage(context.Background(), "com.example.plugin")
	if err == nil {
		t.Fatal("DeleteInstalledPluginPackage() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "still has configured server instances") {
		t.Fatalf("DeleteInstalledPluginPackage() error = %q, want configured server instances", err)
	}
}
