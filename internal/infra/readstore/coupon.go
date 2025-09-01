package readstore

import (
	"context"
	"strings"
	"time"

	"gin-clean-starter/internal/infra"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/internal/pkg/pgconv"
	"gin-clean-starter/internal/usecase/shared"
)

type CouponReadQueries interface {
	GetCouponByCode(ctx context.Context, db sqlc.DBTX, code string) (sqlc.Coupons, error)
}

type CouponStore interface {
	FindByCode(ctx context.Context, code string) (*shared.CouponSnapshot, error)
}

type CouponReadStore struct {
	queries CouponReadQueries
}

func NewCouponReadStore(queries CouponReadQueries) *CouponReadStore {
	return &CouponReadStore{
		queries: queries,
	}
}

func (r *CouponReadStore) FindByCode(ctx context.Context, db sqlc.DBTX, code string) (*shared.CouponSnapshot, error) {
	normalizedCode := strings.ToLower(code)
	row, err := r.queries.GetCouponByCode(ctx, db, normalizedCode)
	if err != nil {
		if pgconv.IsNoRows(err) {
			return nil, infra.WrapRepoErr("coupon not found", err, infra.KindNotFound)
		}
		return nil, infra.WrapRepoErr("failed to find coupon by code", err)
	}

	return toCouponSnapshotFromRow(row), nil
}

func toCouponSnapshotFromRow(row sqlc.Coupons) *shared.CouponSnapshot {
	percentOff, _ := pgconv.Float64PtrFromNumeric(row.PercentOff)

	var validFrom *time.Time
	if row.ValidFrom.Valid {
		t := row.ValidFrom.Time
		validFrom = &t
	}

	var validTo *time.Time
	if row.ValidTo.Valid {
		t := row.ValidTo.Time
		validTo = &t
	}

	return &shared.CouponSnapshot{
		ID:             row.ID,
		Code:           row.Code,
		AmountOffCents: pgconv.Int32PtrFromPgtype(row.AmountOffCents),
		PercentOff:     percentOff,
		ValidFrom:      validFrom,
		ValidTo:        validTo,
	}
}
