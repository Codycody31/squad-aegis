package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/db"
	"go.codycody31.dev/squad-aegis/internal/models"
)

func CreateServer(ctx context.Context, database db.Executor, server *models.Server) (*models.Server, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Insert("servers").Columns("name", "ip_address", "game_port", "rcon_port", "rcon_password").Values(server.Name, server.IpAddress, server.GamePort, server.RconPort, server.RconPassword).ToSql()
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
		err = rows.Scan(&server.Id, &server.Name, &server.IpAddress, &server.GamePort, &server.RconPort, &server.RconPassword, &server.CreatedAt, &server.UpdatedAt)
		if err != nil {
			return nil, err
		}
		servers = append(servers, &server)
	}

	return servers, nil
}

func GetServerById(ctx context.Context, database db.Executor, serverId uuid.UUID, user *models.User) (*models.Server, error) {
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
		err = rows.Scan(&server.Id, &server.Name, &server.IpAddress, &server.GamePort, &server.RconPort, &server.RconPassword, &server.CreatedAt, &server.UpdatedAt)
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
	sql, args, err := psql.Select("*").From("server_admins").Where(squirrel.Eq{"server_id": serverId}).ToSql()
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
		err = rows.Scan(&admin.Id, &admin.ServerId, &admin.UserId, &admin.ServerRoleId, &admin.CreatedAt)
		if err != nil {
			return nil, err
		}
		admins = append(admins, &admin)
	}

	return admins, nil
}
