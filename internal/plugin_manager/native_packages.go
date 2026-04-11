package plugin_manager

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/plugin_signing"
	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

const (
	nativePluginEntrySymbol     = "GetAegisPlugin"
	pluginManifestFile          = "manifest.json"
	pluginSignatureFile         = plugin_signing.ManifestSignatureFile
	pluginPublicKeyFile         = plugin_signing.ManifestPublicKeyFile
	defaultPluginMaxUploadSize  = 50 * 1024 * 1024
	pluginArchiveMetadataBudget = 1 * 1024 * 1024
	// pluginArchiveMaxEntries bounds the central directory entry count to
	// prevent metadata-scanning DoS via zip files with millions of tiny
	// headers. A legitimate plugin bundle has at most a handful of files.
	pluginArchiveMaxEntries = 256
)

var nativePluginDefinitionLoader = loadNativePluginDefinition

func nativePluginsEnabled() bool {
	return config.Config != nil && config.Config.Plugins.NativeEnabled
}

func pluginRuntimeDir() string {
	if config.Config == nil {
		return "plugins"
	}

	if runtimeDir := strings.TrimSpace(config.Config.Plugins.RuntimeDir); runtimeDir != "" {
		return runtimeDir
	}

	if config.Config.App.InContainer {
		return "/etc/squad-aegis/plugins"
	}

	return "plugins"
}

func connectorRuntimeDir() string {
	if config.Config == nil {
		return "connectors"
	}

	if runtimeDir := strings.TrimSpace(config.Config.Plugins.ConnectorRuntimeDir); runtimeDir != "" {
		return runtimeDir
	}

	if config.Config.App.InContainer {
		return "/etc/squad-aegis/connectors"
	}

	return "connectors"
}

// writeRuntimeLibrary unlinks any existing file then delegates to the
// per-platform writer which uses O_EXCL|O_NOFOLLOW on Unix to defeat
// symlink-clobber attacks.
func writeRuntimeLibrary(runtimePath string, libraryBytes []byte) error {
	if err := os.Remove(runtimePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear existing runtime library at %s: %w", runtimePath, err)
	}

	if err := writeRuntimeLibraryPlatform(runtimePath, libraryBytes); err != nil {
		_ = os.Remove(runtimePath)
		return err
	}
	return nil
}

func hashFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// validateRuntimePathWithinRoot rejects a runtime_path that does not resolve
// inside root, guarding against DB tampering that would otherwise make the
// loader open arbitrary files on the host.
func validateRuntimePathWithinRoot(runtimePath, root string) (string, error) {
	if strings.TrimSpace(runtimePath) == "" {
		return "", fmt.Errorf("runtime path is empty")
	}
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("runtime root is empty")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("failed to resolve runtime root: %w", err)
	}
	absRoot = filepath.Clean(absRoot)

	absPath, err := filepath.Abs(runtimePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve runtime path: %w", err)
	}
	absPath = filepath.Clean(absPath)

	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return "", fmt.Errorf("runtime path %s is not under %s", absPath, absRoot)
	}
	if rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("runtime path %s escapes runtime root %s", absPath, absRoot)
	}

	return absPath, nil
}

func allowUnsafeSideload() bool {
	return config.Config != nil && config.Config.Plugins.AllowUnsafeSideload
}

func pluginMaxUploadSize() int64 {
	if config.Config == nil || config.Config.Plugins.MaxUploadSize <= 0 {
		return defaultPluginMaxUploadSize
	}

	return config.Config.Plugins.MaxUploadSize
}

func pluginMaxArchiveUncompressedSize() int64 {
	maxUploadSize := pluginMaxUploadSize()
	if maxUploadSize > math.MaxInt64-pluginArchiveMetadataBudget {
		return math.MaxInt64
	}

	return maxUploadSize + pluginArchiveMetadataBudget
}

func sanitizeRuntimeSegment(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "..", "")
	value = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-', r == '_', r == '.':
			return r
		default:
			return '_'
		}
	}, value)

	return strings.Trim(value, "._")
}

