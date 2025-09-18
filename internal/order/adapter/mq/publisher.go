package mq

import (
	"context"

	"wheres-my-pizza/internal/rabbitmq"
)

type Publisher struct{ client *rabbitmq.Client }

func NewPublisher(c *rabbitmq.Client) *Publisher { return &Publisher{client: c} }

func (p *Publisher) Publish(ctx context.Context, routingKey string, priority uint8, payload []byte) error {
	return p.client.PublishJSON(ctx, "orders_topic", routingKey, priority, payload)
}
