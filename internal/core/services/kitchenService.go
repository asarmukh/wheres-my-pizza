package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"wheres-my-pizza/internal/core/domain"
	"wheres-my-pizza/internal/core/ports"
	"wheres-my-pizza/internal/logger"

	amqp "github.com/rabbitmq/amqp091-go"
)

type KitchenWorkerService struct {
	name       string
	orderTypes map[string]bool
	repo       ports.WorkerRepo
	dbRepo     ports.OrderRepo
	channel    *amqp.Channel
	prefetch   int
}

type WorkerArgs struct {
	Name             string
	OrderTypes       []string
	Prefetch         int
	HeartbeatSeconds int
}

func NewKitchenWorker(dbRepo ports.OrderRepo, repo ports.WorkerRepo, ch *amqp.Channel, args WorkerArgs) *KitchenWorkerService {
	types := make(map[string]bool)
	for _, t := range args.OrderTypes {
		types[t] = true
	}

	return &KitchenWorkerService{
		name:       args.Name,
		orderTypes: types,
		repo:       repo,
		dbRepo:     dbRepo,
		channel:    ch,
		prefetch:   args.Prefetch,
	}
}

func (w *KitchenWorkerService) Start(ctx context.Context) error {
	reqID := "startup"

	// Регистрация воркера
	if err := w.repo.RegisterWorker(ctx, w.name, "general"); err != nil {
		logger.Error("kitchen-worker", "worker_registration_failed", "cannot register worker", reqID, err)
		return err
	}
	logger.Info("kitchen-worker", "worker_registered", "Worker registered", reqID, map[string]any{
		"name": w.name,
	}, 0)

	// Настраиваем prefetch
	if err := w.channel.Qos(w.prefetch, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	msgs, err := w.channel.Consume("kitchen_queue", "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to consume messages: %w", err)
	}

	// Heartbeat
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = w.repo.Heartbeat(ctx, w.name)
				logger.Debug("kitchen-worker", "heartbeat_sent", "heartbeat updated", reqID, nil)
			}
		}
	}()

	for msg := range msgs {
		var order domain.OrderMessage
		if err := json.Unmarshal(msg.Body, &order); err != nil {
			logger.Error("kitchen-worker", "message_processing_failed", "invalid message", reqID, err)
			_ = msg.Nack(false, false) // DLQ
			continue
		}

		if len(w.orderTypes) > 0 && !w.orderTypes[order.OrderType] {
			_ = msg.Nack(false, true)
			continue
		}

		logger.Debug("kitchen-worker", "order_processing_started", "Processing order", reqID, map[string]any{
			"order_number": order.OrderNumber,
		})

		if err := w.ProcessOrder(ctx, order); err != nil {
			_ = msg.Nack(false, true)
			continue
		}

		_ = msg.Ack(false)
	}
	return nil
}

func (w *KitchenWorkerService) ProcessOrder(ctx context.Context, order domain.OrderMessage) error {
	reqID := logger.NewRequestID()

	// --- 1. Транзакция: ставим cooking
	tx, err := w.db.Begin(ctx)
	if err != nil {
		logger.Error("kitchen-worker", "db_transaction_failed", "cannot start tx", reqID, err)
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`UPDATE orders 
		 SET status = $1, processed_by = $2, updated_at = now()
		 WHERE number = $3 AND status = 'received'`,
		"cooking", w.workerName, order.OrderNumber,
	)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO order_status_log (order_id, status, changed_by)
		 SELECT id, $1, $2 FROM orders WHERE number = $3`,
		"cooking", w.workerName, order.OrderNumber,
	)
	if err != nil {
		return fmt.Errorf("failed to insert order log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	// --- 2. Публикуем статус "cooking"
	if err := w.publishStatus(ctx, order.OrderNumber, "received", "cooking"); err != nil {
		return err
	}

	// --- 3. Симуляция готовки
	time.Sleep(time.Duration(estimateCookingTime(order.OrderType)) * time.Second)

	// --- 4. Ставим ready
	tx2, err := w.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx2.Rollback(ctx)

	_, err = tx2.Exec(ctx,
		`UPDATE orders 
		 SET status = $1, completed_at = now(), updated_at = now()
		 WHERE number = $2`,
		"ready", order.OrderNumber,
	)
	if err != nil {
		return err
	}

	_, err = tx2.Exec(ctx,
		`INSERT INTO order_status_log (order_id, status, changed_by)
		 SELECT id, $1, $2 FROM orders WHERE number = $3`,
		"ready", w.workerName, order.OrderNumber,
	)
	if err != nil {
		return err
	}

	_, err = tx2.Exec(ctx,
		`UPDATE workers SET orders_processed = orders_processed + 1, last_seen = now()
		 WHERE name = $1`,
		w.workerName,
	)
	if err != nil {
		return err
	}

	if err := tx2.Commit(ctx); err != nil {
		return err
	}

	// --- 5. Публикуем статус "ready"
	if err := w.publishStatus(ctx, order.OrderNumber, "cooking", "ready"); err != nil {
		return err
	}

	logger.Debug("kitchen-worker", "order_completed", "Order fully processed", reqID, map[string]any{
		"order_number": order.OrderNumber,
	})

	return nil
}

func (w *KitchenWorkerService) publishStatus(ctx context.Context, orderNumber, oldStatus, newStatus string) error {
	body, _ := json.Marshal(map[string]any{
		"order_number":         orderNumber,
		"old_status":           oldStatus,
		"new_status":           newStatus,
		"changed_by":           w.workerName,
		"timestamp":            time.Now().UTC().Format(time.RFC3339),
		"estimated_completion": time.Now().Add(5 * time.Minute).UTC(),
	})

	return w.notifyCh.PublishWithContext(ctx,
		"notifications_fanout", "", false, false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: 2,
		})
}

func estimateCookingTime(orderType string) int {
	switch orderType {
	case "dine_in":
		return 8
	case "takeout":
		return 10
	case "delivery":
		return 12
	default:
		return 10
	}
}
