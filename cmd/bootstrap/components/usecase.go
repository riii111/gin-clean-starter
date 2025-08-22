package components

import (
	"gin-clean-starter/internal/domain/reservation"
	"gin-clean-starter/internal/pkg/clock"
	"gin-clean-starter/internal/usecase"

	"go.uber.org/fx"
)

var UseCaseModule = fx.Module("usecase",
	fx.Provide(
		clock.NewRealClock,
		fx.Annotate(
			reservation.NewDefaultPriceCalculator,
			fx.As(new(reservation.PriceCalculator)),
		),
		reservation.NewFactory,
		usecase.NewAuthUseCase,
		usecase.NewReservationUseCase,
	),
)
