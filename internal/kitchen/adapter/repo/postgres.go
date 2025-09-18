package repo

import (
	"context"
	"errors"

	"wheres-my-pizza/internal/db"

	"github.com/jackc/pgx/v5"
)

type Repo struct{ pool *db.Pool }

func New(pool *db.Pool) *Repo { return &Repo{pool: pool} }

func (r *Repo) Register(ctx context.Context, workerName, types string) error {
	tx, err := r.pool.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	err = tx.QueryRow(ctx, `select status from workers where name=$1`, workerName).Scan(&status)
	if err == nil {
		if status == "online" {
			return errors.New("duplicate worker")
		}
		if _, err := tx.Exec(ctx, `update workers set status='online', type=$2, last_seen=now() where name=$1`, workerName, types); err != nil {
			return err
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		if _, err := tx.Exec(ctx, `insert into workers(name,type,status,last_seen) values ($1,$2,'online',now())`, workerName, types); err != nil {
			return err
		}
	} else {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repo) Heartbeat(ctx context.Context, workerName string, online bool) error {
	status := "offline"
	if online {
		status = "online"
	}
	_, err := r.pool.Pool().Exec(ctx, `update workers set status=$2, last_seen=now() where name=$1`, workerName, status)
	return err
}

func (r *Repo) StartCooking(ctx context.Context, orderNumber, workerName string) (string, error) {
	tx, err := r.pool.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var orderID int64
	var status string
	if err := tx.QueryRow(ctx, `select id,status from orders where number=$1 for update`, orderNumber).Scan(&orderID, &status); err != nil {
		return "", err
	}
	if status != "cooking" {
		if _, err := tx.Exec(ctx, `update orders set status='cooking', processed_by=$2 where id=$1`, orderID, workerName); err != nil {
			return "", err
		}
		if _, err := tx.Exec(ctx, `insert into order_status_log(order_id,status,changed_by,notes) values ($1,'cooking',$2,'')`, orderID, workerName); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return status, nil
}

func (r *Repo) FinishOrder(ctx context.Context, orderNumber, workerName string) error {
	tx, err := r.pool.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `update orders set status='ready', completed_at=now() where number=$1`, orderNumber); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `insert into order_status_log(order_id,status,changed_by,notes) values ((select id from orders where number=$1),'ready',$2,'')`, orderNumber, workerName); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `update workers set orders_processed=orders_processed+1, last_seen=now(), status='online' where name=$1`, workerName); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
