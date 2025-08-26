package review

import (
	"time"

	"gin-clean-starter/internal/pkg/clock"

	"github.com/google/uuid"
)

type Services struct {
	Clock              clock.Clock
	EligibilityChecker EligibilityChecker
}

type EligibilityInput struct {
	ReservationID uuid.UUID
	UserID        uuid.UUID
	ResourceID    uuid.UUID
	Now           time.Time
}

type EligibilityChecker interface {
	CanPostReview(input EligibilityInput) error
}
