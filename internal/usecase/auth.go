package usecase

import (
	"context"
	"log/slog"

	"gin-clean-starter/internal/domain/user"
	reqdto "gin-clean-starter/internal/handler/dto/request"
	"gin-clean-starter/internal/pkg/errs"
	"gin-clean-starter/internal/pkg/jwt"
	"gin-clean-starter/internal/pkg/password"
	"gin-clean-starter/internal/usecase/queries"

	"github.com/google/uuid"
)

var (
	ErrUserNotFound         = errs.New("user not found")
	ErrInvalidCredentials   = errs.New("invalid email or password")
	ErrUserInactive         = errs.New("user account is inactive")
	ErrAuthenticationFailed = errs.New("authentication failed")
	ErrTokenGeneration      = errs.New("token generation failed")
	ErrTokenValidation      = errs.New("token validation failed")
)

type UserRepository interface {
	FindByEmail(ctx context.Context, email user.Email) (*queries.AuthorizedUserView, string, error)
	FindByID(ctx context.Context, id uuid.UUID) (*queries.AuthorizedUserView, error)
	UpdateLastLogin(ctx context.Context, userID uuid.UUID) error
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type AuthUseCase interface {
	Login(ctx context.Context, req reqdto.LoginRequest) (*TokenPair, *queries.AuthorizedUserView, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
	GetCurrentUser(ctx context.Context, userID uuid.UUID) (*queries.AuthorizedUserView, error)
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

func (a *authUseCaseImpl) Login(ctx context.Context, req reqdto.LoginRequest) (*TokenPair, *queries.AuthorizedUserView, error) {
	credentials, err := req.ToDomain()
	if err != nil {
		return nil, nil, ErrAuthenticationFailed
	}

	userReadModel, err := a.validateUser(ctx, credentials)
	if err != nil {
		return nil, nil, err
	}

	role, err := user.NewRole(userReadModel.Role)
	if err != nil {
		return nil, nil, errs.Mark(err, ErrAuthenticationFailed)
	}

	accessToken, err := a.jwtService.GenerateAccessToken(userReadModel.ID, role)
	if err != nil {
		return nil, nil, errs.Mark(err, ErrTokenGeneration)
	}

	refreshToken, err := a.jwtService.GenerateRefreshToken(userReadModel.ID, role)
	if err != nil {
		return nil, nil, errs.Mark(err, ErrTokenGeneration)
	}

	err = a.userRepo.UpdateLastLogin(ctx, userReadModel.ID)
	if err != nil {
		slog.Warn("failed to update last login", "user_id", userReadModel.ID, "error", err.Error())
	}

	tokenPair := &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}

	return tokenPair, userReadModel, nil
}

func (a *authUseCaseImpl) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	claims, err := a.jwtService.ValidateToken(refreshToken)
	if err != nil {
		return nil, errs.Mark(err, ErrTokenValidation)
	}

	if claims.TokenType != jwt.TokenTypeRefresh {
		return nil, ErrTokenValidation
	}

	role, err := user.NewRole(claims.Role)
	if err != nil {
		return nil, errs.Mark(err, ErrTokenValidation)
	}

	userReadModel, err := a.userRepo.FindByID(ctx, claims.UserID)
	if err != nil || userReadModel == nil {
		return nil, ErrUserNotFound
	}

	if !userReadModel.IsActive {
		return nil, ErrUserInactive
	}

	accessToken, err := a.jwtService.GenerateAccessToken(claims.UserID, role)
	if err != nil {
		return nil, errs.Mark(err, ErrTokenGeneration)
	}

	newRefreshToken, err := a.jwtService.GenerateRefreshToken(claims.UserID, role)
	if err != nil {
		return nil, errs.Mark(err, ErrTokenGeneration)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (a *authUseCaseImpl) validateUser(ctx context.Context, credentials user.Credentials) (*queries.AuthorizedUserView, error) {
	userReadModel, hashedPassword, err := a.userRepo.FindByEmail(ctx, credentials.Email())
	if err != nil {
		// Return same error as password mismatch to prevent user enumeration attacks
		return nil, ErrInvalidCredentials
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

func (a *authUseCaseImpl) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*queries.AuthorizedUserView, error) {
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
	if err != nil || claims.TokenType != jwt.TokenTypeAccess {
		if err != nil {
			return uuid.Nil, "", errs.Mark(err, ErrTokenValidation)
		}
		return uuid.Nil, "", ErrTokenValidation
	}

	role, err := user.NewRole(claims.Role)
	if err != nil {
		return uuid.Nil, "", errs.Mark(err, ErrTokenValidation)
	}

	return claims.UserID, role, nil
}
