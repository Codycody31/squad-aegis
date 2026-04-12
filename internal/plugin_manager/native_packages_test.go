package plugin_manager

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/plugin_signing"
	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

type noopPlugin struct{}

func (p *noopPlugin) GetDefinition() PluginDefinition {
	return PluginDefinition{}
}

func (p *noopPlugin) Initialize(map[string]interface{}, *PluginAPIs) error {
	return nil
}

func (p *noopPlugin) Start(context.Context) error {
	return nil
}

func (p *noopPlugin) Stop() error {
	return nil
}

func (p *noopPlugin) HandleEvent(*PluginEvent) error {
	return nil
}

func (p *noopPlugin) GetStatus() PluginStatus {
	return PluginStatusStopped
}

func (p *noopPlugin) GetConfig() map[string]interface{} {
	return map[string]interface{}{}
}

func (p *noopPlugin) UpdateConfig(map[string]interface{}) error {
	return nil
}

func (p *noopPlugin) GetCommands() []PluginCommand {
	return nil
}

func (p *noopPlugin) ExecuteCommand(string, map[string]interface{}) (*CommandResult, error) {
	return nil, nil
}

func (p *noopPlugin) GetCommandExecutionStatus(string) (*CommandExecutionStatus, error) {
	return nil, nil
}

func setPluginTestConfig(t *testing.T, mutate func(*config.Struct)) {
	t.Helper()

	prev := config.Config
	cfg := config.Struct{}
	if prev != nil {
		cfg = *prev
	}

	mutate(&cfg)
	config.Config = &cfg
	ResetRuntimeDirCache()

	t.Cleanup(func() {
		config.Config = prev
		ResetRuntimeDirCache()
	})
}

func requireLinuxNativePlugins(t *testing.T) {
	t.Helper()

	if runtime.GOOS != "linux" {
		t.Skip("native plugin tests require a Linux host")
	}
}

func testManifest(pluginID string) PluginPackageManifest {
	return PluginPackageManifest{
		PluginID:    pluginID,
		Name:        "Test Plugin",
		Description: "A native test plugin",
		Version:     "1.0.0",
		Targets: []PluginPackageTarget{
			{
				MinHostAPIVersion: NativePluginHostAPIVersion,
				TargetOS:          runtime.GOOS,
				TargetArch:        runtime.GOARCH,
				LibraryPath:       "bin/plugin.so",
			},
		},
	}
}

func primaryManifestLibraryPath(manifest PluginPackageManifest) string {
	if len(manifest.Targets) > 0 {
		return manifest.Targets[0].LibraryPath
	}
	return ""
}

func buildPluginArchive(t *testing.T, manifest PluginPackageManifest, libraryName string, libraryRaw []byte, signatureRaw []byte, publicKeyRaw []byte) []byte {
	t.Helper()

	return buildPluginArchiveWithLibraries(t, manifest, map[string][]byte{libraryName: libraryRaw}, signatureRaw, publicKeyRaw)
}

