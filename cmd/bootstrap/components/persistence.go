package components

import (
	"gin-clean-starter/internal/infra/readstore"
	"gin-clean-starter/internal/infra/repository"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/internal/infra/uow"
	"gin-clean-starter/internal/usecase/queries"
	"gin-clean-starter/internal/usecase/shared"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

var PersistenceModule = fx.Module("persistence",
	baseOption,
	readstoreModule,
	repositoryModule,
)

var baseOption = fx.Provide(
	NewSQLQueries,
	NewDBTX,
)

var readstoreModule = fx.Module("persistence/readstore",
	fx.Provide(
		// Resource
		fx.Annotate(
			NewSQLQueries,
			fx.As(new(readstore.ResourceReadQueries)),
		),
		fx.Annotate(
			readstore.NewResourceReadStore,
			fx.As(new(shared.ResourceReadStore)),
		),
		// Coupon
		fx.Annotate(
			NewSQLQueries,
			fx.As(new(readstore.CouponReadQueries)),
		),
		fx.Annotate(
			readstore.NewCouponReadStore,
			fx.As(new(shared.CouponReadStore)),
		),
		// Idempotency
		fx.Annotate(
			NewSQLQueries,
			fx.As(new(readstore.IdempotencyReadQueries)),
		),
		fx.Annotate(
			readstore.NewIdempotencyReadStore,
			fx.As(new(shared.IdempotencyReadStore)),
		),
		// User
		fx.Annotate(
			NewSQLQueries,
			fx.As(new(readstore.UserReadQueries)),
		),
		fx.Annotate(
			readstore.NewUserReadStore,
			fx.As(new(queries.UserReadStore)),
		),
		// Reservation
		fx.Annotate(
			NewSQLQueries,
			fx.As(new(readstore.ReservationViewQueries)),
		),
		fx.Annotate(
			readstore.NewReservationReadStore,
			fx.As(new(queries.ReservationReadStore)),
			fx.As(new(shared.ReservationSnapshotReadStore)),
		),
		// Review
		fx.Annotate(
			NewSQLQueries,
			fx.As(new(readstore.ReviewReadQueries)),
		),
		fx.Annotate(
			readstore.NewReviewReadStore,
			fx.As(new(queries.ReviewReadStore)),
			fx.As(new(shared.ReviewReadStore)),
		),
	),
)

var repositoryModule = fx.Module("persistence/repository",
	fx.Provide(
		// UnitOfWork
		fx.Annotate(
			uow.NewPostgresUoW,
			fx.As(new(shared.UnitOfWork)),
		),
		// User
		fx.Annotate(
			NewSQLQueries,
			fx.As(new(repository.UserWriteQueries)),
		),
		fx.Annotate(
			repository.NewUserRepository,
			fx.As(new(shared.UserRepository)),
		),
		// Reservation
		fx.Annotate(
			NewSQLQueries,
			fx.As(new(repository.ReservationWriteQueries)),
		),
		fx.Annotate(
			repository.NewReservationRepository,
			fx.As(new(shared.ReservationRepository)),
		),
		// Review
		fx.Annotate(
			NewSQLQueries,
			fx.As(new(repository.ReviewWriteQueries)),
		),
		fx.Annotate(
			repository.NewReviewRepository,
			fx.As(new(shared.ReviewRepository)),
		),
		// RatingStats
		fx.Annotate(
			NewSQLQueries,
			fx.As(new(repository.RatingStatsQueries)),
		),
		fx.Annotate(
			repository.NewRatingStatsRepository,
			fx.As(new(shared.RatingStatsRepository)),
		),
		// Idempotency
		fx.Annotate(
			NewSQLQueries,
			fx.As(new(repository.IdempotencyWriteQueries)),
		),
		fx.Annotate(
			repository.NewIdempotencyRepository,
			fx.As(new(shared.IdempotencyRepository)),
		),
		// Notification
		fx.Annotate(
			NewSQLQueries,
			fx.As(new(repository.NotificationWriteQueries)),
		),
		fx.Annotate(
			repository.NewNotificationRepository,
			fx.As(new(shared.NotificationRepository)),
		),
	),
)

func NewSQLQueries(_ *pgxpool.Pool) *sqlc.Queries {
	return sqlc.New()
}

func NewDBTX(pool *pgxpool.Pool) sqlc.DBTX {
	return pool
}
