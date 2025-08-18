package bootstrap

import (
	"time"

	"gin-clean-starter/internal/pkg/config"
	"gin-clean-starter/internal/pkg/jwt"

	"go.uber.org/fx"
)

var JWTModule = fx.Module("jwt",
	fx.Provide(
		NewJWTService,
	),
)

func NewJWTService(cfg config.Config) *jwt.Service {
	accessTokenDuration, err := time.ParseDuration(cfg.JWT.AccessTokenDuration)
	if err != nil {
		panic("invalid JWT_ACCESS_TOKEN_DURATION: " + err.Error())
	}

	refreshTokenDuration, err := time.ParseDuration(cfg.JWT.RefreshTokenDuration)
	if err != nil {
		panic("invalid JWT_REFRESH_TOKEN_DURATION: " + err.Error())
	}

	return jwt.NewService(cfg.JWT.Secret, accessTokenDuration, refreshTokenDuration)
}