func buildPluginArchiveWithLibraries(t *testing.T, manifest PluginPackageManifest, libraries map[string][]byte, signatureRaw []byte, publicKeyRaw []byte) []byte {
	t.Helper()

	if len(libraries) == 0 {
		libraries = map[string][]byte{
			primaryManifestLibraryPath(manifest): []byte("fake-so-contents"),
		}
	}

	for index := range manifest.Targets {
		if strings.TrimSpace(manifest.Targets[index].SHA256) != "" {
			continue
		}
		libName := manifest.Targets[index].LibraryPath
		raw, ok := libraries[libName]
		if !ok && libName != "" {
			raw = []byte("fake-so-contents")
		}
		if len(raw) == 0 {
			raw = []byte("fake-so-contents")
		}
		manifest.Targets[index].SHA256 = fmt.Sprintf("%x", sha256.Sum256(raw))
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

	if len(signatureRaw) > 0 {
		signatureFile, err := writer.Create(pluginSignatureFile)
		if err != nil {
			t.Fatalf("writer.Create(signature) error = %v", err)
		}
		if _, err := signatureFile.Write(signatureRaw); err != nil {
			t.Fatalf("signatureFile.Write() error = %v", err)
		}
	}
	if len(publicKeyRaw) > 0 {
		publicKeyFile, err := writer.Create(pluginPublicKeyFile)
		if err != nil {
			t.Fatalf("writer.Create(public key) error = %v", err)
		}
		if _, err := publicKeyFile.Write(publicKeyRaw); err != nil {
			t.Fatalf("publicKeyFile.Write() error = %v", err)
		}
	}

	for libraryName, libraryRaw := range libraries {
		if libraryName == "" {
			libraryName = primaryManifestLibraryPath(manifest)
		}
		if len(libraryRaw) == 0 {
			libraryRaw = []byte("fake-so-contents")
		}

		libraryFile, err := writer.Create(libraryName)
		if err != nil {
			t.Fatalf("writer.Create(library) error = %v", err)
		}
		if _, err := libraryFile.Write(libraryRaw); err != nil {
			t.Fatalf("libraryFile.Write() error = %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	return archive.Bytes()
}

func TestEnrichPluginDefinitionUsesNativePackageMetadata(t *testing.T) {
	t.Parallel()

	pm := &PluginManager{
		nativePackages: map[string]*InstalledPluginPackage{
			"com.example.test": {
				PluginID:             "com.example.test",
				Name:                 "External Test Plugin",
				Description:          "Loaded from a native package",
				Version:              "1.2.3",
				Source:               PluginSourceNative,
				Distribution:         PluginDistributionSideload,
				Official:             false,
				InstallState:         PluginInstallStateReady,
				RuntimePath:          "/tmp/plugins/test.so",
				SignatureVerified:    true,
				Unsafe:               false,
				MinHostAPIVersion:    2,
				RequiredCapabilities: []string{"api.rcon"},
				TargetOS:             "linux",
				TargetArch:           "amd64",
			},
		},
	}

	definition := pm.enrichPluginDefinition(PluginDefinition{
		ID:     "com.example.test",
		Source: PluginSourceNative,
	})

	if got, want := definition.Name, "External Test Plugin"; got != want {
		t.Fatalf("definition.Name = %q, want %q", got, want)
	}
	if got, want := definition.InstallState, PluginInstallStateReady; got != want {
		t.Fatalf("definition.InstallState = %q, want %q", got, want)
	}
	if got, want := definition.Distribution, PluginDistributionSideload; got != want {
		t.Fatalf("definition.Distribution = %q, want %q", got, want)
	}
	if got, want := definition.RuntimePath, "/tmp/plugins/test.so"; got != want {
		t.Fatalf("definition.RuntimePath = %q, want %q", got, want)
	}
}

func TestReadPluginBundleUsesManifestAndLibrary(t *testing.T) {
	t.Parallel()

	manifestBytes, err := json.Marshal(PluginPackageManifest{
		PluginID:    "com.example.bundle",
		Name:        "Bundle Plugin",
		Description: "A packaged plugin",
		Version:     "1.0.0",
		Official:    false,
		Targets: []PluginPackageTarget{
			{
				MinHostAPIVersion: NativePluginHostAPIVersion,
				TargetOS:          runtime.GOOS,
				TargetArch:        runtime.GOARCH,
				LibraryPath:       "bin/plugin.so",
			},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)

	manifestFile, err := writer.Create(pluginManifestFile)
	if err != nil {
		t.Fatalf("writer.Create(manifest) error = %v", err)
	}
	if _, err := manifestFile.Write(manifestBytes); err != nil {
		t.Fatalf("manifestFile.Write() error = %v", err)
	}

	libraryFile, err := writer.Create("bin/plugin.so")
	if err != nil {
		t.Fatalf("writer.Create(library) error = %v", err)
	}
	if _, err := libraryFile.Write([]byte("fake-so-contents")); err != nil {
		t.Fatalf("libraryFile.Write() error = %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	manifest, selectedTarget, manifestRaw, _, _, libraryRaw, libraryName, err := readPluginBundle(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err != nil {
		t.Fatalf("readPluginBundle() error = %v", err)
	}

	if got, want := manifest.PluginID, "com.example.bundle"; got != want {
		t.Fatalf("manifest.PluginID = %q, want %q", got, want)
	}
	if got, want := libraryName, "bin/plugin.so"; got != want {
		t.Fatalf("libraryName = %q, want %q", got, want)
	}
	if got, want := string(libraryRaw), "fake-so-contents"; got != want {
		t.Fatalf("libraryRaw = %q, want %q", got, want)
	}
	if got, want := selectedTarget.TargetOS, runtime.GOOS; got != want {
		t.Fatalf("selectedTarget.TargetOS = %q, want %q", got, want)
	}
	if len(manifestRaw) == 0 {
		t.Fatal("manifestRaw should not be empty")
	}
}

func TestReadPluginBundleSelectsMatchingTargetFromMatrix(t *testing.T) {
	t.Parallel()

	manifest := PluginPackageManifest{
		PluginID:    "com.example.bundle",
		Name:        "Bundle Plugin",
		Description: "A packaged plugin",
		Version:     "1.0.0",
		Official:    false,
		Targets: []PluginPackageTarget{
			{
				MinHostAPIVersion: NativePluginHostAPIVersion,
				TargetOS:          runtime.GOOS,
				TargetArch:        runtime.GOARCH,
				LibraryPath:       "bin/host-plugin.so",
			},
			{
				MinHostAPIVersion: NativePluginHostAPIVersion,
				TargetOS:          "linux",
				TargetArch:        "arm64",
				LibraryPath:       "bin/arm64-plugin.so",
			},
		},
	}

	archive := buildPluginArchiveWithLibraries(t, manifest, map[string][]byte{
		"bin/host-plugin.so":  []byte("host-so"),
		"bin/arm64-plugin.so": []byte("arm-so"),
	}, nil, nil)

	_, selectedTarget, _, _, _, libraryRaw, libraryName, err := readPluginBundle(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("readPluginBundle() error = %v", err)
	}

	if got, want := libraryName, "bin/host-plugin.so"; got != want {
		t.Fatalf("libraryName = %q, want %q", got, want)
	}
	if got, want := string(libraryRaw), "host-so"; got != want {
		t.Fatalf("libraryRaw = %q, want %q", got, want)
	}
	if got, want := selectedTarget.LibraryPath, "bin/host-plugin.so"; got != want {
		t.Fatalf("selectedTarget.LibraryPath = %q, want %q", got, want)
	}
}

func TestValidatePluginManifestRejectsDuplicateTargets(t *testing.T) {
	t.Parallel()

	manifest := PluginPackageManifest{
		PluginID:    "com.example.duplicate-targets",
		Name:        "Duplicate Targets",
		Description: "A packaged plugin",
		Version:     "1.0.0",
		Official:    false,
		Targets: []PluginPackageTarget{
			{
				MinHostAPIVersion: NativePluginHostAPIVersion,
				TargetOS:          runtime.GOOS,
				TargetArch:        runtime.GOARCH,
				LibraryPath:       "bin/plugin-a.so",
				SHA256:            "deadbeef",
			},
			{
				MinHostAPIVersion: NativePluginHostAPIVersion,
				TargetOS:          runtime.GOOS,
				TargetArch:        runtime.GOARCH,
				LibraryPath:       "bin/plugin-b.so",
				SHA256:            "cafebabe",
			},
		},
	}

	err := validatePluginManifest(manifest)
	if err == nil {
		t.Fatal("validatePluginManifest() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "duplicate target") {
		t.Fatalf("validatePluginManifest() error = %q, want duplicate target", err)
	}
}

func TestValidatePluginManifestRejectsPluginIDThatChangesUnderRuntimeSanitization(t *testing.T) {
	t.Parallel()

	manifest := testManifest("foo/bar")

	err := validatePluginManifest(manifest)
	if err == nil {
		t.Fatal("validatePluginManifest() error = nil, want error")
	}
	if !strings.Contains(err.Error(), `plugin_id "foo/bar"`) {
		t.Fatalf("validatePluginManifest() error = %q, want plugin_id sanitization rejection", err)
	}
}

func TestValidatePluginManifestRejectsVersionThatChangesUnderRuntimeSanitization(t *testing.T) {
	t.Parallel()

	manifest := testManifest("com.example.version-prefix")
	manifest.Version = "v1.0.0"

	err := validatePluginManifest(manifest)
	if err == nil {
		t.Fatal("validatePluginManifest() error = nil, want error")
	}
	if !strings.Contains(err.Error(), `version "v1.0.0"`) {
		t.Fatalf("validatePluginManifest() error = %q, want version sanitization rejection", err)
	}
}

func TestPluginRuntimeDirUsesLocalDefaultOutsideContainer(t *testing.T) {
	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.App.InContainer = false
		cfg.Plugins.RuntimeDir = ""
	})

	wantAbs, err := filepath.Abs("plugins")
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	if got := pluginRuntimeDir(); got != wantAbs {
		t.Fatalf("pluginRuntimeDir() = %q, want %q", got, wantAbs)
	}
}

func TestPluginRuntimeDirUsesContainerDefaultInsideContainer(t *testing.T) {
	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.App.InContainer = true
		cfg.Plugins.RuntimeDir = ""
	})

	if got, want := pluginRuntimeDir(), "/etc/squad-aegis/plugins"; got != want {
		t.Fatalf("pluginRuntimeDir() = %q, want %q", got, want)
	}
}

func TestPluginRuntimeDirUsesExplicitOverride(t *testing.T) {
	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.App.InContainer = false
		cfg.Plugins.RuntimeDir = "/tmp/custom-plugins"
	})

	if got, want := pluginRuntimeDir(), "/tmp/custom-plugins"; got != want {
		t.Fatalf("pluginRuntimeDir() = %q, want %q", got, want)
	}
}

func TestListAvailablePluginsOmitsNativePluginsThatAreNotReady(t *testing.T) {
	t.Parallel()

	registry := NewPluginRegistry()
	if err := registry.RegisterPlugin(PluginDefinition{
		ID:             "bundled.plugin",
		Name:           "Bundled Plugin",
		CreateInstance: func() Plugin { return &noopPlugin{} },
	}); err != nil {
		t.Fatalf("registry.RegisterPlugin(bundled) error = %v", err)
	}
	if err := registry.RegisterPlugin(PluginDefinition{
		ID:             "native.plugin",
		Name:           "Native Plugin",
		Source:         PluginSourceNative,
		CreateInstance: func() Plugin { return &noopPlugin{} },
	}); err != nil {
		t.Fatalf("registry.RegisterPlugin(native) error = %v", err)
	}

	pm := &PluginManager{
		registry: registry,
		nativePackages: map[string]*InstalledPluginPackage{
			"native.plugin": {
				PluginID:     "native.plugin",
				Name:         "Native Plugin",
				Source:       PluginSourceNative,
				Distribution: PluginDistributionSideload,
				InstallState: PluginInstallStateError,
			},
		},
	}

	available := pm.ListAvailablePlugins()
	if len(available) != 1 {
		t.Fatalf("len(available) = %d, want 1", len(available))
	}
	if got, want := available[0].ID, "bundled.plugin"; got != want {
		t.Fatalf("available[0].ID = %q, want %q", got, want)
	}
}

func TestDeleteInstalledPluginPackageRejectsConfiguredInstances(t *testing.T) {
	t.Parallel()

	serverID := uuid.New()
	instanceID := uuid.New()
	pm := &PluginManager{
		plugins: map[uuid.UUID]map[uuid.UUID]*PluginInstance{
			serverID: {
				instanceID: {
					ID:       instanceID,
					ServerID: serverID,
					PluginID: "com.example.plugin",
				},
			},
		},
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

func TestDeleteInstalledPluginPackageUnregistersUnloadedNativeRuntime(t *testing.T) {
	t.Parallel()

	db := openTestSQLDB(t, &testSQLDriver{
		queryContext: func(query string, _ []driver.NamedValue) (driver.Rows, error) {
			if !strings.Contains(query, "SELECT EXISTS") {
				return nil, fmt.Errorf("unexpected query: %s", query)
			}

			return &testSQLRows{
				columns: []string{"exists"},
				values:  [][]driver.Value{{false}},
			}, nil
		},
		execContext: func(query string, _ []driver.NamedValue) (driver.Result, error) {
			if !strings.Contains(query, "DELETE FROM plugin_packages") {
				return nil, fmt.Errorf("unexpected exec: %s", query)
			}

			return driver.RowsAffected(1), nil
		},
	})

	registry := NewPluginRegistry()
	if err := registry.RegisterPlugin(PluginDefinition{
		ID:             "com.example.plugin",
		Name:           "Native Plugin",
		Source:         PluginSourceNative,
		InstallState:   PluginInstallStateReady,
		CreateInstance: func() Plugin { return &noopPlugin{} },
	}); err != nil {
		t.Fatalf("registry.RegisterPlugin() error = %v", err)
	}

	pm := &PluginManager{
		db:       db,
		registry: registry,
		nativePackages: map[string]*InstalledPluginPackage{
			"com.example.plugin": {
				PluginID:     "com.example.plugin",
				Source:       PluginSourceNative,
				InstallState: PluginInstallStateReady,
			},
		},
		loadedNativePlugins: make(map[string]string),
	}

	if err := pm.DeleteInstalledPluginPackage(context.Background(), "com.example.plugin"); err != nil {
		t.Fatalf("DeleteInstalledPluginPackage() error = %v", err)
	}
	if pkg := pm.getNativePackage("com.example.plugin"); pkg != nil {
		t.Fatalf("getNativePackage() = %#v, want nil", pkg)
	}
	if _, ok := pm.getLoadedNativePluginVersion("com.example.plugin"); ok {
		t.Fatal("getLoadedNativePluginVersion() ok = true, want false")
	}
	if _, err := pm.registry.GetPlugin("com.example.plugin"); err == nil {
		t.Fatal("registry.GetPlugin() error = nil, want plugin to be unregistered")
	}
}

func TestDeleteInstalledPluginPackageCleansUpLoadedPlugin(t *testing.T) {
	t.Parallel()

	db := openTestSQLDB(t, &testSQLDriver{
		queryContext: func(query string, _ []driver.NamedValue) (driver.Rows, error) {
			if !strings.Contains(query, "SELECT EXISTS") {
				return nil, fmt.Errorf("unexpected query: %s", query)
			}

			return &testSQLRows{
				columns: []string{"exists"},
				values:  [][]driver.Value{{false}},
			}, nil
		},
		execContext: func(query string, _ []driver.NamedValue) (driver.Result, error) {
			if !strings.Contains(query, "DELETE FROM plugin_packages") {
				return nil, fmt.Errorf("unexpected exec: %s", query)
			}

			return driver.RowsAffected(1), nil
		},
	})

	registry := NewPluginRegistry()
	if err := registry.RegisterPlugin(PluginDefinition{
		ID:             "com.example.plugin",
		Name:           "Native Plugin",
		Source:         PluginSourceNative,
		InstallState:   PluginInstallStateReady,
		CreateInstance: func() Plugin { return &noopPlugin{} },
	}); err != nil {
		t.Fatalf("registry.RegisterPlugin() error = %v", err)
	}

	pm := &PluginManager{
		db:       db,
		registry: registry,
		nativePackages: map[string]*InstalledPluginPackage{
			"com.example.plugin": {
				PluginID:     "com.example.plugin",
				Source:       PluginSourceNative,
				Distribution: PluginDistributionSideload,
				InstallState: PluginInstallStateReady,
				Checksum:     "checksum",
			},
		},
		loadedNativePlugins: map[string]string{
			"com.example.plugin": "1.0.0",
		},
	}

	if err := pm.DeleteInstalledPluginPackage(context.Background(), "com.example.plugin"); err != nil {
		t.Fatalf("DeleteInstalledPluginPackage() error = %v", err)
	}

	if pkg := pm.getNativePackage("com.example.plugin"); pkg != nil {
		t.Fatalf("getNativePackage() = %+v, want nil (fully removed)", pkg)
	}
	if _, ok := pm.getLoadedNativePluginVersion("com.example.plugin"); ok {
		t.Fatal("getLoadedNativePluginVersion() still present, want removed")
	}
	if _, err := pm.registry.GetPlugin("com.example.plugin"); err == nil {
		t.Fatal("registry.GetPlugin() error = nil, want plugin definition to be unregistered")
	}
}

func TestInstallPluginBundleAfterDeletingLoadedPackageIsCleanInstall(t *testing.T) {
	requireLinuxNativePlugins(t)

	runtimeDir := t.TempDir()
	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.Plugins.NativeEnabled = true
		cfg.Plugins.RuntimeDir = runtimeDir
		cfg.Plugins.AllowUnsafeSideload = true
	})

	var insertCalls int
	db := openTestSQLDB(t, &testSQLDriver{
		queryContext: func(query string, _ []driver.NamedValue) (driver.Rows, error) {
			if !strings.Contains(query, "SELECT EXISTS") {
				return nil, fmt.Errorf("unexpected query: %s", query)
			}

			return &testSQLRows{
				columns: []string{"exists"},
				values:  [][]driver.Value{{false}},
			}, nil
		},
		execContext: func(query string, args []driver.NamedValue) (driver.Result, error) {
			switch {
			case strings.Contains(query, "DELETE FROM plugin_packages"):
				return driver.RowsAffected(1), nil
			case strings.Contains(query, "INSERT INTO plugin_packages"):
				insertCalls++
				return driver.RowsAffected(1), nil
			default:
				return nil, fmt.Errorf("unexpected exec: %s", query)
			}
		},
	})

	registry := NewPluginRegistry()
	if err := registry.RegisterPlugin(PluginDefinition{
		ID:             "com.example.plugin",
		Name:           "Native Plugin",
		Source:         PluginSourceNative,
		InstallState:   PluginInstallStateReady,
		CreateInstance: func() Plugin { return &noopPlugin{} },
	}); err != nil {
		t.Fatalf("registry.RegisterPlugin() error = %v", err)
	}

	pm := &PluginManager{
		db:       db,
		registry: registry,
		nativePackages: map[string]*InstalledPluginPackage{
			"com.example.plugin": {
				PluginID:     "com.example.plugin",
				Source:       PluginSourceNative,
				Distribution: PluginDistributionSideload,
				InstallState: PluginInstallStateReady,
				RuntimePath:  "/tmp/com.example.plugin.so",
				Checksum:     "old-checksum",
			},
		},
		loadedNativePlugins: map[string]string{
			"com.example.plugin": "1.0.0",
		},
	}

	if err := pm.DeleteInstalledPluginPackage(context.Background(), "com.example.plugin"); err != nil {
		t.Fatalf("DeleteInstalledPluginPackage() error = %v", err)
	}

	// After delete, the plugin is fully removed — no pending_restart tombstone.
	if pkg := pm.getNativePackage("com.example.plugin"); pkg != nil {
		t.Fatalf("getNativePackage() = %+v after delete, want nil", pkg)
	}

	// Re-install should succeed as a clean install (not pending_restart).
	// Hook the verified loader since there's no real subprocess binary.
	previousLoader := nativePluginVerifiedLoader
	nativePluginVerifiedLoader = func(string, string, PluginPackageManifest, PluginPackageTarget) (PluginDefinition, error) {
		return PluginDefinition{
			ID:             "com.example.plugin",
			Name:           "Native Plugin",
			Source:         PluginSourceNative,
			CreateInstance: func() Plugin { return &noopPlugin{} },
		}, nil
	}
	t.Cleanup(func() { nativePluginVerifiedLoader = previousLoader })

	manifest := testManifest("com.example.plugin")
	archive := buildPluginArchive(t, manifest, primaryManifestLibraryPath(manifest), []byte("replacement-so"), nil, nil)

	pkg, err := pm.installPluginBundle(context.Background(), bytes.NewReader(archive), int64(len(archive)), "replacement.zip", PluginDistributionSideload)
	if err != nil {
		t.Fatalf("installPluginBundle() error = %v", err)
	}
	if got, want := pkg.InstallState, PluginInstallStateReady; got != want {
		t.Fatalf("pkg.InstallState = %q, want %q", got, want)
	}
	if insertCalls != 1 {
		t.Fatalf("insertCalls = %d, want 1", insertCalls)
	}
}

func TestReadPluginBundleRejectsUnsafePath(t *testing.T) {
	t.Parallel()

	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)

	file, err := writer.Create("../plugin.so")
	if err != nil {
		t.Fatalf("writer.Create() error = %v", err)
	}
	if _, err := file.Write([]byte("fake-so-contents")); err != nil {
		t.Fatalf("file.Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	_, _, _, _, _, _, _, err = readPluginBundle(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err == nil {
		t.Fatal("readPluginBundle() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unsafe path") {
		t.Fatalf("readPluginBundle() error = %q, want unsafe path", err)
	}
}

func TestReadPluginBundleRejectsSymlink(t *testing.T) {
	t.Parallel()

	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)

	header := &zip.FileHeader{Name: "bin/plugin.so", Method: zip.Deflate}
	header.SetMode(os.ModeSymlink | 0o777)
	file, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatalf("writer.CreateHeader() error = %v", err)
	}
	if _, err := file.Write([]byte("plugin-real.so")); err != nil {
		t.Fatalf("file.Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	_, _, _, _, _, _, _, err = readPluginBundle(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err == nil {
		t.Fatal("readPluginBundle() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unsupported symlink") {
		t.Fatalf("readPluginBundle() error = %q, want unsupported symlink", err)
	}
}

func TestReadPluginBundleRejectsSignatureWithoutPublicKey(t *testing.T) {
	t.Parallel()

	manifest := testManifest("com.example.signature-only")
	archive := buildPluginArchive(t, manifest, primaryManifestLibraryPath(manifest), []byte("fake-so-contents"), []byte("signature-only"), nil)

	_, _, _, _, _, _, _, err := readPluginBundle(bytes.NewReader(archive), int64(len(archive)))
	if err == nil {
		t.Fatal("readPluginBundle() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "must include manifest.sig and manifest.pub together") {
		t.Fatalf("readPluginBundle() error = %q, want manifest.sig and manifest.pub together", err)
	}
}

func TestReadPluginBundleRejectsOversizedUncompressedLibrary(t *testing.T) {
	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.Plugins.MaxUploadSize = 1024
	})

	manifest := testManifest("com.example.oversized-library")
	archive := buildPluginArchive(t, manifest, primaryManifestLibraryPath(manifest), bytes.Repeat([]byte("a"), 1025), nil, nil)

	_, _, _, _, _, _, _, err := readPluginBundle(bytes.NewReader(archive), int64(len(archive)))
	if err == nil {
		t.Fatal("readPluginBundle() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "exceeds uncompressed size limit") {
		t.Fatalf("readPluginBundle() error = %q, want uncompressed size limit", err)
	}
}

func TestReadPluginBundleRejectsExcessTotalUncompressedSize(t *testing.T) {
	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.Plugins.MaxUploadSize = 700 * 1024
	})

	manifest := testManifest("com.example.total-limit")
	archive := buildPluginArchive(
		t,
		manifest,
		primaryManifestLibraryPath(manifest),
		bytes.Repeat([]byte("a"), 700*1024),
		bytes.Repeat([]byte("b"), 600*1024),
		bytes.Repeat([]byte("c"), 500*1024),
	)

	_, _, _, _, _, _, _, err := readPluginBundle(bytes.NewReader(archive), int64(len(archive)))
	if err == nil {
		t.Fatal("readPluginBundle() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "total uncompressed size limit") {
		t.Fatalf("readPluginBundle() error = %q, want total uncompressed size limit", err)
	}
}

func TestValidatePluginCompatibilityRejectsWrongArchitecture(t *testing.T) {
	t.Parallel()
	requireLinuxNativePlugins(t)

	target := clonePluginPackageTargets(testManifest("com.example.arch").Targets)[0]
	if runtime.GOARCH == "amd64" {
		target.TargetArch = "arm64"
	} else {
		target.TargetArch = "amd64"
	}

	err := validatePluginCompatibility(PluginPackageManifest{}, target)
	if err == nil {
		t.Fatal("validatePluginCompatibility() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "host architecture") {
		t.Fatalf("validatePluginCompatibility() error = %q, want host architecture", err)
	}
}

func TestValidatePluginCompatibilityRejectsNewerHostAPIRequirement(t *testing.T) {
	t.Parallel()
	requireLinuxNativePlugins(t)

	target := clonePluginPackageTargets(testManifest("com.example.api-version").Targets)[0]
	target.MinHostAPIVersion = NativePluginHostAPIVersion + 1

	err := validatePluginCompatibility(PluginPackageManifest{}, target)
	if err == nil {
		t.Fatal("validatePluginCompatibility() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "requires host API version") {
		t.Fatalf("validatePluginCompatibility() error = %q, want host API requirement", err)
	}
}

func TestValidatePluginCompatibilityRejectsMissingCapabilities(t *testing.T) {
	t.Parallel()
	requireLinuxNativePlugins(t)

	target := clonePluginPackageTargets(testManifest("com.example.capabilities").Targets)[0]
	target.RequiredCapabilities = []string{"api.missing"}

	err := validatePluginCompatibility(PluginPackageManifest{}, target)
	if err == nil {
		t.Fatal("validatePluginCompatibility() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unsupported host capabilities") {
		t.Fatalf("validatePluginCompatibility() error = %q, want missing capabilities", err)
	}
}

func TestVerifyManifestSignatureAcceptsCanonicalizedManifest(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey() error = %v", err)
	}

	trustedKey, err := plugin_signing.EncodePublicKeyString(publicKey)
	if err != nil {
		t.Fatalf("EncodePublicKeyString() error = %v", err)
	}
	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.Plugins.TrustedSigningKeys = trustedKey
	})

	manifestRaw := []byte(`{"version":"1.0.0","plugin_id":"com.example.signed","nested":{"b":2,"a":1}}`)
	signatureRaw, publicKeyRaw, err := plugin_signing.SignManifest(manifestRaw, privateKey)
	if err != nil {
		t.Fatalf("plugin_signing.SignManifest() error = %v", err)
	}

	reorderedManifestRaw := []byte(`{"nested":{"a":1,"b":2},"plugin_id":"com.example.signed","version":"1.0.0"}`)
	ok, err := verifyManifestSignature(reorderedManifestRaw, signatureRaw, publicKeyRaw)
	if err != nil {
		t.Fatalf("verifyManifestSignature() error = %v", err)
	}
	if !ok {
		t.Fatal("verifyManifestSignature() = false, want true")
	}
}

func TestVerifyManifestSignatureRejectsUntrustedKey(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey() error = %v", err)
	}
	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.Plugins.TrustedSigningKeys = ""
	})

	manifestRaw := []byte(`{"plugin_id":"com.example.signed"}`)
	signatureRaw, publicKeyRaw, err := plugin_signing.SignManifest(manifestRaw, privateKey)
	if err != nil {
		t.Fatalf("plugin_signing.SignManifest() error = %v", err)
	}

	// Untrusted key should yield (false, nil) — i.e. "not verified" — rather
	// than a hard error. This way the AllowUnsafeSideload gate can apply
	// uniformly to "unsigned" and "signed by wrong key" without inverting
	// the security gradient.
	ok, err := verifyManifestSignature(manifestRaw, signatureRaw, publicKeyRaw)
	if err != nil {
		t.Fatalf("verifyManifestSignature() error = %v, want nil for untrusted-key", err)
	}
	if ok {
		t.Fatal("verifyManifestSignature() = true, want false for untrusted-key")
	}
}

