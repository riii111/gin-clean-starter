package bootstrap

import (
	"gin-clean-starter/cmd/bootstrap/components"

	"go.uber.org/fx"
)

var Module = fx.Options(
	ConfigModule,
	LoggerModule,
	DBModule,
	JWTModule,
	components.RepositoryModule,
	components.UseCaseModule,
	components.HandlerModule,
)
