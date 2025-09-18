package mq

import (
	"context"

	"wheres-my-pizza/internal/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Consumer struct{ c *rabbitmq.Client }

func NewConsumer(c *rabbitmq.Client) *Consumer { return &Consumer{c: c} }
func (c *Consumer) ConsumeNotifications(ctx context.Context) (<-chan amqp.Delivery, error) {
	return c.c.ConsumeNotifications(ctx)
}
