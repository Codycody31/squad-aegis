package core

import (
	"context"
	"fmt"
	"time"

	"github.com/guregu/null/v5"

	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/db"
	"go.codycody31.dev/squad-aegis/internal/models"
)

func CreateSession(ctx context.Context, database db.Executor, userId uuid.UUID, userIp string, expiresIn time.Duration) (*models.Session, error) {
	session := &models.Session{
		Id:         uuid.New(),
		UserId:     userId,
		Token:      uuid.New().String(),
		LastSeen:   time.Now(),
		LastSeenIp: userIp,
	}
	_, err := database.ExecContext(ctx, "INSERT INTO sessions (user_id, token, last_seen, last_seen_ip) VALUES ($1, $2, $3, $4)", session.UserId, session.Token, session.LastSeen, session.LastSeenIp)
	if err != nil {
		return nil, err
	}

	// If expiresAt is not duration of 0, set the expiration time
	if expiresIn != 0 {
		_, err = database.ExecContext(ctx, "UPDATE sessions SET expires_at = $1 WHERE token = $2", time.Now().Add(expiresIn), session.Token)
		if err != nil {
			return nil, err
		}
		session.ExpiresAt = null.TimeFrom(time.Now().Add(expiresIn))
	}

	return session, nil
}

func GetSessionsByUserId(ctx context.Context, database db.Executor, userId uuid.UUID) ([]models.Session, error) {
	rows, err := database.QueryContext(ctx, "SELECT id, user_id, token, expires_at, last_seen, last_seen_ip FROM sessions WHERE user_id = $1", userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []models.Session
	for rows.Next() {
		var session models.Session
		if err := rows.Scan(&session.Id, &session.UserId, &session.Token, &session.ExpiresAt, &session.LastSeen, &session.LastSeenIp); err != nil {
			fmt.Println(err)
			return nil, err
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

func GetSessionById(ctx context.Context, database db.Executor, sessionId uuid.UUID) (*models.Session, error) {
	row := database.QueryRowContext(ctx, "SELECT id, user_id, token, expires_at, last_seen, last_seen_ip FROM sessions WHERE id = $1", sessionId)
	var session models.Session
	if err := row.Scan(&session.Id, &session.UserId, &session.Token, &session.ExpiresAt, &session.LastSeen, &session.LastSeenIp); err != nil {
		return nil, err
	}

	return &session, nil
}

func DeleteSessionById(ctx context.Context, database db.Executor, sessionId uuid.UUID) error {
	_, err := database.ExecContext(ctx, "DELETE FROM sessions WHERE id = $1", sessionId)
	return err
}
