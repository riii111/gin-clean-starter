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
	GetUserByID(ctx context.Context, actorID uuid.UUID, actorRole string, targetID uuid.UUID) (*AuthorizedUserView, error)
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
			return nil, errs.Mark(err, ErrUserNotFound)
		}
		return nil, err
	}

	if !user.IsActive {
		return nil, ErrUserInactive
	}

	return user, nil
}

func (q *userQueriesImpl) GetUserByID(ctx context.Context, actorID uuid.UUID, actorRole string, targetID uuid.UUID) (*AuthorizedUserView, error) {
	if !canAccessUser(actorID, actorRole, targetID) {
		return nil, ErrUserAccess
	}

	user, err := q.readStore.FindByID(ctx, targetID)
	if err != nil {
		if infra.IsKind(err, infra.KindNotFound) {
			return nil, errs.Mark(err, ErrUserNotFound)
		}
		return nil, err
	}

	if !user.IsActive && actorRole != RoleAdmin {
		// Only admins can see inactive users
		return nil, ErrUserNotFound
	}

	return user, nil
}

func canAccessUser(actorID uuid.UUID, actorRole string, targetID uuid.UUID) bool {
	if actorID == targetID {
		return true
	}

	if actorRole == RoleAdmin {
		return true
	}

	// TODO: Implement company-based access control for operators
	if actorRole == RoleOperator {
		return true
	}

	return false
}
