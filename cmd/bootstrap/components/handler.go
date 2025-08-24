package components

import (
	"gin-clean-starter/internal/handler"
	"gin-clean-starter/internal/handler/api"
	"gin-clean-starter/internal/handler/middleware"

	"go.uber.org/fx"
)

var HandlerModule = fx.Module("handler",
	fx.Provide(
		api.NewAuthHandler,
		api.NewReservationHandler,
		middleware.NewAuthMiddleware,
	),
	fx.Invoke(handler.NewRouter),
)
