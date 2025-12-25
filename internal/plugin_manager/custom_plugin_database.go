package plugin_manager

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/plugin_sdk"
)

// CustomPluginRecord represents a custom plugin record in the database
type CustomPluginRecord struct {
	ID                     uuid.UUID
	PluginID               string
	Name                   string
	Version                string
	Author                 string
	SDKVersion             string
	Description            string
	Website                string
	StoragePath            string
	Signature              []byte
	UploadedBy             uuid.UUID
	UploadedAt             time.Time
	RequiredFeatures       []string
	ProvidedFeatures       []string
	RequiredPermissions    []string
	AllowMultipleInstances bool
	LongRunning            bool
	Enabled                bool
	Verified               bool
	UpdatedAt              time.Time
}

// SaveCustomPlugin saves a custom plugin to the database
func SaveCustomPlugin(db *sql.DB, plugin *CustomPluginRecord) error {
	query := `
		INSERT INTO custom_plugins (
			id, plugin_id, name, version, author, sdk_version, description, website,
			storage_path, signature, uploaded_by, uploaded_at, required_features,
			provided_features, required_permissions, allow_multiple_instances,
			long_running, enabled, verified, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		ON CONFLICT (plugin_id) DO UPDATE SET
			name = EXCLUDED.name,
			version = EXCLUDED.version,
			author = EXCLUDED.author,
			sdk_version = EXCLUDED.sdk_version,
			description = EXCLUDED.description,
			website = EXCLUDED.website,
			storage_path = EXCLUDED.storage_path,
			signature = EXCLUDED.signature,
			required_features = EXCLUDED.required_features,
			provided_features = EXCLUDED.provided_features,
			required_permissions = EXCLUDED.required_permissions,
			allow_multiple_instances = EXCLUDED.allow_multiple_instances,
			long_running = EXCLUDED.long_running,
			updated_at = EXCLUDED.updated_at
	`
	
	_, err := db.Exec(query,
		plugin.ID,
		plugin.PluginID,
		plugin.Name,
		plugin.Version,
		plugin.Author,
		plugin.SDKVersion,
		plugin.Description,
		plugin.Website,
		plugin.StoragePath,
		plugin.Signature,
		plugin.UploadedBy,
		plugin.UploadedAt,
		pq.Array(plugin.RequiredFeatures),
		pq.Array(plugin.ProvidedFeatures),
		pq.Array(plugin.RequiredPermissions),
		plugin.AllowMultipleInstances,
		plugin.LongRunning,
		plugin.Enabled,
		plugin.Verified,
		plugin.UpdatedAt,
	)
	
	if err != nil {
		return fmt.Errorf("failed to save custom plugin: %w", err)
	}
	
	log.Info().
		Str("plugin_id", plugin.PluginID).
		Str("version", plugin.Version).
		Msg("Saved custom plugin to database")
	
	return nil
}

// GetCustomPlugin retrieves a custom plugin from the database by plugin ID
func GetCustomPlugin(db *sql.DB, pluginID string) (*CustomPluginRecord, error) {
	query := `
		SELECT id, plugin_id, name, version, author, sdk_version, description, website,
			   storage_path, signature, uploaded_by, uploaded_at, required_features,
			   provided_features, required_permissions, allow_multiple_instances,
			   long_running, enabled, verified, updated_at
		FROM custom_plugins
		WHERE plugin_id = $1
	`
	
	var plugin CustomPluginRecord
	err := db.QueryRow(query, pluginID).Scan(
		&plugin.ID,
		&plugin.PluginID,
		&plugin.Name,
		&plugin.Version,
		&plugin.Author,
		&plugin.SDKVersion,
		&plugin.Description,
		&plugin.Website,
		&plugin.StoragePath,
		&plugin.Signature,
		&plugin.UploadedBy,
		&plugin.UploadedAt,
		pq.Array(&plugin.RequiredFeatures),
		pq.Array(&plugin.ProvidedFeatures),
		pq.Array(&plugin.RequiredPermissions),
		&plugin.AllowMultipleInstances,
		&plugin.LongRunning,
		&plugin.Enabled,
		&plugin.Verified,
		&plugin.UpdatedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("custom plugin %s not found", pluginID)
		}
		return nil, fmt.Errorf("failed to get custom plugin: %w", err)
	}
	
	return &plugin, nil
}

