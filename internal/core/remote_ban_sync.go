package core

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/db"
	"go.codycody31.dev/squad-aegis/internal/models"
)

type RemoteBanSyncService struct {
	database   db.Executor
	dbInstance *sql.DB // Keep reference to the database instance for transactions
}

func NewRemoteBanSyncService(database db.Executor, dbInstance *sql.DB) *RemoteBanSyncService {
	return &RemoteBanSyncService{
		database:   database,
		dbInstance: dbInstance,
	}
}

// SyncAllSources syncs all enabled remote ban sources
func (s *RemoteBanSyncService) SyncAllSources(ctx context.Context) error {
	sources, err := GetRemoteBanSources(ctx, s.database)
	if err != nil {
		return fmt.Errorf("failed to get remote ban sources: %w", err)
	}

	for _, source := range sources {
		if !source.SyncEnabled {
			continue
		}

		// Check if it's time to sync based on interval
		if source.LastSyncedAt != nil {
			nextSync := source.LastSyncedAt.Add(time.Duration(source.SyncIntervalMinutes) * time.Minute)
			if time.Now().Before(nextSync) {
				continue
			}
		}

		err := s.SyncSource(ctx, source)
		if err != nil {
			log.Error().Err(err).Str("source", source.Name).Msg("Failed to sync remote ban source")

			// Update sync status with error
			updateData := map[string]interface{}{
				"last_synced_at":   time.Now(),
				"last_sync_status": "error",
				"last_sync_error":  err.Error(),
			}
			UpdateRemoteBanSource(ctx, s.database, source.ID, updateData)
		} else {
			log.Info().Str("source", source.Name).Msg("Successfully synced remote ban source")

			// Update sync status with success
			updateData := map[string]interface{}{
				"last_synced_at":   time.Now(),
				"last_sync_status": "success",
				"last_sync_error":  nil,
			}
			UpdateRemoteBanSource(ctx, s.database, source.ID, updateData)
		}
	}

	return nil
}

// SyncSource syncs a specific remote ban source
func (s *RemoteBanSyncService) SyncSource(ctx context.Context, source *models.RemoteBanSource) error {
	log.Info().Str("source", source.Name).Str("url", source.URL).Msg("Starting sync of remote ban source")

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Get(source.URL)
	if err != nil {
		return fmt.Errorf("failed to fetch from %s: %w", source.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, source.URL)
	}

	// Get or create a remote ban list for this source
	banList, err := s.getOrCreateRemoteBanList(ctx, source)
	if err != nil {
		return fmt.Errorf("failed to get or create ban list: %w", err)
	}

	// Determine format and process bans
	contentType := resp.Header.Get("Content-Type")

	var bans []RemoteBan
	if strings.Contains(contentType, "text/csv") || strings.HasSuffix(source.URL, ".csv") {
		bans, err = s.parseCSVBans(resp.Body)
	} else {
		bans, err = s.parseTextBans(resp.Body)
	}

	if err != nil {
		return fmt.Errorf("failed to parse bans: %w", err)
	}

	// Clear existing bans from this ban list and add new ones
	err = s.updateBanListBans(ctx, banList.ID, bans)
	if err != nil {
		return fmt.Errorf("failed to update ban list: %w", err)
	}

	log.Info().Str("source", source.Name).Int("bans_count", len(bans)).Msg("Successfully synced remote bans")
	return nil
}

type RemoteBan struct {
	SteamID   string
	Reason    string
	Duration  int // 0 for permanent, minutes for temporary
	CreatedAt time.Time
}

