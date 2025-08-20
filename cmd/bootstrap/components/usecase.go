package components

import (
	"gin-clean-starter/internal/usecase"

	"go.uber.org/fx"
)

var UseCaseModule = fx.Module("usecase",
	fx.Provide(
		usecase.NewAuthUseCase,
		usecase.NewReservationUseCase,
	),
)