// ListCustomPlugins lists all custom plugins in the database
func ListCustomPlugins(db *sql.DB) ([]CustomPluginRecord, error) {
	query := `
		SELECT id, plugin_id, name, version, author, sdk_version, description, website,
			   storage_path, signature, uploaded_by, uploaded_at, required_features,
			   provided_features, required_permissions, allow_multiple_instances,
			   long_running, enabled, verified, updated_at
		FROM custom_plugins
		ORDER BY uploaded_at DESC
	`
	
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list custom plugins: %w", err)
	}
	defer rows.Close()
	
	var plugins []CustomPluginRecord
	for rows.Next() {
		var plugin CustomPluginRecord
		err := rows.Scan(
			&plugin.ID,
			&plugin.PluginID,
			&plugin.Name,
			&plugin.Version,
			&plugin.Author,
			&plugin.SDKVersion,
			&plugin.Description,
			&plugin.Website,
			&plugin.StoragePath,
			&plugin.Signature,
			&plugin.UploadedBy,
			&plugin.UploadedAt,
			pq.Array(&plugin.RequiredFeatures),
			pq.Array(&plugin.ProvidedFeatures),
			pq.Array(&plugin.RequiredPermissions),
			&plugin.AllowMultipleInstances,
			&plugin.LongRunning,
			&plugin.Enabled,
			&plugin.Verified,
			&plugin.UpdatedAt,
		)
		
		if err != nil {
			return nil, fmt.Errorf("failed to scan custom plugin: %w", err)
		}
		
		plugins = append(plugins, plugin)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating custom plugins: %w", err)
	}
	
	return plugins, nil
}

