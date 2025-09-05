package services

import (
	"context"
	"time"
	"wheres-my-pizza/internal/core/domain"
)

type KitchenProcessor struct {
	repo OrderRepository
}

func NewKitchenProcessor(repo OrderRepository) *KitchenProcessor {
	return &KitchenProcessor{repo: repo}
}

func (kp *KitchenProcessor) ProcessOrder(ctx context.Context, order domain.Order) error {
	order.Status = "IN_PROGRESS"
	order.UpdatedAt = time.Now()
	if err := kp.repo.UpdateOrderStatus(ctx, order.ID, order.Status); err != nil {
		return err
	}
	// имитация готовки
	time.Sleep(3 * time.Second)

	order.Status = "READY"
	order.CompletedAt = ptrTime(time.Now())
	return kp.repo.UpdateOrderStatus(ctx, order.ID, order.Status)
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
