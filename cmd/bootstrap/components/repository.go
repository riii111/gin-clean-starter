package components

import (
	"gin-clean-starter/internal/infra/repo_impl"
	"gin-clean-starter/internal/infra/sqlc"
	"gin-clean-starter/internal/usecase"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

var RepositoryModule = fx.Module("repository",
	fx.Provide(
		NewSQLQueries,
		fx.Annotate(
			repo_impl.NewUserRepository,
			fx.As(new(usecase.UserRepository)),
		),
	),
)

func NewSQLQueries(_ *pgxpool.Pool) *sqlc.Queries {
	return sqlc.New()
}
