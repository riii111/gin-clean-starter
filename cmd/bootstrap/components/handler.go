package components

import (
	"gin-clean-starter/internal/handler"
	"gin-clean-starter/internal/handler/api"

	"go.uber.org/fx"
)

var HandlerModule = fx.Module("handler",
	fx.Provide(
		api.NewAuthHandler,
	),
	fx.Invoke(handler.NewRouter),
)
