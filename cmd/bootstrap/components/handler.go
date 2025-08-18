package components

import (
	"gin-clean-starter/internal/handler"
	"gin-clean-starter/internal/handler/api"
	"gin-clean-starter/internal/handler/middleware"
	"gin-clean-starter/internal/pkg/config"
	"gin-clean-starter/internal/pkg/jwt"
	"gin-clean-starter/internal/usecase"

	"go.uber.org/fx"
)

var HandlerModule = fx.Module("handler",
	fx.Provide(
		NewAuthHandler,
		middleware.NewAuthMiddleware,
	),
	fx.Invoke(handler.NewRouter),
)

func NewAuthHandler(authUseCase usecase.AuthUseCase, jwtService *jwt.Service, cfg config.Config) *api.AuthHandler {
	return api.NewAuthHandler(authUseCase, jwtService, cfg)
}
