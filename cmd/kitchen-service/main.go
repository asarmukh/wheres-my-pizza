// cmd/kitchen/main.go
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"gopkg.in/yaml.v3"
)

/*
kitchen-worker

Flags:
  --config (string) path to config.json (default: ./config.json)
  --worker-name (string) required
  --order-types (string) optional comma-separated (dine_in,takeout,delivery)
  --heartbeat-interval (int) seconds (default 30)
  --prefetch (int) (default 1)
  --rabbit-reconnect-interval (int) seconds (default 5)

Config file (JSON):
{
  "database": {
    "host":"localhost",
    "port":5432,
    "user":"restaurant_user",
    "password":"restaurant_pass",
    "database":"restaurant_db"
  },
  "rabbitmq": {
    "host":"localhost",
    "port":5672,
    "user":"guest",
    "password":"guest"
  }
}
*/

const serviceName = "kitchen-worker"

type Config struct {
	Database struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		Database string `json:"database"`
	} `json:"database"`
	RabbitMQ struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
	} `json:"rabbitmq"`
}

type LogLevel string

const (
	LevelInfo  LogLevel = "INFO"
	LevelDebug LogLevel = "DEBUG"
	LevelError LogLevel = "ERROR"
)

type Logger struct {
	service string
}

func NewLogger(service string) *Logger {
	return &Logger{service: service}
}

func (l *Logger) base(action, message, requestID string) map[string]interface{} {
	hn, _ := os.Hostname()
	return map[string]interface{}{
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		"level":      string(LevelInfo),
		"service":    l.service,
		"action":     action,
		"message":    message,
		"hostname":   hn,
		"request_id": requestID,
	}
}

func (l *Logger) Info(action, message, requestID string, extra map[string]interface{}) {
	e := l.base(action, message, requestID)
	e["level"] = string(LevelInfo)
	for k, v := range extra {
		e[k] = v
	}
	enc, _ := json.Marshal(e)
	os.Stdout.Write(append(enc, '\n'))
}

func (l *Logger) Debug(action, message, requestID string, extra map[string]interface{}) {
	e := l.base(action, message, requestID)
	e["level"] = string(LevelDebug)
	for k, v := range extra {
		e[k] = v
	}
	enc, _ := json.Marshal(e)
	os.Stdout.Write(append(enc, '\n'))
}

func (l *Logger) Error(action, message, requestID string, err error, stack string, extra map[string]interface{}) {
	e := l.base(action, message, requestID)
	e["level"] = string(LevelError)
	e["error"] = map[string]string{"msg": err.Error()}
	if stack != "" {
		e["error"].(map[string]string)["stack"] = stack
	}
	for k, v := range extra {
		e[k] = v
	}
	enc, _ := json.Marshal(e)
	os.Stdout.Write(append(enc, '\n'))
}

// --- DB helper
type DB struct {
	pool *pgxpool.Pool
}

func NewDB(ctx context.Context, cfg *Config) (*DB, error) {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Database,
	)
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, err
	}
	// quick ping with short ctx
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}
	return &DB{pool: pool}, nil
}

func (d *DB) Close() {
	if d.pool != nil {
		d.pool.Close()
	}
}

// --- AMQP client with reconnect
type AMQPClient struct {
	url     string
	conn    *amqp.Connection
	ch      *amqp.Channel
	mu      sync.RWMutex
	closing chan struct{}
	logger  *Logger
}

func NewAMQPClient(url string, logger *Logger) (*AMQPClient, error) {
	c := &AMQPClient{
		url:     url,
		closing: make(chan struct{}),
		logger:  logger,
	}
	if err := c.connect(); err != nil {
		return nil, err
	}
	go c.monitor()
	return c, nil
}

func (a *AMQPClient) connect() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.conn != nil && !a.conn.IsClosed() {
		return nil
	}
	conn, err := amqp.Dial(a.url)
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return err
	}
	a.conn = conn
	a.ch = ch
	a.logger.Info("rabbitmq_connected", "Connected to RabbitMQ", "startup-001", map[string]interface{}{"url": a.url})
	return nil
}

