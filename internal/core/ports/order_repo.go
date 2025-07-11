package ports

import (
	"context"
	"wheres-my-pizza/internal/core/domain"
)

type OrderRepo interface {
	Create(ctx context.Context, order *domain.Order)
}
