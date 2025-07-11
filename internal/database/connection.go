package database

import (
	"context"
	"fmt"
	"time"
	"wheres-my-pizza/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Pool = *pgxpool.Pool

var DB Pool

func Connect(cfg config.DatabaseConfig) (Pool, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid DSN: %w", err)
	}

	db, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("could not connect to PostgreSQL: %w", err)
	}

	if err := db.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	DB = db
	return db, nil
}
