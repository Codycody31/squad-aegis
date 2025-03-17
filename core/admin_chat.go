package core

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/models"
)

// GetAdminChatMessages retrieves admin chat messages
func GetAdminChatMessages(ctx context.Context, db *sql.DB, serverId *uuid.UUID, limit int, offset int) ([]*models.AdminChatMessage, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	query := psql.Select("acm.id", "acm.server_id", "acm.user_id", "acm.message", "acm.created_at", "acm.updated_at",
		"u.id as user_id", "u.username", "u.steam_id", "u.super_admin", "u.created_at as user_created_at", "u.updated_at as user_updated_at").
		From("admin_chat_messages acm").
		Join("users u ON acm.user_id = u.id").
		OrderBy("acm.created_at DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset))

	if serverId != nil {
		query = query.Where(squirrel.Eq{"acm.server_id": *serverId})
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []*models.AdminChatMessage{}
	for rows.Next() {
		message := &models.AdminChatMessage{
			User: &models.User{},
		}

		var serverIdValue interface{}

		err := rows.Scan(
			&message.Id,
			&serverIdValue,
			&message.UserId,
			&message.Message,
			&message.CreatedAt,
			&message.UpdatedAt,
			&message.User.Id,
			&message.User.Username,
			&message.User.SteamId,
			&message.User.SuperAdmin,
			&message.User.CreatedAt,
			&message.User.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Handle server ID as string if not null
		if serverIdValue != nil {
			if serverIdStr, ok := serverIdValue.(string); ok {
				parsedUUID, err := uuid.Parse(serverIdStr)
				if err == nil {
					message.ServerId = &parsedUUID
				}
			}
		}

		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}

// CreateAdminChatMessage creates a new admin chat message
func CreateAdminChatMessage(ctx context.Context, db *sql.DB, userId uuid.UUID, serverId *uuid.UUID, message string) (*models.AdminChatMessage, error) {
	if message == "" {
		return nil, errors.New("message cannot be empty")
	}

	now := time.Now()
	msgId := uuid.New()

	// Create the message in the database
	query := `INSERT INTO admin_chat_messages (id, server_id, user_id, message, created_at, updated_at) 
	          VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := db.ExecContext(ctx, query, msgId, serverId, userId, message, now, now)
	if err != nil {
		return nil, err
	}

	// Create the message object
	chatMessage := &models.AdminChatMessage{
		Id:        msgId,
		ServerId:  serverId,
		UserId:    userId,
		Message:   message,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Get the user
	user, err := GetUserById(ctx, db, userId, nil)
	if err != nil {
		return chatMessage, err
	}

	chatMessage.User = user
	return chatMessage, nil
}
