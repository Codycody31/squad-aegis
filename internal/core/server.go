package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/db"
	"go.codycody31.dev/squad-aegis/internal/models"
)

func CreateServer(ctx context.Context, database db.Executor, server *models.Server) (*models.Server, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Insert("servers").Columns(
		"id", "name", "ip_address", "game_port", "rcon_ip_address", "rcon_port", "rcon_password",
		"log_source_type", "log_file_path", "log_host", "log_port", "log_username", "log_password",
		"log_poll_frequency", "log_read_from_start", "created_at", "updated_at",
	).Values(
		server.Id, server.Name, server.IpAddress, server.GamePort, server.RconIpAddress, server.RconPort, server.RconPassword,
		server.LogSourceType, server.LogFilePath, server.LogHost, server.LogPort, server.LogUsername, server.LogPassword,
		server.LogPollFrequency, server.LogReadFromStart, server.CreatedAt, server.UpdatedAt,
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
		sql, args, err = psql.Select("*").From("servers").ToSql()
		if err != nil {
			return nil, err
		}
	} else {
		sql, args, err = psql.Select("*").From("servers").Where(squirrel.Expr("id IN (SELECT server_id FROM server_admins WHERE user_id = $1)", user.Id)).ToSql()
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
			&server.Id, &server.Name, &server.IpAddress, &server.GamePort, &server.RconPort, &server.RconPassword,
			&server.CreatedAt, &server.UpdatedAt, &server.RconIpAddress, &server.LogSourceType, &server.LogFilePath,
			&server.LogHost, &server.LogPort, &server.LogUsername, &server.LogPassword, &server.LogPollFrequency, &server.LogReadFromStart,
		)
		if err != nil {
			return nil, err
		}
		servers = append(servers, &server)
	}

	return servers, nil
}

func GetServerById(ctx context.Context, database db.Executor, serverId uuid.UUID, user *models.User) (*models.Server, error) {
	if user == nil {
		psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
		sql, args, err := psql.Select("*").From("servers").Where(squirrel.Eq{"id": serverId}).ToSql()
		if err != nil {
			return nil, fmt.Errorf("failed to create SQL query: %w", err)
		}

		rows, err := database.QueryContext(ctx, sql, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to execute SQL query: %w", err)
		}
		defer rows.Close()

		server := &models.Server{}

		for rows.Next() {
			err = rows.Scan(
				&server.Id, &server.Name, &server.IpAddress, &server.GamePort, &server.RconPort, &server.RconPassword,
				&server.CreatedAt, &server.UpdatedAt, &server.RconIpAddress, &server.LogSourceType, &server.LogFilePath,
				&server.LogHost, &server.LogPort, &server.LogUsername, &server.LogPassword, &server.LogPollFrequency, &server.LogReadFromStart,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to scan row: %w", err)
			}
		}

		return server, nil
	}

	isSuperAdmin := user.SuperAdmin

	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	var sql string
	var args []interface{}
	var err error

	if isSuperAdmin {
		sql, args, err = psql.Select("*").From("servers").Where(squirrel.Eq{"id": serverId}).ToSql()
		if err != nil {
			return nil, fmt.Errorf("failed to create SQL query: %w", err)
		}
	} else {
		sql, args, err = psql.Select("*").From("servers").Where(squirrel.Eq{"id": serverId}).Where(squirrel.Expr("id IN (SELECT server_id FROM server_admins WHERE user_id = $2)", user.Id)).ToSql()
		if err != nil {
			return nil, fmt.Errorf("failed to create SQL query: %w", err)
		}
	}

	rows, err := database.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute SQL query: %w", err)
	}
	defer rows.Close()

	server := &models.Server{}

	for rows.Next() {
		err = rows.Scan(
			&server.Id, &server.Name, &server.IpAddress, &server.GamePort, &server.RconPort, &server.RconPassword,
			&server.CreatedAt, &server.UpdatedAt, &server.RconIpAddress, &server.LogSourceType, &server.LogFilePath,
			&server.LogHost, &server.LogPort, &server.LogUsername, &server.LogPassword, &server.LogPollFrequency, &server.LogReadFromStart,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
	}

	return server, nil
}

func GetServerRoles(ctx context.Context, database db.Executor, serverId uuid.UUID) ([]*models.ServerRole, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Select("*").From("server_roles").Where(squirrel.Eq{"server_id": serverId}).ToSql()
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
		err = rows.Scan(&role.Id, &role.ServerId, &role.Name, &permissionsStr, &role.CreatedAt)
		if err != nil {
			return nil, err
		}
		role.Permissions = strings.Split(permissionsStr, ",")
		if err != nil {
			return nil, err
		}
		roles = append(roles, &role)
	}

	return roles, nil
}