func sanitizeVersionSegment(value string) string {
	return sanitizeRuntimeSegment(strings.TrimPrefix(strings.TrimSpace(value), "v"))
}

func validateRuntimeStorageSegment(fieldName, value string, sanitize func(string) string) (string, error) {
	trimmed := strings.TrimSpace(value)
	sanitized := sanitize(trimmed)

	if sanitized == "" {
		return "", fmt.Errorf("plugin manifest %s %q collapses to an empty runtime path segment", fieldName, value)
	}
	if sanitized != trimmed {
		return "", fmt.Errorf("plugin manifest %s %q is ambiguous after runtime sanitization; stored path segment would be %q", fieldName, value, sanitized)
	}

	return sanitized, nil
}

func pluginRuntimeSegments(pluginID, version string) (string, string, error) {
	pluginSegment, err := validateRuntimeStorageSegment("plugin_id", pluginID, sanitizeRuntimeSegment)
	if err != nil {
		return "", "", err
	}

	versionSegment, err := validateRuntimeStorageSegment("version", version, sanitizeVersionSegment)
	if err != nil {
		return "", "", err
	}

	return pluginSegment, versionSegment, nil
}

func cloneInstalledPluginPackage(pkg *InstalledPluginPackage) *InstalledPluginPackage {
	if pkg == nil {
		return nil
	}

	cloned := *pkg
	cloned.Manifest.Targets = clonePluginPackageTargets(pkg.Manifest.Targets)
	cloned.RequiredCapabilities = cloneRequiredCapabilities(pkg.RequiredCapabilities)
	if len(pkg.ManifestJSON) > 0 {
		cloned.ManifestJSON = append(json.RawMessage(nil), pkg.ManifestJSON...)
	}
	if len(pkg.ManifestSignature) > 0 {
		cloned.ManifestSignature = append([]byte(nil), pkg.ManifestSignature...)
	}
	if len(pkg.ManifestPublicKey) > 0 {
		cloned.ManifestPublicKey = append([]byte(nil), pkg.ManifestPublicKey...)
	}

	return &cloned
}

func applyInstalledPackageTarget(pkg *InstalledPluginPackage, target PluginPackageTarget) {
	pkg.MinHostAPIVersion = target.MinHostAPIVersion
	pkg.RequiredCapabilities = cloneRequiredCapabilities(target.RequiredCapabilities)
	pkg.TargetOS = target.TargetOS
	pkg.TargetArch = target.TargetArch
}

func mustMarshalRequiredCapabilities(capabilities []string) []byte {
	encoded, err := json.Marshal(capabilities)
	if err != nil {
		return []byte("[]")
	}
	return encoded
}

func (pm *PluginManager) getNativePackage(pluginID string) *InstalledPluginPackage {
	pm.nativeMu.RLock()
	defer pm.nativeMu.RUnlock()

	return cloneInstalledPluginPackage(pm.nativePackages[pluginID])
}

func (pm *PluginManager) getLoadedNativePluginVersion(pluginID string) (string, bool) {
	pm.nativeMu.RLock()
	defer pm.nativeMu.RUnlock()

	version, ok := pm.loadedNativePlugins[pluginID]
	return version, ok
}

func (pm *PluginManager) setNativePackage(pkg *InstalledPluginPackage) {
	pm.nativeMu.Lock()
	defer pm.nativeMu.Unlock()

	pm.nativePackages[pkg.PluginID] = cloneInstalledPluginPackage(pkg)
}

func (pm *PluginManager) removeNativePackage(pluginID string) {
	pm.nativeMu.Lock()
	defer pm.nativeMu.Unlock()

	delete(pm.nativePackages, pluginID)
	delete(pm.loadedNativePlugins, pluginID)
}

func (pm *PluginManager) unregisterNativePlugin(pluginID string) {
	pm.unregisterNativePluginDefinition(pluginID)

	pm.nativeMu.Lock()
	defer pm.nativeMu.Unlock()

	delete(pm.loadedNativePlugins, pluginID)
}

