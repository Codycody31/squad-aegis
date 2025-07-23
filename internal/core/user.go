package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/db"
	"go.codycody31.dev/squad-aegis/internal/models"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrorUserNotFound    = errors.New("user not found")
	ErrorInvalidPassword = errors.New("invalid password")
)

func scanUsers(ctx context.Context, database db.Executor, rows *sql.Rows, viewer *uuid.UUID) ([]*models.User, error) {
	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		dest := []any{
			&user.Id, &user.SteamId, &user.Name, &user.Username, &user.Password, &user.SuperAdmin, &user.CreatedAt, &user.UpdatedAt,
		}
		err := rows.Scan(dest...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		users = append(users, user)
	}

	rows.Close()

	if len(users) <= 0 {
		return users, ErrorUserNotFound
	}

	return users, rows.Err()
}

func RegisterUser(ctx context.Context, database db.Executor, user *models.User) (*models.User, error) {
	time := time.Now()

	if err := user.Validate(); err != nil {
		return nil, err
	}

	if _, err := GetUserByUsername(ctx, database, user.Username, nil); err == nil {
		return nil, ErrUserAlreadyExists
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user.Password = string(passwordHash)

	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sql, args, err := psql.Insert("users").Columns("id", "steam_id", "name", "username", "password", "super_admin", "created_at", "updated_at").Values(user.Id, user.SteamId, user.Name, user.Username, user.Password, user.SuperAdmin, time, time).ToSql()
	if err != nil {
		return nil, err
	}

	_, err = database.ExecContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	return GetUserByUsername(ctx, database, user.Username, nil)
}

func GetUserByUsername(ctx context.Context, database db.Executor, username string, viewer *uuid.UUID) (*models.User, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	query, args, err := psql.Select("*").From("users").Where(squirrel.Eq{"username": username}).ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	users, err := scanUsers(ctx, database, rows, viewer)
	if err != nil {
		return nil, err
	}

	return users[0], nil
}

func GetUserById(ctx context.Context, database db.Executor, id uuid.UUID, viewer *uuid.UUID) (*models.User, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	query, args, err := psql.Select("*").From("users").Where(squirrel.Eq{"id": id}).ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	users, err := scanUsers(ctx, database, rows, viewer)
	if err != nil {
		return nil, err
	}

	return users[0], nil
}

func AuthenticateUser(ctx context.Context, database db.Executor, username string, password string) (*models.User, error) {
	user, err := GetUserByUsername(ctx, database, username, nil)
	if err != nil {
		return nil, err
	}

	if err := user.ComparePassword(password); err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return nil, ErrorInvalidPassword
		}

		return nil, err
	}

	return user, nil
}

func GetUsers(ctx context.Context, database db.Executor) ([]*models.User, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	query, args, err := psql.Select("*").From("users").ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	users, err := scanUsers(ctx, database, rows, nil)
	if err != nil {
		return nil, err
	}

	return users, nil
}

func DeleteUser(ctx context.Context, database db.Executor, id uuid.UUID) error {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	query, args, err := psql.Delete("users").Where(squirrel.Eq{"id": id}).ToSql()
	if err != nil {
		return err
	}

	_, err = database.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	return nil
}
