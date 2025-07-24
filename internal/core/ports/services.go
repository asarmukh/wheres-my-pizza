package ports

import (
	"context"
	"wheres-my-pizza/internal/core/domain"
)

type OrderService interface {
	CreateOrder(ctx context.Context, order *domain.Order) error
	GetOrderByID(ctx context.Context, id int) (*domain.Order, error)
}
