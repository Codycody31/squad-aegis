package plugin_manager

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
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

type pluginArchiveReadBudget struct {
	entryLimit int64
	totalLimit int64
	remaining  int64
}

func newPluginArchiveReadBudget() pluginArchiveReadBudget {
	totalLimit := pluginMaxArchiveUncompressedSize()
	return pluginArchiveReadBudget{
		entryLimit: pluginMaxUploadSize(),
		totalLimit: totalLimit,
		remaining:  totalLimit,
	}
}

func (b *pluginArchiveReadBudget) read(file *zip.File) ([]byte, error) {
	if file == nil {
		return nil, nil
	}

	if file.UncompressedSize64 > uint64(b.entryLimit) {
		return nil, fmt.Errorf("plugin archive entry %s exceeds uncompressed size limit of %d bytes", file.Name, b.entryLimit)
	}
	if file.UncompressedSize64 > uint64(b.remaining) {
		return nil, fmt.Errorf("plugin archive exceeds total uncompressed size limit of %d bytes", b.totalLimit)
	}

	rc, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open archive entry %s: %w", file.Name, err)
	}

	readLimit := b.entryLimit
	totalLimited := false
	if b.remaining < readLimit {
		readLimit = b.remaining
		totalLimited = true
	}

	limited := &io.LimitedReader{R: rc, N: readLimit + 1}
	data, readErr := io.ReadAll(limited)
	closeErr := rc.Close()
	if readErr != nil {
		return nil, fmt.Errorf("failed to read archive entry %s: %w", file.Name, readErr)
	}
	if int64(len(data)) > readLimit {
		if totalLimited {
			return nil, fmt.Errorf("plugin archive exceeds total uncompressed size limit of %d bytes", b.totalLimit)
		}
		return nil, fmt.Errorf("plugin archive entry %s exceeds uncompressed size limit of %d bytes", file.Name, b.entryLimit)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("failed to close archive entry %s: %w", file.Name, closeErr)
	}

	b.remaining -= int64(len(data))

	return data, nil
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

func clonePluginPackageTargets(targets []PluginPackageTarget) []PluginPackageTarget {
	if len(targets) == 0 {
		return nil
	}

	cloned := make([]PluginPackageTarget, len(targets))
	copy(cloned, targets)
	return cloned
}

func cloneRequiredCapabilities(capabilities []string) []string {
	if len(capabilities) == 0 {
		return nil
	}

	cloned := make([]string, len(capabilities))
	copy(cloned, capabilities)
	return cloned
}

func hostNativePluginCapabilitiesSet() map[string]struct{} {
	capabilities := make(map[string]struct{}, len(nativePluginHostCapabilities))
	for _, capability := range NativePluginHostCapabilities() {
		capabilities[capability] = struct{}{}
	}
	return capabilities
}

func missingRequiredCapabilities(required []string) []string {
	if len(required) == 0 {
		return nil
	}

	hostCapabilities := hostNativePluginCapabilitiesSet()
	missingSet := make(map[string]struct{})
	for _, capability := range required {
		capability = strings.TrimSpace(capability)
		if capability == "" {
			continue
		}
		if _, ok := hostCapabilities[capability]; !ok {
			missingSet[capability] = struct{}{}
		}
	}

	missing := make([]string, 0, len(missingSet))
	for capability := range missingSet {
		missing = append(missing, capability)
	}
	sort.Strings(missing)
	return missing
}

func isCompatibleHostTarget(target PluginPackageTarget) bool {
	return target.MinHostAPIVersion <= NativePluginHostAPIVersion && len(missingRequiredCapabilities(target.RequiredCapabilities)) == 0
}

func betterPluginTarget(left, right PluginPackageTarget) bool {
	if left.MinHostAPIVersion != right.MinHostAPIVersion {
		return left.MinHostAPIVersion > right.MinHostAPIVersion
	}
	if len(left.RequiredCapabilities) != len(right.RequiredCapabilities) {
		return len(left.RequiredCapabilities) > len(right.RequiredCapabilities)
	}
	return left.LibraryPath < right.LibraryPath
}

