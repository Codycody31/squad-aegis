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

	"go.codycody31.dev/squad-aegis/pkg/connectorrpc"
)

// nativeConnectorVerifiedLoader is the subprocess-isolated counterpart of
// nativePluginVerifiedLoader for connector packages. The default production
// impl spawns a peek subprocess via hashicorp/go-plugin to fetch the
// static connector definition, then kills it; actual instances are spawned
// per-connector via the CreateInstance factory attached to the returned
// ConnectorDefinition.
var nativeConnectorVerifiedLoader = peekNativeConnectorDefinition

// peekNativeConnectorDefinition spawns a throwaway connector subprocess,
// fetches the wire definition, merges it with the identity fields from the
// signed manifest, kills the peek subprocess, and returns a host-typed
// definition whose CreateInstance factory spawns a fresh subprocess per
// instance.
func peekNativeConnectorDefinition(runtimePath, expectedSHA256 string, manifest ConnectorPackageManifest, target PluginPackageTarget) (ConnectorDefinition, error) {
	handle, err := nativeConnectorSubprocessLauncher(runtimePath, expectedSHA256)
	if err != nil {
		return ConnectorDefinition{}, err
	}
	defer handle.Kill()

	wire, err := handle.rpc.GetDefinition()
	if err != nil {
		return ConnectorDefinition{}, fmt.Errorf("failed to fetch connector definition: %w", err)
	}

	// Guard against adversarial config schemas with unbounded nesting depth.
	const maxConfigFieldDepth = 10
	if err := validateConnectorConfigFieldDepth(wire.ConfigSchema.Fields, maxConfigFieldDepth); err != nil {
		return ConnectorDefinition{}, fmt.Errorf("connector %q: %w", manifest.ConnectorID, err)
	}

	if wire.ConnectorID != manifest.ConnectorID {
		return ConnectorDefinition{}, fmt.Errorf("connector subprocess reported id %q but manifest declares %q", wire.ConnectorID, manifest.ConnectorID)
	}

	hostDef, err := mergeWireConnectorIntoHost(wire, manifest, target)
	if err != nil {
		return ConnectorDefinition{}, err
	}

	captured := hostDef
	captured.CreateInstance = func() Connector {
		return &subprocessConnectorShim{
			connectorID:  captured.ID,
			definition:   captured,
			runtimePath:  runtimePath,
			expectedHash: expectedSHA256,
			status:       ConnectorStatusStopped,
		}
	}
	return captured, nil
}

// validateConnectorConfigFieldDepth mirrors validateConfigFieldDepth for the
// connectorrpc.ConfigField type, preventing adversarial deeply-nested schemas.
func validateConnectorConfigFieldDepth(fields []connectorrpc.ConfigField, maxDepth int) error {
	if maxDepth <= 0 {
		return fmt.Errorf("config schema exceeds maximum nesting depth")
	}
	for _, f := range fields {
		if len(f.Nested) > 0 {
			if err := validateConnectorConfigFieldDepth(f.Nested, maxDepth-1); err != nil {
				return err
			}
		}
	}
	return nil
}

// ConnectorPackageManifest is the signed manifest.json shipped with every
// native connector bundle. Like PluginPackageManifest, it carries only
// identity and distribution metadata. Runtime behavior (config schema)
// lives in the connector binary and is fetched via connectorrpc at load
// time. LegacyIDs and InstanceKey are identity-level migration helpers
// and belong with the other distribution metadata.
type ConnectorPackageManifest struct {
	ConnectorID string                `json:"connector_id"`
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Version     string                `json:"version"`
	Author      string                `json:"author,omitempty"`
	License     string                `json:"license,omitempty"`
	Official    bool                  `json:"official,omitempty"`
	InstanceKey string                `json:"instance_key,omitempty"`
	LegacyIDs   []string              `json:"legacy_ids,omitempty"`
	Targets     []PluginPackageTarget `json:"targets"`
}

