package components

import (
	"gin-clean-starter/internal/domain/reservation"
	"gin-clean-starter/internal/pkg/clock"
	"gin-clean-starter/internal/usecase"
	"gin-clean-starter/internal/usecase/commands"
	"gin-clean-starter/internal/usecase/queries"

	"go.uber.org/fx"
)

var UseCaseModule = fx.Module("usecase",
	fx.Provide(
		clock.NewRealClock,
		fx.Annotate(
			reservation.NewDefaultPriceCalculator,
			fx.As(new(reservation.PriceCalculator)),
		),
		// Aggregated domain services
		func(clock clock.Clock, calc reservation.PriceCalculator) *reservation.Services {
			return &reservation.Services{
				Clock:           clock,
				PriceCalculator: calc,
			}
		},

		// Queries
		queries.NewUserQueries,
		queries.NewReservationQueries,
		queries.NewReviewQueries,

		// Validators / helpers
		usecase.NewTokenValidator,

		// Commands
		commands.NewAuthCommands,
		commands.NewReservationCommands,
		commands.NewReviewCommands,
	),
)
