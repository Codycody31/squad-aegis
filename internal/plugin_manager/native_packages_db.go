package plugin_manager

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"
)

func (pm *PluginManager) loadInstalledPluginPackages() error {
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
		SELECT plugin_id, name, description, version, source, distribution, official, install_state,
		       runtime_path, manifest_json, signature_verified, unsafe, min_host_api_version,
		       required_capabilities, target_os, target_arch, last_error, created_at, updated_at,
		       manifest_signature, manifest_public_key, signed_manifest_json, signature_key_id,
		       signature_signed_at, signature_expires_at
		FROM plugin_packages
		ORDER BY created_at
	`)
	if err != nil {
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
		var signedManifestJSON sql.NullString
		var signatureKeyID sql.NullString
		var signatureSignedAt sql.NullTime
		var signatureExpiresAt sql.NullTime

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
			&pkg.MinHostAPIVersion,
			&requiredCapabilitiesJSON,
			&pkg.TargetOS,
			&pkg.TargetArch,
			&pkg.LastError,
			&pkg.CreatedAt,
			&pkg.UpdatedAt,
			&manifestSignature,
			&manifestPublicKey,
			&signedManifestJSON,
			&signatureKeyID,
			&signatureSignedAt,
			&signatureExpiresAt,
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
		if signedManifestJSON.Valid {
			pkg.SignedManifestJSON = []byte(signedManifestJSON.String)
		}
		if signatureKeyID.Valid {
			pkg.SignatureKeyID = signatureKeyID.String
		}
		if signatureSignedAt.Valid {
			pkg.SignatureSignedAt = signatureSignedAt.Time
		}
		if signatureExpiresAt.Valid {
			pkg.SignatureExpiresAt = signatureExpiresAt.Time
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
		// signature_verified flag is not authoritative on its own. A package
		// is "previously trusted" only if it actually has signature material
		// to re-verify; otherwise the flag is stale state we cannot act on.
		hasSignatureMaterial := len(pkg.ManifestSignature) > 0 && len(pkg.ManifestPublicKey) > 0 && len(pkg.SignedManifestJSON) > 0 && len(manifestJSON) > 0
		previouslyTrusted := pkg.SignatureVerified && hasSignatureMaterial
		var verification signatureVerification
		if hasSignatureMaterial {
			var verifyErr error
			verification, verifyErr = verifyManifestSignature(pkg.SignedManifestJSON, manifestJSON, pkg.ManifestSignature, pkg.ManifestPublicKey)
			if verifyErr != nil {
				log.Warn().Err(verifyErr).Str("plugin_id", pkg.PluginID).Msg("Stored plugin signature no longer verifies against trust store")
			}
			if verification.Payload.KeyID != "" {
				pkg.SignatureKeyID = verification.Payload.KeyID
				pkg.SignatureSignedAt = verification.Payload.SignedAt
				pkg.SignatureExpiresAt = verification.Payload.ExpiresAt
			}
		}
		pkg.SignatureVerified = verification.Verified
		pkg.Unsafe = !verification.Verified
		// A package previously stored as trusted that no longer re-verifies
		// indicates the operator's trust store revoked the signing key, the
		// signature expired, or the key_id was added to the CRL. Keep the
		// package quarantined regardless of allowUnsafeSideload so a revoked
		// or expired key cannot be silently re-loaded by flipping the
		// sideload flag. Operators must re-upload to un-quarantine.
		mustQuarantine := !verification.Verified && (previouslyTrusted || !allowUnsafeSideload())
		if mustQuarantine {
			if pkg.InstallState == PluginInstallStateReady || pkg.InstallState == PluginInstallStatePendingRestart {
				if previouslyTrusted {
					reason := formatVerificationFailure(verification.Payload)
					log.Warn().Str("plugin_id", pkg.PluginID).Str("reason", reason).Msg("Quarantining native plugin: previously trusted signature no longer verifies")
					pkg.LastError = reason
				} else {
					// Quarantine the row but DO NOT remove the runtime file.
					// Operators should re-upload to clear quarantine; binaries
					// remain on disk until they explicitly delete the package.
					// Removing the .so on every quarantine triggered the
					// down/up migration cycle hazard described in M-23 and
					// turned a recoverable trust-state mismatch into manual
					// re-upload work for the operator.
					log.Warn().Str("plugin_id", pkg.PluginID).Msg("Quarantining native plugin: stored signature cannot be re-verified against trusted keys; runtime file preserved for re-upload")
					pkg.LastError = "plugin signature cannot be re-verified against trusted keys"
				}
				pkg.InstallState = PluginInstallStateError
				if err := pm.savePluginPackageToDatabaseContext(context.Background(), &pkg); err != nil {
					log.Warn().Err(err).Str("plugin_id", pkg.PluginID).Msg("Failed to persist quarantine state to database")
				}
			}
		}

		loadedPackages[pkg.PluginID] = &pkg
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating plugin package rows: %w", err)
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
			// Promote any pending side-file written during a same-version
			// upgrade while running plugin instances kept the runtime path
			// pinned (M-12). Atomic rename means a partial host crash
			// cannot leave us with mixed bytes.
			if pkg.RuntimePath != "" {
				pendingPath := pkg.RuntimePath + ".pending"
				if _, statErr := os.Stat(pendingPath); statErr == nil {
					if safePending, pathErr := validateRuntimePathWithinRoot(pendingPath, pluginRuntimeDir()); pathErr == nil {
						if safeRuntime, runtimePathErr := validateRuntimePathWithinRoot(pkg.RuntimePath, pluginRuntimeDir()); runtimePathErr == nil {
							if renameErr := os.Rename(safePending, safeRuntime); renameErr == nil {
								log.Info().Str("plugin_id", pkg.PluginID).Str("path", safeRuntime).Msg("Promoted pending native plugin runtime file")
							} else {
								log.Warn().Err(renameErr).Str("plugin_id", pkg.PluginID).Str("from", safePending).Str("to", safeRuntime).Msg("Failed to promote pending native plugin runtime file")
							}
						}
					}
				}
			}
			pkg.InstallState = PluginInstallStateReady
			pkg.LastError = ""
			pkg.UpdatedAt = time.Now()
			shouldPersistReadyState = true
		}

		loadErr := pm.loadNativePluginPackage(pkg)
		if loadErr != nil {
			pkg.InstallState = PluginInstallStateError
			pkg.LastError = loadErr.Error()
			pkg.UpdatedAt = time.Now()
			if saveErr := pm.savePluginPackageToDatabaseContext(ctx, pkg); saveErr != nil {
				return fmt.Errorf("failed to persist load error for plugin %s: %w (load error: %v)", pkg.PluginID, saveErr, loadErr)
			}
			pm.setNativePackage(pkg)
			continue
		}

		if shouldPersistReadyState || pkg.LastError != "" {
			pkg.LastError = ""
			pkg.UpdatedAt = time.Now()
			if saveErr := pm.savePluginPackageToDatabaseContext(ctx, pkg); saveErr != nil {
				return fmt.Errorf("failed to persist native plugin package activation for %s: %w", pkg.PluginID, saveErr)
			}
			pm.setNativePackage(pkg)
		}

		log.Info().Str("plugin_id", pkg.PluginID).Str("version", pkg.Version).Bool("signature_verified", pkg.SignatureVerified).Msg("Loaded native plugin package")
	}

	return nil
}

func (pm *PluginManager) savePluginPackageToDatabase(pkg *InstalledPluginPackage) error {
	return pm.savePluginPackageToDatabaseContext(context.Background(), pkg)
}

func (pm *PluginManager) savePluginPackageToDatabaseContext(ctx context.Context, pkg *InstalledPluginPackage) error {
	if ctx == nil {
		ctx = context.Background()
	}
	manifestJSON := pkg.ManifestJSON
	if len(manifestJSON) == 0 {
		var err error
		manifestJSON, err = json.Marshal(pkg.Manifest)
		if err != nil {
			return fmt.Errorf("failed to marshal plugin manifest: %w", err)
		}
	}

	var signatureSignedAt sql.NullTime
	var signatureExpiresAt sql.NullTime
	if !pkg.SignatureSignedAt.IsZero() {
		signatureSignedAt = sql.NullTime{Time: pkg.SignatureSignedAt, Valid: true}
	}
	if !pkg.SignatureExpiresAt.IsZero() {
		signatureExpiresAt = sql.NullTime{Time: pkg.SignatureExpiresAt, Valid: true}
	}

	_, err := pm.db.ExecContext(ctx, `
		INSERT INTO plugin_packages (
			plugin_id, name, description, version, source, distribution, official, install_state,
			runtime_path, manifest_json, signature_verified, unsafe, min_host_api_version,
			required_capabilities, target_os, target_arch, last_error, created_at, updated_at,
			manifest_signature, manifest_public_key, signed_manifest_json, signature_key_id,
			signature_signed_at, signature_expires_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18, $19,
			$20, $21, $22, $23,
			$24, $25
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
			min_host_api_version = EXCLUDED.min_host_api_version,
			required_capabilities = EXCLUDED.required_capabilities,
			target_os = EXCLUDED.target_os,
			target_arch = EXCLUDED.target_arch,
			last_error = EXCLUDED.last_error,
			updated_at = EXCLUDED.updated_at,
			manifest_signature = EXCLUDED.manifest_signature,
			manifest_public_key = EXCLUDED.manifest_public_key,
			signed_manifest_json = EXCLUDED.signed_manifest_json,
			signature_key_id = EXCLUDED.signature_key_id,
			signature_signed_at = EXCLUDED.signature_signed_at,
			signature_expires_at = EXCLUDED.signature_expires_at
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
		pkg.MinHostAPIVersion,
		string(mustMarshalRequiredCapabilities(pkg.RequiredCapabilities)),
		pkg.TargetOS,
		pkg.TargetArch,
		pkg.LastError,
		pkg.CreatedAt,
		pkg.UpdatedAt,
		string(pkg.ManifestSignature),
		string(pkg.ManifestPublicKey),
		string(pkg.SignedManifestJSON),
		pkg.SignatureKeyID,
		signatureSignedAt,
		signatureExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert plugin package: %w", err)
	}

	return nil
}

func (pm *PluginManager) deletePluginPackageFromDatabase(pluginID string) error {
	return pm.deletePluginPackageFromDatabaseContext(context.Background(), pluginID)
}

func (pm *PluginManager) deletePluginPackageFromDatabaseContext(ctx context.Context, pluginID string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	_, err := pm.db.ExecContext(ctx, `DELETE FROM plugin_packages WHERE plugin_id = $1`, pluginID)
	if err != nil {
		return fmt.Errorf("failed to delete plugin package: %w", err)
	}

	return nil
}

// removeRuntimeFile is a best-effort cleanup helper that the install flow uses
// to roll back a partial write when the database persistence step fails.
func removeRuntimeFile(path string) {
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Warn().Err(err).Str("path", path).Msg("Failed to roll back native plugin runtime file")
	}
}
