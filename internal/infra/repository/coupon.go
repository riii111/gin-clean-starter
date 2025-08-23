package repository

import (
	"context"

	"gin-clean-starter/internal/infra"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/internal/pkg/pgconv"
	"gin-clean-starter/internal/usecase/commands"

	"github.com/google/uuid"
)

type CouponWriteQueries interface {
	GetCouponByCode(ctx context.Context, db sqlc.DBTX, code string) (sqlc.Coupons, error)
	GetCouponByID(ctx context.Context, db sqlc.DBTX, id uuid.UUID) (sqlc.Coupons, error)
}

type CouponRepository struct {
	queries CouponWriteQueries
	db      sqlc.DBTX
}

func NewCouponRepository(queries *sqlc.Queries, db sqlc.DBTX) *CouponRepository {
	return &CouponRepository{
		queries: queries,
		db:      db,
	}
}

func (r *CouponRepository) FindByCode(ctx context.Context, code string) (*commands.CouponSnapshot, error) {
	row, err := r.queries.GetCouponByCode(ctx, r.db, code)
	if err != nil {
		if pgconv.IsNoRows(err) {
			return nil, infra.WrapRepoErr("coupon not found", err, infra.KindNotFound)
		}
		return nil, infra.WrapRepoErr("failed to find coupon by code", err)
	}

	rm, err := toCouponSnapshotFromRow(row)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to convert coupon row", err)
	}
	return rm, nil
}

func (r *CouponRepository) FindByID(ctx context.Context, id uuid.UUID) (*commands.CouponSnapshot, error) {
	row, err := r.queries.GetCouponByID(ctx, r.db, id)
	if err != nil {
		if pgconv.IsNoRows(err) {
			return nil, infra.WrapRepoErr("coupon not found", err, infra.KindNotFound)
		}
		return nil, infra.WrapRepoErr("failed to find coupon by ID", err)
	}

	rm, err := toCouponSnapshotFromRow(row)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to convert coupon row", err)
	}
	return rm, nil
}

func toCouponSnapshotFromRow(row sqlc.Coupons) (*commands.CouponSnapshot, error) {
	rm := &commands.CouponSnapshot{
		ID:   row.ID,
		Code: row.Code,
	}

	if row.AmountOffCents.Valid {
		amountOff := row.AmountOffCents.Int32
		rm.AmountOffCents = &amountOff
	}

	percentOff, err := pgconv.Float64PtrFromNumeric(row.PercentOff)
	if err != nil {
		return nil, err
	}
	rm.PercentOff = percentOff

	if row.ValidFrom.Valid {
		validFrom := row.ValidFrom.Time
		rm.ValidFrom = &validFrom
	}

	if row.ValidTo.Valid {
		validTo := row.ValidTo.Time
		rm.ValidTo = &validTo
	}

	return rm, nil
}
