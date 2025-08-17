package bootstrap

import (
	"gin-clean-starter/internal/pkg/config"

	"go.uber.org/fx"
)

var ConfigModule = fx.Module("config",
	fx.Provide(
		config.LoadConfig,
	),
)
