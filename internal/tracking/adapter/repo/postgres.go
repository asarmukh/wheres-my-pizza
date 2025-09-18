package repo

import (
	"context"
	"time"

	"wheres-my-pizza/internal/db"
	trackapp "wheres-my-pizza/internal/tracking/app"
)

type Repo struct{ pool *db.Pool }

func New(pool *db.Pool) *Repo { return &Repo{pool: pool} }

func (r *Repo) GetOrderStatus(ctx context.Context, orderNumber string) (string, string, time.Time, error) {
	var st, by string
	var at time.Time
	err := r.pool.Pool().QueryRow(ctx, `select status, coalesce(processed_by,''), updated_at from orders where number=$1`, orderNumber).Scan(&st, &by, &at)
	return st, by, at, err
}

func (r *Repo) GetOrderHistory(ctx context.Context, orderNumber string) ([]trackapp.HistoryEntry, error) {
	rows, err := r.pool.Pool().Query(ctx, `select status, changed_at, coalesce(changed_by,'') from order_status_log where order_id=(select id from orders where number=$1) order by changed_at asc`, orderNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []trackapp.HistoryEntry
	for rows.Next() {
		var st, by string
		var ts time.Time
		if err := rows.Scan(&st, &ts, &by); err != nil {
			return nil, err
		}
		out = append(out, trackapp.HistoryEntry{Status: st, ChangedBy: by, Timestamp: ts})
	}
	return out, nil
}

func (r *Repo) GetWorkers(ctx context.Context) ([]trackapp.WorkerEntry, error) {
	rows, err := r.pool.Pool().Query(ctx, `select name, status, orders_processed, last_seen from workers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []trackapp.WorkerEntry
	for rows.Next() {
		var e trackapp.WorkerEntry
		if err := rows.Scan(&e.Name, &e.Status, &e.OrdersProcessed, &e.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}