func preferredPluginTarget(targets []PluginPackageTarget) PluginPackageTarget {
	if len(targets) == 0 {
		return PluginPackageTarget{}
	}

	hostOS := runtime.GOOS
	hostArch := runtime.GOARCH
	var bestCompatible *PluginPackageTarget
	var bestHostTarget *PluginPackageTarget
	var bestLinuxAMD64 *PluginPackageTarget

	for _, target := range targets {
		target.RequiredCapabilities = cloneRequiredCapabilities(target.RequiredCapabilities)

		if target.TargetOS == hostOS && target.TargetArch == hostArch {
			if bestHostTarget == nil || betterPluginTarget(target, *bestHostTarget) {
				targetCopy := target
				bestHostTarget = &targetCopy
			}
			if isCompatibleHostTarget(target) && (bestCompatible == nil || betterPluginTarget(target, *bestCompatible)) {
				targetCopy := target
				bestCompatible = &targetCopy
			}
		}

		if target.TargetOS == "linux" && target.TargetArch == "amd64" {
			if bestLinuxAMD64 == nil || betterPluginTarget(target, *bestLinuxAMD64) {
				targetCopy := target
				bestLinuxAMD64 = &targetCopy
			}
		}
	}

	if bestCompatible != nil {
		return *bestCompatible
	}
	if bestHostTarget != nil {
		return *bestHostTarget
	}
	if bestLinuxAMD64 != nil {
		return *bestLinuxAMD64
	}
	return targets[0]
}