func (a *AMQPClient) ensureChannel() (*amqp.Channel, error) {
	a.mu.RLock()
	ch := a.ch
	a.mu.RUnlock()
	if ch != nil && a.conn != nil && !a.conn.IsClosed() {
		return ch, nil
	}
	// attempt reconnect
	if err := a.connect(); err != nil {
		return nil, err
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.ch == nil {
		return nil, errors.New("channel not ready")
	}
	return a.ch, nil
}

func (a *AMQPClient) monitor() {
	for {
		select {
		case <-a.closing:
			return
		default:
		}
		a.mu.RLock()
		conn := a.conn
		a.mu.RUnlock()
		if conn == nil || conn.IsClosed() {
			// try reconnect with backoff
			for i := 0; ; i++ {
				select {
				case <-a.closing:
					return
				default:
				}
				a.logger.Info("rabbitmq_reconnect_attempt", "Attempting to reconnect to RabbitMQ", "reconnect", map[string]interface{}{"attempt": i + 1})
				if err := a.connect(); err == nil {
					break
				}
				time.Sleep(5 * time.Second)
			}
		}
		time.Sleep(2 * time.Second)
	}
}

func (a *AMQPClient) Close() {
	close(a.closing)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ch != nil {
		_ = a.ch.Close()
	}
	if a.conn != nil {
		_ = a.conn.Close()
	}
}

func (a *AMQPClient) DeclareExchanges() error {
	ch, err := a.ensureChannel()
	if err != nil {
		return err
	}
	// orders_topic (topic)
	if err := ch.ExchangeDeclare("orders_topic", "topic", true, false, false, false, nil); err != nil {
		return err
	}
	// notifications_fanout
	if err := ch.ExchangeDeclare("notifications_fanout", "fanout", true, false, false, false, nil); err != nil {
		return err
	}
	return nil
}

// Publish with persistence and priority
func (a *AMQPClient) Publish(ctx context.Context, exchange, routingKey string, body interface{}, priority uint8) error {
	ch, err := a.ensureChannel()
	if err != nil {
		return err
	}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	msg := amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		ContentType:  "application/json",
		Body:         b,
		Priority:     priority,
	}
	return ch.PublishWithContext(ctx, exchange, routingKey, false, false, msg)
}

// Declare and bind queue for kitchen worker; returns queueName and channel
func (a *AMQPClient) DeclareQueueAndConsume(queueName string, bindingKeys []string, prefetch int, consumerTag string) (<-chan amqp.Delivery, error) {
	ch, err := a.ensureChannel()
	if err != nil {
		return nil, err
	}
	// declare durable queue
	q, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		return nil, err
	}
	// bind keys
	for _, key := range bindingKeys {
		if err := ch.QueueBind(q.Name, key, "orders_topic", false, nil); err != nil {
			return nil, err
		}
	}
	// QoS
	if err := ch.Qos(prefetch, 0, false); err != nil {
		return nil, err
	}
	msgs, err := ch.Consume(q.Name, consumerTag, false, false, false, false, nil)
	if err != nil {
		return nil, err
	}
	return msgs, nil
}

// --- message payload struct (expected from Order Service)
type OrderMessage struct {
	OrderNumber     string  `json:"order_number"`
	CustomerName    string  `json:"customer_name"`
	OrderType       string  `json:"order_type"`
	TableNumber     *int    `json:"table_number"`
	DeliveryAddress *string `json:"delivery_address"`
	Items           []struct {
		Name     string  `json:"name"`
		Quantity int     `json:"quantity"`
		Price    float64 `json:"price"`
	} `json:"items"`
	TotalAmount float64 `json:"total_amount"`
	Priority    int     `json:"priority"`
}