func TestInstallPluginBundleRejectsUnsignedSideloadByDefault(t *testing.T) {
	requireLinuxNativePlugins(t)

	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.Plugins.NativeEnabled = true
		cfg.Plugins.RuntimeDir = t.TempDir()
		cfg.Plugins.AllowUnsafeSideload = false
	})

	manifest := testManifest("com.example.unsigned")
	archive := buildPluginArchive(t, manifest, primaryManifestLibraryPath(manifest), []byte("fake-so-contents"), nil, nil)

	pm := &PluginManager{
		registry:            NewPluginRegistry(),
		plugins:             make(map[uuid.UUID]map[uuid.UUID]*PluginInstance),
		nativePackages:      make(map[string]*InstalledPluginPackage),
		loadedNativePlugins: make(map[string]string),
	}

	_, err := pm.installPluginBundle(context.Background(), bytes.NewReader(archive), int64(len(archive)), "unsigned.zip", PluginDistributionSideload)
	if err == nil {
		t.Fatal("installPluginBundle() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unsigned sideloads are disabled") {
		t.Fatalf("installPluginBundle() error = %q, want unsigned sideload rejection", err)
	}
}

func TestLoadInstalledPluginPackagesDoesNotRequireCreatingRuntimeDir(t *testing.T) {
	runtimeFile, err := os.CreateTemp(t.TempDir(), "plugin-runtime-file-*")
	if err != nil {
		t.Fatalf("os.CreateTemp() error = %v", err)
	}
	runtimePath := runtimeFile.Name() + "/plugins"
	if err := runtimeFile.Close(); err != nil {
		t.Fatalf("runtimeFile.Close() error = %v", err)
	}

	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.Plugins.NativeEnabled = true
		cfg.Plugins.RuntimeDir = runtimePath
	})

	db := openTestSQLDB(t, &testSQLDriver{
		queryContext: func(query string, _ []driver.NamedValue) (driver.Rows, error) {
			return &testSQLRows{}, nil
		},
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
	if len(pm.ListInstalledPluginPackages()) != 0 {
		t.Fatalf("len(ListInstalledPluginPackages()) = %d, want 0", len(pm.ListInstalledPluginPackages()))
	}
}

func TestLoadInstalledPluginPackagesClearsStaleNativeRuntimeState(t *testing.T) {
	t.Parallel()

	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.Plugins.NativeEnabled = true
		cfg.Plugins.RuntimeDir = t.TempDir()
	})

	db := openTestSQLDB(t, &testSQLDriver{
		queryContext: func(query string, _ []driver.NamedValue) (driver.Rows, error) {
			if !strings.Contains(query, "FROM plugin_packages") {
				return nil, fmt.Errorf("unexpected query: %s", query)
			}

			return &testSQLRows{
				columns: []string{
					"plugin_id", "name", "description", "version", "source", "distribution", "official", "install_state",
					"runtime_path", "manifest_json", "signature_verified", "unsafe", "checksum", "min_host_api_version",
					"required_capabilities", "target_os", "target_arch", "last_error", "created_at", "updated_at",
				},
			}, nil
		},
	})

	registry := NewPluginRegistry()
	if err := registry.RegisterPlugin(PluginDefinition{
		ID:             "com.example.plugin",
		Name:           "Native Plugin",
		Source:         PluginSourceNative,
		InstallState:   PluginInstallStateReady,
		CreateInstance: func() Plugin { return &noopPlugin{} },
	}); err != nil {
		t.Fatalf("registry.RegisterPlugin() error = %v", err)
	}

	pm := &PluginManager{
		db:       db,
		registry: registry,
		nativePackages: map[string]*InstalledPluginPackage{
			"com.example.plugin": {
				PluginID:     "com.example.plugin",
				Source:       PluginSourceNative,
				InstallState: PluginInstallStateReady,
			},
		},
		loadedNativePlugins: map[string]string{
			"com.example.plugin": "1.0.0",
		},
	}

	if err := pm.loadInstalledPluginPackages(); err != nil {
		t.Fatalf("loadInstalledPluginPackages() error = %v", err)
	}
	if pkg := pm.getNativePackage("com.example.plugin"); pkg != nil {
		t.Fatalf("getNativePackage() = %#v, want nil", pkg)
	}
	if _, ok := pm.getLoadedNativePluginVersion("com.example.plugin"); ok {
		t.Fatal("getLoadedNativePluginVersion() ok = true, want false")
	}
	if _, err := pm.registry.GetPlugin("com.example.plugin"); err == nil {
		t.Fatal("registry.GetPlugin() error = nil, want stale native runtime to be cleared")
	}
}