func selectedManifestTarget(manifest PluginPackageManifest) (PluginPackageTarget, error) {
	targets := clonePluginPackageTargets(manifest.Targets)
	if len(targets) == 0 {
		return PluginPackageTarget{}, fmt.Errorf("plugin manifest is missing targets")
	}

	hostOS := runtime.GOOS
	hostArch := runtime.GOARCH
	var osMatches []PluginPackageTarget
	var archMatches []PluginPackageTarget
	var compatible []PluginPackageTarget
	minRequiredHostAPI := 0
	missingCapabilities := make(map[string]struct{})

	for _, target := range targets {
		target.RequiredCapabilities = cloneRequiredCapabilities(target.RequiredCapabilities)
		if target.TargetOS != hostOS {
			continue
		}
		osMatches = append(osMatches, target)
		if target.TargetArch != hostArch {
			continue
		}
		archMatches = append(archMatches, target)

		if target.MinHostAPIVersion > NativePluginHostAPIVersion {
			if minRequiredHostAPI == 0 || target.MinHostAPIVersion < minRequiredHostAPI {
				minRequiredHostAPI = target.MinHostAPIVersion
			}
			continue
		}

		for _, capability := range missingRequiredCapabilities(target.RequiredCapabilities) {
			missingCapabilities[capability] = struct{}{}
		}
		if len(missingRequiredCapabilities(target.RequiredCapabilities)) > 0 {
			continue
		}

		compatible = append(compatible, target)
	}

	if len(compatible) > 0 {
		best := compatible[0]
		for _, target := range compatible[1:] {
			if betterPluginTarget(target, best) {
				best = target
			}
		}
		return best, nil
	}

	if len(osMatches) == 0 {
		return PluginPackageTarget{}, fmt.Errorf("plugin does not support host OS %s", hostOS)
	}
	if len(archMatches) == 0 {
		return PluginPackageTarget{}, fmt.Errorf("plugin does not support host architecture %s on %s", hostArch, hostOS)
	}

	missingCapabilityList := make([]string, 0, len(missingCapabilities))
	for capability := range missingCapabilities {
		missingCapabilityList = append(missingCapabilityList, capability)
	}
	sort.Strings(missingCapabilityList)

	if minRequiredHostAPI > 0 && len(missingCapabilityList) > 0 {
		return PluginPackageTarget{}, fmt.Errorf(
			"plugin has no compatible target for %s/%s: requires host API version >= %d and capabilities %s (host API version is %d)",
			hostOS,
			hostArch,
			minRequiredHostAPI,
			strings.Join(missingCapabilityList, ", "),
			NativePluginHostAPIVersion,
		)
	}
	if minRequiredHostAPI > 0 {
		return PluginPackageTarget{}, fmt.Errorf(
			"plugin requires host API version >= %d, but host provides %d",
			minRequiredHostAPI,
			NativePluginHostAPIVersion,
		)
	}
	if len(missingCapabilityList) > 0 {
		return PluginPackageTarget{}, fmt.Errorf(
			"plugin requires unsupported host capabilities: %s",
			strings.Join(missingCapabilityList, ", "),
		)
	}

	return PluginPackageTarget{}, fmt.Errorf("plugin has no compatible target for %s/%s", hostOS, hostArch)
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
	if pm.registry != nil {
		if definition, err := pm.registry.GetPlugin(pluginID); err == nil && definition.Source == PluginSourceNative {
			pm.registry.UnregisterPlugin(pluginID)
		}
	}

	pm.nativeMu.Lock()
	defer pm.nativeMu.Unlock()

	delete(pm.loadedNativePlugins, pluginID)
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

func (pm *PluginManager) loadInstalledPluginPackages() error {
	if !nativePluginsEnabled() {
		return nil
	}

	rows, err := pm.db.Query(`
		SELECT plugin_id, name, description, version, source, distribution, official, install_state,
		       runtime_path, manifest_json, signature_verified, unsafe, checksum, min_host_api_version,
		       required_capabilities, target_os, target_arch, last_error, created_at, updated_at
		FROM plugin_packages
		ORDER BY created_at
	`)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("failed to query plugin packages: %w", err)
	}
	defer rows.Close()

	loadedPackages := make(map[string]*InstalledPluginPackage)

	for rows.Next() {
		var pkg InstalledPluginPackage
		var manifestJSON []byte
		var requiredCapabilitiesJSON []byte

		if err := rows.Scan(
			&pkg.PluginID,
			&pkg.Name,
			&pkg.Description,
			&pkg.Version,
			&pkg.Source,
			&pkg.Distribution,
			&pkg.Official,
			&pkg.InstallState,
			&pkg.RuntimePath,
			&manifestJSON,
			&pkg.SignatureVerified,
			&pkg.Unsafe,
			&pkg.Checksum,
			&pkg.MinHostAPIVersion,
			&requiredCapabilitiesJSON,
			&pkg.TargetOS,
			&pkg.TargetArch,
			&pkg.LastError,
			&pkg.CreatedAt,
			&pkg.UpdatedAt,
		); err != nil {
			return fmt.Errorf("failed to scan plugin package row: %w", err)
		}

		pkg.ManifestJSON = append(json.RawMessage(nil), manifestJSON...)
		if len(requiredCapabilitiesJSON) > 0 {
			if err := json.Unmarshal(requiredCapabilitiesJSON, &pkg.RequiredCapabilities); err != nil {
				pkg.InstallState = PluginInstallStateError
				pkg.LastError = fmt.Sprintf("invalid required capabilities metadata: %v", err)
			}
		}
		if len(manifestJSON) > 0 {
			if err := json.Unmarshal(manifestJSON, &pkg.Manifest); err != nil {
				pkg.InstallState = PluginInstallStateError
				pkg.LastError = fmt.Sprintf("invalid manifest: %v", err)
			} else if target, targetErr := selectedManifestTarget(pkg.Manifest); targetErr != nil {
				pkg.InstallState = PluginInstallStateError
				pkg.LastError = targetErr.Error()
			} else {
				applyInstalledPackageTarget(&pkg, target)
			}
		}

		loadedPackages[pkg.PluginID] = &pkg
	}

	pm.resetNativeRuntimeState()

	pm.nativeMu.Lock()
	pm.nativePackages = loadedPackages
	pm.nativeMu.Unlock()

	for _, pkg := range loadedPackages {
		if pkg.InstallState != PluginInstallStateReady && pkg.InstallState != PluginInstallStatePendingRestart {
			continue
		}

		shouldPersistReadyState := false
		if pkg.InstallState == PluginInstallStatePendingRestart {
			pkg.InstallState = PluginInstallStateReady
			pkg.LastError = ""
			pkg.UpdatedAt = time.Now()
			shouldPersistReadyState = true
		}

		if err := pm.loadNativePluginPackage(pkg); err != nil {
			pkg.InstallState = PluginInstallStateError
			pkg.LastError = err.Error()
			pkg.UpdatedAt = time.Now()
			if saveErr := pm.savePluginPackageToDatabase(pkg); saveErr != nil {
				log.Error().Err(saveErr).Str("plugin_id", pkg.PluginID).Msg("Failed to persist native plugin load error")
			}
			pm.setNativePackage(pkg)
			continue
		}

		if shouldPersistReadyState || pkg.LastError != "" {
			pkg.LastError = ""
			pkg.UpdatedAt = time.Now()
			if saveErr := pm.savePluginPackageToDatabase(pkg); saveErr != nil {
				log.Error().Err(saveErr).Str("plugin_id", pkg.PluginID).Msg("Failed to persist native plugin package activation")
			}
			pm.setNativePackage(pkg)
		}
	}

	return nil
}

