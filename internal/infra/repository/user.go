package repository

import (
	"context"

	"gin-clean-starter/internal/infra"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"

	"github.com/google/uuid"
)

type UserWriteQueries interface {
	UpdateUserLastLogin(ctx context.Context, db sqlc.DBTX, id uuid.UUID) error
	CreateUser(ctx context.Context, db sqlc.DBTX, arg sqlc.CreateUserParams) (sqlc.CreateUserRow, error)
}

type UserRepository struct {
	queries UserWriteQueries
}

func NewUserRepository(queries UserWriteQueries) *UserRepository {
	return &UserRepository{
		queries: queries,
	}
}

func (r *UserRepository) UpdateLastLogin(ctx context.Context, tx sqlc.DBTX, userID uuid.UUID) error {
	err := r.queries.UpdateUserLastLogin(ctx, tx, userID)
	if err != nil {
		return infra.WrapRepoErr("failed to update user last login", err)
	}
	return nil
}

func (r *UserRepository) Create(ctx context.Context, tx sqlc.DBTX, params sqlc.CreateUserParams) (uuid.UUID, error) {
	row, err := r.queries.CreateUser(ctx, tx, params)
	if err != nil {
		return uuid.Nil, infra.WrapRepoErr("failed to create user", err)
	}
	return row.ID, nil
}
