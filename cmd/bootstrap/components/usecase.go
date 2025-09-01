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
	usecaseBaseOption,
	usecaseQueriesModule,
	usecaseValidatorsModule,
	usecaseCommandsModule,
)

var usecaseBaseOption = fx.Provide(
	clock.NewRealClock,
	fx.Annotate(
		reservation.NewDefaultPriceCalculator,
		fx.As(new(reservation.PriceCalculator)),
	),
	func(clock clock.Clock, calc reservation.PriceCalculator) *reservation.Services {
		return &reservation.Services{
			Clock:           clock,
			PriceCalculator: calc,
		}
	},
)

var usecaseCommandsModule = fx.Module("usecase/commands",
	fx.Provide(
		commands.NewAuthCommands,
		commands.NewReservationCommands,
		commands.NewReviewCommands,
	),
)

var usecaseQueriesModule = fx.Module("usecase/queries",
	fx.Provide(
		queries.NewUserQueries,
		queries.NewReservationQueries,
		queries.NewReviewQueries,
	),
)

var usecaseValidatorsModule = fx.Module("usecase/validators",
	fx.Provide(
		usecase.NewTokenValidator,
	),
)
