package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"wheres-my-pizza/internal/core/domain"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQPublisher struct {
	channel  *amqp.Channel
	exchange string
}

func NewRabbitMQPublisher(ch *amqp.Channel, exchange string) *RabbitMQPublisher {
	return &RabbitMQPublisher{
		channel:  ch,
		exchange: exchange,
	}
}

func (p *RabbitMQPublisher) PublishOrder(ctx context.Context, order *domain.Order) error {
	msg := map[string]interface{}{
		"order_number":     order.Number,
		"customer_name":    order.CustomerName,
		"order_type":       order.Type,
		"table_number":     order.TableNumber,
		"delivery_address": order.DeliveryAddress,
		"items":            order.Items,
		"total_amount":     order.TotalAmount,
		"priority":         order.Priority,
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal order message: %w", err)
	}

	routingKey := fmt.Sprintf("kitchen.%s.%d", order.Type, order.Priority)

	err = p.channel.PublishWithContext(ctx,
		p.exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Priority:     uint8(order.Priority),
			Timestamp:    time.Now(),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}
