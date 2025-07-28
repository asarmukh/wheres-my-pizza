package postgres

import (
	"context"
	"fmt"
	"time"
	"wheres-my-pizza/internal/core/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderRepo struct {
	db *pgxpool.Pool
}

func NewOrderRepo(db *pgxpool.Pool) *OrderRepo {
	return &OrderRepo{db: db}
}

func (r *OrderRepo) Save(ctx context.Context, order *domain.Order) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `INSERT INTO orders (number, customer_name, type, table_number, delivery_address, total_amount, priority, processed_by, completed_at, created_at, updated_at) VALUES
	($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	RETURNING id`

	var orderID int
	err = tx.QueryRow(ctx, query,
		order.Number,
		order.CustomerName,
		order.Type,
		order.TableNumber,
		order.DeliveryAddress,
		order.TotalAmount,
		order.Priority,
		order.ProcessedBy,
		order.CompletedAt,
		order.CreatedAt,
		order.UpdatedAt,
	).Scan(&orderID)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}

	itemQuery := `INSERT INTO order_items (order_id, name, quantity, price, created_at) VALUES ($1, $2, $3, $4, $5)`

	for _, item := range order.Items {
		_, err := tx.Exec(ctx, itemQuery,
			orderID,
			item.Name,
			item.Quantity,
			item.Price,
			time.Now(),
		)
		if err != nil {
			return fmt.Errorf("failed to insert order item: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *OrderRepo) GetByID(ctx context.Context, id int) (*domain.Order, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, number, customer_name, type, table_number, delivery_address, total_amount, priority, status, processed_by, completed_at, created_at, updated_at
		 FROM orders
		 WHERE id = $1`, id)

	var order domain.Order
	err := row.Scan(
		&order.ID,
		&order.Number,
		&order.CustomerName,
		&order.Type,
		&order.TableNumber,
		&order.DeliveryAddress,
		&order.TotalAmount,
		&order.Priority,
		&order.Status,
		&order.ProcessedBy,
		&order.CompletedAt,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan order: %w", err)
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, order_id, name, quantity, price, created_at
		 FROM order_items
		 WHERE order_id = $1`, order.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to query order items: %w", err)
	}
	defer rows.Close()

	var items []domain.OrderItem
	for rows.Next() {
		var item domain.OrderItem
		err := rows.Scan(
			&item.ID,
			&item.OrderID,
			&item.Name,
			&item.Quantity,
			&item.Price,
			&item.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order item: %w", err)
		}
		items = append(items, item)
	}

	order.Items = items

	return &order, nil
}

func (r *OrderRepo) ListByStatus(ctx context.Context, status string) ([]*domain.Order, error) {
	return nil, nil
}

func (r *OrderRepo) UpdateStatus(ctx context.Context, id int, newStaus string) error {
	return nil
}

func (r *OrderRepo) LogStatusChange(ctx context.Context, log *domain.OrderStatusLog) error {
	return nil
}

func (r *OrderRepo) GetAndIncrementOrderCounter(ctx context.Context, date time.Time) (int, error) {
	var counter int

	err := r.db.QueryRow(ctx,
		`UPDATE order_sequence
	SET counter = counter + 1
	WHERE sequence_date = $1
	RETURNING counter`, date).Scan(&counter)

	if err == pgx.ErrNoRows {
		err = r.db.QueryRow(ctx,
			`INSERT INTO order_sequence (sequence_date, counter) 
			VALUES ($1, 1)
		RETURNING counter`, date).Scan(&counter)
	}

	if err != nil {
		return 0, fmt.Errorf("failed to get or insert order counter: %w", err)
	}

	return counter, nil
}
