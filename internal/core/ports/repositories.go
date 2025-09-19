package ports

import (
	"context"
	"time"
	"wheres-my-pizza/internal/core/domain"
)

type OrderRepo interface {
	Save(ctx context.Context, order *domain.Order) error
	GetByNumber(ctx context.Context, orderNumber string) (*domain.Order, error)
	ListByStatus(ctx context.Context, status string) ([]*domain.Order, error)
	UpdateStatus(ctx context.Context, id int, newStaus string) error
	LogStatusChange(ctx context.Context, log *domain.OrderStatusLog) error
	GetAndIncrementOrderCounter(ctx context.Context, date time.Time) (int, error)
}

type WorkerRepo interface {
	RegisterWorker(ctx context.Context, name, workerType string) error
	UpdateStatus(ctx context.Context, name, status string) error
	IncrementOrders(ctx context.Context, name string) error
	Heartbeat(ctx context.Context, name string) error
}

type KitchenWorker interface {
	Start(ctx context.Context) error
	ProcessOrder(ctx context.Context, order domain.OrderMessage) error
}