func TestInstallPluginBundleRejectsBundledPluginConflict(t *testing.T) {
	requireLinuxNativePlugins(t)

	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.Plugins.NativeEnabled = true
		cfg.Plugins.RuntimeDir = t.TempDir()
		cfg.Plugins.AllowUnsafeSideload = true
	})

	pm := &PluginManager{
		registry:            NewPluginRegistry(),
		plugins:             make(map[uuid.UUID]map[uuid.UUID]*PluginInstance),
		nativePackages:      make(map[string]*InstalledPluginPackage),
		loadedNativePlugins: make(map[string]string),
	}
	if err := pm.registry.RegisterPlugin(PluginDefinition{
		ID:             "com.example.conflict",
		Name:           "Bundled Plugin",
		CreateInstance: func() Plugin { return &noopPlugin{} },
	}); err != nil {
		t.Fatalf("registry.RegisterPlugin() error = %v", err)
	}

	manifest := testManifest("com.example.conflict")
	archive := buildPluginArchive(t, manifest, primaryManifestLibraryPath(manifest), []byte("fake-so-contents"), nil, nil)

	_, err := pm.installPluginBundle(context.Background(), bytes.NewReader(archive), int64(len(archive)), "conflict.zip", PluginDistributionSideload)
	if err == nil {
		t.Fatal("installPluginBundle() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "conflicts with a bundled plugin") {
		t.Fatalf("installPluginBundle() error = %q, want bundled conflict", err)
	}
}