// --- kitchen worker state
type Worker struct {
	name              string
	types             map[string]bool
	heartbeatInterval time.Duration
	prefetch          int
	amqpClient        *AMQPClient
	db                *DB
	logger            *Logger
	wg                sync.WaitGroup
	shutdownOnce      sync.Once
	shutdownCtx       context.Context
	shutdownCancel    context.CancelFunc
	inFlightMu        sync.Mutex
	inFlight          int
	consumerTag       string
	queueName         string
	rabbitReconnect   int
}

// NewWorker constructs worker
func NewWorker(name string, types []string, hbInterval time.Duration, prefetch int, amqpClient *AMQPClient, db *DB, logger *Logger, rabbitReconnect int) *Worker {
	typesMap := make(map[string]bool)
	for _, t := range types {
		tt := strings.TrimSpace(t)
		if tt != "" {
			typesMap[tt] = true
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		name:              name,
		types:             typesMap,
		heartbeatInterval: hbInterval,
		prefetch:          prefetch,
		amqpClient:        amqpClient,
		db:                db,
		logger:            logger,
		shutdownCtx:       ctx,
		shutdownCancel:    cancel,
		consumerTag:       "ctag-" + name + "-" + strconv.FormatInt(time.Now().Unix(), 10),
		queueName:         "kitchen_" + name,
		rabbitReconnect:   rabbitReconnect,
	}
}

// register in workers table
func (w *Worker) register(ctx context.Context) error {
	// try to insert or update
	// logic:
	// SELECT status FROM workers WHERE name=$1
	// if exists and status='online' -> fail
	// else insert or update to online
	conn, err := w.db.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	var status sql.NullString
	err = conn.QueryRow(ctx, `select status from workers where name=$1`, w.name).Scan(&status)
	if err != nil {
		// no rows => insert
		if errors.Is(err, pgx.ErrNoRows) || err == pgx.ErrNoRows {
			// insert
			_, err = conn.Exec(ctx, `insert into workers(name,type,status,last_seen) values ($1,$2,'online',now())`, w.name, w.typesList())
			if err != nil {
				return err
			}
			w.logger.Info("worker_registered", "Worker inserted and marked online", w.name, map[string]interface{}{"worker_name": w.name})
			return nil
		}
		// other error: but note pgx returns pgx.ErrNoRows not sql package
		// But Acquire/QueryRow with no rows returns pgx.ErrNoRows
		if err == nil {
			// shouldn't happen
		}
		// attempt insert fallback
		_, err2 := conn.Exec(ctx, `insert into workers(name,type,status,last_seen) values ($1,$2,'online',now())`, w.name, w.typesList())
		if err2 != nil {
			return err2
		}
		w.logger.Info("worker_registered", "Worker inserted (fallback) and marked online", w.name, map[string]interface{}{"worker_name": w.name})
		return nil
	}
	// if select succeeded
	if status.Valid && status.String == "online" {
		return fmt.Errorf("worker_registration_failed: worker '%s' already online", w.name)
	}
	// update existing row to online
	_, err = conn.Exec(ctx, `update workers set type=$1, status='online', last_seen=now() where name=$2`, w.typesList(), w.name)
	if err != nil {
		return err
	}
	w.logger.Info("worker_registered", "Worker updated and marked online", w.name, map[string]interface{}{"worker_name": w.name})
	return nil
}

func (w *Worker) typesList() string {
	if len(w.types) == 0 {
		return "all"
	}
	var arr []string
	for k := range w.types {
		arr = append(arr, k)
	}
	return strings.Join(arr, ",")
}

func (w *Worker) heartbeatLoop() {
	ticker := time.NewTicker(w.heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.shutdownCtx.Done():
			return
		case <-ticker.C:
			// update last_seen
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			conn, err := w.db.pool.Acquire(ctx)
			if err != nil {
				cancel()
				w.logger.Error("db_connection_failed", "Failed to acquire DB for heartbeat", w.name, err, "", nil)
				continue
			}
			_, err = conn.Exec(ctx, `update workers set last_seen=now(), status='online' where name=$1`, w.name)
			conn.Release()
			cancel()
			if err != nil {
				w.logger.Error("db_connection_failed", "Failed to exec heartbeat update", w.name, err, "", nil)
			} else {
				w.logger.Debug("heartbeat_sent", "Heartbeat updated", w.name, map[string]interface{}{"worker_name": w.name})
			}
		}
	}
}

