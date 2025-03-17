package models

import (
	"time"

	"github.com/google/uuid"
)

// AdminChatMessage represents a message in the admin chat
type AdminChatMessage struct {
	Id        uuid.UUID  `json:"id"`
	ServerId  *uuid.UUID `json:"server_id"`
	UserId    uuid.UUID  `json:"user_id"`
	User      *User      `json:"user,omitempty"`
	Message   string     `json:"message"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