// TestValidatePluginManifestRequiresSHA256 regression-tests that sha256 is
// mandatory on every target. Without this, a signed manifest plus an
// attacker-swapped .so would pass verification.
func TestValidatePluginManifestRequiresSHA256(t *testing.T) {
	t.Parallel()

	manifest := PluginPackageManifest{
		PluginID:    "com.example.missing-sha",
		Name:        "Missing SHA",
		Version:     "1.0.0",
		Targets: []PluginPackageTarget{
			{
				MinHostAPIVersion: NativePluginHostAPIVersion,
				TargetOS:          runtime.GOOS,
				TargetArch:        runtime.GOARCH,
				LibraryPath:       "bin/plugin.so",
				// SHA256 intentionally omitted.
			},
		},
	}

	err := validatePluginManifest(manifest)
	if err == nil {
		t.Fatal("validatePluginManifest() error = nil, want missing-sha256 error")
	}
	if !strings.Contains(err.Error(), "missing sha256") {
		t.Fatalf("validatePluginManifest() error = %q, want missing-sha256", err)
	}
}

// TestValidateRuntimePathWithinRootRejectsEscape regression-tests the H6
// clamp that prevents DB-tampered runtime_path values from turning plugin
// load or delete into arbitrary file access.
func TestValidateRuntimePathWithinRootRejectsEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	cases := []struct {
		name string
		path string
	}{
		{"absolute outside root", "/etc/passwd"},
		{"parent traversal", filepath.Join(root, "..", "outside.so")},
		{"symlink-like trailing .. ", filepath.Join(root, "..")},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := validateRuntimePathWithinRoot(tc.path, root); err == nil {
				t.Fatalf("validateRuntimePathWithinRoot(%q, %q) error = nil, want escape rejection", tc.path, root)
			}
		})
	}

	inside := filepath.Join(root, "com.example", "1.0.0", "plugin.so")
	if _, err := validateRuntimePathWithinRoot(inside, root); err != nil {
		t.Fatalf("validateRuntimePathWithinRoot() unexpected error for in-root path: %v", err)
	}
}

