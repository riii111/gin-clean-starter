package repo_impl

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"gin-clean-starter/internal/domain/user"
	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/infra/sqlc"
	"gin-clean-starter/internal/usecase"
	"gin-clean-starter/internal/usecase/readmodel"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jinzhu/copier"
)

type UserQueries interface {
	FindUserByEmail(ctx context.Context, email string) (sqlc.Users, error)
	FindUserByID(ctx context.Context, id uuid.UUID) (sqlc.FindUserByIDRow, error)
	UpdateUserLastLogin(ctx context.Context, params sqlc.UpdateUserLastLoginParams) error
}

type userRepository struct {
	queries UserQueries
}

func NewUserRepository(queries *sqlc.Queries) usecase.UserRepository {
	return &userRepository{
		queries: queries,
	}
}

func (r *userRepository) FindByEmail(ctx context.Context, email user.Email) (*readmodel.AuthorizedUserRM, string, error) {
	row, err := r.queries.FindUserByEmail(ctx, email.Value())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", infra.WrapRepoErr(slog.Default(), infra.KindNotFound, "user not found", err)
		}
		return nil, "", infra.WrapRepoErr(slog.Default(), infra.KindDBFailure, "failed to find user by email", err)
	}

	readModel := &readmodel.AuthorizedUserRM{}

	if err := copier.Copy(readModel, &row); err != nil {
		return nil, "", infra.WrapRepoErr(slog.Default(), infra.KindDBFailure, "failed to copy fields", err)
	}

	if row.CompanyID.Valid {
		id := uuid.UUID(row.CompanyID.Bytes)
		readModel.CompanyID = &id
	}

	return readModel, row.PasswordHash, nil
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*readmodel.AuthorizedUserRM, error) {
	row, err := r.queries.FindUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, infra.WrapRepoErr(slog.Default(), infra.KindNotFound, "user not found", err)
		}
		return nil, infra.WrapRepoErr(slog.Default(), infra.KindDBFailure, "failed to find user by ID", err)
	}

	readModel := &readmodel.AuthorizedUserRM{}

	if err := copier.Copy(readModel, &row); err != nil {
		return nil, infra.WrapRepoErr(slog.Default(), infra.KindDBFailure, "failed to copy fields", err)
	}

	if row.CompanyID.Valid {
		id := uuid.UUID(row.CompanyID.Bytes)
		readModel.CompanyID = &id
	}

	return readModel, nil
}

func (r *userRepository) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	params := sqlc.UpdateUserLastLoginParams{
		ID: userID,
		LastLogin: pgtype.Timestamptz{
			Time:  time.Now(),
			Valid: true,
		},
	}
	err := r.queries.UpdateUserLastLogin(ctx, params)
	if err != nil {
		return infra.WrapRepoErr(slog.Default(), infra.KindDBFailure, "failed to update user last login", err)
	}
	return nil
}
