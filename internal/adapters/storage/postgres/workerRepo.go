package postgres

import (
	"context"
	"fmt"
	"wheres-my-pizza/internal/core/ports"

	"github.com/jackc/pgx/v5/pgxpool"
)

type workerRepo struct {
	db *pgxpool.Pool
}

func NewWorkerRepo(db *pgxpool.Pool) ports.WorkerRepo {
	return &workerRepo{db: db}
}

func (r *workerRepo) RegisterWorker(ctx context.Context, name, workerType string) error {
	// Проверяем уникальность и статус
	_, err := r.db.Exec(ctx, `
		INSERT INTO workers (name, type, status)
		VALUES ($1, $2, 'online')
		ON CONFLICT (name) DO UPDATE
		SET status = 'online', last_seen = now()
		WHERE workers.status = 'offline'`, name, workerType)
	if err != nil {
		return fmt.Errorf("failed to register worker: %w", err)
	}
	return nil
}

func (r *workerRepo) UpdateStatus(ctx context.Context, name, status string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE workers SET status=$1, last_seen=now() WHERE name=$2`,
		status, name,
	)
	return err
}

func (r *workerRepo) IncrementOrders(ctx context.Context, name string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE workers SET orders_processed = orders_processed + 1, last_seen=now() WHERE name=$1`,
		name,
	)
	return err
}

func (r *workerRepo) Heartbeat(ctx context.Context, name string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE workers SET last_seen=now(), status='online' WHERE name=$1`,
		name,
	)
	return err
}