// TestLoadNativePluginPackageRejectsOnDiskChecksumMismatch regression-tests
// H7: if the on-disk library is swapped between install and load, the loader
// must refuse to dlopen it.
func TestLoadNativePluginPackageRejectsOnDiskChecksumMismatch(t *testing.T) {
	requireLinuxNativePlugins(t)

	runtimeDir := t.TempDir()
	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.Plugins.NativeEnabled = true
		cfg.Plugins.RuntimeDir = runtimeDir
	})

	subdir := filepath.Join(runtimeDir, "com.example.tamper", "1.0.0")
	if err := os.MkdirAll(subdir, 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	runtimePath := filepath.Join(subdir, "plugin.so")
	if err := os.WriteFile(runtimePath, []byte("tampered-contents"), 0o640); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	pm := &PluginManager{
		registry:            NewPluginRegistry(),
		plugins:             make(map[uuid.UUID]map[uuid.UUID]*PluginInstance),
		nativePackages:      make(map[string]*InstalledPluginPackage),
		loadedNativePlugins: make(map[string]string),
	}

	// Checksum recorded at install time for DIFFERENT content.
	pkg := &InstalledPluginPackage{
		PluginID:    "com.example.tamper",
		Version:     "1.0.0",
		RuntimePath: runtimePath,
		Checksum:    fmt.Sprintf("%x", sha256.Sum256([]byte("original-contents"))),
	}

	err := pm.loadNativePluginPackage(pkg)
	if err == nil {
		t.Fatal("loadNativePluginPackage() error = nil, want checksum mismatch")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("loadNativePluginPackage() error = %q, want checksum mismatch", err)
	}
}

