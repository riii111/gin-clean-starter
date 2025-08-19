package bootstrap

import (
	"log/slog"
	"os"

	"go.uber.org/fx"
)

var LoggerModule = fx.Module("logger",
	fx.Provide(
		NewLogger,
	),
)

func NewLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}
