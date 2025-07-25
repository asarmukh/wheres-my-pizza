package services

import (
	"context"
	"time"
	"wheres-my-pizza/internal/core/domain"
	"wheres-my-pizza/internal/core/ports"
	"wheres-my-pizza/internal/helper"
)

type OrderService struct {
	repo ports.OrderRepo
}

func NewOrderService(repo ports.OrderRepo) *OrderService {
	return &OrderService{repo: repo}
}

func (s *OrderService) GetOrderByID(ctx context.Context, id int) (*domain.Order, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *OrderService) CreateOrder(ctx context.Context, order *domain.Order) error {
	if err := helper.ValidateOrder(order); err != nil {
		return err
	}

	totalAmount := 0.0

	for _, item := range order.Items {
		totalAmount += float64(item.Quantity) + item.Price
	}

	order.TotalAmount = totalAmount
	order.Priority = SetPriority(totalAmount)

	order.CreatedAt = time.Now().UTC()
	order.UpdatedAt = order.CreatedAt

	return s.repo.Save(ctx, order)
}
