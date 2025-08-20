package readmodel

import (
	"time"

	"github.com/google/uuid"
)

type ResourceRM struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	LeadTimeMin int32     `json:"lead_time_min"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
