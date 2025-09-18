package app

import (
	"context"
	"encoding/json"
	"fmt"

	"wheres-my-pizza/internal/logger"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Consumer interface {
	ConsumeNotifications(ctx context.Context) (<-chan amqp.Delivery, error)
}

type Subscriber struct {
	cons Consumer
	log  *logger.Logger
}

func New(cons Consumer, log *logger.Logger) *Subscriber { return &Subscriber{cons: cons, log: log} }

func (s *Subscriber) Run(ctx context.Context) error {
	ch, err := s.cons.ConsumeNotifications(ctx)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			var m map[string]any
			_ = json.Unmarshal(msg.Body, &m)
			on, _ := m["order_number"].(string)
			if on != "" {
				s.log.Info(ctx, "notification_received", fmt.Sprintf("Received status update for order %s", on), logger.M{"order_number": on, "new_status": m["new_status"]})
				fmt.Printf("Notification for order %s: Status changed from '%v' to '%v' by %v.\n", on, m["old_status"], m["new_status"], m["changed_by"])
			} else {
				s.log.Info(ctx, "notification_received", "Received status update", nil)
			}
			_ = msg.Ack(false)
		}
	}
}
