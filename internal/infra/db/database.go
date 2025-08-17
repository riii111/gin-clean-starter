package db

import (
	"database/sql"
	"fmt"
	"time"

	"gin-clean-starter/internal/pkg/config"

	_ "github.com/lib/pq" // PostgreSQL driver
)

func Connect(cfg config.DBConfig) (*sql.DB, func(), error) {
	dsn := cfg.BuildDSN()

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(100)
	db.SetConnMaxLifetime(time.Hour)

	cleanup := func() {
		if err := db.Close(); err != nil {
			// Log error in production
			fmt.Printf("Error closing database: %v\n", err)
		}
	}

	return db, cleanup, nil
}
