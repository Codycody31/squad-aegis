package plugin_manager

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

var nativeConnectorDefinitionLoader = loadNativeConnectorDefinition

// ConnectorPackageManifest is the manifest.json format for native connector bundles.
type ConnectorPackageManifest struct {
	ConnectorID string                `json:"connector_id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Version     string                `json:"version"`
	Official    bool                  `json:"official"`
	License     string                `json:"license,omitempty"`
	EntrySymbol string                `json:"entry_symbol"`
	Targets     []PluginPackageTarget `json:"targets"`

	Author       string                          `json:"author,omitempty"`
	InstanceKey  string                          `json:"instance_key,omitempty"`
	LegacyIDs    []string                        `json:"legacy_ids,omitempty"`
	ConfigSchema plug_config_schema.ConfigSchema `json:"config_schema,omitempty"`
}

func (m ConnectorPackageManifest) asPluginManifest() PluginPackageManifest {
	return PluginPackageManifest{
		PluginID:     m.ConnectorID,
		Name:         m.Name,
		Description:  m.Description,
		Version:      m.Version,
		Official:     m.Official,
		License:      m.License,
		EntrySymbol:  m.EntrySymbol,
		Targets:      m.Targets,
		Author:       m.Author,
		ConfigSchema: m.ConfigSchema,
	}
}

// InstalledConnectorPackage tracks a native connector on disk and in connector_packages.
type InstalledConnectorPackage struct {
	ConnectorID          string                   `json:"connector_id"`
	Name                 string                   `json:"name"`
	Description          string                   `json:"description"`
	Version              string                   `json:"version"`
	Source               PluginSource             `json:"source"`
	Distribution         PluginDistribution       `json:"distribution"`
	Official             bool                     `json:"official"`
	InstallState         PluginInstallState       `json:"install_state"`
	RuntimePath          string                   `json:"runtime_path,omitempty"`
	Manifest             ConnectorPackageManifest `json:"manifest"`
	ManifestJSON         json.RawMessage          `json:"-"`
	ManifestSignature    []byte                   `json:"-"`
	ManifestPublicKey    []byte                   `json:"-"`
	SignatureVerified    bool                     `json:"signature_verified"`
	Unsafe               bool                     `json:"unsafe"`
	Checksum             string                   `json:"checksum"`
	MinHostAPIVersion    int                      `json:"min_host_api_version"`
	RequiredCapabilities []string                 `json:"required_capabilities,omitempty"`
	TargetOS             string                   `json:"target_os"`
	TargetArch           string                   `json:"target_arch"`
	LastError            string                   `json:"last_error,omitempty"`
	CreatedAt            time.Time                `json:"created_at"`
	UpdatedAt            time.Time                `json:"updated_at"`
}

func applyInstalledConnectorTarget(pkg *InstalledConnectorPackage, target PluginPackageTarget) {
	pkg.MinHostAPIVersion = target.MinHostAPIVersion
	pkg.RequiredCapabilities = cloneRequiredCapabilities(target.RequiredCapabilities)
	pkg.TargetOS = target.TargetOS
	pkg.TargetArch = target.TargetArch
}

func connectorInstanceKeysForPackage(pkg *InstalledConnectorPackage) []string {
	if pkg == nil {
		return nil
	}

	keys := make([]string, 0, 2+len(pkg.Manifest.LegacyIDs))
	seen := make(map[string]struct{}, 2+len(pkg.Manifest.LegacyIDs))
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		keys = append(keys, value)
	}

	definition := ConnectorDefinition{
		ID:          pkg.ConnectorID,
		InstanceKey: pkg.Manifest.InstanceKey,
		LegacyIDs:   pkg.Manifest.LegacyIDs,
	}
	add(definition.ConnectorInstanceStorageKey())
	add(pkg.ConnectorID)
	for _, legacyID := range pkg.Manifest.LegacyIDs {
		add(legacyID)
	}

	return keys
}

func (pm *PluginManager) resetNativeConnectorRuntimeState() {
	for _, definition := range pm.connectorRegistry.ListConnectors() {
		if definition.Source == PluginSourceNative {
			pm.connectorRegistry.UnregisterConnector(definition.ID)
		}
	}

	pm.nativeMu.Lock()
	pm.nativeConnectorPackages = make(map[string]*InstalledConnectorPackage)
	pm.loadedNativeConnectors = make(map[string]string)
	pm.nativeMu.Unlock()
}

func cloneInstalledConnectorPackage(pkg *InstalledConnectorPackage) *InstalledConnectorPackage {
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

func (pm *PluginManager) getNativeConnectorPackage(connectorID string) *InstalledConnectorPackage {
	pm.nativeMu.RLock()
	defer pm.nativeMu.RUnlock()
	return cloneInstalledConnectorPackage(pm.nativeConnectorPackages[connectorID])
}

func (pm *PluginManager) setNativeConnectorPackage(pkg *InstalledConnectorPackage) {
	pm.nativeMu.Lock()
	defer pm.nativeMu.Unlock()
	pm.nativeConnectorPackages[pkg.ConnectorID] = cloneInstalledConnectorPackage(pkg)
}

func (pm *PluginManager) removeNativeConnectorPackage(connectorID string) {
	pm.nativeMu.Lock()
	defer pm.nativeMu.Unlock()
	delete(pm.nativeConnectorPackages, connectorID)
	delete(pm.loadedNativeConnectors, connectorID)
}

func (pm *PluginManager) getLoadedNativeConnectorVersion(connectorID string) (string, bool) {
	pm.nativeMu.RLock()
	defer pm.nativeMu.RUnlock()
	v, ok := pm.loadedNativeConnectors[connectorID]
	return v, ok
}

func (pm *PluginManager) hasConnectorInstances(ctx context.Context, instanceKeys []string) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(instanceKeys) == 0 {
		return false, nil
	}

	if pm.db != nil {
		args := make([]interface{}, len(instanceKeys))
		placeholders := make([]string, len(instanceKeys))
		for i, instanceKey := range instanceKeys {
			args[i] = instanceKey
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}

		query := fmt.Sprintf(`
			SELECT EXISTS (
				SELECT 1
				FROM connectors
				WHERE id IN (%s)
			)
		`, strings.Join(placeholders, ", "))

		var exists bool
		err := pm.db.QueryRowContext(ctx, query, args...).Scan(&exists)
		if err != nil {
			return false, fmt.Errorf("failed to check configured connector instances for %s: %w", strings.Join(instanceKeys, ", "), err)
		}
		if exists {
			return true, nil
		}
	}

	pm.connectorMu.RLock()
	defer pm.connectorMu.RUnlock()

	for _, instanceKey := range instanceKeys {
		if _, exists := pm.connectors[instanceKey]; exists {
			return true, nil
		}
	}

	return false, nil
}

func (pm *PluginManager) loadNativeConnectorPackage(pkg *InstalledConnectorPackage) error {
	safePath, pathErr := validateRuntimePathWithinRoot(pkg.RuntimePath, connectorRuntimeDir())
	if pathErr != nil {
		return fmt.Errorf("connector runtime path rejected: %w", pathErr)
	}
	pkg.RuntimePath = safePath

	if expected := strings.TrimSpace(pkg.Checksum); expected != "" {
		actual, err := hashFileSHA256(safePath)
		if err != nil {
			return fmt.Errorf("failed to hash connector runtime library: %w", err)
		}
		if !strings.EqualFold(expected, actual) {
			return fmt.Errorf("connector runtime library checksum mismatch: expected %s, got %s", expected, actual)
		}
	}

	definition, err := nativeConnectorDefinitionLoader(pkg.RuntimePath)
	if err != nil {
		return err
	}

	if definition.ID == "" {
		definition.ID = pkg.ConnectorID
	}
	if definition.ID != pkg.ConnectorID {
		return fmt.Errorf("native connector definition ID %s does not match manifest ID %s", definition.ID, pkg.ConnectorID)
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

	if err := pm.connectorRegistry.RegisterConnector(definition); err != nil {
		return fmt.Errorf("failed to register native connector: %w", err)
	}

	pm.nativeMu.Lock()
	pm.loadedNativeConnectors[pkg.ConnectorID] = pkg.Version
	pm.nativeMu.Unlock()

	return nil
}

func (pm *PluginManager) enrichConnectorDefinition(definition ConnectorDefinition) ConnectorDefinition {
	if definition.Source == "" {
		definition.Source = PluginSourceBundled
	}
	if definition.Source != PluginSourceNative {
		return definition
	}

	if pkg := pm.getNativeConnectorPackage(definition.ID); pkg != nil {
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

	return definition
}

// ListInstalledConnectorPackages returns persisted native connector packages.
func (pm *PluginManager) ListInstalledConnectorPackages() []*InstalledConnectorPackage {
	pm.nativeMu.RLock()
	defer pm.nativeMu.RUnlock()

	out := make([]*InstalledConnectorPackage, 0, len(pm.nativeConnectorPackages))
	for _, pkg := range pm.nativeConnectorPackages {
		out = append(out, cloneInstalledConnectorPackage(pkg))
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Name < out[j].Name
	})

	return out
}

// InstallConnectorPackageFromBundle installs a native connector from a zip bundle (super-admin upload).
func (pm *PluginManager) InstallConnectorPackageFromBundle(ctx context.Context, archive io.ReaderAt, size int64, originalName string) (*InstalledConnectorPackage, error) {
	return pm.installConnectorBundle(ctx, archive, size, originalName, PluginDistributionSideload)
}

func (pm *PluginManager) installConnectorBundle(ctx context.Context, archive io.ReaderAt, size int64, originalName string, distribution PluginDistribution) (*InstalledConnectorPackage, error) {
	_ = ctx
	pm.installMu.Lock()
	defer pm.installMu.Unlock()

	if !nativePluginsEnabled() {
		return nil, fmt.Errorf("native plugins and connectors are disabled")
	}

	if err := os.MkdirAll(connectorRuntimeDir(), 0o750); err != nil {
		return nil, fmt.Errorf("failed to create connector runtime directory: %w", err)
	}

	manifest, selectedTarget, manifestBytes, signatureBytes, publicKeyBytes, libraryBytes, libraryName, err := readConnectorBundle(archive, size)
	if err != nil {
		return nil, err
	}

	if err := validateConnectorManifest(&manifest); err != nil {
		return nil, err
	}
	if err := validateConnectorCompatibility(manifest, selectedTarget); err != nil {
		return nil, err
	}

	if existingDefinition, err := pm.connectorRegistry.GetConnector(manifest.ConnectorID); err == nil {
		if existingDefinition.Source == PluginSourceBundled {
			return nil, fmt.Errorf("connector %s conflicts with a bundled connector", manifest.ConnectorID)
		}
	}

	checksum := fmt.Sprintf("%x", sha256.Sum256(libraryBytes))
	if !strings.EqualFold(selectedTarget.SHA256, checksum) {
		return nil, fmt.Errorf("connector checksum mismatch")
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

	idSegment, versionSegment, err := pluginRuntimeSegments(manifest.ConnectorID, manifest.Version)
	if err != nil {
		return nil, err
	}

	connectorDir := filepath.Join(connectorRuntimeDir(), idSegment, versionSegment)
	runtimePath := filepath.Join(connectorDir, sanitizeRuntimeSegment(filepath.Base(libraryName)))
	if filepath.Ext(runtimePath) != ".so" {
		runtimePath = filepath.Join(connectorDir, idSegment+".so")
	}

	if err := os.MkdirAll(filepath.Dir(runtimePath), 0o750); err != nil {
		return nil, fmt.Errorf("failed to create connector directory: %w", err)
	}
	if err := writeRuntimeLibrary(runtimePath, libraryBytes); err != nil {
		return nil, err
	}

	pkgSource := PluginSourceNative

	now := time.Now()
	pkg := &InstalledConnectorPackage{
		ConnectorID:       manifest.ConnectorID,
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
	applyInstalledConnectorTarget(pkg, selectedTarget)

	if existing := pm.getNativeConnectorPackage(manifest.ConnectorID); existing != nil {
		pkg.CreatedAt = existing.CreatedAt
	}

	if loadedVersion, loaded := pm.getLoadedNativeConnectorVersion(manifest.ConnectorID); loaded {
		if loadedVersion == manifest.Version {
			if existing := pm.getNativeConnectorPackage(manifest.ConnectorID); existing != nil && existing.Checksum == checksum {
				return existing, nil
			}
		}

		pkg.InstallState = PluginInstallStatePendingRestart
		pkg.LastError = "Restart Aegis to activate the updated connector package"
	} else if err := pm.loadNativeConnectorPackage(pkg); err != nil {
		pkg.InstallState = PluginInstallStateError
		pkg.LastError = err.Error()
	}

	if err := pm.saveConnectorPackageToDatabase(pkg); err != nil {
		return nil, err
	}
	pm.setNativeConnectorPackage(pkg)

	return cloneInstalledConnectorPackage(pkg), nil
}

// DeleteInstalledConnectorPackage removes a native connector package from disk and registry.
func (pm *PluginManager) DeleteInstalledConnectorPackage(ctx context.Context, connectorID string) error {
	pm.installMu.Lock()
	defer pm.installMu.Unlock()
	pkg := pm.getNativeConnectorPackage(connectorID)
	if pkg == nil {
		return fmt.Errorf("connector package %s not found", connectorID)
	}

	instanceKeys := connectorInstanceKeysForPackage(pkg)
	if hasInstances, err := pm.hasConnectorInstances(ctx, instanceKeys); err != nil {
		return err
	} else if hasInstances {
		instanceKey := connectorID
		if len(instanceKeys) > 0 {
			instanceKey = instanceKeys[0]
		}
		return fmt.Errorf("connector %s still has configured instances; delete the instance first", instanceKey)
	}

	if err := pm.deleteConnectorPackageFromDatabase(connectorID); err != nil {
		return err
	}

	pm.connectorRegistry.UnregisterConnector(connectorID)
	pm.removeNativeConnectorPackage(connectorID)

	if pkg.RuntimePath != "" {
		safePath, pathErr := validateRuntimePathWithinRoot(pkg.RuntimePath, connectorRuntimeDir())
		if pathErr != nil {
			log.Warn().Err(pathErr).Str("connector_id", connectorID).Str("path", pkg.RuntimePath).Msg("Refusing to remove native connector library outside runtime root")
		} else if err := os.Remove(safePath); err != nil && !os.IsNotExist(err) {
			log.Warn().Err(err).Str("connector_id", connectorID).Str("path", safePath).Msg("Failed to remove native connector library")
		}
	}

	return nil
}