// DeleteCustomPlugin deletes a custom plugin from the database
func DeleteCustomPlugin(db *sql.DB, pluginID string) error {
	query := `DELETE FROM custom_plugins WHERE plugin_id = $1`
	
	result, err := db.Exec(query, pluginID)
	if err != nil {
		return fmt.Errorf("failed to delete custom plugin: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("custom plugin %s not found", pluginID)
	}
	
	log.Info().Str("plugin_id", pluginID).Msg("Deleted custom plugin from database")
	
	return nil
}

// UpdateCustomPluginEnabled updates the enabled status of a custom plugin
func UpdateCustomPluginEnabled(db *sql.DB, pluginID string, enabled bool) error {
	query := `UPDATE custom_plugins SET enabled = $1, updated_at = $2 WHERE plugin_id = $3`
	
	result, err := db.Exec(query, enabled, time.Now(), pluginID)
	if err != nil {
		return fmt.Errorf("failed to update custom plugin enabled status: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("custom plugin %s not found", pluginID)
	}
	
	log.Info().
		Str("plugin_id", pluginID).
		Bool("enabled", enabled).
		Msg("Updated custom plugin enabled status")
	
	return nil
}

// UpdateCustomPluginVerified updates the verified status of a custom plugin
func UpdateCustomPluginVerified(db *sql.DB, pluginID string, verified bool) error {
	query := `UPDATE custom_plugins SET verified = $1, updated_at = $2 WHERE plugin_id = $3`
	
	result, err := db.Exec(query, verified, time.Now(), pluginID)
	if err != nil {
		return fmt.Errorf("failed to update custom plugin verified status: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("custom plugin %s not found", pluginID)
	}
	
	log.Info().
		Str("plugin_id", pluginID).
		Bool("verified", verified).
		Msg("Updated custom plugin verified status")
	
	return nil
}

// ConvertManifestToRecord converts a plugin manifest to a database record
func ConvertManifestToRecord(manifest plugin_sdk.PluginManifest, storagePath string, uploadedBy uuid.UUID) *CustomPluginRecord {
	requiredFeatures := make([]string, len(manifest.RequiredFeatures))
	for i, f := range manifest.RequiredFeatures {
		requiredFeatures[i] = string(f)
	}
	
	providedFeatures := make([]string, len(manifest.ProvidedFeatures))
	for i, f := range manifest.ProvidedFeatures {
		providedFeatures[i] = string(f)
	}
	
	requiredPermissions := make([]string, len(manifest.RequiredPermissions))
	for i, p := range manifest.RequiredPermissions {
		requiredPermissions[i] = string(p)
	}
	
	return &CustomPluginRecord{
		ID:                     uuid.New(),
		PluginID:               manifest.ID,
		Name:                   manifest.Name,
		Version:                manifest.Version,
		Author:                 manifest.Author,
		SDKVersion:             manifest.SDKVersion,
		Description:            manifest.Description,
		Website:                manifest.Website,
		StoragePath:            storagePath,
		Signature:              manifest.Signature,
		UploadedBy:             uploadedBy,
		UploadedAt:             time.Now(),
		RequiredFeatures:       requiredFeatures,
		ProvidedFeatures:       providedFeatures,
		RequiredPermissions:    requiredPermissions,
		AllowMultipleInstances: manifest.AllowMultipleInstances,
		LongRunning:            manifest.LongRunning,
		Enabled:                false, // Start disabled by default
		Verified:               false, // Must be verified before use
		UpdatedAt:              time.Now(),
	}
}

// ConvertRecordToManifest converts a database record back to a manifest
func ConvertRecordToManifest(record *CustomPluginRecord) plugin_sdk.PluginManifest {
	requiredFeatures := make([]plugin_sdk.FeatureID, len(record.RequiredFeatures))
	for i, f := range record.RequiredFeatures {
		requiredFeatures[i] = plugin_sdk.FeatureID(f)
	}
	
	providedFeatures := make([]plugin_sdk.FeatureID, len(record.ProvidedFeatures))
	for i, f := range record.ProvidedFeatures {
		providedFeatures[i] = plugin_sdk.FeatureID(f)
	}
	
	requiredPermissions := make([]plugin_sdk.PermissionID, len(record.RequiredPermissions))
	for i, p := range record.RequiredPermissions {
		requiredPermissions[i] = plugin_sdk.PermissionID(p)
	}
	
	return plugin_sdk.PluginManifest{
		ID:                     record.PluginID,
		Name:                   record.Name,
		Version:                record.Version,
		Author:                 record.Author,
		SDKVersion:             record.SDKVersion,
		Description:            record.Description,
		Website:                record.Website,
		RequiredFeatures:       requiredFeatures,
		ProvidedFeatures:       providedFeatures,
		RequiredPermissions:    requiredPermissions,
		AllowMultipleInstances: record.AllowMultipleInstances,
		LongRunning:            record.LongRunning,
		Signature:              record.Signature,
	}
}

// CustomPluginJSON is the JSON representation of a custom plugin (for API responses)
type CustomPluginJSON struct {
	ID                     string    `json:"id"`
	PluginID               string    `json:"plugin_id"`
	Name                   string    `json:"name"`
	Version                string    `json:"version"`
	Author                 string    `json:"author"`
	SDKVersion             string    `json:"sdk_version"`
	Description            string    `json:"description"`
	Website                string    `json:"website,omitempty"`
	UploadedBy             string    `json:"uploaded_by"`
	UploadedAt             string    `json:"uploaded_at"`
	RequiredFeatures       []string  `json:"required_features"`
	ProvidedFeatures       []string  `json:"provided_features"`
	RequiredPermissions    []string  `json:"required_permissions"`
	AllowMultipleInstances bool      `json:"allow_multiple_instances"`
	LongRunning            bool      `json:"long_running"`
	Enabled                bool      `json:"enabled"`
	Verified               bool      `json:"verified"`
	UpdatedAt              string    `json:"updated_at"`
	HasSignature           bool      `json:"has_signature"`
}

// ToJSON converts a CustomPluginRecord to JSON representation
func (cpr *CustomPluginRecord) ToJSON() CustomPluginJSON {
	return CustomPluginJSON{
		ID:                     cpr.ID.String(),
		PluginID:               cpr.PluginID,
		Name:                   cpr.Name,
		Version:                cpr.Version,
		Author:                 cpr.Author,
		SDKVersion:             cpr.SDKVersion,
		Description:            cpr.Description,
		Website:                cpr.Website,
		UploadedBy:             cpr.UploadedBy.String(),
		UploadedAt:             cpr.UploadedAt.Format(time.RFC3339),
		RequiredFeatures:       cpr.RequiredFeatures,
		ProvidedFeatures:       cpr.ProvidedFeatures,
		RequiredPermissions:    cpr.RequiredPermissions,
		AllowMultipleInstances: cpr.AllowMultipleInstances,
		LongRunning:            cpr.LongRunning,
		Enabled:                cpr.Enabled,
		Verified:               cpr.Verified,
		UpdatedAt:              cpr.UpdatedAt.Format(time.RFC3339),
		HasSignature:           len(cpr.Signature) > 0,
	}
}

// MarshalJSON implements json.Marshaler
func (cpr *CustomPluginRecord) MarshalJSON() ([]byte, error) {
	return json.Marshal(cpr.ToJSON())
}

