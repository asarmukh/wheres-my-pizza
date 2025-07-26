package services

import (
	"context"
	"fmt"
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
		totalAmount += float64(item.Quantity) * item.Price
	}

	order.TotalAmount = totalAmount
	order.Priority = SetPriority(totalAmount)

	var err error
	order.Number, err = s.generateOrderNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to generate order number: %w", err)
	}

	order.CreatedAt = time.Now().UTC()
	order.UpdatedAt = order.CreatedAt

	return s.repo.Save(ctx, order)
}

func (s *OrderService) generateOrderNumber(ctx context.Context) (string, error) {
	today := time.Now().UTC().Format("2006-01-02")
	parsedDate, _ := time.Parse("2006-01-02", today)

	counter, err := s.repo.GetAndIncrementOrderCounter(ctx, parsedDate)
	if err != nil {
		return "", err
	}

	datePart := parsedDate.Format("20060102")
	return fmt.Sprintf("ORD_%s_%03d", datePart, counter), nil
}
