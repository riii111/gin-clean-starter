package readmodel

import (
	"time"

	"github.com/google/uuid"
)

type IdempotencyKeyRM struct {
	Key              uuid.UUID `json:"key"`
	UserID           uuid.UUID `json:"user_id"`
	Endpoint         string    `json:"endpoint"`
	RequestHash      string    `json:"request_hash"`
	ResponseBodyHash *string   `json:"response_body_hash,omitempty"`
	Status           string    `json:"status"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
