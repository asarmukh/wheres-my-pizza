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
	repo      ports.OrderRepo
	publisher ports.OrderPublisher
}

func NewOrderService(repo ports.OrderRepo, publisher ports.OrderPublisher) *OrderService {
	return &OrderService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *OrderService) GetOrderByNumber(ctx context.Context, orderNumber string) (*domain.Order, error) {
	return s.repo.GetByNumber(ctx, orderNumber)
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

	order.Status = "received"
	order.CreatedAt = time.Now().UTC()
	order.UpdatedAt = order.CreatedAt

	if err := s.repo.Save(ctx, order); err != nil {
		// logger.Error("order-service", "db_transaction_failed", "failed to save order", nil, err)
		return err
	}

	// logger.Debug("order-service", "order_received", "Order saved successfully", nil, map[string]interface{}{
	// 	"order_number": order.Number,
	// 	"priority":     order.Priority,
	// })

	if err := s.publisher.PublishOrder(ctx, order); err != nil {
		// logger.Error("order-service", "rabbitmq_publish_failed", "Failed to publish order", nil, err)
		return err
	}

	// logger.Debug("order-service", "order_published", "Order published to RabbitMQ", nil, map[string]interface{}{
	// 	"order_number": order.Number,
	// })

	return nil
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