func (w *Worker) setOffline() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := w.db.pool.Acquire(ctx)
	if err != nil {
		w.logger.Error("db_connection_failed", "Failed to acquire DB to set offline", w.name, err, "", nil)
		return
	}
	defer conn.Release()
	_, err = conn.Exec(ctx, `update workers set status='offline' where name=$1`, w.name)
	if err != nil {
		w.logger.Error("db_connection_failed", "Failed to set worker offline", w.name, err, "", nil)
		return
	}
	w.logger.Info("graceful_shutdown", "Worker set to offline", w.name, map[string]interface{}{"worker_name": w.name})
}

// Start consuming messages and processing
func (w *Worker) Start() error {
	// declare exchanges
	if err := w.amqpClient.DeclareExchanges(); err != nil {
		return err
	}
	// register in DB
	if err := w.register(context.Background()); err != nil {
		return err
	}
	// start heartbeat
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.heartbeatLoop()
	}()

	// bind keys: if types specified, bind keys per type with wildcard for priority; else bind "kitchen.#"
	var bindingKeys []string
	if len(w.types) == 0 {
		bindingKeys = []string{"kitchen.#"}
	} else {
		for t := range w.types {
			// kitchen.<type>.* where * is priority
			bindingKeys = append(bindingKeys, fmt.Sprintf("kitchen.%s.*", t))
		}
	}

	// Start consumption (in goroutine)
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		for {
			select {
			case <-w.shutdownCtx.Done():
				return
			default:
			}
			msgs, err := w.amqpClient.DeclareQueueAndConsume(w.queueName, bindingKeys, w.prefetch, w.consumerTag)
			if err != nil {
				w.logger.Error("rabbitmq_connection_failed", "Failed to declare queue or consume", w.name, err, "", nil)
				time.Sleep(time.Duration(w.rabbitReconnect) * time.Second)
				continue
			}
			w.logger.Info("consumer_started", "Started consuming messages", w.name, map[string]interface{}{"queue": w.queueName, "bindings": bindingKeys})
			consumeLoop(w, msgs)
			// if consumeLoop returns (channel closed) — try to reconnect after short sleep
			time.Sleep(time.Duration(w.rabbitReconnect) * time.Second)
		}
	}()

	return nil
}

func consumeLoop(w *Worker, msgs <-chan amqp.Delivery) {
	for {
		select {
		case <-w.shutdownCtx.Done():
			// on shutdown, we must break and eventually nack unacked messages by closing channel/connection
			return
		case d, ok := <-msgs:
			if !ok {
				// channel closed; return to outer loop to trigger reconnect
				w.logger.Info("consumer_channel_closed", "AMQP delivery channel closed", w.name, nil)
				return
			}
			// process message
			w.inFlightMu.Lock()
			w.inFlight++
			w.inFlightMu.Unlock()
			w.wg.Add(1)
			go func(delivery amqp.Delivery) {
				defer w.wg.Done()
				defer func() {
					w.inFlightMu.Lock()
					w.inFlight--
					w.inFlightMu.Unlock()
				}()
				w.processMessage(delivery)
			}(d)
		}
	}
}

