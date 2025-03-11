package models

import (
	"time"

	"github.com/guregu/null/v5"

	"github.com/google/uuid"
)

type Session struct {
	Id         uuid.UUID `json:"id"`
	UserId     uuid.UUID `json:"user_id"`
	Token      string    `json:"-"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  null.Time `json:"expires_at"`
	LastSeen   time.Time `json:"last_seen"`
	LastSeenIp string    `json:"last_seen_ip"`
}
