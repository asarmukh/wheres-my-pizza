package postgres

import (
	"context"
	"fmt"
	"wheres-my-pizza/internal/core/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderRepo struct {
	db *pgxpool.Pool
}

func NewOrderRepo(db *pgxpool.Pool) *OrderRepo {
	return &OrderRepo{db: db}
}

func (r *OrderRepo) Create(ctx context.Context, order *domain.Order) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `INSERT INTO order (number, custom_name, type, table_number, delivery_address, total_amount, priority, status, processed_by, completed_at, created_at, updated_at) VALUES
	($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
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
		order.Status,
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
			item.CreatedAt,
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
	return nil, nil
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