func (pm *PluginManager) unregisterNativePluginDefinition(pluginID string) {
	if pm.registry != nil {
		if definition, err := pm.registry.GetPlugin(pluginID); err == nil &&
			definition.Source == PluginSourceNative {
			pm.registry.UnregisterPlugin(pluginID)
		}
	}
}

func (pm *PluginManager) resetNativeRuntimeState() {
	if pm.registry != nil {
		for _, definition := range pm.registry.ListPlugins() {
			if definition.Source == PluginSourceNative {
				pm.registry.UnregisterPlugin(definition.ID)
			}
		}
	}

	pm.nativeMu.Lock()
	defer pm.nativeMu.Unlock()

	pm.loadedNativePlugins = make(map[string]string)
}

func (pm *PluginManager) enrichPluginDefinition(definition PluginDefinition) PluginDefinition {
	if definition.Source == "" {
		definition.Source = PluginSourceBundled
	}

	switch definition.Source {
	case PluginSourceBundled:
		if definition.Distribution == "" {
			definition.Distribution = PluginDistributionBundled
		}
		if definition.InstallState == "" {
			definition.InstallState = PluginInstallStateReady
		}
		definition.Official = true
	case PluginSourceNative:
		if pkg := pm.getNativePackage(definition.ID); pkg != nil {
			definition.Source = pkg.Source
			definition.Official = pkg.Official
			definition.InstallState = pkg.InstallState
			definition.Distribution = pkg.Distribution
			definition.MinHostAPIVersion = pkg.MinHostAPIVersion
			definition.RequiredCapabilities = cloneRequiredCapabilities(pkg.RequiredCapabilities)
			definition.TargetOS = pkg.TargetOS
			definition.TargetArch = pkg.TargetArch
			definition.RuntimePath = pkg.RuntimePath
			definition.SignatureVerified = pkg.SignatureVerified
			definition.Unsafe = pkg.Unsafe
			if definition.Version == "" {
				definition.Version = pkg.Version
			}
			if definition.Name == "" {
				definition.Name = pkg.Name
			}
			if definition.Description == "" {
				definition.Description = pkg.Description
			}
		} else {
			definition.InstallState = PluginInstallStateNotInstalled
			if definition.Distribution == "" {
				definition.Distribution = PluginDistributionSideload
			}
			definition.RuntimePath = ""
			definition.SignatureVerified = false
			definition.Unsafe = false
		}
	}

	return definition
}

func (pm *PluginManager) maskAndEnrichPluginInstance(instance *PluginInstance) *PluginInstance {
	if instance == nil {
		return nil
	}

	maskedInstance := *instance
	if definition, err := pm.registry.GetPlugin(instance.PluginID); err == nil {
		enrichedDefinition := pm.enrichPluginDefinition(*definition)
		maskedInstance.PluginName = enrichedDefinition.Name
		maskedInstance.Source = enrichedDefinition.Source
		maskedInstance.Official = enrichedDefinition.Official
		maskedInstance.Distribution = enrichedDefinition.Distribution
		maskedInstance.InstallState = enrichedDefinition.InstallState
		maskedInstance.MinHostAPIVersion = enrichedDefinition.MinHostAPIVersion
		maskedInstance.Config = enrichedDefinition.ConfigSchema.MaskSensitiveFields(instance.Config)
		return &maskedInstance
	}

	if pkg := pm.getNativePackage(instance.PluginID); pkg != nil {
		if pkg.Name != "" {
			maskedInstance.PluginName = pkg.Name
		}
		maskedInstance.Source = pkg.Source
		maskedInstance.Official = pkg.Official
		maskedInstance.Distribution = pkg.Distribution
		maskedInstance.InstallState = pkg.InstallState
		maskedInstance.MinHostAPIVersion = pkg.MinHostAPIVersion
		if maskedInstance.LastError == "" {
			maskedInstance.LastError = pkg.LastError
		}
	}
	if maskedInstance.PluginName == "" {
		maskedInstance.PluginName = instance.PluginID
	}
	if maskedInstance.Config != nil {
		// The config schema is unavailable when the plugin definition cannot be loaded,
		// so avoid returning raw persisted values that may contain secrets.
		maskedInstance.Config = map[string]interface{}{}
	}

	return &maskedInstance
}

