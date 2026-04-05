package plugin_manager

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
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

	Author         string                          `json:"author,omitempty"`
	InstanceKey    string                          `json:"instance_key,omitempty"`
	LegacyIDs      []string                        `json:"legacy_ids,omitempty"`
	ConfigSchema   plug_config_schema.ConfigSchema `json:"config_schema,omitempty"`
}

func (m ConnectorPackageManifest) asPluginManifest() PluginPackageManifest {
	return PluginPackageManifest{
		PluginID:        m.ConnectorID,
		Name:            m.Name,
		Description:     m.Description,
		Version:         m.Version,
		Official:        m.Official,
		License:         m.License,
		EntrySymbol:     m.EntrySymbol,
		Targets:         m.Targets,
		Author:          m.Author,
		ConfigSchema:    m.ConfigSchema,
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

func selectedConnectorManifestTarget(manifest ConnectorPackageManifest) (PluginPackageTarget, error) {
	return selectedManifestTarget(manifest.asPluginManifest())
}

func validateConnectorManifest(manifest *ConnectorPackageManifest) error {
	if strings.TrimSpace(manifest.ConnectorID) == "" {
		return fmt.Errorf("connector manifest is missing connector_id")
	}
	if manifest.Name == "" {
		return fmt.Errorf("connector manifest is missing name")
	}
	if manifest.Version == "" {
		return fmt.Errorf("connector manifest is missing version")
	}
	if _, _, err := pluginRuntimeSegments(manifest.ConnectorID, manifest.Version); err != nil {
		return err
	}
	if manifest.EntrySymbol == "" {
		manifest.EntrySymbol = nativeConnectorEntrySymbol
	}
	if manifest.EntrySymbol != nativeConnectorEntrySymbol {
		return fmt.Errorf("unsupported connector entry symbol %s", manifest.EntrySymbol)
	}

	targets := clonePluginPackageTargets(manifest.Targets)
	if len(targets) == 0 {
		return fmt.Errorf("connector manifest is missing targets")
	}

	seenTargets := make(map[string]struct{}, len(targets))
	for index, target := range targets {
		if target.MinHostAPIVersion <= 0 {
			return fmt.Errorf("connector manifest target %d is missing min_host_api_version", index)
		}
		if target.TargetOS == "" {
			return fmt.Errorf("connector manifest target %d is missing target_os", index)
		}
		if target.TargetArch == "" {
			return fmt.Errorf("connector manifest target %d is missing target_arch", index)
		}
		if strings.TrimSpace(target.LibraryPath) == "" {
			return fmt.Errorf("connector manifest target %d is missing library_path", index)
		}
		requiredCapabilitySet := make(map[string]struct{}, len(target.RequiredCapabilities))
		requiredCapabilities := cloneRequiredCapabilities(target.RequiredCapabilities)
		sort.Strings(requiredCapabilities)
		for _, capability := range requiredCapabilities {
			capability = strings.TrimSpace(capability)
			if capability == "" {
				return fmt.Errorf("connector manifest target %d has an empty required capability", index)
			}
			if _, exists := requiredCapabilitySet[capability]; exists {
				return fmt.Errorf("connector manifest target %d repeats required capability %s", index, capability)
			}
			requiredCapabilitySet[capability] = struct{}{}
		}

		key := target.TargetOS + "|" + target.TargetArch + "|" + strconv.Itoa(target.MinHostAPIVersion) + "|" + strings.Join(requiredCapabilities, ",")
		if _, exists := seenTargets[key]; exists {
			return fmt.Errorf("connector manifest has a duplicate target for %s/%s with min_host_api_version %d and capabilities %s", target.TargetOS, target.TargetArch, target.MinHostAPIVersion, strings.Join(requiredCapabilities, ","))
		}
		seenTargets[key] = struct{}{}
	}

	return nil
}

func validateConnectorCompatibility(manifest ConnectorPackageManifest, target PluginPackageTarget) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("native connectors are only supported on Linux")
	}
	if target.TargetOS != runtime.GOOS {
		return fmt.Errorf("connector targets %s but host is %s", target.TargetOS, runtime.GOOS)
	}
	if target.TargetArch != runtime.GOARCH {
		return fmt.Errorf("connector targets %s but host architecture is %s", target.TargetArch, runtime.GOARCH)
	}
	if target.MinHostAPIVersion > NativeConnectorHostAPIVersion {
		return fmt.Errorf("connector requires host API version >= %d, but host provides %d", target.MinHostAPIVersion, NativeConnectorHostAPIVersion)
	}
	if missing := missingRequiredCapabilities(target.RequiredCapabilities); len(missing) > 0 {
		return fmt.Errorf("connector requires unsupported host capabilities: %s", strings.Join(missing, ", "))
	}
	return nil
}

