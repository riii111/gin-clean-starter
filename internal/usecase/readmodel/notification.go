package readmodel

import (
	"time"

	"github.com/google/uuid"
)

type NotificationJobRM struct {
	ID        uuid.UUID `json:"id"`
	Kind      string    `json:"kind"`
	Topic     string    `json:"topic"`
	Payload   []byte    `json:"payload"`
	RunAt     time.Time `json:"run_at"`
	Attempts  int32     `json:"attempts"`
	Status    string    `json:"status"`
	LastError *string   `json:"last_error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