func (m ConnectorPackageManifest) asPluginManifest() PluginPackageManifest {
	return PluginPackageManifest{
		PluginID:    m.ConnectorID,
		Name:        m.Name,
		Description: m.Description,
		Version:     m.Version,
		Author:      m.Author,
		License:     m.License,
		Official:    m.Official,
		Targets:     m.Targets,
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
	pm.connectorMu.RLock()
	defer pm.connectorMu.RUnlock()
	return pm.hasConnectorInstancesLocked(ctx, instanceKeys)
}

// hasConnectorInstancesLocked is the lock-free helper used when the caller
// already holds pm.connectorMu (read or write). The connector delete flow
// holds it as a writer to block concurrent CreateConnectorInstance.
func (pm *PluginManager) hasConnectorInstancesLocked(ctx context.Context, instanceKeys []string) (bool, error) {
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

	// Reconstruct the target snapshot from the stored compatibility fields.
	target := PluginPackageTarget{
		MinHostAPIVersion:    pkg.MinHostAPIVersion,
		RequiredCapabilities: cloneRequiredCapabilities(pkg.RequiredCapabilities),
		TargetOS:             pkg.TargetOS,
		TargetArch:           pkg.TargetArch,
	}

	definition, err := nativeConnectorVerifiedLoader(safePath, pkg.Checksum, pkg.Manifest, target)
	if err != nil {
		return err
	}

	// Overlay install-time state the manifest does not carry.
	definition.InstallState = pkg.InstallState
	definition.Distribution = pkg.Distribution
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
	pm.installMu.Lock()
	defer pm.installMu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

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
	runtimeBase := sanitizeRuntimeSegment(filepath.Base(libraryName))
	if runtimeBase == "" {
		runtimeBase = idSegment
	}
	runtimePath := filepath.Join(connectorDir, runtimeBase)

	if err := os.MkdirAll(filepath.Dir(runtimePath), 0o750); err != nil {
		return nil, fmt.Errorf("failed to create connector directory: %w", err)
	}

	// Same-bytes short circuit before writing.
	if loadedVersion, loaded := pm.getLoadedNativeConnectorVersion(manifest.ConnectorID); loaded && loadedVersion == manifest.Version {
		if existing := pm.getNativeConnectorPackage(manifest.ConnectorID); existing != nil && existing.Checksum == checksum {
			log.Info().Str("connector_id", manifest.ConnectorID).Str("version", manifest.Version).Msg("Skipping native connector re-install: bytes match already-loaded package")
			return existing, nil
		}
	}

	if err := writeRuntimeLibrary(runtimePath, libraryBytes); err != nil {
		return nil, err
	}
	installCommitted := false
	defer func() {
		if !installCommitted {
			if safePath, pathErr := validateRuntimePathWithinRoot(runtimePath, connectorRuntimeDir()); pathErr != nil {
				log.Warn().Err(pathErr).Str("path", runtimePath).Msg("Refusing to roll back runtime file outside connector runtime root")
			} else {
				removeRuntimeFile(safePath)
			}
		}
	}()

	now := time.Now()
	pkg := &InstalledConnectorPackage{
		ConnectorID:       manifest.ConnectorID,
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
	applyInstalledConnectorTarget(pkg, selectedTarget)

	if existing := pm.getNativeConnectorPackage(manifest.ConnectorID); existing != nil {
		pkg.CreatedAt = existing.CreatedAt
	}

	loadFailureRecorded := false
	// If a previous version is already loaded, unload it first so the new
	// version can take its place immediately. With subprocess-based connectors
	// (hashicorp/go-plugin) there is no in-process state to worry about.
	if _, loaded := pm.getLoadedNativeConnectorVersion(manifest.ConnectorID); loaded {
		pm.connectorRegistry.UnregisterConnector(manifest.ConnectorID)
		pm.nativeMu.Lock()
		delete(pm.loadedNativeConnectors, manifest.ConnectorID)
		pm.nativeMu.Unlock()
	}
	if err := pm.loadNativeConnectorPackage(pkg); err != nil {
		pm.connectorRegistry.UnregisterConnector(manifest.ConnectorID)
		pkg.InstallState = PluginInstallStateError
		pkg.LastError = err.Error()
		loadFailureRecorded = true
		log.Warn().Err(err).Str("connector_id", manifest.ConnectorID).Str("version", manifest.Version).Msg("Failed to load uploaded native connector package")
	} else {
		pkg.InstallState = PluginInstallStateReady
		pkg.LastError = ""
	}
	pkg.UpdatedAt = time.Now()

	if err := pm.saveConnectorPackageToDatabaseContext(ctx, pkg); err != nil {
		// Roll back the live registry so it does not outlive the (now-rolled-back) DB row.
		pm.connectorRegistry.UnregisterConnector(manifest.ConnectorID)
		pm.nativeMu.Lock()
		delete(pm.loadedNativeConnectors, manifest.ConnectorID)
		pm.nativeMu.Unlock()
		return nil, fmt.Errorf("failed to persist connector package state: %w", err)
	}

	pm.setNativeConnectorPackage(pkg)
	installCommitted = true
	if loadFailureRecorded {
		log.Info().Str("connector_id", pkg.ConnectorID).Str("version", pkg.Version).Str("install_state", string(pkg.InstallState)).Msg("Persisted native connector package install error state")
	} else {
		log.Info().Str("connector_id", pkg.ConnectorID).Str("version", pkg.Version).Str("install_state", string(pkg.InstallState)).Bool("signature_verified", pkg.SignatureVerified).Msg("Installed native connector package")
	}

	return cloneInstalledConnectorPackage(pkg), nil
}

// DeleteInstalledConnectorPackage removes a native connector package from disk and registry.
func (pm *PluginManager) DeleteInstalledConnectorPackage(ctx context.Context, connectorID string) error {
	pm.installMu.Lock()
	defer pm.installMu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}

	pkg := pm.getNativeConnectorPackage(connectorID)
	if pkg == nil {
		return fmt.Errorf("connector package %s not found", connectorID)
	}

	instanceKeys := connectorInstanceKeysForPackage(pkg)

	// Hold connectorMu (write) for the existence check + DB delete to block
	// concurrent CreateConnectorInstance, closing the delete-during-create race.
	pm.connectorMu.Lock()
	hasInstances, err := pm.hasConnectorInstancesLocked(ctx, instanceKeys)
	if err != nil {
		pm.connectorMu.Unlock()
		return err
	}
	if hasInstances {
		pm.connectorMu.Unlock()
		instanceKey := connectorID
		if len(instanceKeys) > 0 {
			instanceKey = instanceKeys[0]
		}
		return fmt.Errorf("connector %s still has configured instances; delete the instance first", instanceKey)
	}

	if err := pm.deleteConnectorPackageFromDatabaseContext(ctx, connectorID); err != nil {
		pm.connectorMu.Unlock()
		return err
	}
	pm.connectorMu.Unlock()

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
