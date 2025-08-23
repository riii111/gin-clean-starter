package components

import (
	"gin-clean-starter/internal/infra/readstore"
	repo_impl "gin-clean-starter/internal/infra/repository"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
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
			repo_impl.NewIdempotencyRepository,
			fx.As(new(commands.IdempotencyRepository)),
		),
		fx.Annotate(
			repo_impl.NewNotificationRepository,
			fx.As(new(commands.NotificationRepository)),
		),
		// Read-side stores for Command layer
		fx.Annotate(
			readstore.NewReservationReadStore,
			fx.As(new(readstore.ReservationStore)),
		),
		fx.Annotate(
			readstore.NewResourceReadStore,
			fx.As(new(readstore.ResourceStore)),
		),
		fx.Annotate(
			readstore.NewCouponReadStore,
			fx.As(new(readstore.CouponStore)),
		),
		fx.Annotate(
			readstore.NewIdempotencyReadStore,
			fx.As(new(readstore.IdempotencyStore)),
		),
		// Query use cases for Handlers
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