func (pm *PluginManager) savePluginPackageToDatabase(pkg *InstalledPluginPackage) error {
	manifestJSON := pkg.ManifestJSON
	if len(manifestJSON) == 0 {
		var err error
		manifestJSON, err = json.Marshal(pkg.Manifest)
		if err != nil {
			return fmt.Errorf("failed to marshal plugin manifest: %w", err)
		}
	}

	_, err := pm.db.Exec(`
		INSERT INTO plugin_packages (
			plugin_id, name, description, version, source, distribution, official, install_state,
			runtime_path, manifest_json, signature_verified, unsafe, checksum, min_host_api_version,
			required_capabilities, target_os, target_arch, last_error, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14,
			$15, $16, $17, $18, $19, $20
		)
		ON CONFLICT (plugin_id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			version = EXCLUDED.version,
			source = EXCLUDED.source,
			distribution = EXCLUDED.distribution,
			official = EXCLUDED.official,
			install_state = EXCLUDED.install_state,
			runtime_path = EXCLUDED.runtime_path,
			manifest_json = EXCLUDED.manifest_json,
			signature_verified = EXCLUDED.signature_verified,
			unsafe = EXCLUDED.unsafe,
			checksum = EXCLUDED.checksum,
			min_host_api_version = EXCLUDED.min_host_api_version,
			required_capabilities = EXCLUDED.required_capabilities,
			target_os = EXCLUDED.target_os,
			target_arch = EXCLUDED.target_arch,
			last_error = EXCLUDED.last_error,
			updated_at = EXCLUDED.updated_at
	`,
		pkg.PluginID,
		pkg.Name,
		pkg.Description,
		pkg.Version,
		pkg.Source,
		pkg.Distribution,
		pkg.Official,
		pkg.InstallState,
		pkg.RuntimePath,
		string(manifestJSON),
		pkg.SignatureVerified,
		pkg.Unsafe,
		pkg.Checksum,
		pkg.MinHostAPIVersion,
		string(mustMarshalRequiredCapabilities(pkg.RequiredCapabilities)),
		pkg.TargetOS,
		pkg.TargetArch,
		pkg.LastError,
		pkg.CreatedAt,
		pkg.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert plugin package: %w", err)
	}

	return nil
}