func (pm *PluginManager) ListInstalledPluginPackages() []*InstalledPluginPackage {
	packages := make([]*InstalledPluginPackage, 0)

	for _, definition := range pm.registry.ListPlugins() {
		enrichedDefinition := pm.enrichPluginDefinition(definition)
		if enrichedDefinition.Source != PluginSourceBundled {
			continue
		}

		packages = append(packages, &InstalledPluginPackage{
			PluginID:             enrichedDefinition.ID,
			Name:                 enrichedDefinition.Name,
			Description:          enrichedDefinition.Description,
			Version:              enrichedDefinition.Version,
			Source:               enrichedDefinition.Source,
			Distribution:         enrichedDefinition.Distribution,
			Official:             enrichedDefinition.Official,
			InstallState:         enrichedDefinition.InstallState,
			RuntimePath:          enrichedDefinition.RuntimePath,
			Manifest:             PluginPackageManifest{},
			SignatureVerified:    true,
			Unsafe:               false,
			Checksum:             "",
			MinHostAPIVersion:    enrichedDefinition.MinHostAPIVersion,
			RequiredCapabilities: cloneRequiredCapabilities(enrichedDefinition.RequiredCapabilities),
			TargetOS:             enrichedDefinition.TargetOS,
			TargetArch:           enrichedDefinition.TargetArch,
		})
	}

	pm.nativeMu.RLock()
	for _, pkg := range pm.nativePackages {
		packages = append(packages, cloneInstalledPluginPackage(pkg))
	}
	pm.nativeMu.RUnlock()

	sort.Slice(packages, func(i, j int) bool {
		if packages[i].Source != packages[j].Source {
			return packages[i].Source < packages[j].Source
		}
		return packages[i].Name < packages[j].Name
	})

	return packages
}

func (pm *PluginManager) InstallPluginPackageFromBundle(ctx context.Context, archive io.ReaderAt, size int64, originalName string) (*InstalledPluginPackage, error) {
	return pm.installPluginBundle(ctx, archive, size, originalName, PluginDistributionSideload)
}

