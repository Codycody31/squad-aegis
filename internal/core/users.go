package core

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/db"
	"go.codycody31.dev/squad-aegis/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

func UpdateUser(ctx context.Context, db db.Executor, user *models.User) (*models.User, error) {
	query := `
		UPDATE users
		SET name = $1, username = $2, super_admin = $3, updated_at = NOW()
		WHERE id = $4
	`

	result, err := db.ExecContext(ctx, query, user.Name, user.Username, user.SuperAdmin, user.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}

// UpdateUserProfile updates a user's profile information
func UpdateUserProfile(ctx context.Context, db db.Executor, userId uuid.UUID, name string, steamId *int64) error {
	query := `
		UPDATE users
		SET name = $1, steam_id = $2, updated_at = NOW()
		WHERE id = $3
	`

	result, err := db.ExecContext(ctx, query, name, steamId, userId)
	if err != nil {
		return fmt.Errorf("failed to update user profile: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// UpdateUserPassword updates a user's password
func UpdateUserPassword(ctx context.Context, db db.Executor, userId uuid.UUID, newPassword string) error {
	// Hash the new password
	hashedPassword, err := HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	query := `
		UPDATE users
		SET password_hash = $1, updated_at = NOW()
		WHERE id = $2
	`

	result, err := db.ExecContext(ctx, query, hashedPassword, userId)
	if err != nil {
		return fmt.Errorf("failed to update user password: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}
