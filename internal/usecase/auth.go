package usecase

import (
	"context"
	"errors"

	"gin-clean-starter/internal/domain/user"
	"gin-clean-starter/internal/pkg/jwt"
	"gin-clean-starter/internal/pkg/password"
	"gin-clean-starter/internal/usecase/readmodel"

	"github.com/google/uuid"
)

var (
	ErrUserNotFound         = errors.New("user not found")
	ErrInvalidCredentials   = errors.New("invalid email or password")
	ErrUserInactive         = errors.New("user account is inactive")
	ErrAuthenticationFailed = errors.New("authentication failed")
	ErrTokenGeneration      = errors.New("token generation failed")
	ErrTokenValidation      = errors.New("token validation failed")
)

type UserRepository interface {
	FindByEmail(ctx context.Context, email user.Email) (*readmodel.AuthorizedUserRM, string, error)
	FindByID(ctx context.Context, id uuid.UUID) (*readmodel.AuthorizedUserRM, error)
	UpdateLastLogin(ctx context.Context, userID uuid.UUID) error
}

type AuthUseCase interface {
	Login(ctx context.Context, credentials user.Credentials) (*TokenPair, *readmodel.AuthorizedUserRM, error)
	GetCurrentUser(ctx context.Context, userID uuid.UUID) (*readmodel.AuthorizedUserRM, error)
	ValidateToken(tokenString string) (uuid.UUID, user.Role, error)
}

type authUseCaseImpl struct {
	userRepo   UserRepository
	jwtService *jwt.Service
}

func NewAuthUseCase(userRepo UserRepository, jwtService *jwt.Service) AuthUseCase {
	return &authUseCaseImpl{
		userRepo:   userRepo,
		jwtService: jwtService,
	}
}

func (a *authUseCaseImpl) Login(ctx context.Context, credentials user.Credentials) (*TokenPair, *readmodel.AuthorizedUserRM, error) {
	userReadModel, err := a.validateUser(ctx, credentials)
	if err != nil {
		return "", nil, err
	}

	role, err := user.NewRole(userReadModel.Role)
	if err != nil {
		return "", nil, ErrAuthenticationFailed
	}

	token, err := a.jwtService.GenerateToken(userReadModel.ID, role)
	if err != nil {
		return "", nil, ErrTokenGeneration
	}

	err = a.userRepo.UpdateLastLogin(ctx, userReadModel.ID)
	if err != nil {
		return "", nil, err
	}

	return token, userReadModel, nil
}

func (a *authUseCaseImpl) validateUser(ctx context.Context, credentials auth.Credentials) (*readmodel.AuthorizedUserRM, error) {
	userReadModel, hashedPassword, err := a.userRepo.FindByEmail(ctx, credentials.Email())
	if err != nil {
		return nil, ErrUserNotFound
	}

	if userReadModel == nil {
		return nil, ErrUserNotFound
	}

	if !userReadModel.IsActive {
		return nil, ErrUserInactive
	}

	err = password.ComparePassword(hashedPassword, credentials.Password().Value())
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	return userReadModel, nil
}

func (a *authUseCaseImpl) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*readmodel.AuthorizedUserRM, error) {
	user, err := a.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, ErrUserNotFound
	}

	if !user.IsActive {
		return nil, ErrUserInactive
	}

	return user, nil
}

func (a *authUseCaseImpl) ValidateToken(tokenString string) (uuid.UUID, user.Role, error) {
	claims, err := a.jwtService.ValidateToken(tokenString)
	if err != nil {
		return uuid.Nil, "", ErrTokenValidation
	}

	role, err := user.NewRole(claims.Role)
	if err != nil {
		return uuid.Nil, "", ErrTokenValidation
	}

	return claims.UserID, role, nil
}