func (w *Worker) processMessage(d amqp.Delivery) {
	reqID := "msg-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	w.logger.Debug("order_processing_started", "Picked order from queue", reqID, map[string]interface{}{"delivery_tag": d.DeliveryTag})
	// parse
	var msg OrderMessage
	if err := json.Unmarshal(d.Body, &msg); err != nil {
		w.logger.Error("message_processing_failed", "Invalid message JSON, sending to DLQ (nack no-requeue)", reqID, err, "", map[string]interface{}{"body": string(d.Body)})
		// nack without requeue -> dead-letter if DLQ configured; else drop
		_ = d.Nack(false, false)
		return
	}
	// specialization check
	if len(w.types) > 0 {
		if _, ok := w.types[msg.OrderType]; !ok {
			// requeue so other workers can process
			w.logger.Debug("order_requeued", "Worker not specialized for this order type, requeueing", reqID, map[string]interface{}{"order_number": msg.OrderNumber, "order_type": msg.OrderType})
			_ = d.Nack(false, true)
			return
		}
	}
	// begin transactional update to set status cooking
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	conn, err := w.db.pool.Acquire(ctx)
	if err != nil {
		w.logger.Error("db_connection_failed", "Failed to acquire DB connection", reqID, err, "", nil)
		_ = d.Nack(false, true)
		return
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		w.logger.Error("db_transaction_failed", "Failed to begin tx", reqID, err, "", nil)
		_ = d.Nack(false, true)
		return
	}
	// get current status FOR UPDATE
	var currentStatus string
	err = tx.QueryRow(ctx, `select status from orders where number=$1 for update`, msg.OrderNumber).Scan(&currentStatus)
	if err != nil {
		_ = tx.Rollback(ctx)
		if errors.Is(err, pgx.ErrNoRows) || err != nil {
			// order not found -> move to DLQ (nack no requeue)
			w.logger.Error("message_processing_failed", "Order not found in DB; nack no-requeue", reqID, err, "", map[string]interface{}{"order_number": msg.OrderNumber})
			_ = d.Nack(false, false)
			return
		}
		w.logger.Error("db_query_failed", "Failed to select order", reqID, err, "", map[string]interface{}{"order_number": msg.OrderNumber})
		_ = d.Nack(false, true)
		return
	}
	// idempotency: if already cooking or beyond, ack and return
	if currentStatus == "cooking" || currentStatus == "ready" || currentStatus == "completed" {
		_ = tx.Commit(ctx)
		w.logger.Debug("order_already_processing", "Order already in cooking/ready/completed; acking", reqID, map[string]interface{}{"order_number": msg.OrderNumber, "status": currentStatus})
		_ = d.Ack(false)
		return
	}
	// update orders -> cooking
	_, err = tx.Exec(ctx, `update orders set status='cooking', processed_by=$1, updated_at=now() where number=$2`, w.name, msg.OrderNumber)
	if err != nil {
		_ = tx.Rollback(ctx)
		w.logger.Error("db_transaction_failed", "Failed to update order to cooking", reqID, err, "", map[string]interface{}{"order_number": msg.OrderNumber})
		_ = d.Nack(false, true)
		return
	}
	// insert status log
	_, err = tx.Exec(ctx, `insert into order_status_log(order_id, status, changed_by) select id, 'cooking', $1 from orders where number=$2`, w.name, msg.OrderNumber)
	if err != nil {
		_ = tx.Rollback(ctx)
		w.logger.Error("db_transaction_failed", "Failed to insert order_status_log", reqID, err, "", map[string]interface{}{"order_number": msg.OrderNumber})
		_ = d.Nack(false, true)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		w.logger.Error("db_transaction_failed", "Failed to commit cooking tx", reqID, err, "", map[string]interface{}{"order_number": msg.OrderNumber})
		_ = d.Nack(false, true)
		return
	}
	// publish cooking status
	est := time.Now().Add(estDuration(msg.OrderType)).UTC().Format(time.RFC3339Nano)
	statusMsg := map[string]interface{}{
		"order_number":         msg.OrderNumber,
		"old_status":           "received",
		"new_status":           "cooking",
		"changed_by":           w.name,
		"timestamp":            time.Now().UTC().Format(time.RFC3339Nano),
		"estimated_completion": est,
	}
	_ = w.amqpClient.Publish(context.Background(), "notifications_fanout", "", statusMsg, 0)
	w.logger.Debug("order_published_status", "Published cooking status", reqID, map[string]interface{}{"order_number": msg.OrderNumber, "estimated_completion": est})

	// simulate cooking
	time.Sleep(estDuration(msg.OrderType))

	// finalize: set ready and increment worker orders_processed in transaction
	ctx2, cancel2 := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel2()
	conn2, err := w.db.pool.Acquire(ctx2)
	if err != nil {
		w.logger.Error("db_connection_failed", "Failed to acquire DB (finalize)", reqID, err, "", nil)
		_ = d.Nack(false, true)
		return
	}
	defer conn2.Release()
	tx2, err := conn2.Begin(ctx2)
	if err != nil {
		w.logger.Error("db_transaction_failed", "Failed to begin finalize tx", reqID, err, "", nil)
		_ = d.Nack(false, true)
		return
	}
	_, err = tx2.Exec(ctx2, `update orders set status='ready', completed_at=now(), updated_at=now() where number=$1`, msg.OrderNumber)
	if err != nil {
		_ = tx2.Rollback(ctx2)
		w.logger.Error("db_transaction_failed", "Failed to update order to ready", reqID, err, "", map[string]interface{}{"order_number": msg.OrderNumber})
		_ = d.Nack(false, true)
		return
	}
	_, err = tx2.Exec(ctx2, `insert into order_status_log(order_id, status, changed_by) select id, 'ready', $1 from orders where number=$2`, w.name, msg.OrderNumber)
	if err != nil {
		_ = tx2.Rollback(ctx2)
		w.logger.Error("db_transaction_failed", "Failed to insert ready log", reqID, err, "", map[string]interface{}{"order_number": msg.OrderNumber})
		_ = d.Nack(false, true)
		return
	}
	_, err = tx2.Exec(ctx2, `update workers set orders_processed = orders_processed + 1 where name=$1`, w.name)
	if err != nil {
		_ = tx2.Rollback(ctx2)
		w.logger.Error("db_transaction_failed", "Failed to increment orders_processed", reqID, err, "", map[string]interface{}{"worker_name": w.name})
		_ = d.Nack(false, true)
		return
	}
	if err := tx2.Commit(ctx2); err != nil {
		w.logger.Error("db_transaction_failed", "Failed to commit finalize tx", reqID, err, "", map[string]interface{}{"order_number": msg.OrderNumber})
		_ = d.Nack(false, true)
		return
	}

	// publish ready
	readyMsg := map[string]interface{}{
		"order_number": msg.OrderNumber,
		"old_status":   "cooking",
		"new_status":   "ready",
		"changed_by":   w.name,
		"timestamp":    time.Now().UTC().Format(time.RFC3339Nano),
	}
	_ = w.amqpClient.Publish(context.Background(), "notifications_fanout", "", readyMsg, 0)
	w.logger.Debug("order_completed", "Order processed and ready", reqID, map[string]interface{}{"order_number": msg.OrderNumber})
	// ack message
	_ = d.Ack(false)
}

