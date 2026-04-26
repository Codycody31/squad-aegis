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
	"time"

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

	return buildPluginArchiveWithLibraries(t, manifest, map[string][]byte{libraryName: libraryRaw}, nil, signatureRaw, publicKeyRaw)
}

func buildSignedPluginArchive(t *testing.T, manifest PluginPackageManifest, libraryName string, libraryRaw []byte, signedPayload []byte, signatureRaw []byte, publicKeyRaw []byte) []byte {
	t.Helper()

	return buildPluginArchiveWithLibraries(t, manifest, map[string][]byte{libraryName: libraryRaw}, signedPayload, signatureRaw, publicKeyRaw)
}

func buildPluginArchiveWithLibraries(t *testing.T, manifest PluginPackageManifest, libraries map[string][]byte, signedPayloadRaw []byte, signatureRaw []byte, publicKeyRaw []byte) []byte {
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

	if len(signedPayloadRaw) > 0 {
		signedFile, err := writer.Create(pluginSignedManifestFile)
		if err != nil {
			t.Fatalf("writer.Create(signed payload) error = %v", err)
		}
		if _, err := signedFile.Write(signedPayloadRaw); err != nil {
			t.Fatalf("signedFile.Write() error = %v", err)
		}
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

func TestMaskAndEnrichPluginInstanceConcurrentStatusSnapshot(t *testing.T) {
	t.Parallel()

	pm := &PluginManager{
		registry:       NewPluginRegistry(),
		nativePackages: make(map[string]*InstalledPluginPackage),
	}
	instance := &PluginInstance{
		ID:       uuid.New(),
		ServerID: uuid.New(),
		PluginID: "com.example.concurrent",
		Config: map[string]interface{}{
			"secret": "value",
		},
		Status:  PluginStatusRunning,
		Enabled: true,
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				instance.setError(PluginStatusError, "subprocess exited")
				instance.clearError(PluginStatusRunning)
			}
		}
	}()

	for i := 0; i < 1000; i++ {
		if masked := pm.maskAndEnrichPluginInstance(instance); masked == nil {
			t.Fatal("maskAndEnrichPluginInstance() = nil")
		}
	}
	close(stop)
	<-done
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

	parts, err := readPluginBundle(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err != nil {
		t.Fatalf("readPluginBundle() error = %v", err)
	}

	if got, want := parts.Manifest.PluginID, "com.example.bundle"; got != want {
		t.Fatalf("manifest.PluginID = %q, want %q", got, want)
	}
	if got, want := parts.LibraryName, "bin/plugin.so"; got != want {
		t.Fatalf("libraryName = %q, want %q", got, want)
	}
	if got, want := string(parts.LibraryBytes), "fake-so-contents"; got != want {
		t.Fatalf("libraryRaw = %q, want %q", got, want)
	}
	if got, want := parts.SelectedTarget.TargetOS, runtime.GOOS; got != want {
		t.Fatalf("selectedTarget.TargetOS = %q, want %q", got, want)
	}
	if len(parts.ManifestBytes) == 0 {
		t.Fatal("manifestBytes should not be empty")
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
	}, nil, nil, nil)

	parts, err := readPluginBundle(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("readPluginBundle() error = %v", err)
	}

	if got, want := parts.LibraryName, "bin/host-plugin.so"; got != want {
		t.Fatalf("libraryName = %q, want %q", got, want)
	}
	if got, want := string(parts.LibraryBytes), "host-so"; got != want {
		t.Fatalf("libraryRaw = %q, want %q", got, want)
	}
	if got, want := parts.SelectedTarget.LibraryPath, "bin/host-plugin.so"; got != want {
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

func TestInstallPluginBundleMarksLiveUpdatePendingRestart(t *testing.T) {
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
			if !strings.Contains(query, "SELECT EXISTS") {
				return nil, fmt.Errorf("unexpected query: %s", query)
			}
			if len(args) != 1 {
				t.Fatalf("len(args) = %d, want 1", len(args))
			}
			if got, want := fmt.Sprint(args[0].Value), "com.example.plugin"; got != want {
				t.Fatalf("args[0] = %q, want %q", got, want)
			}

			return &testSQLRows{
				columns: []string{"exists"},
				values:  [][]driver.Value{{false}},
			}, nil
		},
		execContext: func(query string, args []driver.NamedValue) (driver.Result, error) {
			if !strings.Contains(query, "INSERT INTO plugin_packages") {
				return nil, fmt.Errorf("unexpected exec: %s", query)
			}
			saveCalls++
			if got, want := fmt.Sprint(args[7].Value), string(PluginInstallStatePendingRestart); got != want {
				t.Fatalf("persisted install state = %q, want %q", got, want)
			}
			if got, want := fmt.Sprint(args[16].Value), nativePluginPendingRestartMessage; got != want {
				t.Fatalf("persisted last error = %q, want %q", got, want)
			}
			return driver.RowsAffected(1), nil
		},
	})

	registry := NewPluginRegistry()
	oldRuntimePath := filepath.Join(runtimeDir, "com.example.plugin", "1.0.0", "plugin.so")
	if err := registry.RegisterPlugin(PluginDefinition{
		ID:             "com.example.plugin",
		Name:           "Native Plugin",
		Version:        "1.0.0",
		Source:         PluginSourceNative,
		RuntimePath:    oldRuntimePath,
		CreateInstance: func() Plugin { return &noopPlugin{} },
	}); err != nil {
		t.Fatalf("registry.RegisterPlugin() error = %v", err)
	}

	serverID := uuid.New()
	instanceID := uuid.New()
	pm := &PluginManager{
		db:       db,
		registry: registry,
		plugins: map[uuid.UUID]map[uuid.UUID]*PluginInstance{
			serverID: {
				instanceID: {
					ID:       instanceID,
					ServerID: serverID,
					PluginID: "com.example.plugin",
					Plugin:   &noopPlugin{},
				},
			},
		},
		nativePackages: map[string]*InstalledPluginPackage{
			"com.example.plugin": {
				PluginID:     "com.example.plugin",
				Name:         "Native Plugin",
				Version:      "1.0.0",
				Source:       PluginSourceNative,
				Distribution: PluginDistributionSideload,
				InstallState: PluginInstallStateReady,
				RuntimePath:  oldRuntimePath,
			},
		},
		loadedNativePlugins: map[string]string{
			"com.example.plugin": "1.0.0",
		},
	}

	var loaderCalls int
	previousLoader := nativePluginVerifiedLoader
	nativePluginVerifiedLoader = func(string, string, PluginPackageManifest, PluginPackageTarget) (PluginDefinition, error) {
		loaderCalls++
		return PluginDefinition{
			ID:             "com.example.plugin",
			Name:           "Native Plugin",
			Version:        "2.0.0",
			Source:         PluginSourceNative,
			CreateInstance: func() Plugin { return &noopPlugin{} },
		}, nil
	}
	t.Cleanup(func() { nativePluginVerifiedLoader = previousLoader })

	manifest := testManifest("com.example.plugin")
	manifest.Version = "2.0.0"
	archive := buildPluginArchive(t, manifest, primaryManifestLibraryPath(manifest), []byte("replacement-so"), nil, nil)

	pkg, err := pm.installPluginBundle(context.Background(), bytes.NewReader(archive), int64(len(archive)), "replacement.zip", PluginDistributionSideload)
	if err != nil {
		t.Fatalf("installPluginBundle() error = %v", err)
	}
	if got, want := pkg.InstallState, PluginInstallStatePendingRestart; got != want {
		t.Fatalf("pkg.InstallState = %q, want %q", got, want)
	}
	if got, want := pkg.LastError, nativePluginPendingRestartMessage; got != want {
		t.Fatalf("pkg.LastError = %q, want %q", got, want)
	}
	if loaderCalls != 0 {
		t.Fatalf("nativePluginVerifiedLoader() calls = %d, want 0", loaderCalls)
	}
	if saveCalls != 1 {
		t.Fatalf("saveCalls = %d, want 1", saveCalls)
	}

	loadedVersion, ok := pm.getLoadedNativePluginVersion("com.example.plugin")
	if !ok {
		t.Fatal("getLoadedNativePluginVersion() ok = false, want true")
	}
	if got, want := loadedVersion, "1.0.0"; got != want {
		t.Fatalf("loaded version = %q, want %q", got, want)
	}

	definition, err := pm.registry.GetPlugin("com.example.plugin")
	if err != nil {
		t.Fatalf("registry.GetPlugin() error = %v", err)
	}
	if got, want := definition.Version, "1.0.0"; got != want {
		t.Fatalf("definition.Version = %q, want %q", got, want)
	}
	if got, want := definition.RuntimePath, oldRuntimePath; got != want {
		t.Fatalf("definition.RuntimePath = %q, want %q", got, want)
	}

	persisted := pm.getNativePackage("com.example.plugin")
	if persisted == nil {
		t.Fatal("getNativePackage() returned nil")
	}
	if got, want := persisted.Version, "2.0.0"; got != want {
		t.Fatalf("persisted.Version = %q, want %q", got, want)
	}
	if got, want := persisted.InstallState, PluginInstallStatePendingRestart; got != want {
		t.Fatalf("persisted.InstallState = %q, want %q", got, want)
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

	_, err = readPluginBundle(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
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

	_, err = readPluginBundle(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
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

	_, err := readPluginBundle(bytes.NewReader(archive), int64(len(archive)))
	if err == nil {
		t.Fatal("readPluginBundle() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "must include") || !strings.Contains(err.Error(), "manifest.sig") {
		t.Fatalf("readPluginBundle() error = %q, want manifest.signed.json/manifest.sig/manifest.pub together rejection", err)
	}
}

func TestReadPluginBundleRejectsOversizedUncompressedLibrary(t *testing.T) {
	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.Plugins.MaxUploadSize = 1024
	})

	manifest := testManifest("com.example.oversized-library")
	archive := buildPluginArchive(t, manifest, primaryManifestLibraryPath(manifest), bytes.Repeat([]byte("a"), 1025), nil, nil)

	_, err := readPluginBundle(bytes.NewReader(archive), int64(len(archive)))
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
	archive := buildSignedPluginArchive(
		t,
		manifest,
		primaryManifestLibraryPath(manifest),
		bytes.Repeat([]byte("a"), 700*1024),
		bytes.Repeat([]byte("d"), 700*1024),
		bytes.Repeat([]byte("b"), 600*1024),
		bytes.Repeat([]byte("c"), 500*1024),
	)

	_, err := readPluginBundle(bytes.NewReader(archive), int64(len(archive)))
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
	resetRevokedKeyIDsForTest()

	manifestRaw := []byte(`{"version":"1.0.0","plugin_id":"com.example.signed","nested":{"b":2,"a":1}}`)
	signedAt := time.Now().UTC()
	signedPayload, err := plugin_signing.BuildSignedPayload(manifestRaw, "ops-key", signedAt, signedAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("BuildSignedPayload() error = %v", err)
	}
	signatureRaw, publicKeyRaw, err := plugin_signing.SignSignedPayload(signedPayload, privateKey)
	if err != nil {
		t.Fatalf("SignSignedPayload() error = %v", err)
	}

	reorderedManifestRaw := []byte(`{"nested":{"a":1,"b":2},"plugin_id":"com.example.signed","version":"1.0.0"}`)
	verification, err := verifyManifestSignature(signedPayload, reorderedManifestRaw, signatureRaw, publicKeyRaw)
	if err != nil {
		t.Fatalf("verifyManifestSignature() error = %v", err)
	}
	if !verification.Verified {
		t.Fatal("verifyManifestSignature() = false, want true")
	}
	if verification.Payload.KeyID != "ops-key" {
		t.Fatalf("verification.Payload.KeyID = %q, want %q", verification.Payload.KeyID, "ops-key")
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
	signedAt := time.Now().UTC()
	signedPayload, err := plugin_signing.BuildSignedPayload(manifestRaw, "ops-key", signedAt, signedAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("BuildSignedPayload() error = %v", err)
	}
	signatureRaw, publicKeyRaw, err := plugin_signing.SignSignedPayload(signedPayload, privateKey)
	if err != nil {
		t.Fatalf("SignSignedPayload() error = %v", err)
	}

	// Untrusted key should yield verified=false with no error — i.e. "not
	// verified" — rather than a hard error. This way the AllowUnsafeSideload
	// gate can apply uniformly to "unsigned" and "signed by wrong key"
	// without inverting the security gradient.
	verification, err := verifyManifestSignature(signedPayload, manifestRaw, signatureRaw, publicKeyRaw)
	if err != nil {
		t.Fatalf("verifyManifestSignature() error = %v, want nil for untrusted-key", err)
	}
	if verification.Verified {
		t.Fatal("verifyManifestSignature() verified = true, want false for untrusted-key")
	}
}

func TestVerifyManifestSignatureRejectsExpiredSignature(t *testing.T) {
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
	resetRevokedKeyIDsForTest()

	manifestRaw := []byte(`{"plugin_id":"com.example.expired","version":"1.0.0"}`)
	signedAt := time.Now().UTC().Add(-48 * time.Hour)
	signedPayload, err := plugin_signing.BuildSignedPayload(manifestRaw, "ops-key", signedAt, signedAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("BuildSignedPayload() error = %v", err)
	}
	signatureRaw, publicKeyRaw, err := plugin_signing.SignSignedPayload(signedPayload, privateKey)
	if err != nil {
		t.Fatalf("SignSignedPayload() error = %v", err)
	}

	verification, err := verifyManifestSignature(signedPayload, manifestRaw, signatureRaw, publicKeyRaw)
	if err != nil {
		t.Fatalf("verifyManifestSignature() error = %v", err)
	}
	if verification.Verified {
		t.Fatal("verifyManifestSignature() verified = true, want false for expired signature")
	}
}

func TestVerifyManifestSignatureRejectsRevokedKeyID(t *testing.T) {
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
	resetRevokedKeyIDsForTest("ops-key-leaked")
	t.Cleanup(func() { resetRevokedKeyIDsForTest() })

	manifestRaw := []byte(`{"plugin_id":"com.example.revoked","version":"1.0.0"}`)
	signedAt := time.Now().UTC()
	signedPayload, err := plugin_signing.BuildSignedPayload(manifestRaw, "ops-key-leaked", signedAt, signedAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("BuildSignedPayload() error = %v", err)
	}
	signatureRaw, publicKeyRaw, err := plugin_signing.SignSignedPayload(signedPayload, privateKey)
	if err != nil {
		t.Fatalf("SignSignedPayload() error = %v", err)
	}

	verification, err := verifyManifestSignature(signedPayload, manifestRaw, signatureRaw, publicKeyRaw)
	if err != nil {
		t.Fatalf("verifyManifestSignature() error = %v", err)
	}
	if verification.Verified {
		t.Fatal("verifyManifestSignature() verified = true, want false for revoked key_id")
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
					"runtime_path", "manifest_json", "signature_verified", "unsafe", "min_host_api_version",
					"required_capabilities", "target_os", "target_arch", "last_error", "created_at", "updated_at",
					"manifest_signature", "manifest_public_key", "signed_manifest_json", "signature_key_id",
					"signature_signed_at", "signature_expires_at",
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
		PluginID: "com.example.missing-sha",
		Name:     "Missing SHA",
		Version:  "1.0.0",
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
// must refuse to dlopen it. The expected SHA is now derived from the
// manifest target (which the install pipeline anchors to the verified
// library bytes), so this test stages a manifest target SHA that no longer
// matches the on-disk file.
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

	manifest := testManifest("com.example.tamper")
	manifest.Targets[0].SHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte("original-contents")))
	pkg := &InstalledPluginPackage{
		PluginID:    "com.example.tamper",
		Version:     "1.0.0",
		RuntimePath: runtimePath,
		Manifest:    manifest,
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

	_, err := readPluginBundle(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err == nil {
		t.Fatal("readPluginBundle() error = nil, want entry-count rejection")
	}
	if !strings.Contains(err.Error(), "exceeding limit") {
		t.Fatalf("readPluginBundle() error = %q, want entry-count rejection", err)
	}
}

// TestInstallPluginBundleAcceptsTrustedSignedBundle regression-tests the
// trusted-key happy path: a signed bundle with manifest.pub in
// trusted_signing_keys must install with SignatureVerified=true and
// preserve the signed-payload metadata (key_id, signed_at, expires_at).
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
	resetRevokedKeyIDsForTest()

	libraryBytes := []byte("fake-so-contents")
	manifest := testManifest("com.example.signed")
	manifest.Targets[0].SHA256 = fmt.Sprintf("%x", sha256.Sum256(libraryBytes))

	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	signedAt := time.Now().UTC()
	expiresAt := signedAt.Add(24 * time.Hour)
	signedPayload, err := plugin_signing.BuildSignedPayload(manifestRaw, "ops-key", signedAt, expiresAt)
	if err != nil {
		t.Fatalf("BuildSignedPayload() error = %v", err)
	}
	signatureRaw, publicKeyRaw, err := plugin_signing.SignSignedPayload(signedPayload, privateKey)
	if err != nil {
		t.Fatalf("SignSignedPayload() error = %v", err)
	}

	archive := buildSignedPluginArchive(t, manifest, primaryManifestLibraryPath(manifest), libraryBytes, signedPayload, signatureRaw, publicKeyRaw)

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
		db: openTestSQLDB(t, &testSQLDriver{
			queryContext: func(query string, args []driver.NamedValue) (driver.Rows, error) {
				if !strings.Contains(query, "SELECT EXISTS") {
					return nil, fmt.Errorf("unexpected query: %s", query)
				}
				return &testSQLRows{
					columns: []string{"exists"},
					values:  [][]driver.Value{{false}},
				}, nil
			},
			execContext: func(string, []driver.NamedValue) (driver.Result, error) {
				return driver.RowsAffected(1), nil
			},
		}),
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
	if pkg.SignatureKeyID != "ops-key" {
		t.Fatalf("pkg.SignatureKeyID = %q, want %q", pkg.SignatureKeyID, "ops-key")
	}
	if !pkg.SignatureExpiresAt.Equal(expiresAt) {
		t.Fatalf("pkg.SignatureExpiresAt = %v, want %v", pkg.SignatureExpiresAt, expiresAt)
	}
}

// TestInstallPluginBundleRejectsExpiredSignature regression-tests that a
// signed bundle whose signature has expired is rejected at install time
// when AllowUnsafeSideload is off.
func TestInstallPluginBundleRejectsExpiredSignature(t *testing.T) {
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
	resetRevokedKeyIDsForTest()

	libraryBytes := []byte("fake-so-contents")
	manifest := testManifest("com.example.expired")
	manifest.Targets[0].SHA256 = fmt.Sprintf("%x", sha256.Sum256(libraryBytes))

	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	signedAt := time.Now().UTC().Add(-48 * time.Hour)
	signedPayload, err := plugin_signing.BuildSignedPayload(manifestRaw, "ops-key", signedAt, signedAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("BuildSignedPayload() error = %v", err)
	}
	signatureRaw, publicKeyRaw, err := plugin_signing.SignSignedPayload(signedPayload, privateKey)
	if err != nil {
		t.Fatalf("SignSignedPayload() error = %v", err)
	}

	archive := buildSignedPluginArchive(t, manifest, primaryManifestLibraryPath(manifest), libraryBytes, signedPayload, signatureRaw, publicKeyRaw)

	pm := &PluginManager{
		registry:            NewPluginRegistry(),
		plugins:             make(map[uuid.UUID]map[uuid.UUID]*PluginInstance),
		nativePackages:      make(map[string]*InstalledPluginPackage),
		loadedNativePlugins: make(map[string]string),
	}

	_, err = pm.installPluginBundle(context.Background(), bytes.NewReader(archive), int64(len(archive)), "expired.zip", PluginDistributionSideload)
	if err == nil {
		t.Fatal("installPluginBundle() error = nil, want expired-signature rejection")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Fatalf("installPluginBundle() error = %q, want expiration rejection", err)
	}
}

// TestInstallPluginBundleRejectsRevokedKeyID regression-tests that a
// signed bundle whose key_id is in the CRL is rejected at install time
// when AllowUnsafeSideload is off.
func TestInstallPluginBundleRejectsRevokedKeyID(t *testing.T) {
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
	resetRevokedKeyIDsForTest("ops-key-leaked")
	t.Cleanup(func() { resetRevokedKeyIDsForTest() })

	libraryBytes := []byte("fake-so-contents")
	manifest := testManifest("com.example.revoked")
	manifest.Targets[0].SHA256 = fmt.Sprintf("%x", sha256.Sum256(libraryBytes))

	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	signedAt := time.Now().UTC()
	signedPayload, err := plugin_signing.BuildSignedPayload(manifestRaw, "ops-key-leaked", signedAt, signedAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("BuildSignedPayload() error = %v", err)
	}
	signatureRaw, publicKeyRaw, err := plugin_signing.SignSignedPayload(signedPayload, privateKey)
	if err != nil {
		t.Fatalf("SignSignedPayload() error = %v", err)
	}

	archive := buildSignedPluginArchive(t, manifest, primaryManifestLibraryPath(manifest), libraryBytes, signedPayload, signatureRaw, publicKeyRaw)

	pm := &PluginManager{
		registry:            NewPluginRegistry(),
		plugins:             make(map[uuid.UUID]map[uuid.UUID]*PluginInstance),
		nativePackages:      make(map[string]*InstalledPluginPackage),
		loadedNativePlugins: make(map[string]string),
	}

	_, err = pm.installPluginBundle(context.Background(), bytes.NewReader(archive), int64(len(archive)), "revoked.zip", PluginDistributionSideload)
	if err == nil {
		t.Fatal("installPluginBundle() error = nil, want revoked-key rejection")
	}
	if !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("installPluginBundle() error = %q, want revoked-key rejection", err)
	}
}

// TestLoadInstalledPluginPackagesQuarantinesRevokedKeyOnBoot regression-tests
// that a previously-trusted package whose key_id is now in the CRL is
// quarantined at boot rather than silently re-loaded.
func TestLoadInstalledPluginPackagesQuarantinesRevokedKeyOnBoot(t *testing.T) {
	requireLinuxNativePlugins(t)

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey() error = %v", err)
	}
	trustedKey, err := plugin_signing.EncodePublicKeyString(publicKey)
	if err != nil {
		t.Fatalf("EncodePublicKeyString() error = %v", err)
	}

	runtimeDir := t.TempDir()
	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.Plugins.NativeEnabled = true
		cfg.Plugins.RuntimeDir = runtimeDir
		cfg.Plugins.TrustedSigningKeys = trustedKey
	})
	resetRevokedKeyIDsForTest("ops-key-leaked")
	t.Cleanup(func() { resetRevokedKeyIDsForTest() })

	pluginSubdir := filepath.Join(runtimeDir, "com.example.crl", "1.0.0")
	if err := os.MkdirAll(pluginSubdir, 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	runtimePath := filepath.Join(pluginSubdir, "com.example.crl.so")
	libraryBytes := []byte("crl-runtime")
	if err := os.WriteFile(runtimePath, libraryBytes, 0o640); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	manifest := testManifest("com.example.crl")
	manifest.Targets[0].SHA256 = fmt.Sprintf("%x", sha256.Sum256(libraryBytes))
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	signedAt := time.Now().UTC()
	signedPayload, err := plugin_signing.BuildSignedPayload(manifestRaw, "ops-key-leaked", signedAt, signedAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("BuildSignedPayload() error = %v", err)
	}
	signatureRaw, publicKeyRaw, err := plugin_signing.SignSignedPayload(signedPayload, privateKey)
	if err != nil {
		t.Fatalf("SignSignedPayload() error = %v", err)
	}

	now := time.Now().UTC()
	var lastSavedState string
	var lastSavedError string

	db := openTestSQLDB(t, &testSQLDriver{
		queryContext: func(query string, _ []driver.NamedValue) (driver.Rows, error) {
			if !strings.Contains(query, "FROM plugin_packages") {
				return nil, fmt.Errorf("unexpected query: %s", query)
			}
			return &testSQLRows{
				columns: []string{
					"plugin_id", "name", "description", "version", "source", "distribution", "official", "install_state",
					"runtime_path", "manifest_json", "signature_verified", "unsafe", "min_host_api_version",
					"required_capabilities", "target_os", "target_arch", "last_error", "created_at", "updated_at",
					"manifest_signature", "manifest_public_key", "signed_manifest_json", "signature_key_id",
					"signature_signed_at", "signature_expires_at",
				},
				values: [][]driver.Value{{
					"com.example.crl",
					manifest.Name,
					manifest.Description,
					manifest.Version,
					string(PluginSourceNative),
					string(PluginDistributionSideload),
					false,
					string(PluginInstallStateReady),
					runtimePath,
					manifestRaw,
					true,
					false,
					int64(NativePluginHostAPIVersion),
					[]byte("[]"),
					manifest.Targets[0].TargetOS,
					manifest.Targets[0].TargetArch,
					"",
					now,
					now,
					string(signatureRaw),
					string(publicKeyRaw),
					string(signedPayload),
					"ops-key-leaked",
					signedAt,
					signedAt.Add(time.Hour),
				}},
			}, nil
		},
		execContext: func(query string, args []driver.NamedValue) (driver.Result, error) {
			if !strings.Contains(query, "INSERT INTO plugin_packages") {
				return nil, fmt.Errorf("unexpected exec: %s", query)
			}
			lastSavedState = fmt.Sprint(args[7].Value)
			lastSavedError = fmt.Sprint(args[16].Value)
			return driver.RowsAffected(1), nil
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

	pkg := pm.getNativePackage("com.example.crl")
	if pkg == nil {
		t.Fatal("getNativePackage() = nil, want quarantined package")
	}
	if pkg.InstallState != PluginInstallStateError {
		t.Fatalf("pkg.InstallState = %q, want %q", pkg.InstallState, PluginInstallStateError)
	}
	if pkg.SignatureVerified {
		t.Fatal("pkg.SignatureVerified = true, want false after CRL revocation")
	}
	if !strings.Contains(pkg.LastError, "revoked") {
		t.Fatalf("pkg.LastError = %q, want revoked", pkg.LastError)
	}
	if lastSavedState != string(PluginInstallStateError) {
		t.Fatalf("persisted install state = %q, want %q", lastSavedState, PluginInstallStateError)
	}
	if !strings.Contains(lastSavedError, "revoked") {
		t.Fatalf("persisted last error = %q, want revoked", lastSavedError)
	}
}
