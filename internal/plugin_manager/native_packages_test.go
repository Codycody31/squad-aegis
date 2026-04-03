package plugin_manager

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql/driver"
	"encoding/json"
	"os"
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

	t.Cleanup(func() {
		config.Config = prev
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
		EntrySymbol: nativePluginEntrySymbol,
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
				InstallState:         PluginInstallStatePendingRestart,
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
	if got, want := definition.InstallState, PluginInstallStatePendingRestart; got != want {
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
		EntrySymbol: nativePluginEntrySymbol,
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
		EntrySymbol: nativePluginEntrySymbol,
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
		EntrySymbol: nativePluginEntrySymbol,
		Targets: []PluginPackageTarget{
			{
				MinHostAPIVersion: NativePluginHostAPIVersion,
				TargetOS:          runtime.GOOS,
				TargetArch:        runtime.GOARCH,
				LibraryPath:       "bin/plugin-a.so",
			},
			{
				MinHostAPIVersion: NativePluginHostAPIVersion,
				TargetOS:          runtime.GOOS,
				TargetArch:        runtime.GOARCH,
				LibraryPath:       "bin/plugin-b.so",
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

func TestPluginRuntimeDirUsesLocalDefaultOutsideContainer(t *testing.T) {
	setPluginTestConfig(t, func(cfg *config.Struct) {
		cfg.App.InContainer = false
		cfg.Plugins.RuntimeDir = ""
	})

	if got, want := pluginRuntimeDir(), "plugins"; got != want {
		t.Fatalf("pluginRuntimeDir() = %q, want %q", got, want)
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
				InstallState: PluginInstallStatePendingRestart,
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

	err := validatePluginCompatibility(target)
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

	err := validatePluginCompatibility(target)
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

	err := validatePluginCompatibility(target)
	if err == nil {
		t.Fatal("validatePluginCompatibility() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unsupported host capabilities") {
		t.Fatalf("validatePluginCompatibility() error = %q, want missing capabilities", err)
	}
}

func TestVerifyManifestSignatureAcceptsCanonicalizedManifest(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey() error = %v", err)
	}

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
