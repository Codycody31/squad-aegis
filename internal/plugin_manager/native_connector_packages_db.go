package plugin_manager

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

func (pm *PluginManager) loadInstalledConnectorPackages() error {
	if !nativePluginsEnabled() {
		return nil
	}
	if pm.db == nil {
		return nil
	}

	ctx := pm.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	rows, err := pm.db.QueryContext(ctx, `
		SELECT connector_id, name, description, version, source, distribution, official, install_state,
		       runtime_path, manifest_json, signature_verified, unsafe, checksum, min_host_api_version,
		       required_capabilities, target_os, target_arch, last_error, created_at, updated_at,
		       manifest_signature, manifest_public_key
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
		var manifestSignature sql.NullString
		var manifestPublicKey sql.NullString

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
			&manifestSignature,
			&manifestPublicKey,
		); err != nil {
			return fmt.Errorf("failed to scan connector package row: %w", err)
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
			} else if target, targetErr := selectedConnectorManifestTarget(pkg.Manifest); targetErr != nil {
				pkg.InstallState = PluginInstallStateError
				pkg.LastError = targetErr.Error()
			} else {
				applyInstalledConnectorTarget(&pkg, target)
			}
		}

		reverified := false
		if len(pkg.ManifestSignature) > 0 && len(pkg.ManifestPublicKey) > 0 && len(manifestJSON) > 0 {
			ok, verifyErr := verifyManifestSignature(manifestJSON, pkg.ManifestSignature, pkg.ManifestPublicKey)
			if verifyErr != nil {
				log.Warn().Err(verifyErr).Str("connector_id", pkg.ConnectorID).Msg("Stored connector signature no longer verifies against trust store")
			}
			reverified = ok
		}
		pkg.SignatureVerified = reverified
		pkg.Unsafe = !reverified
		if !reverified && !allowUnsafeSideload() {
			if pkg.InstallState == PluginInstallStateReady || pkg.InstallState == PluginInstallStatePendingRestart {
				log.Warn().Str("connector_id", pkg.ConnectorID).Msg("Quarantining native connector: stored signature cannot be re-verified against trusted keys; runtime file will be removed")
				pkg.InstallState = PluginInstallStateError
				pkg.LastError = "connector signature cannot be re-verified against trusted keys"
				if pkg.RuntimePath != "" {
					if safePath, pathErr := validateRuntimePathWithinRoot(pkg.RuntimePath, connectorRuntimeDir()); pathErr == nil {
						removeRuntimeFile(safePath)
					}
				}
			}
		}

		loaded[pkg.ConnectorID] = &pkg
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating connector package rows: %w", err)
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

		loadErr := pm.loadNativeConnectorPackage(pkg)
		if loadErr != nil {
			pkg.InstallState = PluginInstallStateError
			pkg.LastError = loadErr.Error()
			pkg.UpdatedAt = time.Now()
			if saveErr := pm.saveConnectorPackageToDatabaseContext(ctx, pkg); saveErr != nil {
				return fmt.Errorf("failed to persist load error for connector %s: %w (load error: %v)", pkg.ConnectorID, saveErr, loadErr)
			}
			pm.setNativeConnectorPackage(pkg)
			continue
		}

		if shouldPersistReadyState || pkg.LastError != "" {
			pkg.LastError = ""
			pkg.UpdatedAt = time.Now()
			if saveErr := pm.saveConnectorPackageToDatabaseContext(ctx, pkg); saveErr != nil {
				return fmt.Errorf("failed to persist native connector package activation for %s: %w", pkg.ConnectorID, saveErr)
			}
			pm.setNativeConnectorPackage(pkg)
		}

		log.Info().Str("connector_id", pkg.ConnectorID).Str("version", pkg.Version).Bool("signature_verified", pkg.SignatureVerified).Msg("Loaded native connector package")
	}

	return nil
}

func (pm *PluginManager) saveConnectorPackageToDatabase(pkg *InstalledConnectorPackage) error {
	return pm.saveConnectorPackageToDatabaseContext(context.Background(), pkg)
}

func (pm *PluginManager) saveConnectorPackageToDatabaseContext(ctx context.Context, pkg *InstalledConnectorPackage) error {
	if ctx == nil {
		ctx = context.Background()
	}
	manifestJSON := pkg.ManifestJSON
	if len(manifestJSON) == 0 {
		var err error
		manifestJSON, err = json.Marshal(pkg.Manifest)
		if err != nil {
			return fmt.Errorf("failed to marshal connector manifest: %w", err)
		}
	}

	_, err := pm.db.ExecContext(ctx, `
		INSERT INTO connector_packages (
			connector_id, name, description, version, source, distribution, official, install_state,
			runtime_path, manifest_json, signature_verified, unsafe, checksum, min_host_api_version,
			required_capabilities, target_os, target_arch, last_error, created_at, updated_at,
			manifest_signature, manifest_public_key
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14,
			$15, $16, $17, $18, $19, $20,
			$21, $22
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
			updated_at = EXCLUDED.updated_at,
			manifest_signature = EXCLUDED.manifest_signature,
			manifest_public_key = EXCLUDED.manifest_public_key
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
		string(pkg.ManifestSignature),
		string(pkg.ManifestPublicKey),
	)
	if err != nil {
		return fmt.Errorf("failed to upsert connector package: %w", err)
	}

	return nil
}

func (pm *PluginManager) deleteConnectorPackageFromDatabase(connectorID string) error {
	return pm.deleteConnectorPackageFromDatabaseContext(context.Background(), connectorID)
}

func (pm *PluginManager) deleteConnectorPackageFromDatabaseContext(ctx context.Context, connectorID string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	_, err := pm.db.ExecContext(ctx, `DELETE FROM connector_packages WHERE connector_id = $1`, connectorID)
	if err != nil {
		return fmt.Errorf("failed to delete connector package: %w", err)
	}
	return nil
}
