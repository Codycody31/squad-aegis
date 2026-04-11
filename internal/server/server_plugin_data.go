package server

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// ServerPluginDataGet returns all plugin data for a plugin instance
func (s *Server) ServerPluginDataGet(c *gin.Context) {
	if s.Dependencies.DB == nil {
		responses.InternalServerError(c, errors.New("database not available"), nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	instanceID, err := uuid.Parse(c.Param("pluginId"))
	if err != nil {
		responses.BadRequest(c, "Invalid plugin instance ID", &gin.H{"error": err.Error()})
		return
	}

	// Verify plugin instance exists and belongs to the server
	if s.Dependencies.PluginManager != nil {
		_, err := s.Dependencies.PluginManager.GetPluginInstance(serverID, instanceID)
		if err != nil {
			responses.NotFound(c, "Plugin instance not found", &gin.H{"error": err.Error()})
			return
		}
	}

	query := `SELECT key, value, created_at, updated_at FROM plugin_data WHERE plugin_instance_id = $1 ORDER BY key`
	rows, err := s.Dependencies.DB.Query(query, instanceID)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to query plugin data: %w", err), nil)
		return
	}
	defer rows.Close()

	type PluginDataItem struct {
		Key       string    `json:"key"`
		Value     string    `json:"value"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	var data []PluginDataItem
	for rows.Next() {
		var item PluginDataItem
		if err := rows.Scan(&item.Key, &item.Value, &item.CreatedAt, &item.UpdatedAt); err != nil {
			responses.InternalServerError(c, fmt.Errorf("failed to scan plugin data: %w", err), nil)
			return
		}
		data = append(data, item)
	}

	if err := rows.Err(); err != nil {
		responses.InternalServerError(c, fmt.Errorf("error iterating plugin data: %w", err), nil)
		return
	}

	responses.Success(c, "Plugin data fetched successfully", &gin.H{"data": data})
}

// ServerPluginDataClear clears all plugin data for a plugin instance
func (s *Server) ServerPluginDataClear(c *gin.Context) {
	if s.Dependencies.DB == nil {
		responses.InternalServerError(c, errors.New("database not available"), nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	instanceID, err := uuid.Parse(c.Param("pluginId"))
	if err != nil {
		responses.BadRequest(c, "Invalid plugin instance ID", &gin.H{"error": err.Error()})
		return
	}

	if s.Dependencies.PluginManager != nil {
		_, err := s.Dependencies.PluginManager.GetPluginInstance(serverID, instanceID)
		if err != nil {
			responses.NotFound(c, "Plugin instance not found", &gin.H{"error": err.Error()})
			return
		}
	}

	query := `DELETE FROM plugin_data WHERE plugin_instance_id = $1`
	result, err := s.Dependencies.DB.Exec(query, instanceID)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to clear plugin data: %w", err), nil)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to get rows affected: %w", err), nil)
		return
	}

	log.Info().
		Str("server_id", serverID.String()).
		Str("plugin_instance_id", instanceID.String()).
		Int64("rows_deleted", rowsAffected).
		Msg("Cleared plugin data")

	responses.Success(c, "Plugin data cleared successfully", &gin.H{"rows_deleted": rowsAffected})
}

// ServerPluginDataSet sets or updates a plugin data item
func (s *Server) ServerPluginDataSet(c *gin.Context) {
	if s.Dependencies.DB == nil {
		responses.InternalServerError(c, errors.New("database not available"), nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	instanceID, err := uuid.Parse(c.Param("pluginId"))
	if err != nil {
		responses.BadRequest(c, "Invalid plugin instance ID", &gin.H{"error": err.Error()})
		return
	}

	if s.Dependencies.PluginManager != nil {
		_, err := s.Dependencies.PluginManager.GetPluginInstance(serverID, instanceID)
		if err != nil {
			responses.NotFound(c, "Plugin instance not found", &gin.H{"error": err.Error()})
			return
		}
	}

	var requestBody struct {
		Key   string `json:"key" binding:"required"`
		Value string `json:"value" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		responses.BadRequest(c, "Invalid request body", &gin.H{"error": err.Error()})
		return
	}

	query := `INSERT INTO plugin_data (plugin_instance_id, key, value, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (plugin_instance_id, key)
		DO UPDATE SET value = $3, updated_at = NOW()`

	_, err = s.Dependencies.DB.Exec(query, instanceID, requestBody.Key, requestBody.Value)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to set plugin data: %w", err), nil)
		return
	}

	log.Info().
		Str("server_id", serverID.String()).
		Str("plugin_instance_id", instanceID.String()).
		Str("key", requestBody.Key).
		Msg("Set plugin data")

	responses.Success(c, "Plugin data set successfully", nil)
}

// ServerPluginDataDelete deletes a specific plugin data item
func (s *Server) ServerPluginDataDelete(c *gin.Context) {
	if s.Dependencies.DB == nil {
		responses.InternalServerError(c, errors.New("database not available"), nil)
		return
	}

	serverID, err := uuid.Parse(c.Param("serverId"))
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	instanceID, err := uuid.Parse(c.Param("pluginId"))
	if err != nil {
		responses.BadRequest(c, "Invalid plugin instance ID", &gin.H{"error": err.Error()})
		return
	}

	key := c.Param("key")
	if key == "" {
		responses.BadRequest(c, "Key parameter is required", nil)
		return
	}

	if s.Dependencies.PluginManager != nil {
		_, err := s.Dependencies.PluginManager.GetPluginInstance(serverID, instanceID)
		if err != nil {
			responses.NotFound(c, "Plugin instance not found", &gin.H{"error": err.Error()})
			return
		}
	}

	query := `DELETE FROM plugin_data WHERE plugin_instance_id = $1 AND key = $2`
	result, err := s.Dependencies.DB.Exec(query, instanceID, key)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to delete plugin data: %w", err), nil)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to get rows affected: %w", err), nil)
		return
	}

	if rowsAffected == 0 {
		responses.NotFound(c, "Plugin data item not found", nil)
		return
	}

	log.Info().
		Str("server_id", serverID.String()).
		Str("plugin_instance_id", instanceID.String()).
		Str("key", key).
		Msg("Deleted plugin data item")

	responses.Success(c, "Plugin data item deleted successfully", nil)
}
