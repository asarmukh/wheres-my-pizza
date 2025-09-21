package ports

import (
	"context"

	"wheres-my-pizza/internal/core/domain"
)

type OrderPublisher interface {
	PublishOrder(ctx context.Context, order *domain.Order) error
}
