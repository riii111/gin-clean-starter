package commands

import (
	"time"

	"github.com/google/uuid"
)

// Write-side snapshots prevent dependency on Read-side query types (CQRS separation)
type ResourceSnapshot struct {
	ID          uuid.UUID
	Name        string
	LeadTimeMin int
}

type CouponSnapshot struct {
	ID             uuid.UUID
	Code           string
	AmountOffCents *int32
	PercentOff     *float64
	ValidFrom      *time.Time
	ValidTo        *time.Time
}

type IdempotencyRecord struct {
	Key                 uuid.UUID
	UserID              uuid.UUID
	Status              string
	RequestHash         string
	ResultReservationID *uuid.UUID
	ExpiresAt           time.Time
}
