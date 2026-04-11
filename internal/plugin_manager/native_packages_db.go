package plugin_manager

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

func (pm *PluginManager) loadInstalledPluginPackages() error {
	if !nativePluginsEnabled() {
		return nil
	}

	rows, err := pm.db.Query(`
		SELECT plugin_id, name, description, version, source, distribution, official, install_state,
		       runtime_path, manifest_json, signature_verified, unsafe, checksum, min_host_api_version,
		       required_capabilities, target_os, target_arch, last_error, created_at, updated_at,
		       manifest_signature, manifest_public_key
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
		var manifestSignature sql.NullString
		var manifestPublicKey sql.NullString

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
			&manifestSignature,
			&manifestPublicKey,
		); err != nil {
			return fmt.Errorf("failed to scan plugin package row: %w", err)
		}

		pkg.ManifestJSON = append(json.RawMessage(nil), manifestJSON...)
		if manifestSignature.Valid {
			pkg.ManifestSignature = []byte(manifestSignature.String)
		}
		if manifestPublicKey.Valid {
			pkg.ManifestPublicKey = []byte(manifestPublicKey.String)
		}
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

		// Recompute trust from stored signature material. The stored
		// signature_verified flag is not authoritative on its own.
		reverified := false
		if len(pkg.ManifestSignature) > 0 && len(pkg.ManifestPublicKey) > 0 && len(manifestJSON) > 0 {
			ok, verifyErr := verifyManifestSignature(manifestJSON, pkg.ManifestSignature, pkg.ManifestPublicKey)
			if verifyErr != nil {
				log.Warn().Err(verifyErr).Str("plugin_id", pkg.PluginID).Msg("Stored plugin signature no longer verifies against trust store")
			}
			reverified = ok
		}
		pkg.SignatureVerified = reverified
		pkg.Unsafe = !reverified
		if !reverified && !allowUnsafeSideload() {
			if pkg.InstallState == PluginInstallStateReady || pkg.InstallState == PluginInstallStatePendingRestart {
				pkg.InstallState = PluginInstallStateError
				pkg.LastError = "plugin signature cannot be re-verified against trusted keys"
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

		var loadErr error
		loadErr = pm.loadNativePluginPackage(pkg)
		if loadErr != nil {
			pkg.InstallState = PluginInstallStateError
			pkg.LastError = loadErr.Error()
			pkg.UpdatedAt = time.Now()
			if saveErr := pm.savePluginPackageToDatabase(pkg); saveErr != nil {
				log.Error().Err(saveErr).Str("plugin_id", pkg.PluginID).Msg("Failed to persist sideloaded plugin load error")
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
			required_capabilities, target_os, target_arch, last_error, created_at, updated_at,
			manifest_signature, manifest_public_key
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14,
			$15, $16, $17, $18, $19, $20,
			$21, $22
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
			updated_at = EXCLUDED.updated_at,
			manifest_signature = EXCLUDED.manifest_signature,
			manifest_public_key = EXCLUDED.manifest_public_key
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
		string(pkg.ManifestSignature),
		string(pkg.ManifestPublicKey),
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
