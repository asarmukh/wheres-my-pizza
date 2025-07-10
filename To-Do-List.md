## PHASE 1: Project Setup

### Project Initialization
- [ ] Initialize Go module: `go mod init github.com/asarmukh /wheres-my-pizza`
- [ ] Create `cmd/` directories for each service
- [ ] Implement `config.yaml` loader (`internal/config`)
- [ ] Implement structured JSON logger (`internal/logger`)
- [ ] Create PostgreSQL connector (`internal/database`)
- [ ] Create RabbitMQ connector with reconnection support (`internal/rabbitmq`)
- [ ] Define shared domain models (`internal/model`)

---

## PHASE 2: Order Service

### API + Business Logic
- [ ] Implement HTTP server with `/orders` route
- [ ] Add request validation (including conditional logic)
- [ ] Calculate `total_amount` and assign `priority`
- [ ] Generate unique `order_number` (format: `ORD_YYYYMMDD_NNN`)
- [ ] Insert into `orders`, `order_items`, `order_status_log` (in a single transaction)

### RabbitMQ Publishing
- [ ] Connect to `orders_topic` exchange
- [ ] Publish persistent order messages with routing key: `kitchen.{order_type}.{priority}`
- [ ] Include `priority` property in message metadata

### Testing
- [ ] Unit test validation and order number generator
- [ ] Integration test full HTTP → DB → RabbitMQ flow

### Logging
- [ ] Log `order_received`, `order_published`, and errors

---

## PHASE 3: Kitchen Worker

### Startup & Registration
- [ ] Parse CLI flags (`--worker-name`, `--order-types`, etc.)
- [ ] Register in `workers` table (error on duplicate online name)

### Message Consumption & Processing
- [ ] Consume from `orders_topic` via multiple queues
- [ ] Use `basic.qos` to limit unacked messages
- [ ] Use `basic.nack` to requeue if not specialized
- [ ] Process order:
  - [ ] Update `status = cooking`, set `processed_by`
  - [ ] Log status in `order_status_log`
  - [ ] Sleep based on `order_type` to simulate cooking
  - [ ] Set `status = ready`, `completed_at`, increment `orders_processed`
- [ ] Publish status updates to `notifications_fanout`

### Heartbeats & Graceful Shutdown
- [ ] Periodically update `last_seen` and `status = online`
- [ ] On SIGINT/SIGTERM:
  - [ ] Stop consuming
  - [ ] Finish current order
  - [ ] Update `status = offline`
  - [ ] Nack unacknowledged messages
  - [ ] Exit gracefully

### Logging
- [ ] Log `worker_registered`, `order_processing_started`, `order_completed`, `heartbeat_sent`

---

## PHASE 4: Notification Subscriber

### Message Consumption
- [ ] Connect to `notifications_fanout`
- [ ] Bind and consume from `notifications_queue`
- [ ] Parse and display notifications
- [ ] Acknowledge all messages

### Logging
- [ ] Log `notification_received` and MQ connection errors

---

## PHASE 5: Tracking Service

### HTTP Endpoints
- [ ] `GET /orders/{order_number}/status`
- [ ] `GET /orders/{order_number}/history`
- [ ] `GET /workers/status`

### Logic
- [ ] Query DB for each endpoint
- [ ] Calculate offline workers if `now - last_seen > 2 * heartbeat_interval`
- [ ] Return structured JSON

### Testing
- [ ] Integration test all endpoints

### Logging
- [ ] Log `request_received` and `db_query_failed`

---

## PHASE 6: Infrastructure

### PostgreSQL
- [ ] Write SQL migrations:
  - [ ] `orders`
  - [ ] `order_items`
  - [ ] `order_status_log`
  - [ ] `workers`

### RabbitMQ
- [ ] Declare `orders_topic` exchange (topic)
- [ ] Declare `notifications_fanout` exchange (fanout)
- [ ] Set up queues for each `order_type.priority`
- [ ] Configure Dead Letter Queue (DLQ)

### Docker & Compose
- [ ] Write `Dockerfile` (multi-stage build)
- [ ] Write `docker-compose.yml`:
  - [ ] Services: order, kitchen, tracking, notification
  - [ ] PostgreSQL and RabbitMQ with management UI

---

## PHASE 7: Finalization

### Graceful Shutdown
- [ ] Ensure all services handle `SIGINT` and `SIGTERM`
- [ ] Release all resources (DB, MQ)

### End-to-End Testing
- [ ] Place test order
- [ ] Simulate kitchen worker processing
- [ ] Track order status
- [ ] Observe notification output

### Documentation
- [ ] Update `README.md`:
  - [ ] Setup instructions
  - [ ] Build/run usage
  - [ ] API reference
  - [ ] Examples (`curl` requests)