func (s *RemoteBanSyncService) getOrCreateRemoteBanList(ctx context.Context, source *models.RemoteBanSource) (*models.BanList, error) {
	// Try to find existing ban list
	banLists, err := GetBanLists(ctx, s.database)
	if err != nil {
		return nil, err
	}

	for _, banList := range banLists {
		if banList.IsRemote && banList.RemoteURL != nil && *banList.RemoteURL == source.URL {
			return banList, nil
		}
	}

	// Create new remote ban list
	banList := &models.BanList{
		ID:                uuid.New(),
		Name:              fmt.Sprintf("Remote: %s", source.Name),
		Description:       &[]string{fmt.Sprintf("Automatically synced from %s", source.URL)}[0],
		IsRemote:          true,
		RemoteURL:         &source.URL,
		RemoteSyncEnabled: true,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	return CreateBanList(ctx, s.database, banList)
}

func (s *RemoteBanSyncService) parseCSVBans(body io.Reader) ([]RemoteBan, error) {
	reader := csv.NewReader(body)
	var bans []RemoteBan

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if len(record) < 1 {
			continue
		}

		steamIDStr := strings.TrimSpace(record[0])
		if steamIDStr == "" || steamIDStr == "steam_id" { // Skip header
			continue
		}

		// Validate Steam ID format (basic check)
		if len(steamIDStr) < 10 {
			continue // Skip invalid steam IDs
		}

		ban := RemoteBan{
			SteamID:   steamIDStr,
			Reason:    "Remote ban",
			Duration:  0, // Default to permanent
			CreatedAt: time.Now(),
		}

		// Try to parse reason if available
		if len(record) > 1 && record[1] != "" {
			ban.Reason = strings.TrimSpace(record[1])
		}

		// Try to parse expiry if available
		if len(record) > 3 && record[3] != "" {
			if expiryTime, err := time.Parse(time.RFC3339, record[3]); err == nil {
				if time.Now().After(expiryTime) {
					continue // Skip expired bans
				}
				ban.Duration = int(time.Until(expiryTime).Minutes())
			}
		}

		bans = append(bans, ban)
	}

	return bans, nil
}

func (s *RemoteBanSyncService) parseTextBans(body io.Reader) ([]RemoteBan, error) {
	scanner := bufio.NewScanner(body)
	var bans []RemoteBan

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		steamIDStr := strings.TrimSpace(parts[0])
		expiryStr := strings.TrimSpace(parts[1])

		// Validate Steam ID format (basic check)
		if len(steamIDStr) < 10 {
			continue
		}

		ban := RemoteBan{
			SteamID:   steamIDStr,
			Reason:    "Remote ban",
			Duration:  0, // Default to permanent
			CreatedAt: time.Now(),
		}

		// Parse expiry
		if expiryStr != "0" {
			if expiryTime, err := strconv.ParseInt(expiryStr, 10, 64); err == nil {
				expiryTimestamp := time.Unix(expiryTime, 0)
				if time.Now().After(expiryTimestamp) {
					continue // Skip expired bans
				}
				ban.Duration = int(time.Until(expiryTimestamp).Minutes())
			}
		}

		bans = append(bans, ban)
	}

	return bans, scanner.Err()
}

func (s *RemoteBanSyncService) updateBanListBans(ctx context.Context, banListID uuid.UUID, bans []RemoteBan) error {
	// Start transaction
	tx, err := s.dbInstance.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing bans from this ban list
	_, err = tx.ExecContext(ctx, "DELETE FROM server_bans WHERE ban_list_id = $1", banListID)
	if err != nil {
		return err
	}

	// Insert new bans, filtering out ignored Steam IDs
	for _, ban := range bans {
		// Check if this Steam ID is in the ignore list
		isIgnored, err := IsIgnoredSteamID(ctx, s.database, ban.SteamID)
		if err != nil {
			log.Warn().Err(err).Str("steam_id", ban.SteamID).Msg("Failed to check if Steam ID is ignored, including ban anyway")
			// Continue processing even if check fails, to avoid losing legitimate bans
		} else if isIgnored {
			log.Info().Str("steam_id", ban.SteamID).Msg("Skipping banned Steam ID - found in ignore list")
			continue
		}

		// For remote bans, use NULL for admin_id and server_id since they don't apply to a specific server/admin
		_, err = tx.ExecContext(ctx, `
			INSERT INTO server_bans (id, server_id, admin_id, steam_id, reason, duration, ban_list_id, created_at, updated_at)
			VALUES ($1, NULL, NULL, $2, $3, $4, $5, $6, $7)
		`, uuid.New(), ban.SteamID, ban.Reason, ban.Duration, banListID, ban.CreatedAt, ban.CreatedAt)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// StartPeriodicSync starts a background goroutine that periodically syncs remote ban sources
func (s *RemoteBanSyncService) StartPeriodicSync(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute) // Check every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := s.SyncAllSources(ctx)
			if err != nil {
				log.Error().Err(err).Msg("Failed to sync remote ban sources")
			}
		}
	}
}
