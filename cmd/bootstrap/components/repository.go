package components

import (
	"gin-clean-starter/internal/infra/readrepo"
	"gin-clean-starter/internal/infra/sqlc"
	repo_impl "gin-clean-starter/internal/infra/writerepo"
	"gin-clean-starter/internal/usecase"
	"gin-clean-starter/internal/usecase/commands"
	"gin-clean-starter/internal/usecase/queries"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

var RepositoryModule = fx.Module("repository",
	fx.Provide(
		NewSQLQueries,
		NewDBTX,
		fx.Annotate(
			repo_impl.NewUserRepository,
			fx.As(new(usecase.UserRepository)),
		),
		fx.Annotate(
			repo_impl.NewReservationRepository,
			fx.As(new(commands.ReservationRepository)),
		),
		fx.Annotate(
			repo_impl.NewResourceRepository,
			fx.As(new(commands.ResourceRepository)),
		),
		fx.Annotate(
			repo_impl.NewCouponRepository,
			fx.As(new(commands.CouponRepository)),
		),
		fx.Annotate(
			repo_impl.NewIdempotencyRepository,
			fx.As(new(commands.IdempotencyRepository)),
		),
		fx.Annotate(
			repo_impl.NewNotificationRepository,
			fx.As(new(commands.NotificationRepository)),
		),
		// Read-side repository for queries
		fx.Annotate(
			readrepo.NewReservationViewRepository,
			fx.As(new(queries.ReservationViewRepo)),
		),
		// Read-side use case
		queries.NewReservationQueries,
	),
)

func NewSQLQueries(_ *pgxpool.Pool) *sqlc.Queries {
	return sqlc.New()
}

func NewDBTX(pool *pgxpool.Pool) sqlc.DBTX {
	return pool
}
