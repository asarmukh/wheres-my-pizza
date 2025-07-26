package ports

import (
	"context"
	"time"
	"wheres-my-pizza/internal/core/domain"
)

type OrderRepo interface {
	Save(ctx context.Context, order *domain.Order) error
	GetByID(ctx context.Context, id int) (*domain.Order, error)
	ListByStatus(ctx context.Context, status string) ([]*domain.Order, error)
	UpdateStatus(ctx context.Context, id int, newStaus string) error
	LogStatusChange(ctx context.Context, log *domain.OrderStatusLog) error
	GetAndIncrementOrderCounter(ctx context.Context, date time.Time) (int, error)
}
