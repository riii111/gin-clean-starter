package commands

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"gin-clean-starter/internal/domain/user"
	reqdto "gin-clean-starter/internal/handler/dto/request"
	"gin-clean-starter/internal/pkg/errs"
	"gin-clean-starter/internal/pkg/jwt"
	"gin-clean-starter/internal/pkg/password"
	"gin-clean-starter/internal/usecase/queries"
	"gin-clean-starter/internal/usecase/shared"
)

var (
	ErrUserNotFound         = errs.New("user not found")
	ErrInvalidCredentials   = errs.New("invalid credentials")
	ErrUserInactive         = errs.New("user inactive")
	ErrAuthenticationFailed = errs.New("authentication failed")
	ErrTokenGeneration      = errs.New("token generation failed")
	ErrTokenValidation      = errs.New("token validation failed")
)

type LoginResult struct {
	UserID     uuid.UUID
	TokenPair  *TokenPair
	IsReplayed bool
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type AuthCommands interface {
	Login(ctx context.Context, req reqdto.LoginRequest) (*LoginResult, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
}

type authCommandsImpl struct {
	uow        shared.UnitOfWork
	readStore  queries.UserReadStore
	jwtService jwt.Service
}

func NewAuthCommands(uow shared.UnitOfWork, readStore queries.UserReadStore, jwtService jwt.Service) AuthCommands {
	return &authCommandsImpl{
		uow:        uow,
		readStore:  readStore,
		jwtService: jwtService,
	}
}

func (a *authCommandsImpl) Login(ctx context.Context, req reqdto.LoginRequest) (*LoginResult, error) {
	credentials, err := req.ToDomain()
	if err != nil {
		return nil, errs.Mark(err, ErrAuthenticationFailed)
	}

	userReadModel, err := a.validateUser(ctx, credentials)
	if err != nil {
		return nil, err
	}

	role, err := user.NewRole(userReadModel.Role)
	if err != nil {
		return nil, errs.Mark(err, ErrAuthenticationFailed)
	}

	accessToken, err := a.jwtService.GenerateAccessToken(userReadModel.ID, role)
	if err != nil {
		return nil, errs.Mark(err, ErrTokenGeneration)
	}

	refreshToken, err := a.jwtService.GenerateRefreshToken(userReadModel.ID, role)
	if err != nil {
		return nil, errs.Mark(err, ErrTokenGeneration)
	}

	err = a.uow.Within(ctx, func(ctx context.Context, tx shared.Tx) error {
		updateErr := tx.Users().UpdateLastLogin(ctx, tx.DB(), userReadModel.ID)
		if updateErr != nil {
			slog.Warn("failed to update last login", "user_id", userReadModel.ID, "error", updateErr.Error())
			// Continue without failing - this is not critical
		}
		return nil
	})
	if err != nil {
		slog.Warn("transaction failed during login", "user_id", userReadModel.ID, "error", err.Error())
		// Continue without failing - login was successful, only last_login update failed
	}

	tokenPair := &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}

	return &LoginResult{
		UserID:     userReadModel.ID,
		TokenPair:  tokenPair,
		IsReplayed: false,
	}, nil
}

func (a *authCommandsImpl) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
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

	// Validate user still exists and is active
	userReadModel, err := a.readStore.FindByID(ctx, claims.UserID)
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

func (a *authCommandsImpl) validateUser(ctx context.Context, credentials user.Credentials) (*queries.AuthorizedUserView, error) {
	userReadModel, hashedPassword, err := a.readStore.FindByEmail(ctx, credentials.Email().Value())
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
