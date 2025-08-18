package repo_impl

import (
	"context"
	"database/sql"
	"errors"

	"gin-clean-starter/internal/domain/user"
	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/infra/sqlc"
	"gin-clean-starter/internal/usecase/readmodel"

	"github.com/google/uuid"
)

type UserQueries interface {
	FindUserByEmail(ctx context.Context, db sqlc.DBTX, email string) (sqlc.Users, error)
	FindUserByID(ctx context.Context, db sqlc.DBTX, id uuid.UUID) (sqlc.FindUserByIDRow, error)
	UpdateUserLastLogin(ctx context.Context, db sqlc.DBTX, id uuid.UUID) error
}

type UserRepository struct {
	queries UserQueries
	db      sqlc.DBTX
}

func NewUserRepository(queries *sqlc.Queries, db sqlc.DBTX) *UserRepository {
	return &UserRepository{
		queries: queries,
		db:      db,
	}
}

func (r *UserRepository) FindByEmail(ctx context.Context, email user.Email) (*readmodel.AuthorizedUserRM, string, error) {
	row, err := r.queries.FindUserByEmail(ctx, r.db, email.Value())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", infra.WrapRepoErr("user not found", err, infra.KindNotFound)
		}
		return nil, "", infra.WrapRepoErr("failed to find user by email", err)
	}

	readModel := toAuthorizedUserRMFromUsers(row)
	return readModel, row.PasswordHash, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*readmodel.AuthorizedUserRM, error) {
	row, err := r.queries.FindUserByID(ctx, r.db, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, infra.WrapRepoErr("user not found", err, infra.KindNotFound)
		}
		return nil, infra.WrapRepoErr("failed to find user by ID", err)
	}

	readModel := toAuthorizedUserRMFromFindByIDRow(row)
	return readModel, nil
}

func (r *UserRepository) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	err := r.queries.UpdateUserLastLogin(ctx, r.db, userID)
	if err != nil {
		return infra.WrapRepoErr("failed to update user last login", err)
	}
	return nil
}

func toAuthorizedUserRMFromUsers(row sqlc.Users) *readmodel.AuthorizedUserRM {
	rm := &readmodel.AuthorizedUserRM{
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

func toAuthorizedUserRMFromFindByIDRow(row sqlc.FindUserByIDRow) *readmodel.AuthorizedUserRM {
	rm := &readmodel.AuthorizedUserRM{
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
