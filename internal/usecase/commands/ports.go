package commands

import (
	"time"

	"github.com/google/uuid"
)

// ResourceSnapshot represents a read-only snapshot of resource data for Write operations
// This separates Write-side repository concerns from Read-side queries
type ResourceSnapshot struct {
	ID          uuid.UUID
	Name        string
	LeadTimeMin int
}

// CouponSnapshot represents a read-only snapshot of coupon data for Write operations
// This separates Write-side repository concerns from Read-side queries
type CouponSnapshot struct {
	ID             uuid.UUID
	Code           string
	AmountOffCents *int32
	PercentOff     *float64
	ValidFrom      *time.Time
	ValidTo        *time.Time
}
