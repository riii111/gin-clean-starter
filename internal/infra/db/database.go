package db

import (
	"context"
	"fmt"
	"time"

	"gin-clean-starter/internal/pkg/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(cfg config.DBConfig) (*pgxpool.Pool, func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.BuildDSN())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("failed to ping database: %w", err)
	}

	cleanup := func() {
		pool.Close()
	}

	return pool, cleanup, nil
}
