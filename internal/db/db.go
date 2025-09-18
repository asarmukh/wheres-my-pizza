package db

import (
	"context"
	"fmt"
	"time"

	"wheres-my-pizza/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Pool struct {
	pool *pgxpool.Pool
}

func Connect(ctx context.Context, cfg config.Database) (*Pool, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
	conf, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	conf.MaxConnLifetime = time.Hour
	conf.MinConns = 0
	conf.MaxConns = 10
	pool, err := pgxpool.NewWithConfig(ctx, conf)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Pool{pool: pool}, nil
}

func (p *Pool) Close() { p.pool.Close() }

func (p *Pool) Pool() *pgxpool.Pool { return p.pool }
