package mq

import (
	"context"
	"encoding/json"

	"wheres-my-pizza/internal/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Consumer struct{ c *rabbitmq.Client }

func NewConsumer(c *rabbitmq.Client) *Consumer { return &Consumer{c: c} }
func (c *Consumer) ConsumeKitchen(ctx context.Context, prefetch int) (<-chan amqp.Delivery, error) {
	return c.c.ConsumeKitchen(ctx, prefetch)
}

type Publisher struct{ c *rabbitmq.Client }

func NewPublisher(c *rabbitmq.Client) *Publisher { return &Publisher{c: c} }
func (p *Publisher) PublishCooking(ctx context.Context, payload map[string]any) error {
	b, _ := json.Marshal(payload)
	return p.c.PublishJSON(ctx, "notifications_fanout", "", 0, b)
}
func (p *Publisher) PublishReady(ctx context.Context, payload map[string]any) error {
	b, _ := json.Marshal(payload)
	return p.c.PublishJSON(ctx, "notifications_fanout", "", 0, b)
}
