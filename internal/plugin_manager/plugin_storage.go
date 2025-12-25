package plugin_manager

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/storage"
)

// PluginStorage manages storage of plugin .so files
type PluginStorage struct {
	storage storage.Storage
}

// NewPluginStorage creates a new plugin storage manager
func NewPluginStorage(storage storage.Storage) *PluginStorage {
	return &PluginStorage{
		storage: storage,
	}
}

// SavePlugin saves a plugin .so file to storage
func (ps *PluginStorage) SavePlugin(ctx context.Context, pluginID, version string, reader io.Reader) (string, error) {
	// Generate storage path: plugins/{plugin_id}/{version}/{plugin_id}.so
	storagePath := ps.getPluginPath(pluginID, version)
	
	log.Info().
		Str("plugin_id", pluginID).
		Str("version", version).
		Str("path", storagePath).
		Msg("Saving plugin to storage")
	
	if err := ps.storage.Save(ctx, storagePath, reader); err != nil {
		return "", fmt.Errorf("failed to save plugin to storage: %w", err)
	}
	
	log.Info().
		Str("plugin_id", pluginID).
		Str("version", version).
		Str("path", storagePath).
		Msg("Plugin saved successfully")
	
	return storagePath, nil
}

// LoadPlugin loads a plugin .so file from storage
func (ps *PluginStorage) LoadPlugin(ctx context.Context, pluginID, version string) (io.ReadCloser, error) {
	storagePath := ps.getPluginPath(pluginID, version)
	
	log.Debug().
		Str("plugin_id", pluginID).
		Str("version", version).
		Str("path", storagePath).
		Msg("Loading plugin from storage")
	
	reader, err := ps.storage.Get(ctx, storagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin from storage: %w", err)
	}
	
	return reader, nil
}

// LoadPluginByPath loads a plugin from a specific storage path
func (ps *PluginStorage) LoadPluginByPath(ctx context.Context, storagePath string) (io.ReadCloser, error) {
	log.Debug().
		Str("path", storagePath).
		Msg("Loading plugin from storage by path")
	
	reader, err := ps.storage.Get(ctx, storagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin from storage: %w", err)
	}
	
	return reader, nil
}

// DeletePlugin deletes a plugin .so file from storage
func (ps *PluginStorage) DeletePlugin(ctx context.Context, pluginID, version string) error {
	storagePath := ps.getPluginPath(pluginID, version)
	
	log.Info().
		Str("plugin_id", pluginID).
		Str("version", version).
		Str("path", storagePath).
		Msg("Deleting plugin from storage")
	
	if err := ps.storage.Delete(ctx, storagePath); err != nil {
		return fmt.Errorf("failed to delete plugin from storage: %w", err)
	}
	
	log.Info().
		Str("plugin_id", pluginID).
		Str("version", version).
		Msg("Plugin deleted successfully")
	
	return nil
}

// DeletePluginByPath deletes a plugin from a specific storage path
func (ps *PluginStorage) DeletePluginByPath(ctx context.Context, storagePath string) error {
	log.Info().
		Str("path", storagePath).
		Msg("Deleting plugin from storage by path")
	
	if err := ps.storage.Delete(ctx, storagePath); err != nil {
		return fmt.Errorf("failed to delete plugin from storage: %w", err)
	}
	
	return nil
}

// PluginExists checks if a plugin file exists in storage
func (ps *PluginStorage) PluginExists(ctx context.Context, pluginID, version string) (bool, error) {
	storagePath := ps.getPluginPath(pluginID, version)
	
	exists, err := ps.storage.Exists(ctx, storagePath)
	if err != nil {
		return false, fmt.Errorf("failed to check plugin existence: %w", err)
	}
	
	return exists, nil
}

// PluginExistsByPath checks if a plugin file exists at a specific path
func (ps *PluginStorage) PluginExistsByPath(ctx context.Context, storagePath string) (bool, error) {
	exists, err := ps.storage.Exists(ctx, storagePath)
	if err != nil {
		return false, fmt.Errorf("failed to check plugin existence: %w", err)
	}
	
	return exists, nil
}