func (pm *PluginManager) deletePluginPackageFromDatabase(pluginID string) error {
	_, err := pm.db.Exec(`DELETE FROM plugin_packages WHERE plugin_id = $1`, pluginID)
	if err != nil {
		return fmt.Errorf("failed to delete plugin package: %w", err)
	}

	return nil
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
	if !nativePluginsEnabled() {
		return nil, fmt.Errorf("native plugins are disabled")
	}

	if err := os.MkdirAll(pluginRuntimeDir(), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create plugin runtime directory: %w", err)
	}

	manifest, selectedTarget, manifestBytes, signatureBytes, publicKeyBytes, libraryBytes, libraryName, err := readPluginBundle(archive, size)
	if err != nil {
		return nil, err
	}

	if err := validatePluginManifest(manifest); err != nil {
		return nil, err
	}
	if err := validatePluginCompatibility(selectedTarget); err != nil {
		return nil, err
	}

	if existingDefinition, err := pm.registry.GetPlugin(manifest.PluginID); err == nil {
		enriched := pm.enrichPluginDefinition(*existingDefinition)
		if enriched.Source == PluginSourceBundled {
			return nil, fmt.Errorf("plugin %s conflicts with a bundled plugin", manifest.PluginID)
		}
	}

	checksum := fmt.Sprintf("%x", sha256.Sum256(libraryBytes))
	if selectedTarget.SHA256 != "" && !strings.EqualFold(selectedTarget.SHA256, checksum) {
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

	if err := os.MkdirAll(filepath.Dir(runtimePath), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create plugin directory: %w", err)
	}
	if err := os.WriteFile(runtimePath, libraryBytes, 0o755); err != nil {
		return nil, fmt.Errorf("failed to write plugin library: %w", err)
	}

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
		pkg.LastError = "Restart Aegis to activate the updated native plugin package"
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

	pm.unregisterNativePlugin(pluginID)
	pm.removeNativePackage(pluginID)

	if pkg.RuntimePath != "" {
		if err := os.Remove(pkg.RuntimePath); err != nil && !os.IsNotExist(err) {
			log.Warn().Err(err).Str("plugin_id", pluginID).Str("path", pkg.RuntimePath).Msg("Failed to remove native plugin library")
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

func readPluginBundle(archive io.ReaderAt, size int64) (PluginPackageManifest, PluginPackageTarget, []byte, []byte, []byte, []byte, string, error) {
	reader, err := zip.NewReader(archive, size)
	if err != nil {
		return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("invalid plugin archive: %w", err)
	}

	var manifest PluginPackageManifest
	var manifestBytes []byte
	var signatureBytes []byte
	var publicKeyBytes []byte
	libraries := make(map[string]*zip.File)
	var manifestFile *zip.File
	var signatureFile *zip.File
	var publicKeyFile *zip.File

	for _, file := range reader.File {
		name := filepath.Clean(strings.TrimPrefix(file.Name, "/"))
		if name == "." || strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("plugin archive contains an unsafe path: %s", file.Name)
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("plugin archive contains an unsupported symlink: %s", file.Name)
		}

		switch filepath.Base(name) {
		case pluginManifestFile:
			manifestFile = file
		case pluginSignatureFile:
			signatureFile = file
		case pluginPublicKeyFile:
			publicKeyFile = file
		default:
			if strings.HasSuffix(name, ".so") {
				libraries[name] = file
			}
		}
	}

	if manifestFile == nil {
		return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("plugin archive is missing %s", pluginManifestFile)
	}

	budget := newPluginArchiveReadBudget()

	manifestBytes, err = budget.read(manifestFile)
	if err != nil {
		return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("invalid plugin manifest: %w", err)
	}

	selectedTarget, err := selectedManifestTarget(manifest)
	if err != nil {
		return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
	}

	if signatureFile != nil {
		signatureBytes, err = budget.read(signatureFile)
		if err != nil {
			return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
		}
	}
	if publicKeyFile != nil {
		publicKeyBytes, err = budget.read(publicKeyFile)
		if err != nil {
			return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
		}
	}

	hasSignature := len(bytes.TrimSpace(signatureBytes)) > 0
	hasPublicKey := len(bytes.TrimSpace(publicKeyBytes)) > 0
	if hasSignature != hasPublicKey {
		return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("plugin archive must include %s and %s together", pluginSignatureFile, pluginPublicKeyFile)
	}

	libraryName, libraryFile, err := selectManifestLibrary(manifest, selectedTarget, libraries)
	if err != nil {
		return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
	}
	libraryBytes, err := budget.read(libraryFile)
	if err != nil {
		return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
	}

	return manifest, selectedTarget, manifestBytes, signatureBytes, publicKeyBytes, libraryBytes, libraryName, nil
}

func validatePluginManifest(manifest PluginPackageManifest) error {
	if manifest.PluginID == "" {
		return fmt.Errorf("plugin manifest is missing plugin_id")
	}
	if manifest.Name == "" {
		return fmt.Errorf("plugin manifest is missing name")
	}
	if manifest.Version == "" {
		return fmt.Errorf("plugin manifest is missing version")
	}
	if _, _, err := pluginRuntimeSegments(manifest.PluginID, manifest.Version); err != nil {
		return err
	}
	if manifest.EntrySymbol == "" {
		manifest.EntrySymbol = nativePluginEntrySymbol
	}
	if manifest.EntrySymbol != nativePluginEntrySymbol {
		return fmt.Errorf("unsupported plugin entry symbol %s", manifest.EntrySymbol)
	}

	targets := clonePluginPackageTargets(manifest.Targets)
	if len(targets) == 0 {
		return fmt.Errorf("plugin manifest is missing targets")
	}

	seenTargets := make(map[string]struct{}, len(targets))
	for index, target := range targets {
		if target.MinHostAPIVersion <= 0 {
			return fmt.Errorf("plugin manifest target %d is missing min_host_api_version", index)
		}
		if target.TargetOS == "" {
			return fmt.Errorf("plugin manifest target %d is missing target_os", index)
		}
		if target.TargetArch == "" {
			return fmt.Errorf("plugin manifest target %d is missing target_arch", index)
		}
		if strings.TrimSpace(target.LibraryPath) == "" {
			return fmt.Errorf("plugin manifest target %d is missing library_path", index)
		}
		requiredCapabilitySet := make(map[string]struct{}, len(target.RequiredCapabilities))
		requiredCapabilities := cloneRequiredCapabilities(target.RequiredCapabilities)
		sort.Strings(requiredCapabilities)
		for _, capability := range requiredCapabilities {
			capability = strings.TrimSpace(capability)
			if capability == "" {
				return fmt.Errorf("plugin manifest target %d has an empty required capability", index)
			}
			if _, exists := requiredCapabilitySet[capability]; exists {
				return fmt.Errorf("plugin manifest target %d repeats required capability %s", index, capability)
			}
			requiredCapabilitySet[capability] = struct{}{}
		}

		key := target.TargetOS + "|" + target.TargetArch + "|" + strconv.Itoa(target.MinHostAPIVersion) + "|" + strings.Join(requiredCapabilities, ",")
		if _, exists := seenTargets[key]; exists {
			return fmt.Errorf("plugin manifest has a duplicate target for %s/%s with min_host_api_version %d and capabilities %s", target.TargetOS, target.TargetArch, target.MinHostAPIVersion, strings.Join(requiredCapabilities, ","))
		}
		seenTargets[key] = struct{}{}
	}

	return nil
}

func validatePluginCompatibility(target PluginPackageTarget) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("native plugins are only supported on Linux")
	}
	if target.TargetOS != runtime.GOOS {
		return fmt.Errorf("plugin targets %s but host is %s", target.TargetOS, runtime.GOOS)
	}
	if target.TargetArch != runtime.GOARCH {
		return fmt.Errorf("plugin targets %s but host architecture is %s", target.TargetArch, runtime.GOARCH)
	}
	if target.MinHostAPIVersion > NativePluginHostAPIVersion {
		return fmt.Errorf("plugin requires host API version >= %d, but host provides %d", target.MinHostAPIVersion, NativePluginHostAPIVersion)
	}
	if missing := missingRequiredCapabilities(target.RequiredCapabilities); len(missing) > 0 {
		return fmt.Errorf("plugin requires unsupported host capabilities: %s", strings.Join(missing, ", "))
	}

	return nil
}

func selectManifestLibrary(manifest PluginPackageManifest, target PluginPackageTarget, libraries map[string]*zip.File) (string, *zip.File, error) {
	libraryPath := strings.TrimSpace(target.LibraryPath)
	if libraryPath != "" {
		declaredPath := filepath.Clean(strings.TrimPrefix(libraryPath, "/"))
		libraryFile := libraries[declaredPath]
		if libraryFile == nil {
			return "", nil, fmt.Errorf("plugin archive is missing declared library %s", libraryPath)
		}
		return declaredPath, libraryFile, nil
	}

	if len(libraries) == 1 {
		for name, file := range libraries {
			return name, file, nil
		}
	}

	if len(libraries) == 0 {
		return "", nil, fmt.Errorf("plugin archive is missing a .so library")
	}

	return "", nil, fmt.Errorf("plugin manifest target %s/%s with min_host_api_version %d is missing library_path", target.TargetOS, target.TargetArch, target.MinHostAPIVersion)
}

func verifyManifestSignature(manifestBytes, signatureBytes, publicKeyBytes []byte) (bool, error) {
	return plugin_signing.VerifyManifest(manifestBytes, signatureBytes, publicKeyBytes)
}
