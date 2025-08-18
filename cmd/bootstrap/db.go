package bootstrap

import (
	"context"

	"gin-clean-starter/internal/infra/db"
	"gin-clean-starter/internal/pkg/config"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

var DBModule = fx.Module("db",
	fx.Provide(
		NewDB,
	),
)

func NewDB(lc fx.Lifecycle, cfg config.Config) (*pgxpool.Pool, error) {
	pool, cleanup, err := db.Connect(cfg.DB)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			if cleanup != nil {
				cleanup()
			}
			return nil
		},
	})

	return pool, nil
}
