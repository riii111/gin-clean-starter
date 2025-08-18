package components

import (
	"gin-clean-starter/internal/infra/repo_impl"
	"gin-clean-starter/internal/usecase"

	"go.uber.org/fx"
)

var RepositoryModule = fx.Module("repository",
	fx.Provide(
		fx.Annotate(
			repo_impl.NewUserRepository,
			fx.As(new(usecase.UserRepository)),
		),
	),
)
