package queries

import (
	"context"

	"github.com/google/uuid"

	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/pkg/errs"
)

var (
	ErrUserNotFound = errs.New("user not found")
	ErrUserInactive = errs.New("user inactive")
	ErrUserAccess   = errs.New("user access denied")
)

type UserQueries interface {
	GetCurrentUser(ctx context.Context, userID uuid.UUID) (*AuthorizedUserView, error)
}

type UserReadStore interface {
	FindByID(ctx context.Context, id uuid.UUID) (*AuthorizedUserView, error)
	FindByEmail(ctx context.Context, email string) (*AuthorizedUserView, string, error)
}

type userQueriesImpl struct {
	readStore UserReadStore
}

func NewUserQueries(readStore UserReadStore) UserQueries {
	return &userQueriesImpl{
		readStore: readStore,
	}
}

func (q *userQueriesImpl) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*AuthorizedUserView, error) {
	user, err := q.readStore.FindByID(ctx, userID)
	if err != nil {
		if infra.IsKind(err, infra.KindNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if !user.IsActive {
		return nil, ErrUserInactive
	}

	return user, nil
}
