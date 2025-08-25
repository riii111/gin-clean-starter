package readstore

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"gin-clean-starter/internal/infra"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/internal/usecase/queries"
)

type UserReadQueries interface {
	FindUserByID(ctx context.Context, db sqlc.DBTX, id uuid.UUID) (sqlc.FindUserByIDRow, error)
	FindUserByEmail(ctx context.Context, db sqlc.DBTX, email string) (sqlc.Users, error)
}

type UserReadStore struct {
	queries UserReadQueries
	db      sqlc.DBTX
}

func NewUserReadStore(queries UserReadQueries, db sqlc.DBTX) *UserReadStore {
	return &UserReadStore{
		queries: queries,
		db:      db,
	}
}

func (r *UserReadStore) FindByID(ctx context.Context, id uuid.UUID) (*queries.AuthorizedUserView, error) {
	row, err := r.queries.FindUserByID(ctx, r.db, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, infra.WrapRepoErr("user not found", err, infra.KindNotFound)
		}
		return nil, infra.WrapRepoErr("failed to find user by ID", err)
	}

	readModel := toAuthorizedUserViewFromFindByIDRow(row)
	return readModel, nil
}

func (r *UserReadStore) FindByEmail(ctx context.Context, email string) (*queries.AuthorizedUserView, string, error) {
	row, err := r.queries.FindUserByEmail(ctx, r.db, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", infra.WrapRepoErr("user not found", err, infra.KindNotFound)
		}
		return nil, "", infra.WrapRepoErr("failed to find user by email", err)
	}

	readModel := toAuthorizedUserViewFromUsers(row)
	return readModel, row.PasswordHash, nil
}

func toAuthorizedUserViewFromUsers(row sqlc.Users) *queries.AuthorizedUserView {
	rm := &queries.AuthorizedUserView{
		ID:       row.ID,
		Email:    row.Email,
		Role:     row.Role,
		IsActive: row.IsActive,
	}

	if row.CompanyID.Valid {
		companyID := uuid.UUID(row.CompanyID.Bytes)
		rm.CompanyID = &companyID
	}

	return rm
}

func toAuthorizedUserViewFromFindByIDRow(row sqlc.FindUserByIDRow) *queries.AuthorizedUserView {
	rm := &queries.AuthorizedUserView{
		ID:       row.ID,
		Email:    row.Email,
		Role:     row.Role,
		IsActive: row.IsActive,
	}

	if row.CompanyID.Valid {
		companyID := uuid.UUID(row.CompanyID.Bytes)
		rm.CompanyID = &companyID
	}

	return rm
}
