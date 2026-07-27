package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.codycody31.dev/squad-aegis/internal/db"
	"go.codycody31.dev/squad-aegis/internal/models"
)

func CreateServer(ctx context.Context, database db.Executor, server *models.Server) (*models.Server, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Insert("servers").Columns(
		"id", "name", "ip_address", "game_port", "rcon_ip_address", "rcon_port", "rcon_password", "enabled",
		"log_source_type", "log_host", "log_port", "log_username", "log_password",
		"log_poll_frequency", "log_read_from_start", "squad_game_path",
		"created_at", "updated_at",
	).Values(
		server.Id, server.Name, server.IpAddress, server.GamePort, server.RconIpAddress, server.RconPort, server.RconPassword, server.Enabled,
		server.LogSourceType, server.LogHost, server.LogPort, server.LogUsername, server.LogPassword,
		server.LogPollFrequency, server.LogReadFromStart, server.SquadGamePath,
		server.CreatedAt, server.UpdatedAt,
	).ToSql()
	if err != nil {
		return nil, err
	}

	_, err = database.ExecContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	return server, nil
}

func GetServers(ctx context.Context, database db.Executor, user *models.User) ([]*models.Server, error) {
	isSuperAdmin := user.SuperAdmin

	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	var sql string
	var args []interface{}
	var err error

	if isSuperAdmin {
		sql, args, err = psql.Select(
			"id", "name", "ip_address", "game_port", "rcon_ip_address", "rcon_port", "rcon_password", "enabled",
			"log_source_type", "log_host", "log_port", "log_username", "log_password",
			"log_poll_frequency", "log_read_from_start", "squad_game_path",
			"created_at", "updated_at",
		).From("servers").ToSql()
		if err != nil {
			return nil, err
		}
	} else {
		sql, args, err = psql.Select(
			"id", "name", "ip_address", "game_port", "rcon_ip_address", "rcon_port", "rcon_password", "enabled",
			"log_source_type", "log_host", "log_port", "log_username", "log_password",
			"log_poll_frequency", "log_read_from_start", "squad_game_path",
			"created_at", "updated_at",
		).From("servers").Where(squirrel.Expr("id IN (SELECT server_id FROM server_admins WHERE user_id = $1)", user.Id)).ToSql()
		if err != nil {
			return nil, err
		}
	}

	rows, err := database.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	servers := []*models.Server{}

	for rows.Next() {
		var server models.Server
		err = rows.Scan(
			&server.Id, &server.Name, &server.IpAddress, &server.GamePort, &server.RconIpAddress, &server.RconPort, &server.RconPassword, &server.Enabled,
			&server.LogSourceType, &server.LogHost, &server.LogPort, &server.LogUsername, &server.LogPassword,
			&server.LogPollFrequency, &server.LogReadFromStart, &server.SquadGamePath,
			&server.CreatedAt, &server.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		servers = append(servers, &server)
	}

	return servers, nil
}

func GetServerById(ctx context.Context, database db.Executor, serverId uuid.UUID, user *models.User) (*models.Server, error) {
	server := &models.Server{}
	query := `
		/* sa.server.get */
		SELECT
			s.id, s.name, s.ip_address, s.game_port, s.rcon_ip_address, s.rcon_port,
			s.rcon_password, s.enabled, s.log_source_type, s.log_host, s.log_port,
			s.log_username, s.log_password, s.log_poll_frequency, s.log_read_from_start,
			s.squad_game_path, s.created_at, s.updated_at
		FROM public.servers AS s
		WHERE s.id = $1
	`
	args := []interface{}{serverId}
	if user != nil && !user.SuperAdmin {
		query += `
			AND EXISTS (
				SELECT 1
				FROM public.server_admins AS sa
				WHERE sa.server_id = s.id
				  AND sa.user_id = $2
				  AND (sa.expires_at IS NULL OR sa.expires_at > CURRENT_TIMESTAMP)
			)
		`
		args = append(args, user.Id)
	}

	err := database.QueryRowContext(ctx, query, args...).Scan(
		&server.Id, &server.Name, &server.IpAddress, &server.GamePort, &server.RconIpAddress, &server.RconPort, &server.RconPassword, &server.Enabled,
		&server.LogSourceType, &server.LogHost, &server.LogPort, &server.LogUsername, &server.LogPassword,
		&server.LogPollFrequency, &server.LogReadFromStart, &server.SquadGamePath,
		&server.CreatedAt, &server.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get server: %w", err)
	}

	return server, nil
}

func GetServerRoles(ctx context.Context, database db.Executor, serverId uuid.UUID) ([]*models.ServerRole, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Select("id", "server_id", "name", "permissions", "is_admin", "created_at").From("server_roles").Where(squirrel.Eq{"server_id": serverId}).ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := database.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := []*models.ServerRole{}

	for rows.Next() {
		var role models.ServerRole
		var permissionsStr string
		err = rows.Scan(&role.Id, &role.ServerId, &role.Name, &permissionsStr, &role.IsAdmin, &role.CreatedAt)
		if err != nil {
			return nil, err
		}
		role.Permissions = strings.Split(permissionsStr, ",")
		roles = append(roles, &role)
	}

	return roles, nil
}

// GetServerAdminRoles retrieves only roles where is_admin = true
func GetServerAdminRoles(ctx context.Context, database db.Executor, serverId uuid.UUID) ([]*models.ServerRole, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Select("id", "server_id", "name", "permissions", "is_admin", "created_at").
		From("server_roles").
		Where(squirrel.Eq{"server_id": serverId, "is_admin": true}).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := database.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := []*models.ServerRole{}

	for rows.Next() {
		var role models.ServerRole
		var permissionsStr string
		err = rows.Scan(&role.Id, &role.ServerId, &role.Name, &permissionsStr, &role.IsAdmin, &role.CreatedAt)
		if err != nil {
			return nil, err
		}
		role.Permissions = strings.Split(permissionsStr, ",")
		roles = append(roles, &role)
	}

	return roles, nil
}

func GetServerAdmins(ctx context.Context, database db.Executor, serverId uuid.UUID) ([]*models.ServerAdmin, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Select("id", "server_id", "user_id", "steam_id", "eos_id", "server_role_id", "expires_at", "notes", "created_at").From("server_admins").Where(squirrel.Eq{"server_id": serverId}).ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := database.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	admins := []*models.ServerAdmin{}

	for rows.Next() {
		var admin models.ServerAdmin
		err = rows.Scan(&admin.Id, &admin.ServerId, &admin.UserId, &admin.SteamId, &admin.EOSId, &admin.ServerRoleId, &admin.ExpiresAt, &admin.Notes, &admin.CreatedAt)
		if err != nil {
			return nil, err
		}
		admins = append(admins, &admin)
	}

	return admins, nil
}

// GetUserServerPermissions retrieves all servers a user has access to along with their permissions
func GetUserServerPermissions(ctx context.Context, database db.Executor, userId uuid.UUID) (map[string][]string, error) {
	const query = `
		/* sa.permissions.user_servers */
		WITH RECURSIVE
		target_user AS (
			SELECT super_admin
			FROM public.users
			WHERE id = $1
		),
		role_hierarchy (server_id, role_id, path) AS (
			SELECT
				sa.server_id,
				sa.server_role_id,
				ARRAY[sa.server_role_id]
			FROM public.server_admins AS sa
			JOIN target_user AS tu ON NOT tu.super_admin
			JOIN public.server_roles AS sr
			  ON sr.id = sa.server_role_id
			 AND sr.server_id = sa.server_id
			WHERE sa.user_id = $1
			  AND (sa.expires_at IS NULL OR sa.expires_at > NOW())

			UNION ALL

			SELECT
				rh.server_id,
				ri.parent_role_id,
				rh.path || ri.parent_role_id
			FROM role_hierarchy AS rh
			JOIN public.role_inheritance AS ri
			  ON ri.child_role_id = rh.role_id
			JOIN public.server_roles AS parent_role
			  ON parent_role.id = ri.parent_role_id
			 AND parent_role.server_id = rh.server_id
			WHERE NOT ri.parent_role_id = ANY(rh.path)
		),
		user_servers AS (
			SELECT s.id AS server_id, TRUE AS super_admin
			FROM public.servers AS s
			JOIN target_user AS tu ON tu.super_admin

			UNION ALL

			SELECT DISTINCT rh.server_id, FALSE
			FROM role_hierarchy AS rh
		),
		permission_rows AS (
			SELECT
				us.server_id,
				CASE
					WHEN us.super_admin THEN ARRAY['*']::text[]
					ELSE COALESCE(
						array_agg(DISTINCT p.code::text ORDER BY p.code::text)
							FILTER (WHERE p.code IS NOT NULL),
						ARRAY[]::text[]
					)
				END AS permissions
			FROM user_servers AS us
			LEFT JOIN role_hierarchy AS rh
			  ON rh.server_id = us.server_id
			 AND NOT us.super_admin
			LEFT JOIN public.server_role_permissions AS srp
			  ON srp.server_role_id = rh.role_id
			LEFT JOIN public.permissions AS p
			  ON p.id = srp.permission_id
			GROUP BY us.server_id, us.super_admin
		),
		result_rows AS (
			SELECT server_id, permissions
			FROM permission_rows

			UNION ALL

			SELECT NULL::uuid, ARRAY[]::text[]
			FROM target_user
			WHERE NOT EXISTS (SELECT 1 FROM permission_rows)
		)
		SELECT server_id, permissions
		FROM result_rows
		ORDER BY server_id NULLS FIRST
	`

	rows, err := database.QueryContext(ctx, query, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to get user server permissions: %w", err)
	}
	defer rows.Close()

	serverPermissions := make(map[string][]string)
	userExists := false
	for rows.Next() {
		var serverID uuid.NullUUID
		var permissionCodes pq.StringArray
		if err := rows.Scan(&serverID, &permissionCodes); err != nil {
			return nil, fmt.Errorf("failed to scan user server permissions: %w", err)
		}
		userExists = true
		if serverID.Valid {
			serverPermissions[serverID.UUID.String()] = []string(permissionCodes)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read user server permissions: %w", err)
	}
	if !userExists {
		return nil, ErrorUserNotFound
	}

	return serverPermissions, nil
}

// UpdateServer updates a server in the database
func UpdateServer(ctx context.Context, db *sql.DB, server *models.Server) error {
	_, err := db.ExecContext(ctx, `
		UPDATE servers
		SET name = $1, ip_address = $2, game_port = $3, rcon_ip_address = $4, rcon_port = $5, rcon_password = $6,
		    log_source_type = $7, log_host = $8, log_port = $9, log_username = $10,
		    log_password = $11, log_poll_frequency = $12, log_read_from_start = $13,
		    squad_game_path = $14, enabled = $15, updated_at = $16
		WHERE id = $17
	`, server.Name, server.IpAddress, server.GamePort, server.RconIpAddress, server.RconPort, server.RconPassword,
		server.LogSourceType, server.LogHost, server.LogPort, server.LogUsername,
		server.LogPassword, server.LogPollFrequency, server.LogReadFromStart,
		server.SquadGamePath, server.Enabled, time.Now(), server.Id)

	if err != nil {
		return fmt.Errorf("failed to update server: %w", err)
	}

	return nil
}

// CleanupExpiredAdmins removes expired admin roles from the database
func CleanupExpiredAdmins(ctx context.Context, database db.Executor) (int64, error) {
	result, err := database.ExecContext(ctx, `
		DELETE FROM server_admins 
		WHERE expires_at IS NOT NULL AND expires_at < NOW()
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired admins: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// GetActiveServerAdmins retrieves all active (non-expired) admins for a server
// Only includes admins with roles where is_admin = true
func GetActiveServerAdmins(ctx context.Context, database db.Executor, serverId uuid.UUID) ([]*models.ServerAdmin, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Select("sa.id", "sa.server_id", "sa.user_id", "sa.steam_id", "sa.eos_id", "sa.server_role_id", "sa.expires_at", "sa.notes", "sa.created_at").
		From("server_admins sa").
		Join("server_roles sr ON sa.server_role_id = sr.id").
		Where(squirrel.Eq{"sa.server_id": serverId, "sr.is_admin": true}).
		Where(squirrel.Or{
			squirrel.Eq{"sa.expires_at": nil},
			squirrel.Gt{"sa.expires_at": time.Now()},
		}).ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := database.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	admins := []*models.ServerAdmin{}

	for rows.Next() {
		var admin models.ServerAdmin
		err = rows.Scan(&admin.Id, &admin.ServerId, &admin.UserId, &admin.SteamId, &admin.EOSId, &admin.ServerRoleId, &admin.ExpiresAt, &admin.Notes, &admin.CreatedAt)
		if err != nil {
			return nil, err
		}
		admins = append(admins, &admin)
	}

	return admins, nil
}

// GetAllActiveServerRoleMembers retrieves all active (non-expired) role members for a server
// Includes all roles regardless of is_admin flag, used for admin.cfg generation where
// both admin and non-admin roles (whitelist, seeder, VIP, etc.) must appear
func GetAllActiveServerRoleMembers(ctx context.Context, database db.Executor, serverId uuid.UUID) ([]*models.ServerAdmin, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Select("sa.id", "sa.server_id", "sa.user_id", "sa.steam_id", "sa.eos_id", "sa.server_role_id", "sa.expires_at", "sa.notes", "sa.created_at").
		From("server_admins sa").
		Join("server_roles sr ON sa.server_role_id = sr.id AND sr.server_id = sa.server_id").
		Where(squirrel.Eq{"sa.server_id": serverId}).
		Where(squirrel.Or{
			squirrel.Eq{"sa.expires_at": nil},
			squirrel.Gt{"sa.expires_at": time.Now()},
		}).ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := database.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []*models.ServerAdmin{}

	for rows.Next() {
		var member models.ServerAdmin
		err = rows.Scan(&member.Id, &member.ServerId, &member.UserId, &member.SteamId, &member.EOSId, &member.ServerRoleId, &member.ExpiresAt, &member.Notes, &member.CreatedAt)
		if err != nil {
			return nil, err
		}
		members = append(members, &member)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return members, nil
}