func estDuration(orderType string) time.Duration {
	switch orderType {
	case "dine_in":
		return 8 * time.Second
	case "takeout":
		return 10 * time.Second
	case "delivery":
		return 12 * time.Second
	default:
		return 10 * time.Second
	}
}

func (w *Worker) ShutdownGracefully() {
	w.shutdownOnce.Do(func() {
		w.logger.Info("graceful_shutdown", "Shutdown initiated", w.name, nil)
		// stop accepting new messages
		w.shutdownCancel()
		// wait for in-flight goroutines to finish (with timeout)
		done := make(chan struct{})
		go func() {
			w.wg.Wait()
			close(done)
		}()
		select {
		case <-done:
			// ok
		case <-time.After(30 * time.Second):
			// timeout — continue anyway
			w.logger.Info("graceful_shutdown", "Timeout waiting for in-flight to finish", w.name, nil)
		}
		// set worker offline
		w.setOffline()
		// close AMQP client (this will cause prefetched messages to be requeued since channel/conn closed)
		w.amqpClient.Close()
	})
}

func loadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config

	// определяем формат по расширению файла
	switch {
	case strings.HasSuffix(path, ".yaml"), strings.HasSuffix(path, ".yml"):
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return nil, err
		}
	default: // по умолчанию json
		if err := json.Unmarshal(b, &cfg); err != nil {
			return nil, err
		}
	}

	// default ports if missing
	if cfg.Database.Port == 0 {
		cfg.Database.Port = 5432
	}
	if cfg.RabbitMQ.Port == 0 {
		cfg.RabbitMQ.Port = 5672
	}
	return &cfg, nil
}