func GetServerAdmins(ctx context.Context, database db.Executor, serverId uuid.UUID) ([]*models.ServerAdmin, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Select("id", "server_id", "user_id", "steam_id", "server_role_id", "expires_at", "notes", "created_at").From("server_admins").Where(squirrel.Eq{"server_id": serverId}).ToSql()
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
		err = rows.Scan(&admin.Id, &admin.ServerId, &admin.UserId, &admin.SteamId, &admin.ServerRoleId, &admin.ExpiresAt, &admin.Notes, &admin.CreatedAt)
		if err != nil {
			return nil, err
		}
		admins = append(admins, &admin)
	}

	return admins, nil
}

// GetUserServerPermissions retrieves all servers a user has access to along with their permissions
func GetUserServerPermissions(ctx context.Context, database db.Executor, userId uuid.UUID) (map[string][]string, error) {
	// For super admins, we need to get all servers
	var user *models.User
	var err error

	user, err = GetUserById(ctx, database, userId, &userId)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Get all servers the user has access to
	servers, err := GetServers(ctx, database, user)
	if err != nil {
		return nil, fmt.Errorf("failed to get servers: %w", err)
	}

	// Initialize the result map
	serverPermissions := make(map[string][]string)

	// If user is super admin, they have all permissions on all servers
	if user.SuperAdmin {
		for _, server := range servers {
			// Super admins have all permissions
			serverPermissions[server.Id.String()] = []string{"*"}
		}
		return serverPermissions, nil
	}

	// For regular users, we need to get their roles for each server
	for _, server := range servers {
		// Get the user's admin record for this server
		psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
		sql, args, err := psql.Select("server_role_id").
			From("server_admins").
			Where(squirrel.Eq{"server_id": server.Id, "user_id": userId}).
			ToSql()
		if err != nil {
			return nil, fmt.Errorf("failed to create SQL query: %w", err)
		}

		var roleId uuid.UUID
		err = database.QueryRowContext(ctx, sql, args...).Scan(&roleId)
		if err != nil {
			if strings.Contains(err.Error(), "no rows") {
				// User doesn't have a role for this server, skip
				continue
			}
			return nil, fmt.Errorf("failed to get role ID: %w", err)
		}

		// Get the role's permissions
		sql, args, err = psql.Select("permissions").
			From("server_roles").
			Where(squirrel.Eq{"id": roleId}).
			ToSql()
		if err != nil {
			return nil, fmt.Errorf("failed to create SQL query: %w", err)
		}

		var permissionsStr string
		err = database.QueryRowContext(ctx, sql, args...).Scan(&permissionsStr)
		if err != nil {
			return nil, fmt.Errorf("failed to get permissions: %w", err)
		}

		// Parse permissions from comma-separated string
		permissions := strings.Split(permissionsStr, ",")
		serverPermissions[server.Id.String()] = permissions
	}

	return serverPermissions, nil
}

// UpdateServer updates a server in the database
func UpdateServer(ctx context.Context, db *sql.DB, server *models.Server) error {
	_, err := db.ExecContext(ctx, `
		UPDATE servers
		SET name = $1, ip_address = $2, game_port = $3, rcon_ip_address = $4, rcon_port = $5, rcon_password = $6,
		    log_source_type = $7, log_file_path = $8, log_host = $9, log_port = $10, log_username = $11, 
		    log_password = $12, log_poll_frequency = $13, log_read_from_start = $14,
		    updated_at = $15
		WHERE id = $16
	`, server.Name, server.IpAddress, server.GamePort, server.RconIpAddress, server.RconPort, server.RconPassword,
		server.LogSourceType, server.LogFilePath, server.LogHost, server.LogPort, server.LogUsername,
		server.LogPassword, server.LogPollFrequency, server.LogReadFromStart,
		time.Now(), server.Id)

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
func GetActiveServerAdmins(ctx context.Context, database db.Executor, serverId uuid.UUID) ([]*models.ServerAdmin, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Select("id", "server_id", "user_id", "steam_id", "server_role_id", "expires_at", "notes", "created_at").
		From("server_admins").
		Where(squirrel.Eq{"server_id": serverId}).
		Where(squirrel.Or{
			squirrel.Eq{"expires_at": nil},
			squirrel.Gt{"expires_at": time.Now()},
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
		err = rows.Scan(&admin.Id, &admin.ServerId, &admin.UserId, &admin.SteamId, &admin.ServerRoleId, &admin.ExpiresAt, &admin.Notes, &admin.CreatedAt)
		if err != nil {
			return nil, err
		}
		admins = append(admins, &admin)
	}

	return admins, nil
}