func readConnectorBundle(archive io.ReaderAt, size int64) (ConnectorPackageManifest, PluginPackageTarget, []byte, []byte, []byte, []byte, string, error) {
	reader, err := zip.NewReader(archive, size)
	if err != nil {
		return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("invalid connector archive: %w", err)
	}

	var manifest ConnectorPackageManifest
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
			return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("connector archive contains an unsafe path: %s", file.Name)
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("connector archive contains an unsupported symlink: %s", file.Name)
		}

		switch filepath.Base(name) {
		case pluginManifestFile:
			manifestFile = file
		case pluginSignatureFile:
			signatureFile = file
		case pluginPublicKeyFile:
			publicKeyFile = file
		default:
			lower := strings.ToLower(name)
			if strings.HasSuffix(lower, ".so") {
				libraries[name] = file
			}
		}
	}

	if manifestFile == nil {
		return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("connector archive is missing %s", pluginManifestFile)
	}

	budget := newPluginArchiveReadBudget()

	manifestBytes, err = budget.read(manifestFile)
	if err != nil {
		return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("invalid connector manifest: %w", err)
	}

	selectedTarget, err := selectedConnectorManifestTarget(manifest)
	if err != nil {
		return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
	}

	if signatureFile != nil {
		signatureBytes, err = budget.read(signatureFile)
		if err != nil {
			return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
		}
	}
	if publicKeyFile != nil {
		publicKeyBytes, err = budget.read(publicKeyFile)
		if err != nil {
			return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
		}
	}

	hasSignature := len(bytes.TrimSpace(signatureBytes)) > 0
	hasPublicKey := len(bytes.TrimSpace(publicKeyBytes)) > 0
	if hasSignature != hasPublicKey {
		return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("connector archive must include %s and %s together", pluginSignatureFile, pluginPublicKeyFile)
	}

	libraryName, libraryFile, err := selectManifestLibrary(manifest.asPluginManifest(), selectedTarget, libraries)
	if err != nil {
		return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
	}
	libraryBytes, err := budget.read(libraryFile)
	if err != nil {
		return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
	}

	return manifest, selectedTarget, manifestBytes, signatureBytes, publicKeyBytes, libraryBytes, libraryName, nil
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

func (pm *PluginManager) loadInstalledConnectorPackages() error {
	if !nativePluginsEnabled() {
		return nil
	}
	if pm.db == nil {
		return nil
	}

	rows, err := pm.db.Query(`
		SELECT connector_id, name, description, version, source, distribution, official, install_state,
		       runtime_path, manifest_json, signature_verified, unsafe, checksum, min_host_api_version,
		       required_capabilities, target_os, target_arch, last_error, created_at, updated_at
		FROM connector_packages
		ORDER BY created_at
	`)
	if err != nil {
		return fmt.Errorf("failed to query connector packages: %w", err)
	}
	defer rows.Close()

	loaded := make(map[string]*InstalledConnectorPackage)

	for rows.Next() {
		var pkg InstalledConnectorPackage
		var manifestJSON []byte
		var requiredCapabilitiesJSON []byte

		if err := rows.Scan(
			&pkg.ConnectorID,
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
			return fmt.Errorf("failed to scan connector package row: %w", err)
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
			} else if target, targetErr := selectedConnectorManifestTarget(pkg.Manifest); targetErr != nil {
				pkg.InstallState = PluginInstallStateError
				pkg.LastError = targetErr.Error()
			} else {
				applyInstalledConnectorTarget(&pkg, target)
			}
		}

		loaded[pkg.ConnectorID] = &pkg
	}

	pm.resetNativeConnectorRuntimeState()

	pm.nativeMu.Lock()
	pm.nativeConnectorPackages = loaded
	pm.nativeMu.Unlock()

	for _, pkg := range loaded {
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

		var loadErr error
		loadErr = pm.loadNativeConnectorPackage(pkg)
		if loadErr != nil {
			pkg.InstallState = PluginInstallStateError
			pkg.LastError = loadErr.Error()
			pkg.UpdatedAt = time.Now()
			if saveErr := pm.saveConnectorPackageToDatabase(pkg); saveErr != nil {
				log.Error().Err(saveErr).Str("connector_id", pkg.ConnectorID).Msg("Failed to persist native connector load error")
			}
			pm.setNativeConnectorPackage(pkg)
			continue
		}

		if shouldPersistReadyState || pkg.LastError != "" {
			pkg.LastError = ""
			pkg.UpdatedAt = time.Now()
			if saveErr := pm.saveConnectorPackageToDatabase(pkg); saveErr != nil {
				log.Error().Err(saveErr).Str("connector_id", pkg.ConnectorID).Msg("Failed to persist native connector package activation")
			}
			pm.setNativeConnectorPackage(pkg)
		}
	}

	return nil
}

