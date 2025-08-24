package components

import (
	"gin-clean-starter/internal/domain/reservation"
	"gin-clean-starter/internal/pkg/clock"
	"gin-clean-starter/internal/usecase"
	"gin-clean-starter/internal/usecase/commands"

	"go.uber.org/fx"
)

var UseCaseModule = fx.Module("usecase",
	fx.Provide(
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
		usecase.NewAuthUseCase,
		commands.NewReservationUseCase,
	),
)
