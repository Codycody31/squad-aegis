package plugin_manager

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

func testConnectorManifest(connectorID string) ConnectorPackageManifest {
	return ConnectorPackageManifest{
		ConnectorID: connectorID,
		Name:        "Test Connector",
		Description: "A native test connector",
		Version:     "1.0.0",
		Targets: []PluginPackageTarget{
			{
				MinHostAPIVersion: NativeConnectorHostAPIVersion,
				TargetOS:          runtime.GOOS,
				TargetArch:        runtime.GOARCH,
				LibraryPath:       "bin/connector.so",
			},
		},
	}
}

func primaryConnectorManifestLibraryPath(manifest ConnectorPackageManifest) string {
	if len(manifest.Targets) > 0 {
		return manifest.Targets[0].LibraryPath
	}
	return ""
}

func buildConnectorArchive(t *testing.T, manifest ConnectorPackageManifest, libraryName string, libraryRaw []byte) []byte {
	t.Helper()

	if libraryName == "" {
		libraryName = primaryConnectorManifestLibraryPath(manifest)
	}
	if len(libraryRaw) == 0 {
		libraryRaw = []byte("fake-so-contents")
	}
	for index := range manifest.Targets {
		if strings.TrimSpace(manifest.Targets[index].SHA256) != "" {
			continue
		}
		manifest.Targets[index].SHA256 = fmt.Sprintf("%x", sha256.Sum256(libraryRaw))
	}

	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)

	manifestFile, err := writer.Create(pluginManifestFile)
	if err != nil {
		t.Fatalf("writer.Create(manifest) error = %v", err)
	}
	if _, err := manifestFile.Write(manifestRaw); err != nil {
		t.Fatalf("manifestFile.Write() error = %v", err)
	}

	libraryFile, err := writer.Create(libraryName)
	if err != nil {
		t.Fatalf("writer.Create(library) error = %v", err)
	}
	if _, err := libraryFile.Write(libraryRaw); err != nil {
		t.Fatalf("libraryFile.Write() error = %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	return archive.Bytes()
}

func TestDeleteInstalledConnectorPackageRejectsPersistedDatabaseInstances(t *testing.T) {
	t.Parallel()

	db := openTestSQLDB(t, &testSQLDriver{
		queryContext: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			if !strings.Contains(query, "FROM connectors") {
				return nil, fmt.Errorf("unexpected query: %s", query)
			}
			if len(args) != 2 {
				t.Fatalf("len(args) = %d, want 2", len(args))
			}
			if got, want := fmt.Sprint(args[0].Value), "discord"; got != want {
				t.Fatalf("args[0] = %q, want %q", got, want)
			}
			if got, want := fmt.Sprint(args[1].Value), "com.example.connector"; got != want {
				t.Fatalf("args[1] = %q, want %q", got, want)
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

	registry := NewConnectorRegistry()
	if err := registry.RegisterConnector(ConnectorDefinition{
		ID:           "com.example.connector",
		LegacyIDs:    []string{"discord"},
		Name:         "Connector",
		Version:      "1.0.0",
		Source:       PluginSourceNative,
		InstallState: PluginInstallStateReady,
		CreateInstance: func() Connector {
			return testConnector{}
		},
	}); err != nil {
		t.Fatalf("registry.RegisterConnector() error = %v", err)
	}

	pm := &PluginManager{
		db:                db,
		connectorRegistry: registry,
		nativeConnectorPackages: map[string]*InstalledConnectorPackage{
			"com.example.connector": {
				ConnectorID: "com.example.connector",
				Manifest: ConnectorPackageManifest{
					ConnectorID: "com.example.connector",
					LegacyIDs:   []string{"discord"},
				},
			},
		},
	}

	err := pm.DeleteInstalledConnectorPackage(context.Background(), "com.example.connector")
	if err == nil {
		t.Fatal("DeleteInstalledConnectorPackage() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "still has configured instances") {
		t.Fatalf("DeleteInstalledConnectorPackage() error = %q, want configured instances", err)
	}
}

func TestInstallConnectorBundleMarksLiveUpdatePendingRestart(t *testing.T) {
	requireLinuxNativePlugins(t)

	runtimeDir := t.TempDir()
	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.Plugins.NativeEnabled = true
		cfg.Plugins.RuntimeDir = runtimeDir
		cfg.Plugins.AllowUnsafeSideload = true
	})

	var saveCalls int
	db := openTestSQLDB(t, &testSQLDriver{
		queryContext: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			if !strings.Contains(query, "FROM connectors") {
				return nil, fmt.Errorf("unexpected query: %s", query)
			}
			return &testSQLRows{
				columns: []string{"exists"},
				values:  [][]driver.Value{{false}},
			}, nil
		},
		execContext: func(query string, args []driver.NamedValue) (driver.Result, error) {
			if !strings.Contains(query, "INSERT INTO connector_packages") {
				return nil, fmt.Errorf("unexpected exec: %s", query)
			}
			saveCalls++
			if got, want := fmt.Sprint(args[7].Value), string(PluginInstallStatePendingRestart); got != want {
				t.Fatalf("persisted install state = %q, want %q", got, want)
			}
			if got, want := fmt.Sprint(args[16].Value), nativeConnectorPendingRestartMessage; got != want {
				t.Fatalf("persisted last error = %q, want %q", got, want)
			}
			return driver.RowsAffected(1), nil
		},
	})

	registry := NewConnectorRegistry()
	oldRuntimePath := filepath.Join(runtimeDir, "com.example.connector", "1.0.0", "connector.so")
	if err := registry.RegisterConnector(ConnectorDefinition{
		ID:             "com.example.connector",
		Name:           "Native Connector",
		Version:        "1.0.0",
		Source:         PluginSourceNative,
		RuntimePath:    oldRuntimePath,
		CreateInstance: func() Connector { return testConnector{} },
	}); err != nil {
		t.Fatalf("registry.RegisterConnector() error = %v", err)
	}

	pm := &PluginManager{
		db:                db,
		connectorRegistry: registry,
		connectors: map[string]*ConnectorInstance{
			"com.example.connector": {
				ID:        "com.example.connector",
				Connector: testConnector{},
			},
		},
		nativeConnectorPackages: map[string]*InstalledConnectorPackage{
			"com.example.connector": {
				ConnectorID:  "com.example.connector",
				Name:         "Native Connector",
				Version:      "1.0.0",
				Source:       PluginSourceNative,
				Distribution: PluginDistributionSideload,
				InstallState: PluginInstallStateReady,
				RuntimePath:  oldRuntimePath,
				Manifest: ConnectorPackageManifest{
					ConnectorID: "com.example.connector",
				},
			},
		},
		loadedNativeConnectors: map[string]string{
			"com.example.connector": "1.0.0",
		},
	}

	var loaderCalls int
	previousLoader := nativeConnectorVerifiedLoader
	nativeConnectorVerifiedLoader = func(string, string, ConnectorPackageManifest, PluginPackageTarget) (ConnectorDefinition, error) {
		loaderCalls++
		return ConnectorDefinition{
			ID:             "com.example.connector",
			Name:           "Native Connector",
			Version:        "2.0.0",
			Source:         PluginSourceNative,
			CreateInstance: func() Connector { return testConnector{} },
		}, nil
	}
	t.Cleanup(func() { nativeConnectorVerifiedLoader = previousLoader })

	manifest := testConnectorManifest("com.example.connector")
	manifest.Version = "2.0.0"
	archive := buildConnectorArchive(t, manifest, primaryConnectorManifestLibraryPath(manifest), []byte("replacement-so"))

	pkg, err := pm.installConnectorBundle(context.Background(), bytes.NewReader(archive), int64(len(archive)), "replacement.zip", PluginDistributionSideload)
	if err != nil {
		t.Fatalf("installConnectorBundle() error = %v", err)
	}
	if got, want := pkg.InstallState, PluginInstallStatePendingRestart; got != want {
		t.Fatalf("pkg.InstallState = %q, want %q", got, want)
	}
	if got, want := pkg.LastError, nativeConnectorPendingRestartMessage; got != want {
		t.Fatalf("pkg.LastError = %q, want %q", got, want)
	}
	if loaderCalls != 0 {
		t.Fatalf("nativeConnectorVerifiedLoader() calls = %d, want 0", loaderCalls)
	}
	if saveCalls != 1 {
		t.Fatalf("saveCalls = %d, want 1", saveCalls)
	}

	loadedVersion, ok := pm.getLoadedNativeConnectorVersion("com.example.connector")
	if !ok {
		t.Fatal("getLoadedNativeConnectorVersion() ok = false, want true")
	}
	if got, want := loadedVersion, "1.0.0"; got != want {
		t.Fatalf("loaded version = %q, want %q", got, want)
	}

	definition, err := pm.connectorRegistry.GetConnector("com.example.connector")
	if err != nil {
		t.Fatalf("connectorRegistry.GetConnector() error = %v", err)
	}
	if got, want := definition.Version, "1.0.0"; got != want {
		t.Fatalf("definition.Version = %q, want %q", got, want)
	}
	if got, want := definition.RuntimePath, oldRuntimePath; got != want {
		t.Fatalf("definition.RuntimePath = %q, want %q", got, want)
	}

	persisted := pm.getNativeConnectorPackage("com.example.connector")
	if persisted == nil {
		t.Fatal("getNativeConnectorPackage() returned nil")
	}
	if got, want := persisted.Version, "2.0.0"; got != want {
		t.Fatalf("persisted.Version = %q, want %q", got, want)
	}
	if got, want := persisted.InstallState, PluginInstallStatePendingRestart; got != want {
		t.Fatalf("persisted.InstallState = %q, want %q", got, want)
	}
}
