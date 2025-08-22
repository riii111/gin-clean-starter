package components

import (
	"gin-clean-starter/internal/pkg/clock"
	"gin-clean-starter/internal/usecase"

	"go.uber.org/fx"
)

var UseCaseModule = fx.Module("usecase",
	fx.Provide(
		clock.NewRealClock,
		usecase.NewAuthUseCase,
		usecase.NewReservationUseCase,
	),
)
