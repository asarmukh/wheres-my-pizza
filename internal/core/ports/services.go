package ports

import (
	"context"
	"wheres-my-pizza/internal/core/domain"
)

type OrderService interface {
	CreateOrder(ctx context.Context, order *domain.Order) error
	GetOrderByNumber(ctx context.Context, orderNumber string) (*domain.Order, error)
}
