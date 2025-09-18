package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"time"

	"wheres-my-pizza/internal/config"
	"wheres-my-pizza/internal/logger"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Client struct {
	cfg  config.RabbitMQ
	log  *logger.Logger
	conn *amqp.Connection
	ch   *amqp.Channel
}

func NewClient(cfg config.RabbitMQ, log *logger.Logger) *Client {
	return &Client{cfg: cfg, log: log}
}

func (c *Client) ConnectWithRetry(ctx context.Context) error {
	dsn := fmt.Sprintf("amqp://%s:%s@%s:%d/", c.cfg.User, c.cfg.Password, c.cfg.Host, c.cfg.Port)
	backoff := time.Second
	for {
		conn, err := amqp.Dial(dsn)
		if err == nil {
			ch, err := conn.Channel()
			if err != nil {
				conn.Close()
			} else {
				c.conn = conn
				c.ch = ch
				return nil
			}
		}
		select {
		case <-ctx.Done():
			if err != nil {
				return err
			}
			return ctx.Err()
		case <-time.After(backoff):
			if backoff < 10*time.Second {
				backoff *= 2
			}
		}
	}
}

func (c *Client) EnsureOrderTopology(ctx context.Context) error {
	return c.ch.ExchangeDeclare("orders_topic", "topic", true, false, false, false, nil)
}

func (c *Client) EnsureKitchenTopology(ctx context.Context, orderTypes string) error {
	if err := c.EnsureOrderTopology(ctx); err != nil {
		return err
	}
	args := amqp.Table{"x-max-priority": int32(10)}
	_, err := c.ch.QueueDeclare("kitchen_queue", true, false, false, false, args)
	if err != nil {
		return err
	}
	// Bind generic routing for all kitchen messages
	if err := c.ch.QueueBind("kitchen_queue", "kitchen.*.*", "orders_topic", false, nil); err != nil {
		return err
	}
	return nil
}

func (c *Client) EnsureNotificationTopology(ctx context.Context) error {
	if err := c.ch.ExchangeDeclare("notifications_fanout", "fanout", true, false, false, false, nil); err != nil {
		return err
	}
	_, err := c.ch.QueueDeclare("notifications_queue", true, false, false, false, nil)
	if err != nil {
		return err
	}
	return c.ch.QueueBind("notifications_queue", "", "notifications_fanout", false, nil)
}

func (c *Client) PublishJSON(ctx context.Context, exchange, routingKey string, priority uint8, body []byte) error {
	publish := amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Priority:     priority,
		Timestamp:    time.Now().UTC(),
		ContentType:  "application/json",
		Body:         body,
	}
	return c.ch.PublishWithContext(ctx, exchange, routingKey, false, false, publish)
}

func (c *Client) ConsumeKitchen(ctx context.Context, prefetch int) (<-chan amqp.Delivery, error) {
	if err := c.ch.Qos(prefetch, 0, false); err != nil {
		return nil, err
	}
	return c.ch.Consume("kitchen_queue", "", false, false, false, false, nil)
}

func (c *Client) ConsumeNotifications(ctx context.Context) (<-chan amqp.Delivery, error) {
	return c.ch.Consume("notifications_queue", "", false, false, false, false, nil)
}

func (c *Client) Close() {
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

var ErrReconnect = errors.New("rabbitmq reconnect")
