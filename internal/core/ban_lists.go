package core

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/db"
	"go.codycody31.dev/squad-aegis/internal/models"
)

// BanList Management Functions

func CreateBanList(ctx context.Context, database db.Executor, banList *models.BanList) (*models.BanList, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Insert("ban_lists").Columns(
		"id", "name", "description", "is_remote", "remote_url", "remote_sync_enabled", "created_at", "updated_at",
	).Values(
		banList.ID, banList.Name, banList.Description, banList.IsRemote, banList.RemoteURL, banList.RemoteSyncEnabled, banList.CreatedAt, banList.UpdatedAt,
	).ToSql()
	if err != nil {
		return nil, err
	}

	_, err = database.ExecContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	return banList, nil
}

func GetBanLists(ctx context.Context, database db.Executor) ([]*models.BanList, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Select("*").From("ban_lists").OrderBy("created_at DESC").ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := database.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var banLists []*models.BanList
	for rows.Next() {
		banList := &models.BanList{}
		err = rows.Scan(
			&banList.ID, &banList.Name, &banList.Description, &banList.IsRemote,
			&banList.RemoteURL, &banList.RemoteSyncEnabled, &banList.LastSyncedAt,
			&banList.CreatedAt, &banList.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		banLists = append(banLists, banList)
	}

	return banLists, nil
}

func GetBanListById(ctx context.Context, database db.Executor, banListId uuid.UUID) (*models.BanList, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Select("*").From("ban_lists").Where(squirrel.Eq{"id": banListId}).ToSql()
	if err != nil {
		return nil, err
	}

	row := database.QueryRowContext(ctx, sql, args...)
	banList := &models.BanList{}
	err = row.Scan(
		&banList.ID, &banList.Name, &banList.Description, &banList.IsRemote,
		&banList.RemoteURL, &banList.RemoteSyncEnabled, &banList.LastSyncedAt,
		&banList.CreatedAt, &banList.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return banList, nil
}

func UpdateBanList(ctx context.Context, database db.Executor, banListId uuid.UUID, updateData map[string]interface{}) error {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	updateData["updated_at"] = time.Now()

	query := psql.Update("ban_lists").Where(squirrel.Eq{"id": banListId})
	for key, value := range updateData {
		query = query.Set(key, value)
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}

	_, err = database.ExecContext(ctx, sql, args...)
	return err
}

func DeleteBanList(ctx context.Context, database db.Executor, banListId uuid.UUID) error {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Delete("ban_lists").Where(squirrel.Eq{"id": banListId}).ToSql()
	if err != nil {
		return err
	}

	_, err = database.ExecContext(ctx, sql, args...)
	return err
}

// Server Ban List Subscription Functions

func GetServerBanListSubscriptions(ctx context.Context, database db.Executor, serverId uuid.UUID) ([]*models.ServerBanListSubscription, error) {
	sql := `
		SELECT sbls.id, sbls.server_id, sbls.ban_list_id, bl.name, sbls.created_at
		FROM server_ban_list_subscriptions sbls
		JOIN ban_lists bl ON sbls.ban_list_id = bl.id
		WHERE sbls.server_id = $1
		ORDER BY sbls.created_at DESC
	`

	rows, err := database.QueryContext(ctx, sql, serverId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subscriptions []*models.ServerBanListSubscription
	for rows.Next() {
		subscription := &models.ServerBanListSubscription{}
		err = rows.Scan(
			&subscription.ID, &subscription.ServerID, &subscription.BanListID,
			&subscription.BanListName, &subscription.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, subscription)
	}

	return subscriptions, nil
}

func CreateServerBanListSubscription(ctx context.Context, database db.Executor, serverId, banListId uuid.UUID) (*models.ServerBanListSubscription, error) {
	subscription := &models.ServerBanListSubscription{
		ID:        uuid.New(),
		ServerID:  serverId,
		BanListID: banListId,
		CreatedAt: time.Now(),
	}

	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Insert("server_ban_list_subscriptions").Columns(
		"id", "server_id", "ban_list_id", "created_at",
	).Values(
		subscription.ID, subscription.ServerID, subscription.BanListID, subscription.CreatedAt,
	).ToSql()
	if err != nil {
		return nil, err
	}

	_, err = database.ExecContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	return subscription, nil
}

func DeleteServerBanListSubscription(ctx context.Context, database db.Executor, serverId, banListId uuid.UUID) error {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Delete("server_ban_list_subscriptions").Where(squirrel.And{
		squirrel.Eq{"server_id": serverId},
		squirrel.Eq{"ban_list_id": banListId},
	}).ToSql()
	if err != nil {
		return err
	}

	_, err = database.ExecContext(ctx, sql, args...)
	return err
}

// Enhanced Ban Functions

func GetServerBans(ctx context.Context, database db.Executor, serverId uuid.UUID) ([]models.ServerBan, error) {
	query := `
		SELECT DISTINCT ON (COALESCE(sb.steam_id::text, sb.eos_id))
			sb.id, sb.server_id, sb.admin_id, u.username, u.steam_id, sb.steam_id, sb.eos_id, sb.reason,
			sb.duration, sb.rule_id, sb.ban_list_id, bl.name as ban_list_name,
			sb.created_at, sb.updated_at
		FROM server_bans sb
		JOIN users u ON sb.admin_id = u.id
		LEFT JOIN ban_lists bl ON sb.ban_list_id = bl.id
		WHERE (
			-- Direct bans on this server
			sb.server_id = $1
			OR
			-- Bans from subscribed ban lists
			sb.ban_list_id IN (
				SELECT sbls.ban_list_id
				FROM server_ban_list_subscriptions sbls
				WHERE sbls.server_id = $1
			)
		)
		ORDER BY COALESCE(sb.steam_id::text, sb.eos_id), sb.created_at DESC
	`

	rows, err := database.QueryContext(ctx, query, serverId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bans []models.ServerBan
	for rows.Next() {
		var ban models.ServerBan
		var steamIDInt sql.NullInt64
		var eosIDStr sql.NullString
		var adminSteamIDInt sql.NullInt64
		var ruleIDStr, banListIDStr, banListNameStr *string

		err := rows.Scan(
			&ban.ID, &ban.ServerID, &ban.AdminID, &ban.AdminName, &adminSteamIDInt,
			&steamIDInt, &eosIDStr, &ban.Reason, &ban.Duration, &ruleIDStr,
			&banListIDStr, &banListNameStr, &ban.CreatedAt, &ban.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if steamIDInt.Valid {
			ban.SteamID = fmt.Sprintf("%d", steamIDInt.Int64)
		}
		if eosIDStr.Valid {
			ban.EOSID = eosIDStr.String
		}
		if ban.SteamID != "" {
			ban.Name = ban.SteamID
		} else {
			ban.Name = ban.EOSID
		}

		if adminSteamIDInt.Valid {
			ban.AdminSteamID = fmt.Sprintf("%d", adminSteamIDInt.Int64)
		}

		// Set optional fields
		ban.RuleID = ruleIDStr
		ban.BanListID = banListIDStr
		ban.BanListName = banListNameStr

		// Calculate if ban is permanent and expiry date
		ban.Permanent = ban.Duration == 0
		if !ban.Permanent {
			ban.ExpiresAt = ban.CreatedAt.AddDate(0, 0, ban.Duration)
		}

		bans = append(bans, ban)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return bans, nil
}

// Remote Ban Source Functions

func GetRemoteBanSources(ctx context.Context, database db.Executor) ([]*models.RemoteBanSource, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Select("*").From("remote_ban_sources").OrderBy("created_at DESC").ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := database.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []*models.RemoteBanSource
	for rows.Next() {
		source := &models.RemoteBanSource{}
		err = rows.Scan(
			&source.ID, &source.Name, &source.URL, &source.SyncEnabled,
			&source.SyncIntervalMinutes, &source.LastSyncedAt, &source.LastSyncStatus,
			&source.LastSyncError, &source.CreatedAt, &source.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}

	return sources, nil
}

func CreateRemoteBanSource(ctx context.Context, database db.Executor, source *models.RemoteBanSource) (*models.RemoteBanSource, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Insert("remote_ban_sources").Columns(
		"id", "name", "url", "sync_enabled", "sync_interval_minutes", "created_at", "updated_at",
	).Values(
		source.ID, source.Name, source.URL, source.SyncEnabled, source.SyncIntervalMinutes, source.CreatedAt, source.UpdatedAt,
	).ToSql()
	if err != nil {
		return nil, err
	}

	_, err = database.ExecContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	return source, nil
}

func UpdateRemoteBanSource(ctx context.Context, database db.Executor, sourceId uuid.UUID, updateData map[string]interface{}) error {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	updateData["updated_at"] = time.Now()

	query := psql.Update("remote_ban_sources").Where(squirrel.Eq{"id": sourceId})
	for key, value := range updateData {
		query = query.Set(key, value)
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}

	_, err = database.ExecContext(ctx, sql, args...)
	return err
}

func DeleteRemoteBanSource(ctx context.Context, database db.Executor, sourceId uuid.UUID) error {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Delete("remote_ban_sources").Where(squirrel.Eq{"id": sourceId}).ToSql()
	if err != nil {
		return err
	}

	_, err = database.ExecContext(ctx, sql, args...)
	return err
}

// Ignored Steam ID Management Functions

func GetIgnoredSteamIDs(ctx context.Context, database db.Executor) ([]*models.IgnoredSteamID, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Select("*").From("ignored_steam_ids").OrderBy("created_at DESC").ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := database.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ignoredSteamIDs []*models.IgnoredSteamID
	for rows.Next() {
		ignoredSteamID := &models.IgnoredSteamID{}
		err := rows.Scan(
			&ignoredSteamID.ID,
			&ignoredSteamID.SteamID,
			&ignoredSteamID.Reason,
			&ignoredSteamID.CreatedBy,
			&ignoredSteamID.CreatedAt,
			&ignoredSteamID.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		ignoredSteamIDs = append(ignoredSteamIDs, ignoredSteamID)
	}

	return ignoredSteamIDs, rows.Err()
}

func GetIgnoredSteamIDByID(ctx context.Context, database db.Executor, id uuid.UUID) (*models.IgnoredSteamID, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Select("*").From("ignored_steam_ids").Where(squirrel.Eq{"id": id}).ToSql()
	if err != nil {
		return nil, err
	}

	row := database.QueryRowContext(ctx, sql, args...)
	ignoredSteamID := &models.IgnoredSteamID{}
	err = row.Scan(
		&ignoredSteamID.ID,
		&ignoredSteamID.SteamID,
		&ignoredSteamID.Reason,
		&ignoredSteamID.CreatedBy,
		&ignoredSteamID.CreatedAt,
		&ignoredSteamID.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return ignoredSteamID, nil
}

func CreateIgnoredSteamID(ctx context.Context, database db.Executor, ignoredSteamID *models.IgnoredSteamID) (*models.IgnoredSteamID, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Insert("ignored_steam_ids").Columns(
		"id", "steam_id", "reason", "created_by", "created_at", "updated_at",
	).Values(
		ignoredSteamID.ID, ignoredSteamID.SteamID, ignoredSteamID.Reason, ignoredSteamID.CreatedBy, ignoredSteamID.CreatedAt, ignoredSteamID.UpdatedAt,
	).ToSql()
	if err != nil {
		return nil, err
	}

	_, err = database.ExecContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	return ignoredSteamID, nil
}

func UpdateIgnoredSteamID(ctx context.Context, database db.Executor, id uuid.UUID, updates models.IgnoredSteamIDUpdateRequest) error {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	query := psql.Update("ignored_steam_ids").Where(squirrel.Eq{"id": id})

	if updates.Reason != nil {
		query = query.Set("reason", *updates.Reason)
	}
	query = query.Set("updated_at", time.Now())

	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}

	_, err = database.ExecContext(ctx, sql, args...)
	return err
}

func DeleteIgnoredSteamID(ctx context.Context, database db.Executor, id uuid.UUID) error {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Delete("ignored_steam_ids").Where(squirrel.Eq{"id": id}).ToSql()
	if err != nil {
		return err
	}

	_, err = database.ExecContext(ctx, sql, args...)
	return err
}

func IsIgnoredSteamID(ctx context.Context, database db.Executor, steamID string) (bool, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Select("COUNT(*)").From("ignored_steam_ids").Where(squirrel.Eq{"steam_id": steamID}).ToSql()
	if err != nil {
		return false, err
	}

	var count int
	err = database.QueryRowContext(ctx, sql, args...).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetActiveBanForServer checks if a steam ID or EOS ID has an active (non-expired) ban
// on the given server, including bans from subscribed ban lists.
// At least one of steamID or eosID must be non-empty.
// Returns nil if no active ban is found.
func GetActiveBanForServer(ctx context.Context, database db.Executor, serverID uuid.UUID, steamID string, eosID string) (*models.ServerBan, error) {
	// Build identifier conditions dynamically to avoid PostgreSQL cast issues.
	// Passing "" to $1::bigint fails even with a "$1 != ''" guard because
	// PostgreSQL does not guarantee short-circuit evaluation of AND/OR.
	var idConditions []string
	args := []any{}
	argIdx := 1

	if steamID != "" {
		steamIDInt, err := strconv.ParseInt(steamID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid steam ID %q: %w", steamID, err)
		}
		idConditions = append(idConditions, fmt.Sprintf("sb.steam_id = $%d", argIdx))
		args = append(args, steamIDInt)
		argIdx++
	}
	if eosID != "" {
		idConditions = append(idConditions, fmt.Sprintf("sb.eos_id = $%d", argIdx))
		args = append(args, eosID)
		argIdx++
	}
	if len(idConditions) == 0 {
		return nil, fmt.Errorf("at least one of steamID or eosID must be non-empty")
	}

	serverPlaceholder := fmt.Sprintf("$%d", argIdx)
	args = append(args, serverID)

	query := fmt.Sprintf(`
		SELECT sb.id, sb.reason, sb.duration, sb.created_at
		FROM server_bans sb
		WHERE (%s)
		AND (sb.duration = 0 OR sb.created_at + (sb.duration || ' days')::interval > NOW())
		AND (
			sb.server_id = %s
			OR sb.ban_list_id IN (
				SELECT sbls.ban_list_id
				FROM server_ban_list_subscriptions sbls
				WHERE sbls.server_id = %s
			)
		)
		ORDER BY sb.created_at DESC
		LIMIT 1
	`, strings.Join(idConditions, " OR "), serverPlaceholder, serverPlaceholder)

	var ban models.ServerBan
	err := database.QueryRowContext(ctx, query, args...).Scan(
		&ban.ID, &ban.Reason, &ban.Duration, &ban.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	ban.SteamID = steamID
	ban.EOSID = eosID
	ban.ServerID = serverID
	ban.Permanent = ban.Duration == 0
	if !ban.Permanent {
		ban.ExpiresAt = ban.CreatedAt.AddDate(0, 0, ban.Duration)
	}

	return &ban, nil
}

