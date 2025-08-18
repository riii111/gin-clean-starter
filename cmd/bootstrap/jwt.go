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
	tokenDuration := 24 * time.Hour
	if cfg.JWT.Duration != "" {
		if duration, err := time.ParseDuration(cfg.JWT.Duration); err == nil {
			tokenDuration = duration
		}
	}

	return jwt.NewService(cfg.JWT.Secret, tokenDuration)
}
