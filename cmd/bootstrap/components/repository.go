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
		NewDBTX,
		fx.Annotate(
			repo_impl.NewUserRepository,
			fx.As(new(usecase.UserRepository)),
		),
		fx.Annotate(
			repo_impl.NewReservationRepository,
			fx.As(new(usecase.ReservationRepository)),
		),
		fx.Annotate(
			repo_impl.NewResourceRepository,
			fx.As(new(usecase.ResourceRepository)),
		),
		fx.Annotate(
			repo_impl.NewCouponRepository,
			fx.As(new(usecase.CouponRepository)),
		),
		fx.Annotate(
			repo_impl.NewIdempotencyRepository,
			fx.As(new(usecase.IdempotencyRepository)),
		),
		fx.Annotate(
			repo_impl.NewNotificationRepository,
			fx.As(new(usecase.NotificationRepository)),
		),
	),
)

func NewSQLQueries(_ *pgxpool.Pool) *sqlc.Queries {
	return sqlc.New()
}

func NewDBTX(pool *pgxpool.Pool) sqlc.DBTX {
	return pool
}
