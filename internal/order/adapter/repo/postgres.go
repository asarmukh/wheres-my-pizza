package repo

import (
	"context"
	"strconv"
	"strings"
	"time"

	"wheres-my-pizza/internal/db"
	"wheres-my-pizza/internal/order/domain"

	"github.com/jackc/pgx/v5"
)

type PostgresRepo struct{ pool *db.Pool }

func NewPostgres(pool *db.Pool) *PostgresRepo { return &PostgresRepo{pool: pool} }

func (r *PostgresRepo) CreateOrder(ctx context.Context, req domain.OrderRequest, total float64, priority int) (string, int64, error) {
	tx, err := r.pool.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	today := time.Now().UTC().Format("2006-01-02")
	if _, err := tx.Exec(ctx, `insert into order_counters(day,last_seq) values ($1,0) on conflict (day) do nothing`, today); err != nil {
		return "", 0, err
	}
	var seq int
	if err := tx.QueryRow(ctx, `update order_counters set last_seq = last_seq + 1 where day=$1 returning last_seq`, today).Scan(&seq); err != nil {
		return "", 0, err
	}
	ymd := time.Now().UTC().Format("20060102")
	orderNumber := "ORD_" + ymd + "_" + leftPad(seq, 3)

	var orderID int64
	var tableNumber *int = req.TableNumber
	var deliveryAddress *string = req.DeliveryAddress
	if err := tx.QueryRow(ctx, `
        insert into orders(number, customer_name, type, table_number, delivery_address, total_amount, priority, status)
        values ($1,$2,$3,$4,$5,$6,$7,'received') returning id
    `, orderNumber, req.CustomerName, req.OrderType, tableNumber, deliveryAddress, total, priority).Scan(&orderID); err != nil {
		return "", 0, err
	}
	for _, it := range req.Items {
		if _, err := tx.Exec(ctx, `insert into order_items(order_id, name, quantity, price) values ($1,$2,$3,$4)`, orderID, it.Name, it.Quantity, it.Price); err != nil {
			return "", 0, err
		}
	}
	if _, err := tx.Exec(ctx, `insert into order_status_log(order_id,status,changed_by,notes) values ($1,'received','order-service','')`, orderID); err != nil {
		return "", 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", 0, err
	}
	return orderNumber, orderID, nil
}

func leftPad(n, width int) string {
	s := strconv.Itoa(n)
	if len(s) >= width {
		return s
	}
	return strings.Repeat("0", width-len(s)) + s
}
