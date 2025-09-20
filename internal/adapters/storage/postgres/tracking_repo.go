package postgres

import (
	"context"
	"wheres-my-pizza/internal/core/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TrackingRepo struct {
	db *pgxpool.Pool
}

func NewTrackingRepo(db *pgxpool.Pool) *TrackingRepo {
	return &TrackingRepo{db: db}
}

func (r *TrackingRepo) GetOrderStatus(orderID int) (*domain.Order, error) {
	var order domain.Order
	err := r.db.QueryRow(context.Background(),
		"SELECT id, number, status, created_at, updated_at FROM orders WHERE id=$1", orderID).
		Scan(&order.ID, &order.Number, &order.Status, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &order, nil
}

func (r *TrackingRepo) GetWorkerStatus(workerID int) (*domain.Worker, error) {
	var worker domain.Worker
	err := r.db.QueryRow(context.Background(),
		"SELECT id, name, status, last_seen FROM workers WHERE id=$1", workerID).
		Scan(&worker.ID, &worker.Name, &worker.Status, &worker.LastSeen)
	if err != nil {
		return nil, err
	}
	return &worker, nil
}