func (pm *PluginManager) installPluginBundle(ctx context.Context, archive io.ReaderAt, size int64, originalName string, distribution PluginDistribution) (*InstalledPluginPackage, error) {
	pm.installMu.Lock()
	defer pm.installMu.Unlock()

	if !nativePluginsEnabled() {
		return nil, fmt.Errorf("plugin sideload is disabled")
	}

	if err := os.MkdirAll(pluginRuntimeDir(), 0o750); err != nil {
		return nil, fmt.Errorf("failed to create plugin runtime directory: %w", err)
	}

	manifest, selectedTarget, manifestBytes, signatureBytes, publicKeyBytes, libraryBytes, libraryName, err := readPluginBundle(archive, size)
	if err != nil {
		return nil, err
	}

	if err := validatePluginManifest(manifest); err != nil {
		return nil, err
	}
	if err := validatePluginCompatibility(manifest, selectedTarget); err != nil {
		return nil, err
	}

	if existingDefinition, err := pm.registry.GetPlugin(manifest.PluginID); err == nil {
		enriched := pm.enrichPluginDefinition(*existingDefinition)
		if enriched.Source == PluginSourceBundled {
			return nil, fmt.Errorf("plugin %s conflicts with a bundled plugin", manifest.PluginID)
		}
	}

	checksum := fmt.Sprintf("%x", sha256.Sum256(libraryBytes))
	if !strings.EqualFold(selectedTarget.SHA256, checksum) {
		return nil, fmt.Errorf("plugin checksum mismatch")
	}

	signatureVerified := false
	if len(signatureBytes) > 0 {
		signatureVerified, err = verifyManifestSignature(manifestBytes, signatureBytes, publicKeyBytes)
		if err != nil {
			return nil, err
		}
	}

	if distribution == PluginDistributionSideload && !signatureVerified && !allowUnsafeSideload() {
		return nil, fmt.Errorf("unsigned sideloads are disabled")
	}

	pluginIDSegment, versionSegment, err := pluginRuntimeSegments(manifest.PluginID, manifest.Version)
	if err != nil {
		return nil, err
	}

	pluginDir := filepath.Join(pluginRuntimeDir(), pluginIDSegment, versionSegment)
	runtimePath := filepath.Join(pluginDir, sanitizeRuntimeSegment(filepath.Base(libraryName)))
	if filepath.Ext(runtimePath) != ".so" {
		runtimePath = filepath.Join(pluginDir, pluginIDSegment+".so")
	}

	if err := os.MkdirAll(filepath.Dir(runtimePath), 0o750); err != nil {
		return nil, fmt.Errorf("failed to create plugin directory: %w", err)
	}
	if err := writeRuntimeLibrary(runtimePath, libraryBytes); err != nil {
		return nil, err
	}

	pkgSource := PluginSourceNative

	now := time.Now()
	pkg := &InstalledPluginPackage{
		PluginID:          manifest.PluginID,
		Name:              manifest.Name,
		Description:       manifest.Description,
		Version:           manifest.Version,
		Source:            pkgSource,
		Distribution:      distribution,
		Official:          false,
		InstallState:      PluginInstallStateReady,
		RuntimePath:       runtimePath,
		Manifest:          manifest,
		ManifestJSON:      append(json.RawMessage(nil), manifestBytes...),
		ManifestSignature: append([]byte(nil), signatureBytes...),
		ManifestPublicKey: append([]byte(nil), publicKeyBytes...),
		SignatureVerified: signatureVerified,
		Unsafe:            !signatureVerified,
		Checksum:          checksum,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	applyInstalledPackageTarget(pkg, selectedTarget)

	if existing := pm.getNativePackage(manifest.PluginID); existing != nil {
		pkg.CreatedAt = existing.CreatedAt
	}

	if loadedVersion, loaded := pm.getLoadedNativePluginVersion(manifest.PluginID); loaded {
		if loadedVersion == manifest.Version {
			if existing := pm.getNativePackage(manifest.PluginID); existing != nil && existing.Checksum == checksum {
				return existing, nil
			}
		}

		pkg.InstallState = PluginInstallStatePendingRestart
		pkg.LastError = "Restart Aegis to activate the updated plugin package"
	} else if err := pm.loadNativePluginPackage(pkg); err != nil {
		pkg.InstallState = PluginInstallStateError
		pkg.LastError = err.Error()
	}

	if err := pm.savePluginPackageToDatabase(pkg); err != nil {
		return nil, err
	}
	pm.setNativePackage(pkg)

	return cloneInstalledPluginPackage(pkg), nil
}

func (pm *PluginManager) DeleteInstalledPluginPackage(ctx context.Context, pluginID string) error {
	pm.installMu.Lock()
	defer pm.installMu.Unlock()

	pkg := pm.getNativePackage(pluginID)
	if pkg == nil {
		return fmt.Errorf("plugin package %s not found", pluginID)
	}

	hasInstances, err := pm.hasPluginInstances(ctx, pluginID)
	if err != nil {
		return err
	}
	if hasInstances {
		return fmt.Errorf("plugin %s still has configured server instances", pluginID)
	}

	if err := pm.deletePluginPackageFromDatabase(pluginID); err != nil {
		return err
	}

	if _, loaded := pm.getLoadedNativePluginVersion(pluginID); loaded {
		pm.unregisterNativePluginDefinition(pluginID)
		pkg.InstallState = PluginInstallStatePendingRestart
		pkg.LastError = "Restart Aegis to finish removing the plugin package"
		pkg.UpdatedAt = time.Now()
		// Force a replacement upload to be re-persisted even if the bytes match the
		// just-deleted package, because the DB row is already gone.
		pkg.Checksum = ""
		pm.setNativePackage(pkg)
	} else {
		pm.unregisterNativePlugin(pluginID)
		pm.removeNativePackage(pluginID)
	}

	if pkg.RuntimePath != "" {
		safePath, pathErr := validateRuntimePathWithinRoot(pkg.RuntimePath, pluginRuntimeDir())
		if pathErr != nil {
			log.Warn().Err(pathErr).Str("plugin_id", pluginID).Str("path", pkg.RuntimePath).Msg("Refusing to remove native plugin library outside runtime root")
		} else if err := os.Remove(safePath); err != nil && !os.IsNotExist(err) {
			log.Warn().Err(err).Str("plugin_id", pluginID).Str("path", safePath).Msg("Failed to remove native plugin library")
		}
	}

	return nil
}

func (pm *PluginManager) hasPluginInstances(ctx context.Context, pluginID string) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if pm.db != nil {
		var exists bool
		err := pm.db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM plugin_instances
				WHERE plugin_id = $1
			)
		`, pluginID).Scan(&exists)
		if err != nil {
			return false, fmt.Errorf("failed to check configured plugin instances for %s: %w", pluginID, err)
		}
		if exists {
			return true, nil
		}
	}

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, serverPlugins := range pm.plugins {
		for _, instance := range serverPlugins {
			if instance.PluginID == pluginID {
				return true, nil
			}
		}
	}

	return false, nil
}

func (pm *PluginManager) loadNativePluginPackage(pkg *InstalledPluginPackage) error {
	safePath, pathErr := validateRuntimePathWithinRoot(pkg.RuntimePath, pluginRuntimeDir())
	if pathErr != nil {
		return fmt.Errorf("plugin runtime path rejected: %w", pathErr)
	}
	pkg.RuntimePath = safePath

	if expected := strings.TrimSpace(pkg.Checksum); expected != "" {
		actual, err := hashFileSHA256(safePath)
		if err != nil {
			return fmt.Errorf("failed to hash plugin runtime library: %w", err)
		}
		if !strings.EqualFold(expected, actual) {
			return fmt.Errorf("plugin runtime library checksum mismatch: expected %s, got %s", expected, actual)
		}
	}

	definition, err := nativePluginDefinitionLoader(pkg.RuntimePath)
	if err != nil {
		return err
	}

	if definition.ID == "" {
		definition.ID = pkg.PluginID
	}
	if definition.ID != pkg.PluginID {
		return fmt.Errorf("native plugin definition ID %s does not match manifest ID %s", definition.ID, pkg.PluginID)
	}

	if definition.Name == "" {
		definition.Name = pkg.Name
	}
	if definition.Description == "" {
		definition.Description = pkg.Description
	}
	if definition.Version == "" {
		definition.Version = pkg.Version
	}

	definition.Source = PluginSourceNative
	definition.Official = pkg.Official
	definition.InstallState = pkg.InstallState
	definition.Distribution = pkg.Distribution
	definition.MinHostAPIVersion = pkg.MinHostAPIVersion
	definition.RequiredCapabilities = cloneRequiredCapabilities(pkg.RequiredCapabilities)
	definition.TargetOS = pkg.TargetOS
	definition.TargetArch = pkg.TargetArch
	definition.RuntimePath = pkg.RuntimePath
	definition.SignatureVerified = pkg.SignatureVerified
	definition.Unsafe = pkg.Unsafe

	if err := pm.registry.RegisterPlugin(definition); err != nil {
		return fmt.Errorf("failed to register native plugin: %w", err)
	}

	pm.nativeMu.Lock()
	pm.loadedNativePlugins[pkg.PluginID] = pkg.Version
	pm.nativeMu.Unlock()

	return nil
}

// verifyManifestSignature rejects any manifest.pub not in the operator-
// configured trust store before delegating to the cryptographic verifier.
// Without this anchor the signed-sideload gate is bypassable, since an
// attacker could ship their own keypair inside the bundle.
