package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"wheres-my-pizza/internal/logger"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Consumer interface {
	ConsumeKitchen(ctx context.Context, prefetch int) (<-chan amqp.Delivery, error)
}

type Repo interface {
	Register(ctx context.Context, workerName, types string) error
	Heartbeat(ctx context.Context, workerName string, online bool) error
	StartCooking(ctx context.Context, orderNumber, workerName string) (oldStatus string, err error)
	FinishOrder(ctx context.Context, orderNumber, workerName string) error
}

type Publisher interface {
	PublishCooking(ctx context.Context, payload map[string]any) error
	PublishReady(ctx context.Context, payload map[string]any) error
}

type Runner struct {
	name       string
	types      map[string]struct{}
	prefetch   int
	heartbeatS int
	cons       Consumer
	repo       Repo
	pub        Publisher
	log        *logger.Logger
}

func NewRunner(name string, typesCSV string, prefetch, heartbeatS int, cons Consumer, repo Repo, pub Publisher, log *logger.Logger) *Runner {
	m := map[string]struct{}{}
	if typesCSV != "" {
		for _, p := range strings.Split(typesCSV, ",") {
			t := strings.TrimSpace(p)
			if t != "" {
				m[t] = struct{}{}
			}
		}
	}
	if prefetch <= 0 {
		prefetch = 1
	}
	if heartbeatS <= 0 {
		heartbeatS = 30
	}
	return &Runner{name: name, types: m, prefetch: prefetch, heartbeatS: heartbeatS, cons: cons, repo: repo, pub: pub, log: log}
}

var ErrSkip = errors.New("skip")

func (r *Runner) Run(ctx context.Context) error {
	if err := r.repo.Register(ctx, r.name, r.typesString()); err != nil {
		r.log.Error(ctx, "worker_registration_failed", "Failed to register", err)
		return err
	}
	r.log.Info(ctx, "worker_registered", "Worker registered", logger.M{"worker": r.name})
	hbCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go r.heartbeatLoop(hbCtx)
	for {
		ch, err := r.cons.ConsumeKitchen(ctx, r.prefetch)
		if err != nil {
			r.log.Error(ctx, "rabbitmq_consume_failed", "Consume failed", err)
			select {
			case <-time.After(2 * time.Second):
				continue
			case <-ctx.Done():
				return nil
			}
		}
		for msg := range ch {
			if ctx.Err() != nil {
				_ = msg.Nack(true, true)
				break
			}
			if err := r.process(ctx, msg); err != nil {
				if errors.Is(err, ErrSkip) {
					_ = msg.Nack(false, true)
				} else {
					_ = msg.Nack(false, true)
				}
				continue
			}
			_ = msg.Ack(false)
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}

func (r *Runner) process(ctx context.Context, msg amqp.Delivery) error {
	var m map[string]any
	if err := json.Unmarshal(msg.Body, &m); err != nil {
		return err
	}
	orderType, _ := m["order_type"].(string)
	orderNumber, _ := m["order_number"].(string)
	if len(r.types) > 0 {
		if _, ok := r.types[orderType]; !ok {
			return ErrSkip
		}
	}
	r.log.Debug(ctx, "order_processing_started", "Picked order", logger.M{"order_number": orderNumber})
	oldStatus, err := r.repo.StartCooking(ctx, orderNumber, r.name)
	if err != nil {
		return err
	}
	cookPayload := map[string]any{
		"order_number":         orderNumber,
		"old_status":           oldStatus,
		"new_status":           "cooking",
		"changed_by":           r.name,
		"timestamp":            time.Now().UTC().Format(time.RFC3339),
		"estimated_completion": time.Now().UTC().Add(r.estimate(orderType)).Format(time.RFC3339),
	}
	_ = r.pub.PublishCooking(ctx, cookPayload)
	time.Sleep(r.estimate(orderType))
	if err := r.repo.FinishOrder(ctx, orderNumber, r.name); err != nil {
		return err
	}
	readyPayload := map[string]any{
		"order_number": orderNumber,
		"old_status":   "cooking",
		"new_status":   "ready",
		"changed_by":   r.name,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	}
	_ = r.pub.PublishReady(ctx, readyPayload)
	r.log.Debug(ctx, "order_completed", "Order processed", logger.M{"order_number": orderNumber})
	return nil
}

func (r *Runner) heartbeatLoop(ctx context.Context) {
	t := time.NewTicker(time.Duration(r.heartbeatS) * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = r.repo.Heartbeat(ctx, r.name, false)
			r.log.Info(ctx, "graceful_shutdown", "Worker shutting down", nil)
			return
		case <-t.C:
			_ = r.repo.Heartbeat(ctx, r.name, true)
			r.log.Debug(ctx, "heartbeat_sent", "Heartbeat updated", nil)
		}
	}
}

func (r *Runner) estimate(t string) time.Duration {
	switch t {
	case "dine_in":
		return 8 * time.Second
	case "takeout":
		return 10 * time.Second
	case "delivery":
		return 12 * time.Second
	}
	return 10 * time.Second
}

func (r *Runner) typesString() string {
	if len(r.types) == 0 {
		return "all"
	}
	parts := make([]string, 0, len(r.types))
	for t := range r.types {
		parts = append(parts, t)
	}
	return strings.Join(parts, ",")
}