func (pm *PluginManager) saveConnectorPackageToDatabase(pkg *InstalledConnectorPackage) error {
	manifestJSON := pkg.ManifestJSON
	if len(manifestJSON) == 0 {
		var err error
		manifestJSON, err = json.Marshal(pkg.Manifest)
		if err != nil {
			return fmt.Errorf("failed to marshal connector manifest: %w", err)
		}
	}

	_, err := pm.db.Exec(`
		INSERT INTO connector_packages (
			connector_id, name, description, version, source, distribution, official, install_state,
			runtime_path, manifest_json, signature_verified, unsafe, checksum, min_host_api_version,
			required_capabilities, target_os, target_arch, last_error, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14,
			$15, $16, $17, $18, $19, $20
		)
		ON CONFLICT (connector_id) DO UPDATE SET
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
		pkg.ConnectorID,
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
		return fmt.Errorf("failed to upsert connector package: %w", err)
	}

	return nil
}

func (pm *PluginManager) deleteConnectorPackageFromDatabase(connectorID string) error {
	_, err := pm.db.Exec(`DELETE FROM connector_packages WHERE connector_id = $1`, connectorID)
	if err != nil {
		return fmt.Errorf("failed to delete connector package: %w", err)
	}
	return nil
}

func (pm *PluginManager) loadNativeConnectorPackage(pkg *InstalledConnectorPackage) error {
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
	if !nativePluginsEnabled() {
		return nil, fmt.Errorf("native plugins and connectors are disabled")
	}

	if err := os.MkdirAll(pluginRuntimeDir(), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create plugin runtime directory: %w", err)
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
	if selectedTarget.SHA256 != "" && !strings.EqualFold(selectedTarget.SHA256, checksum) {
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

	connectorDir := filepath.Join(pluginRuntimeDir(), "connectors", idSegment, versionSegment)
	runtimePath := filepath.Join(connectorDir, sanitizeRuntimeSegment(filepath.Base(libraryName)))
	if filepath.Ext(runtimePath) != ".so" {
		runtimePath = filepath.Join(connectorDir, idSegment+".so")
	}

	if err := os.MkdirAll(filepath.Dir(runtimePath), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create connector directory: %w", err)
	}
	if err := os.WriteFile(runtimePath, libraryBytes, 0o755); err != nil {
		return nil, fmt.Errorf("failed to write connector library: %w", err)
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
	_ = ctx
	pkg := pm.getNativeConnectorPackage(connectorID)
	if pkg == nil {
		return fmt.Errorf("connector package %s not found", connectorID)
	}

	storageKey := connectorID
	if def, err := pm.connectorRegistry.GetConnector(connectorID); err == nil {
		storageKey = def.ConnectorInstanceStorageKey()
	}

	pm.connectorMu.RLock()
	_, hasInstance := pm.connectors[storageKey]
	pm.connectorMu.RUnlock()
	if hasInstance {
		return fmt.Errorf("connector %s still has a running instance; delete the instance first", storageKey)
	}

	if err := pm.deleteConnectorPackageFromDatabase(connectorID); err != nil {
		return err
	}

	pm.connectorRegistry.UnregisterConnector(connectorID)
	pm.removeNativeConnectorPackage(connectorID)

	if pkg.RuntimePath != "" {
		if err := os.Remove(pkg.RuntimePath); err != nil && !os.IsNotExist(err) {
			log.Warn().Err(err).Str("connector_id", connectorID).Str("path", pkg.RuntimePath).Msg("Failed to remove native connector library")
		}
	}

	return nil
}