// ListPluginVersions lists all versions of a plugin in storage
func (ps *PluginStorage) ListPluginVersions(ctx context.Context, pluginID string) ([]string, error) {
	// List all files under plugins/{plugin_id}/
	prefix := fmt.Sprintf("plugins/%s/", pluginID)
	
	files, err := ps.storage.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list plugin versions: %w", err)
	}
	
	// Extract versions from paths
	versions := make([]string, 0)
	seenVersions := make(map[string]bool)
	
	for _, file := range files {
		if file.IsDir {
			continue
		}
		
		// Path format: plugins/{plugin_id}/{version}/{plugin_id}.so
		parts := strings.Split(file.Path, "/")
		if len(parts) >= 3 {
			version := parts[2]
			if !seenVersions[version] {
				versions = append(versions, version)
				seenVersions[version] = true
			}
		}
	}
	
	return versions, nil
}

// GetPluginURL returns a URL to access a plugin file (for download)
func (ps *PluginStorage) GetPluginURL(ctx context.Context, pluginID, version string) (string, error) {
	storagePath := ps.getPluginPath(pluginID, version)
	
	url, err := ps.storage.GetURL(ctx, storagePath)
	if err != nil {
		return "", fmt.Errorf("failed to get plugin URL: %w", err)
	}
	
	return url, nil
}

// GetPluginInfo returns information about a plugin file in storage
func (ps *PluginStorage) GetPluginInfo(ctx context.Context, pluginID, version string) (*storage.FileInfo, error) {
	storagePath := ps.getPluginPath(pluginID, version)
	
	info, err := ps.storage.Stat(ctx, storagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get plugin info: %w", err)
	}
	
	return info, nil
}

// getPluginPath returns the storage path for a plugin
func (ps *PluginStorage) getPluginPath(pluginID, version string) string {
	// Sanitize plugin ID and version to prevent path traversal
	pluginID = sanitizePathComponent(pluginID)
	version = sanitizePathComponent(version)
	
	return filepath.Join("plugins", pluginID, version, fmt.Sprintf("%s.so", pluginID))
}

// sanitizePathComponent removes potentially dangerous characters from path components
func sanitizePathComponent(component string) string {
	// Remove path separators and parent directory references
	component = strings.ReplaceAll(component, "/", "_")
	component = strings.ReplaceAll(component, "\\", "_")
	component = strings.ReplaceAll(component, "..", "_")
	
	// Keep only alphanumeric, dash, underscore, and dot
	var result strings.Builder
	for _, r := range component {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || 
		   (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			result.WriteRune(r)
		}
	}
	
	return result.String()
}

// ListAllPlugins lists all custom plugins in storage
func (ps *PluginStorage) ListAllPlugins(ctx context.Context) ([]string, error) {
	// List all directories under plugins/
	prefix := "plugins/"
	
	files, err := ps.storage.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list plugins: %w", err)
	}
	
	// Extract unique plugin IDs
	pluginIDs := make(map[string]bool)
	for _, file := range files {
		// Path format: plugins/{plugin_id}/...
		parts := strings.Split(file.Path, "/")
		if len(parts) >= 2 {
			pluginID := parts[1]
			pluginIDs[pluginID] = true
		}
	}
	
	// Convert to slice
	result := make([]string, 0, len(pluginIDs))
	for pluginID := range pluginIDs {
		result = append(result, pluginID)
	}
	
	return result, nil
}

// CleanupOldVersions removes all but the N most recent versions of a plugin
func (ps *PluginStorage) CleanupOldVersions(ctx context.Context, pluginID string, keepCount int) error {
	if keepCount < 1 {
		return fmt.Errorf("keepCount must be at least 1")
	}
	
	// List all versions
	versions, err := ps.ListPluginVersions(ctx, pluginID)
	if err != nil {
		return fmt.Errorf("failed to list versions: %w", err)
	}
	
	if len(versions) <= keepCount {
		return nil // Nothing to clean up
	}
	
	// Delete oldest versions (keeping newest)
	// Note: This assumes versions are sortable strings (e.g., semantic versioning)
	// In production, you'd want proper version comparison
	versionsToDelete := versions[:len(versions)-keepCount]
	
	for _, version := range versionsToDelete {
		if err := ps.DeletePlugin(ctx, pluginID, version); err != nil {
			log.Error().
				Err(err).
				Str("plugin_id", pluginID).
				Str("version", version).
				Msg("Failed to delete old plugin version")
			// Continue with other versions
		}
	}
	
	log.Info().
		Str("plugin_id", pluginID).
		Int("deleted_count", len(versionsToDelete)).
		Int("kept_count", keepCount).
		Msg("Cleaned up old plugin versions")
	
	return nil
}