func main() {
	// flags
	var (
		cfgPathFlag       = flag.String("config", "", "path to config file (yaml)")
		workerName        = flag.String("worker-name", "", "unique name for worker (required)")
		orderTypesFlag    = flag.String("order-types", "", "comma-separated order types (optional: dine_in,takeout,delivery)")
		heartbeatInterval = flag.Int("heartbeat-interval", 30, "heartbeat interval seconds")
		prefetch          = flag.Int("prefetch", 1, "rabbitmq prefetch count")
		rabbitReconnect   = flag.Int("rabbit-reconnect-interval", 5, "rabbit reconnect interval seconds")
	)
	flag.Parse()

	if *workerName == "" {
		fmt.Fprintln(os.Stderr, "--worker-name is required")
		os.Exit(2)
	}

	cfgPath := os.Getenv("CONFIG_PATH") // пробуем взять из env
	if cfgPath == "" {
		cfgPath = *cfgPathFlag // если не задано, берём из флага
	}
	if cfgPath == "" {
		fmt.Fprintln(os.Stderr, "CONFIG_PATH or --config is required")
		os.Exit(2)
	}

	types := []string{}
	if *orderTypesFlag != "" {
		for _, t := range strings.Split(*orderTypesFlag, ",") {
			tt := strings.TrimSpace(t)
			if tt != "" {
				types = append(types, tt)
			}
		}
	}

	logger := NewLogger(serviceName)
	requestID := "startup-" + strconv.FormatInt(time.Now().Unix(), 10)
	logger.Info("service_started", "Kitchen worker starting", requestID, map[string]interface{}{
		"worker_name": *workerName, "order_types": types, "prefetch": *prefetch,
	})

	// load config
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		logger.Error("config_load_failed", "Failed to load config", requestID, err, "", nil)
		os.Exit(1)
	}

	// init DB
	ctx := context.Background()
	db, err := NewDB(ctx, cfg)
	if err != nil {
		logger.Error("db_connection_failed", "Failed to connect to DB", requestID, err, "", nil)
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("db_connected", "Connected to PostgreSQL database", requestID, nil)

	// init AMQP client
	amqpURL := fmt.Sprintf("amqp://%s:%s@%s:%d/", cfg.RabbitMQ.User, cfg.RabbitMQ.Password, cfg.RabbitMQ.Host, cfg.RabbitMQ.Port)
	amqpClient, err := NewAMQPClient(amqpURL, logger)
	if err != nil {
		logger.Error("rabbitmq_connection_failed", "Failed to connect to RabbitMQ", requestID, err, "", nil)
		os.Exit(1)
	}
	// ensure exchanges exist
	if err := amqpClient.DeclareExchanges(); err != nil {
		logger.Error("rabbitmq_setup_failed", "Failed to declare exchanges", requestID, err, "", nil)
		amqpClient.Close()
		os.Exit(1)
	}

	worker := NewWorker(*workerName, types, time.Duration(*heartbeatInterval)*time.Second, *prefetch, amqpClient, db, logger, *rabbitReconnect)

	// start worker
	if err := worker.Start(); err != nil {
		logger.Error("worker_start_failed", "Failed to start worker", requestID, err, "", nil)
		os.Exit(1)
	}

	// signals
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// wait
	sigReceived := <-sig
	logger.Info("signal_received", "Shutdown signal received", requestID, map[string]interface{}{"signal": sigReceived.String()})
	worker.ShutdownGracefully()

	logger.Info("service_exited", "Worker exited", requestID, map[string]interface{}{"worker_name": *workerName})
}
