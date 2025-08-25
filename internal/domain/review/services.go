package review

import (
	"time"

	"gin-clean-starter/internal/pkg/clock"

	"github.com/google/uuid"
)

type Services struct {
	Clock              clock.Clock
	EligibilityChecker ReviewEligibilityChecker
}

type ReviewEligibilityInput struct {
	ReservationID uuid.UUID
	UserID        uuid.UUID
	ResourceID    uuid.UUID
	Now           time.Time
}

type ReviewEligibilityChecker interface {
	CanPostReview(input ReviewEligibilityInput) error
}
