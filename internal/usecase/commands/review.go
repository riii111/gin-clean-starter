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
	ErrReviewNotOwned          = errs.New("review not owned by user")
	ErrReviewNotFoundWrite     = errs.New("review not found")
	ErrReviewCreationFailed    = errs.New("review creation failed")
	ErrReviewUpdateFailed      = errs.New("review update failed")
	ErrReviewDeletionFailed    = errs.New("review deletion failed")
	ErrDomainValidationFailed  = errs.New("domain validation failed")
	ErrRatingStatsRecalcFailed = errs.New("rating stats recalculation failed")
	ErrReservationCheckFailed  = errs.New("reservation check failed")
	ErrTransactionFailed       = errs.New("transaction failed")
)

type CreateReviewResult struct {
	ReviewID uuid.UUID
}

type ReviewCommands interface {
	Create(ctx context.Context, req reqdto.CreateReviewRequest, userID uuid.UUID) (*CreateReviewResult, error)
	Update(ctx context.Context, reviewID uuid.UUID, req reqdto.UpdateReviewRequest, actorID uuid.UUID) error
	Delete(ctx context.Context, reviewID uuid.UUID, actorID uuid.UUID, actorRole string) error
}

type reviewCommandsImpl struct {
	uow   shared.UnitOfWork
	clock clock.Clock
}

func NewReviewCommands(uow shared.UnitOfWork, clk clock.Clock) ReviewCommands {
	return &reviewCommandsImpl{uow: uow, clock: clk}
}

func (uc *reviewCommandsImpl) Create(ctx context.Context, req reqdto.CreateReviewRequest, userID uuid.UUID) (*CreateReviewResult, error) {
	if err := uc.canPostReview(ctx, userID, req.ResourceID, req.ReservationID); err != nil {
		return nil, errs.Mark(err, ErrDomainValidationFailed)
	}

	now := uc.clock.Now()
	rev, err := req.ToDomain(userID, now)
	if err != nil {
		return nil, errs.Mark(err, ErrDomainValidationFailed)
	}

	var createdID uuid.UUID
	err = uc.uow.Within(ctx, func(ctx context.Context, tx shared.Tx) error {
		id, derr := tx.Reviews().Create(ctx, tx.DB(), rev)
		if derr != nil {
			return errs.Mark(derr, ErrReviewCreationFailed)
		}
		createdID = id
		if derr := tx.RatingStats().RecalcResourceRatingStats(ctx, tx.DB(), req.ResourceID); derr != nil {
			return errs.Mark(derr, ErrRatingStatsRecalcFailed)
		}
		return nil
	})
	if err != nil {
		return nil, errs.Mark(err, ErrTransactionFailed)
	}
	return &CreateReviewResult{ReviewID: createdID}, nil
}

func (uc *reviewCommandsImpl) Update(ctx context.Context, reviewID uuid.UUID, req reqdto.UpdateReviewRequest, actorID uuid.UUID) error {
	err := uc.uow.Within(ctx, func(ctx context.Context, tx shared.Tx) error {
		existing, err := tx.Reads().ReviewByID(ctx, reviewID)
		if err != nil {
			return errs.Mark(err, ErrReviewNotFoundWrite)
		}

		if existing.UserID != actorID {
			return ErrReviewNotOwned
		}

		now := uc.clock.Now()
		updatedReview, err := req.ToDomain(existing, now)
		if err != nil {
			return errs.Mark(err, ErrDomainValidationFailed)
		}

		if derr := tx.Reviews().Update(ctx, tx.DB(), reviewID, updatedReview); derr != nil {
			return errs.Mark(derr, ErrReviewUpdateFailed)
		}
		if derr := tx.RatingStats().RecalcResourceRatingStats(ctx, tx.DB(), existing.ResourceID); derr != nil {
			return errs.Mark(derr, ErrRatingStatsRecalcFailed)
		}
		return nil
	})
	if err != nil {
		return errs.Mark(err, ErrTransactionFailed)
	}
	return nil
}

func (uc *reviewCommandsImpl) Delete(ctx context.Context, reviewID uuid.UUID, actorID uuid.UUID, actorRole string) error {
	err := uc.uow.Within(ctx, func(ctx context.Context, tx shared.Tx) error {
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
			return errs.Mark(derr, ErrRatingStatsRecalcFailed)
		}
		return nil
	})
	if err != nil {
		return errs.Mark(err, ErrTransactionFailed)
	}
	return nil
}

func (uc *reviewCommandsImpl) canPostReview(ctx context.Context, userID, resourceID, reservationID uuid.UUID) error {
	resSnap, err := uc.uow.CommandReads().ReservationByID(ctx, reservationID)
	if err != nil {
		return errs.Mark(err, ErrReservationCheckFailed)
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
