package repo_impl

import (
	"context"
	"database/sql"
	"errors"

	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/infra/sqlc"
	"gin-clean-starter/internal/usecase/readmodel"

	"github.com/google/uuid"
)

type CouponQueries interface {
	GetCouponByCode(ctx context.Context, db sqlc.DBTX, code string) (sqlc.Coupons, error)
	GetCouponByID(ctx context.Context, db sqlc.DBTX, id uuid.UUID) (sqlc.Coupons, error)
}

type CouponRepository struct {
	queries CouponQueries
	db      sqlc.DBTX
}

func NewCouponRepository(queries *sqlc.Queries, db sqlc.DBTX) *CouponRepository {
	return &CouponRepository{
		queries: queries,
		db:      db,
	}
}

func (r *CouponRepository) FindByCode(ctx context.Context, code string) (*readmodel.CouponRM, error) {
	row, err := r.queries.GetCouponByCode(ctx, r.db, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, infra.WrapRepoErr("coupon not found", err, infra.KindNotFound)
		}
		return nil, infra.WrapRepoErr("failed to find coupon by code", err)
	}

	return toCouponRMFromRow(row), nil
}

func (r *CouponRepository) FindByID(ctx context.Context, id uuid.UUID) (*readmodel.CouponRM, error) {
	row, err := r.queries.GetCouponByID(ctx, r.db, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, infra.WrapRepoErr("coupon not found", err, infra.KindNotFound)
		}
		return nil, infra.WrapRepoErr("failed to find coupon by ID", err)
	}

	return toCouponRMFromRow(row), nil
}

func toCouponRMFromRow(row sqlc.Coupons) *readmodel.CouponRM {
	rm := &readmodel.CouponRM{
		ID:        row.ID,
		Code:      row.Code,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}

	if row.AmountOffCents.Valid {
		amountOff := row.AmountOffCents.Int32
		rm.AmountOffCents = &amountOff
	}

	if row.PercentOff.Valid {
		percentOffFloat8, _ := row.PercentOff.Float64Value()
		percentOff := percentOffFloat8.Float64
		rm.PercentOff = &percentOff
	}

	if row.ValidFrom.Valid {
		validFrom := row.ValidFrom.Time
		rm.ValidFrom = &validFrom
	}

	if row.ValidTo.Valid {
		validTo := row.ValidTo.Time
		rm.ValidTo = &validTo
	}

	return rm
}
