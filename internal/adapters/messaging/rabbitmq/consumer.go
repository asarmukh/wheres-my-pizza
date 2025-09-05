package rabbitmq

import (
	"context"
	"encoding/json"
	"log"
	"wheres-my-pizza/internal/core/domain"

	amqp "github.com/rabbitmq/amqp091-go"
)

type OrderConsumer struct {
	ch        *amqp.Channel
	queueName string
}

func NewOrderConsumer(ch *amqp.Channel, queueName string) *OrderConsumer {
	return &OrderConsumer{ch: ch, queueName: queueName}
}

// слушает события от order service
func (c *OrderConsumer) Start(ctx context.Context, handler func(order domain.Order) error) error {
	msgs, err := c.ch.Consume(
		c.queueName,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	go func() {
		for m := range msgs {
			var order domain.Order
			if err := json.Unmarshal(m.Body, &order); err != nil {
				log.Printf("failed to unmarshal order: %v", err)
			}
		}
	}()
	return nil
}
