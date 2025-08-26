package commands

import (
	"context"

	domreview "gin-clean-starter/internal/domain/review"
	reqdto "gin-clean-starter/internal/handler/dto/request"
	"gin-clean-starter/internal/pkg/clock"
	"gin-clean-starter/internal/pkg/errs"
	"gin-clean-starter/internal/usecase/queries"
	"gin-clean-starter/internal/usecase/shared"

	"github.com/google/uuid"
)

var (
	ErrReviewNotOwned         = errs.New("review not owned by user")
	ErrReviewNotFoundWrite    = errs.New("review not found")
	ErrReviewCreationFailed   = errs.New("review creation failed")
	ErrReviewUpdateFailed     = errs.New("review update failed")
	ErrReviewDeletionFailed   = errs.New("review deletion failed")
	ErrDomainValidationFailed = errs.New("domain validation failed")
)

type CreateReviewResult struct {
	ReviewID uuid.UUID
}

type ReviewCommands interface {
	CreateReview(ctx context.Context, req reqdto.CreateReviewRequest, userID uuid.UUID) (*CreateReviewResult, error)
	UpdateReview(ctx context.Context, reviewID uuid.UUID, req reqdto.UpdateReviewRequest, actorID uuid.UUID) error
	DeleteReview(ctx context.Context, reviewID uuid.UUID, actorID uuid.UUID, actorRole string) error
}

type reviewCommandsImpl struct {
	uow   shared.UnitOfWork
	clock clock.Clock
}

func NewReviewCommands(uow shared.UnitOfWork, clk clock.Clock) ReviewCommands {
	return &reviewCommandsImpl{uow: uow, clock: clk}
}

func (uc *reviewCommandsImpl) CreateReview(ctx context.Context, req reqdto.CreateReviewRequest, userID uuid.UUID) (*CreateReviewResult, error) {
	rating, comment, err := req.ToDomain()
	if err != nil {
		return nil, errs.Mark(err, ErrDomainValidationFailed)
	}

	// Check eligibility before creating review
	if err = uc.canPostReview(ctx, userID, req.ResourceID, req.ReservationID); err != nil {
		return nil, errs.Mark(err, ErrDomainValidationFailed)
	}

	now := uc.clock.Now()
	rev := domreview.NewReview(userID, req.ResourceID, req.ReservationID, rating, comment, now)

	var createdID uuid.UUID
	err = uc.uow.Within(ctx, func(ctx context.Context, tx shared.Tx) error {
		id, derr := tx.Reviews().Create(ctx, tx.DB(), rev)
		if derr != nil {
			return derr
		}
		createdID = id
		return tx.RatingStats().RecalcResourceRatingStats(ctx, tx.DB(), req.ResourceID)
	})
	if err != nil {
		return nil, errs.Mark(err, ErrReviewCreationFailed)
	}
	return &CreateReviewResult{ReviewID: createdID}, nil
}

func (uc *reviewCommandsImpl) UpdateReview(ctx context.Context, reviewID uuid.UUID, req reqdto.UpdateReviewRequest, actorID uuid.UUID) error {
	rating, comment, err := req.ToDomain()
	if err != nil {
		return errs.Mark(err, ErrDomainValidationFailed)
	}

	return uc.uow.Within(ctx, func(ctx context.Context, tx shared.Tx) error {
		snap, derr := tx.Reads().ReviewByID(ctx, reviewID)
		if derr != nil {
			return errs.Mark(derr, ErrReviewNotFoundWrite)
		}
		if snap.UserID != actorID {
			return ErrReviewNotOwned
		}

		agg := domreview.ReconstructReview(snap.ID, snap.UserID, snap.ResourceID, snap.ReservationID, rating, comment, uc.clock.Now(), uc.clock.Now())
		if derr = tx.Reviews().Update(ctx, tx.DB(), agg); derr != nil {
			return errs.Mark(derr, ErrReviewUpdateFailed)
		}
		if derr = tx.RatingStats().RecalcResourceRatingStats(ctx, tx.DB(), snap.ResourceID); derr != nil {
			return errs.Mark(derr, ErrReviewUpdateFailed)
		}
		return nil
	})
}

func (uc *reviewCommandsImpl) DeleteReview(ctx context.Context, reviewID uuid.UUID, actorID uuid.UUID, actorRole string) error {
	return uc.uow.Within(ctx, func(ctx context.Context, tx shared.Tx) error {
		snap, derr := tx.Reads().ReviewByID(ctx, reviewID)
		if derr != nil {
			return errs.Mark(derr, ErrReviewNotFoundWrite)
		}
		if actorRole != queries.RoleAdmin && snap.UserID != actorID {
			return ErrReviewNotOwned
		}
		if derr = tx.Reviews().Delete(ctx, tx.DB(), reviewID); derr != nil {
			return errs.Mark(derr, ErrReviewDeletionFailed)
		}
		if derr = tx.RatingStats().RecalcResourceRatingStats(ctx, tx.DB(), snap.ResourceID); derr != nil {
			return errs.Mark(derr, ErrReviewDeletionFailed)
		}
		return nil
	})
}

func (uc *reviewCommandsImpl) canPostReview(ctx context.Context, userID, resourceID, reservationID uuid.UUID) error {
	resSnap, err := uc.uow.CommandReads().ReservationByID(ctx, reservationID)
	if err != nil {
		return err
	}
	if resSnap.UserID != userID || resSnap.ResourceID != resourceID {
		return domreview.ErrReservationNotEligible
	}
	if resSnap.Status != "confirmed" {
		return domreview.ErrReservationNotEligible
	}
	now := uc.clock.Now()
	if !resSnap.EndTime.Before(now) {
		return domreview.ErrReservationNotEligible
	}
	return nil
}
