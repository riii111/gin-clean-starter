package commands

import (
	"context"

	domreview "gin-clean-starter/internal/domain/review"
	"gin-clean-starter/internal/pkg/clock"
	"gin-clean-starter/internal/pkg/errs"
	"gin-clean-starter/internal/usecase/queries"
	"gin-clean-starter/internal/usecase/shared"

	"github.com/google/uuid"
)

var (
	ErrReviewNotOwned      = errs.New("review not owned by user")
	ErrDuplicateReview     = errs.New("duplicate review for reservation")
	ErrReviewNotFoundWrite = errs.New("review not found")
)

type CreateReviewResult struct {
	ReviewID uuid.UUID
}

type ReviewCommands interface {
	CreateReview(ctx context.Context, req CreateReviewRequest, userID uuid.UUID) (*CreateReviewResult, error)
	UpdateReview(ctx context.Context, reviewID uuid.UUID, req UpdateReviewRequest, actorID uuid.UUID) error
	DeleteReview(ctx context.Context, reviewID uuid.UUID, actorID uuid.UUID, actorRole string) error
}

type reviewUseCaseImpl struct {
	uow   shared.UnitOfWork
	clock clock.Clock
}

func NewReviewUseCase(uow shared.UnitOfWork, clk clock.Clock) ReviewCommands {
	return &reviewUseCaseImpl{uow: uow, clock: clk}
}

type CreateReviewRequest struct {
	ResourceID    uuid.UUID
	ReservationID uuid.UUID
	Rating        int
	Comment       string
}

type UpdateReviewRequest struct {
	Rating  int
	Comment string
}

func (uc *reviewUseCaseImpl) CreateReview(ctx context.Context, req CreateReviewRequest, userID uuid.UUID) (*CreateReviewResult, error) {
	rating, err := domreview.NewRating(req.Rating)
	if err != nil {
		return nil, err
	}
	comment, err := domreview.NewComment(req.Comment)
	if err != nil {
		return nil, err
	}

	services := &domreview.Services{
		Clock:              uc.clock,
		EligibilityChecker: uc,
	}

	var createdID uuid.UUID
	err = uc.uow.Within(ctx, func(ctx context.Context, tx shared.Tx) error {
		rev, derr := domreview.NewReview(services, userID, req.ResourceID, req.ReservationID, rating, comment)
		if derr != nil {
			return derr
		}

		id, derr := tx.Reviews().Create(ctx, tx.DB(), rev)
		if derr != nil {
			return derr
		}
		createdID = id
		return tx.RatingStats().RecalcResourceRatingStats(ctx, tx.DB(), req.ResourceID)
	})
	if err != nil {
		return nil, err
	}
	return &CreateReviewResult{ReviewID: createdID}, nil
}

func (uc *reviewUseCaseImpl) UpdateReview(ctx context.Context, reviewID uuid.UUID, req UpdateReviewRequest, actorID uuid.UUID) error {
	rating, err := domreview.NewRating(req.Rating)
	if err != nil {
		return err
	}
	comment, err := domreview.NewComment(req.Comment)
	if err != nil {
		return err
	}

	return uc.uow.Within(ctx, func(ctx context.Context, tx shared.Tx) error {
		snap, derr := tx.Reads().ReviewByID(ctx, reviewID)
		if derr != nil {
			return derr
		}
		if snap.UserID != actorID {
			return ErrReviewNotOwned
		}

		agg := domreview.ReconstructReview(snap.ID, snap.UserID, snap.ResourceID, snap.ReservationID, rating, comment, uc.clock.Now(), uc.clock.Now())
		if derr = tx.Reviews().Update(ctx, tx.DB(), agg); derr != nil {
			return derr
		}
		return tx.RatingStats().RecalcResourceRatingStats(ctx, tx.DB(), snap.ResourceID)
	})
}

func (uc *reviewUseCaseImpl) DeleteReview(ctx context.Context, reviewID uuid.UUID, actorID uuid.UUID, actorRole string) error {
	return uc.uow.Within(ctx, func(ctx context.Context, tx shared.Tx) error {
		snap, derr := tx.Reads().ReviewByID(ctx, reviewID)
		if derr != nil {
			return derr
		}
		if actorRole != queries.RoleAdmin && snap.UserID != actorID {
			return ErrReviewNotOwned
		}
		if derr = tx.Reviews().Delete(ctx, tx.DB(), reviewID); derr != nil {
			return derr
		}
		return tx.RatingStats().RecalcResourceRatingStats(ctx, tx.DB(), snap.ResourceID)
	})
}

// ReviewEligibilityChecker implementation
func (uc *reviewUseCaseImpl) CanPostReview(input domreview.EligibilityInput) error {
	resSnap, err := uc.uow.CommandReads().ReservationByID(context.Background(), input.ReservationID)
	if err != nil {
		return err
	}
	if resSnap.UserID != input.UserID || resSnap.ResourceID != input.ResourceID {
		return domreview.ErrReservationNotEligible
	}
	if resSnap.Status != "confirmed" {
		return domreview.ErrReservationNotEligible
	}
	if !resSnap.EndTime.Before(input.Now) {
		return domreview.ErrReservationNotEligible
	}
	return nil
}