// TestInstallPluginBundleRejectsZipWithTooManyEntries regression-tests the
// zip-entry count cap against metadata-scanning DoS.
func TestInstallPluginBundleRejectsZipWithTooManyEntries(t *testing.T) {
	requireLinuxNativePlugins(t)

	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.Plugins.NativeEnabled = true
		cfg.Plugins.RuntimeDir = t.TempDir()
		cfg.Plugins.AllowUnsafeSideload = true
	})

	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for i := 0; i <= pluginArchiveMaxEntries; i++ {
		f, err := writer.Create(fmt.Sprintf("noise-%04d.txt", i))
		if err != nil {
			t.Fatalf("writer.Create() error = %v", err)
		}
		if _, err := f.Write([]byte("x")); err != nil {
			t.Fatalf("f.Write() error = %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	_, _, _, _, _, _, _, err := readPluginBundle(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err == nil {
		t.Fatal("readPluginBundle() error = nil, want entry-count rejection")
	}
	if !strings.Contains(err.Error(), "exceeding limit") {
		t.Fatalf("readPluginBundle() error = %q, want entry-count rejection", err)
	}
}

// TestInstallPluginBundleAcceptsTrustedSignedBundle regression-tests the
// trusted-key happy path: a signed bundle with manifest.pub in
// trusted_signing_keys must install with SignatureVerified=true.
func TestInstallPluginBundleAcceptsTrustedSignedBundle(t *testing.T) {
	requireLinuxNativePlugins(t)

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey() error = %v", err)
	}
	trustedKey, err := plugin_signing.EncodePublicKeyString(publicKey)
	if err != nil {
		t.Fatalf("EncodePublicKeyString() error = %v", err)
	}

	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.Plugins.NativeEnabled = true
		cfg.Plugins.RuntimeDir = t.TempDir()
		cfg.Plugins.AllowUnsafeSideload = false
		cfg.Plugins.TrustedSigningKeys = trustedKey
	})

	libraryBytes := []byte("fake-so-contents")
	manifest := testManifest("com.example.signed")
	manifest.Targets[0].SHA256 = fmt.Sprintf("%x", sha256.Sum256(libraryBytes))

	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	signatureRaw, publicKeyRaw, err := plugin_signing.SignManifest(manifestRaw, privateKey)
	if err != nil {
		t.Fatalf("SignManifest() error = %v", err)
	}

	archive := buildPluginArchive(t, manifest, primaryManifestLibraryPath(manifest), libraryBytes, signatureRaw, publicKeyRaw)

	previousLoader := nativePluginVerifiedLoader
	nativePluginVerifiedLoader = func(string, string, PluginPackageManifest, PluginPackageTarget) (PluginDefinition, error) {
		return PluginDefinition{
			ID:             "com.example.signed",
			Name:           manifest.Name,
			Source:         PluginSourceNative,
			CreateInstance: func() Plugin { return &noopPlugin{} },
		}, nil
	}
	t.Cleanup(func() { nativePluginVerifiedLoader = previousLoader })

	pm := &PluginManager{
		registry:            NewPluginRegistry(),
		plugins:             make(map[uuid.UUID]map[uuid.UUID]*PluginInstance),
		nativePackages:      make(map[string]*InstalledPluginPackage),
		loadedNativePlugins: make(map[string]string),
		db:                  openTestSQLDB(t, &testSQLDriver{execContext: func(string, []driver.NamedValue) (driver.Result, error) { return driver.RowsAffected(1), nil }}),
	}

	pkg, err := pm.installPluginBundle(context.Background(), bytes.NewReader(archive), int64(len(archive)), "signed.zip", PluginDistributionSideload)
	if err != nil {
		t.Fatalf("installPluginBundle() error = %v", err)
	}
	if !pkg.SignatureVerified {
		t.Fatal("pkg.SignatureVerified = false, want true after signing with trusted key")
	}
	if pkg.Unsafe {
		t.Fatal("pkg.Unsafe = true, want false after signing with trusted key")
	}
	if pkg.InstallState != PluginInstallStateReady {
		t.Fatalf("pkg.InstallState = %q, want ready", pkg.InstallState)
	}
}
