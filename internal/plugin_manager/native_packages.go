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
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/plugin_signing"
	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

const (
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

// nativePluginVerifiedLoader is a hookable loader that fetches the static
// definition of a native plugin package. In production it spawns a
// hashicorp/go-plugin subprocess, pulls the definition out over net/rpc,
// then kills the peek subprocess. The returned definition's CreateInstance
// factory spawns a fresh subprocess per instance — so a buggy or malicious
// plugin can only ever corrupt its own process image, never the host's.
// Tests override this hook to inject canned definitions without spawning
// a real process.
var nativePluginVerifiedLoader = peekNativePluginDefinition

const nativePluginPendingRestartMessage = "Restart Aegis to activate the updated native plugin package"

// runtimeDirCache caches the resolved absolute paths for the plugin and
// connector runtime directories. Resolving once at first use guarantees the
// containment checks remain consistent even if the process changes its
// working directory after start, and avoids tying the trust boundary of the
// install path to a mutable global state.
var (
	runtimeDirCacheMu          sync.Mutex
	cachedPluginRuntimeDir     string
	cachedConnectorRuntimeDir  string
	runtimeDirRelativeWarnOnce sync.Once
)

// ResetRuntimeDirCache clears the cached runtime directories. Tests use this
// to swap config.Config between cases; production callers must not invoke it.
func ResetRuntimeDirCache() {
	runtimeDirCacheMu.Lock()
	defer runtimeDirCacheMu.Unlock()
	cachedPluginRuntimeDir = ""
	cachedConnectorRuntimeDir = ""
	runtimeDirRelativeWarnOnce = sync.Once{}
}

func nativePluginsEnabled() bool {
	return config.Config != nil && config.Config.Plugins.NativeEnabled
}

func resolveRuntimeDirAbsolute(raw string) string {
	abs, err := filepath.Abs(raw)
	if err != nil {
		log.Warn().Err(err).Str("dir", raw).Msg("Failed to resolve runtime directory to absolute path; falling back to raw value")
		return raw
	}
	return filepath.Clean(abs)
}

func warnIfRelative(field, configured string) {
	if configured == "" {
		runtimeDirRelativeWarnOnce.Do(func() {
			log.Warn().Str("field", field).Msg("Native plugin runtime directory is unset; defaulting to a relative path. Set Plugins." + field + " to an absolute path in production")
		})
	} else if !filepath.IsAbs(configured) {
		log.Warn().Str("field", field).Str("value", configured).Msg("Native plugin runtime directory is relative; resolving against current working directory. Set Plugins." + field + " to an absolute path in production")
	}
}

func pluginRuntimeDir() string {
	runtimeDirCacheMu.Lock()
	defer runtimeDirCacheMu.Unlock()

	if cachedPluginRuntimeDir != "" {
		return cachedPluginRuntimeDir
	}

	raw := "plugins"
	configured := ""
	if config.Config != nil {
		configured = strings.TrimSpace(config.Config.Plugins.RuntimeDir)
		switch {
		case configured != "":
			raw = configured
		case config.Config.App.InContainer:
			raw = "/etc/squad-aegis/plugins"
		}
	}

	warnIfRelative("RuntimeDir", configured)
	cachedPluginRuntimeDir = resolveRuntimeDirAbsolute(raw)
	return cachedPluginRuntimeDir
}

func connectorRuntimeDir() string {
	runtimeDirCacheMu.Lock()
	defer runtimeDirCacheMu.Unlock()

	if cachedConnectorRuntimeDir != "" {
		return cachedConnectorRuntimeDir
	}

	raw := "connectors"
	configured := ""
	if config.Config != nil {
		configured = strings.TrimSpace(config.Config.Plugins.ConnectorRuntimeDir)
		switch {
		case configured != "":
			raw = configured
		case config.Config.App.InContainer:
			raw = "/etc/squad-aegis/connectors"
		}
	}

	warnIfRelative("ConnectorRuntimeDir", configured)
	cachedConnectorRuntimeDir = resolveRuntimeDirAbsolute(raw)
	return cachedConnectorRuntimeDir
}

// writeRuntimeLibrary delegates to the per-platform writer which performs
// an atomic write via temp file + rename(2). On Unix the temp file is opened
// with O_NOFOLLOW to defeat symlink-clobber attacks; the rename atomically
// replaces any existing file at runtimePath without a Remove/Create window.
func writeRuntimeLibrary(runtimePath string, libraryBytes []byte) error {
	return writeRuntimeLibraryPlatform(runtimePath, libraryBytes)
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

	// Build a shallow copy field-by-field to avoid copying the sync.Mutex,
	// and take the lock so Status/LastError are read atomically with writes
	// from the unexpected-exit reporter.
	instance.mu.Lock()
	maskedInstance := &PluginInstance{
		ID:                instance.ID,
		ServerID:          instance.ServerID,
		PluginID:          instance.PluginID,
		PluginName:        instance.PluginName,
		Source:            instance.Source,
		Official:          instance.Official,
		Distribution:      instance.Distribution,
		InstallState:      instance.InstallState,
		MinHostAPIVersion: instance.MinHostAPIVersion,
		Notes:             instance.Notes,
		Config:            instance.Config,
		Status:            instance.Status,
		Enabled:           instance.Enabled,
		LogLevel:          instance.LogLevel,
		LastError:         instance.LastError,
		CreatedAt:         instance.CreatedAt,
		UpdatedAt:         instance.UpdatedAt,
	}
	instance.mu.Unlock()
	if definition, err := pm.registry.GetPlugin(instance.PluginID); err == nil {
		enrichedDefinition := pm.enrichPluginDefinition(*definition)
		maskedInstance.PluginName = enrichedDefinition.Name
		maskedInstance.Source = enrichedDefinition.Source
		maskedInstance.Official = enrichedDefinition.Official
		maskedInstance.Distribution = enrichedDefinition.Distribution
		maskedInstance.InstallState = enrichedDefinition.InstallState
		maskedInstance.MinHostAPIVersion = enrichedDefinition.MinHostAPIVersion
		maskedInstance.Config = enrichedDefinition.ConfigSchema.MaskSensitiveFields(maskedInstance.Config)
		return maskedInstance
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

	return maskedInstance
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

	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

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
	runtimeBase := sanitizeRuntimeSegment(filepath.Base(libraryName))
	if runtimeBase == "" {
		runtimeBase = pluginIDSegment
	}
	runtimePath := filepath.Join(pluginDir, runtimeBase)

	if err := os.MkdirAll(filepath.Dir(runtimePath), 0o750); err != nil {
		return nil, fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Same-bytes short-circuit: if a package with this plugin ID + version
	// is already loaded with the same checksum, return the existing entry
	// without rewriting the runtime file. The bytes on disk are already
	// guaranteed by the previous install.
	if loadedVersion, loaded := pm.getLoadedNativePluginVersion(manifest.PluginID); loaded && loadedVersion == manifest.Version {
		if existing := pm.getNativePackage(manifest.PluginID); existing != nil && existing.Checksum == checksum {
			log.Info().Str("plugin_id", manifest.PluginID).Str("version", manifest.Version).Msg("Skipping native plugin re-install: bytes match already-loaded package")
			return existing, nil
		}
	}

	if err := writeRuntimeLibrary(runtimePath, libraryBytes); err != nil {
		return nil, err
	}
	// Track whether the install completed atomically. On any error after
	// this point that leaves disk and DB out of sync, the deferred cleanup
	// removes the runtime file so the next reload doesn't see a phantom .so.
	installCommitted := false
	defer func() {
		if !installCommitted {
			if safePath, pathErr := validateRuntimePathWithinRoot(runtimePath, pluginRuntimeDir()); pathErr != nil {
				log.Warn().Err(pathErr).Str("path", runtimePath).Msg("Refusing to roll back runtime file outside plugin runtime root")
			} else {
				removeRuntimeFile(safePath)
			}
		}
	}()

	now := time.Now()
	pkg := &InstalledPluginPackage{
		PluginID:          manifest.PluginID,
		Name:              manifest.Name,
		Description:       manifest.Description,
		Version:           manifest.Version,
		Source:            PluginSourceNative,
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

	// Decide the final state. Loading happens BEFORE the database save so
	// that if the load fails we can record the error in a single round-trip,
	// and so that a successful load is rolled back (registry unregister) if
	// the subsequent save fails.
	loadFailureRecorded := false
	requiresRestart, err := pm.hasPluginInstances(ctx, manifest.PluginID)
	if err != nil {
		return nil, err
	}
	if requiresRestart {
		pkg.InstallState = PluginInstallStatePendingRestart
		pkg.LastError = nativePluginPendingRestartMessage
	} else {
		if _, loaded := pm.getLoadedNativePluginVersion(manifest.PluginID); loaded {
			pm.unregisterNativePlugin(manifest.PluginID)
		}
		if err := pm.loadNativePluginPackage(pkg); err != nil {
			pm.unregisterNativePluginDefinition(manifest.PluginID)
			pkg.InstallState = PluginInstallStateError
			pkg.LastError = err.Error()
			loadFailureRecorded = true
			log.Warn().Err(err).Str("plugin_id", manifest.PluginID).Str("version", manifest.Version).Msg("Failed to load uploaded native plugin package")
		} else {
			pkg.InstallState = PluginInstallStateReady
			pkg.LastError = ""
		}
	}
	pkg.UpdatedAt = time.Now()

	if err := pm.savePluginPackageToDatabaseContext(ctx, pkg); err != nil {
		// DB save failed. Clean up the loaded state so the live registry
		// doesn't outlive the (now-rolled-back) database row, and let the
		// deferred file remover clear the runtime file from disk.
		if !requiresRestart {
			pm.unregisterNativePlugin(manifest.PluginID)
		}
		return nil, fmt.Errorf("failed to persist plugin package state: %w", err)
	}

	pm.setNativePackage(pkg)
	installCommitted = true
	if requiresRestart {
		log.Info().Str("plugin_id", pkg.PluginID).Str("version", pkg.Version).Str("install_state", string(pkg.InstallState)).Msg("Installed native plugin package pending restart")
	} else if loadFailureRecorded {
		log.Info().Str("plugin_id", pkg.PluginID).Str("version", pkg.Version).Str("install_state", string(pkg.InstallState)).Msg("Persisted native plugin package install error state")
	} else {
		log.Info().Str("plugin_id", pkg.PluginID).Str("version", pkg.Version).Str("install_state", string(pkg.InstallState)).Bool("signature_verified", pkg.SignatureVerified).Msg("Installed native plugin package")
	}

	return cloneInstalledPluginPackage(pkg), nil
}

func (pm *PluginManager) DeleteInstalledPluginPackage(ctx context.Context, pluginID string) error {
	pm.installMu.Lock()
	defer pm.installMu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}

	pkg := pm.getNativePackage(pluginID)
	if pkg == nil {
		return fmt.Errorf("plugin package %s not found", pluginID)
	}

	// Holding pm.mu blocks CreatePluginInstance for the duration of the
	// existence check + DB delete, closing the delete-during-create race.
	pm.mu.Lock()
	hasInstances, err := pm.hasPluginInstancesLocked(ctx, pluginID)
	if err != nil {
		pm.mu.Unlock()
		return err
	}
	if hasInstances {
		pm.mu.Unlock()
		return fmt.Errorf("plugin %s still has configured server instances", pluginID)
	}

	if err := pm.deletePluginPackageFromDatabaseContext(ctx, pluginID); err != nil {
		pm.mu.Unlock()
		return err
	}
	pm.mu.Unlock()

	pm.unregisterNativePlugin(pluginID)
	pm.removeNativePackage(pluginID)

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
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.hasPluginInstancesLocked(ctx, pluginID)
}

// hasPluginInstancesLocked is the lock-free helper used when the caller
// already holds pm.mu (read or write). It is the only safe entry point from
// within the delete flow which must hold pm.mu.Lock to block concurrent
// CreatePluginInstance.
func (pm *PluginManager) hasPluginInstancesLocked(ctx context.Context, pluginID string) (bool, error) {
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

	// Rebuild the target snapshot from the package's compatibility fields
	// so the peek/merge step has everything it needs. We don't serialize the
	// full manifest.targets array onto InstalledPluginPackage — the selected
	// target was already applied at install time.
	target := PluginPackageTarget{
		MinHostAPIVersion:    pkg.MinHostAPIVersion,
		RequiredCapabilities: cloneRequiredCapabilities(pkg.RequiredCapabilities),
		TargetOS:             pkg.TargetOS,
		TargetArch:           pkg.TargetArch,
	}

	definition, err := nativePluginVerifiedLoader(safePath, pkg.Checksum, pkg.Manifest, target)
	if err != nil {
		return err
	}

	// The manifest is already the source of truth for identity — the peek
	// merger enforced ID match — so we only need to overlay install-time
	// state the manifest does not carry.
	definition.InstallState = pkg.InstallState
	definition.Distribution = pkg.Distribution
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
