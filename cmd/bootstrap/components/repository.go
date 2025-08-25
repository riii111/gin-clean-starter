package components

import (
	"gin-clean-starter/internal/infra/readstore"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/internal/infra/uow"
	"gin-clean-starter/internal/usecase/queries"
	"gin-clean-starter/internal/usecase/shared"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

var RepositoryModule = fx.Module("repository",
	fx.Provide(
		NewSQLQueries,
		NewDBTX,
		fx.Annotate(
			uow.NewPostgresUoW,
			fx.As(new(shared.UnitOfWork)),
		),
		fx.Annotate(
			readstore.NewUserReadStore,
			fx.As(new(queries.UserReadStore)),
		),
		fx.Annotate(
			readstore.NewReservationReadStore,
			fx.As(new(queries.ReservationReadStore)),
		),
		queries.NewReservationQueries,
	),
)

func NewSQLQueries(_ *pgxpool.Pool) *sqlc.Queries {
	return sqlc.New()
}

func NewDBTX(pool *pgxpool.Pool) sqlc.DBTX {
	return pool
}
