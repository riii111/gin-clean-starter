//go:build unit || e2e

package builder

import (
	"time"

	domreview "gin-clean-starter/internal/domain/review"
	reqdto "gin-clean-starter/internal/handler/dto/request"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/internal/usecase/queries"
	"gin-clean-starter/internal/usecase/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type ReviewBuilder struct {
	UserID        uuid.UUID
	UserEmail     string
	ResourceID    uuid.UUID
	ResourceName  string
	ReservationID uuid.UUID
	Rating        int
	Comment       string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func NewReviewBuilder() *ReviewBuilder {
	now := time.Now()
	return &ReviewBuilder{
		UserID:        uuid.New(),
		UserEmail:     "reviewer@example.com",
		ResourceID:    uuid.New(),
		ResourceName:  "Test Resource",
		ReservationID: uuid.New(),
		Rating:        5,
		Comment:       "Excellent service!",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func (r *ReviewBuilder) With(mutate func(*ReviewBuilder)) *ReviewBuilder {
	mutate(r)
	return r
}

// Build methods
func (r *ReviewBuilder) BuildDomain() (*domreview.Review, error) {
	return domreview.NewReview(uuid.Nil, r.UserID, r.ResourceID, r.ReservationID, r.Rating, r.Comment, r.CreatedAt)
}

func (r *ReviewBuilder) BuildInfra() sqlc.Reviews {
	id := uuid.New()
	return sqlc.Reviews{
		ID:            id,
		UserID:        r.UserID,
		ResourceID:    r.ResourceID,
		ReservationID: r.ReservationID,
		Rating:        int32(r.Rating),
		Comment:       r.Comment,
		CreatedAt:     pgtype.Timestamptz{Time: r.CreatedAt, Valid: true},
		UpdatedAt:     pgtype.Timestamptz{Time: r.UpdatedAt, Valid: true},
	}
}

func (r *ReviewBuilder) BuildCreateRequestDTO() reqdto.CreateReviewRequest {
	return reqdto.CreateReviewRequest{
		ResourceID:    r.ResourceID,
		ReservationID: r.ReservationID,
		Rating:        r.Rating,
		Comment:       r.Comment,
	}
}

func (r *ReviewBuilder) BuildUpdateRequestDTO() reqdto.UpdateReviewRequest {
	rating := r.Rating
	comment := r.Comment
	return reqdto.UpdateReviewRequest{
		Rating:  &rating,
		Comment: &comment,
	}
}

func (r *ReviewBuilder) BuildViewQuery() *queries.ReviewView {
	id := uuid.New()
	return &queries.ReviewView{
		ID:            id,
		UserID:        r.UserID,
		UserEmail:     r.UserEmail,
		ResourceID:    r.ResourceID,
		ResourceName:  r.ResourceName,
		ReservationID: r.ReservationID,
		Rating:        int32(r.Rating),
		Comment:       r.Comment,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
}

func (r *ReviewBuilder) BuildListItem() *queries.ReviewListItem {
	id := uuid.New()
	return &queries.ReviewListItem{
		ID:        id,
		UserEmail: r.UserEmail,
		Rating:    int32(r.Rating),
		Comment:   r.Comment,
		CreatedAt: r.CreatedAt,
	}
}

func (r *ReviewBuilder) BuildSnapshot() *shared.ReviewSnapshot {
	id := uuid.New()
	return &shared.ReviewSnapshot{
		ID:            id,
		UserID:        r.UserID,
		ResourceID:    r.ResourceID,
		ReservationID: r.ReservationID,
		Rating:        r.Rating,
		Comment:       r.Comment,
	}
}

// Fluent builder methods
func (r *ReviewBuilder) WithUserID(userID uuid.UUID) *ReviewBuilder {
	r.UserID = userID
	return r
}

func (r *ReviewBuilder) WithUserEmail(email string) *ReviewBuilder {
	r.UserEmail = email
	return r
}

func (r *ReviewBuilder) WithResourceID(resourceID uuid.UUID) *ReviewBuilder {
	r.ResourceID = resourceID
	return r
}

func (r *ReviewBuilder) WithResourceName(name string) *ReviewBuilder {
	r.ResourceName = name
	return r
}

func (r *ReviewBuilder) WithReservationID(reservationID uuid.UUID) *ReviewBuilder {
	r.ReservationID = reservationID
	return r
}

func (r *ReviewBuilder) WithRating(rating int) *ReviewBuilder {
	r.Rating = rating
	return r
}

func (r *ReviewBuilder) WithComment(comment string) *ReviewBuilder {
	r.Comment = comment
	return r
}

func (r *ReviewBuilder) WithCreatedAt(createdAt time.Time) *ReviewBuilder {
	r.CreatedAt = createdAt
	return r
}

func (r *ReviewBuilder) WithUpdatedAt(updatedAt time.Time) *ReviewBuilder {
	r.UpdatedAt = updatedAt
	return r
}

func (r *ReviewBuilder) AsPoorRating() *ReviewBuilder {
	r.Rating = 1
	r.Comment = "Poor service"
	return r
}

func (r *ReviewBuilder) AsExcellentRating() *ReviewBuilder {
	r.Rating = 5
	r.Comment = "Excellent service!"
	return r
}

func (r *ReviewBuilder) BuildResourceRatingStats() *queries.ResourceRatingStats {
	return &queries.ResourceRatingStats{
		ResourceID:    r.ResourceID,
		TotalReviews:  10,
		AverageRating: 4.2,
		Rating1Count:  1,
		Rating2Count:  1,
		Rating3Count:  2,
		Rating4Count:  3,
		Rating5Count:  3,
		UpdatedAt:     r.UpdatedAt,
	}
}
