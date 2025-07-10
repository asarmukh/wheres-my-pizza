# Архитектура проекта Restaurant System

## Общая структура проекта

```
restaurant-system/
├── cmd/
│   └── main.go                 # Точка входа с маршрутизацией по --mode
├── internal/
│   ├── config/
│   │   └── config.go          # Загрузка YAML конфигурации
│   ├── logger/
│   │   └── logger.go          # Структурированное логирование
│   ├── database/
│   │   ├── connection.go      # Подключение к PostgreSQL
│   │   ├── migrations.go      # Миграции БД
│   │   └── models.go          # Модели данных
│   ├── messaging/
│   │   ├── rabbitmq.go        # Клиент RabbitMQ
│   │   ├── exchanges.go       # Настройка exchanges
│   │   └── queues.go          # Настройка очередей
│   ├── services/
│   │   ├── order/
│   │   │   ├── service.go     # Order Service
│   │   │   ├── handlers.go    # HTTP handlers
│   │   │   └── validator.go   # Валидация заказов
│   │   ├── kitchen/
│   │   │   ├── worker.go      # Kitchen Worker
│   │   │   └── processor.go   # Обработка заказов
│   │   ├── tracking/
│   │   │   ├── service.go     # Tracking Service
│   │   │   └── handlers.go    # HTTP handlers
│   │   └── notification/
│   │       └── subscriber.go  # Notification Service
│   └── shared/
│       ├── types.go           # Общие типы и структуры
│       └── utils.go           # Утилиты
├── configs/
│   └── config.yaml            # Конфигурационный файл
├── migrations/
│   ├── 001_create_orders.sql
│   ├── 002_create_workers.sql
│   └── 003_create_indexes.sql
├── go.mod
├── go.sum
└── README.md
```

## Архитектурные принципы

### 1. Hexagonal Architecture (Ports and Adapters)

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Core                          │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │                Business Logic                           │ │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │ │
│  │  │ Order       │  │ Kitchen     │  │ Tracking    │    │ │
│  │  │ Service     │  │ Service     │  │ Service     │    │ │
│  │  └─────────────┘  └─────────────┘  └─────────────┘    │ │
│  └─────────────────────────────────────────────────────────┘ │
│                                                             │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │                    Ports                                │ │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │ │
│  │  │ Repository  │  │ Publisher   │  │ Consumer    │    │ │
│  │  │ Interface   │  │ Interface   │  │ Interface   │    │ │
│  │  └─────────────┘  └─────────────┘  └─────────────┘    │ │
│  └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                                │
                                │
┌─────────────────────────────────────────────────────────────┐
│                    Adapters                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │ PostgreSQL  │  │ RabbitMQ    │  │ HTTP        │        │
│  │ Adapter     │  │ Adapter     │  │ Adapter     │        │
│  └─────────────┘  └─────────────┘  └─────────────┘        │
└─────────────────────────────────────────────────────────────┘
```

### 2. Domain-Driven Design (DDD)

**Основные домены:**
- **Order Domain**: Управление заказами
- **Kitchen Domain**: Обработка заказов
- **Tracking Domain**: Отслеживание статусов
- **Notification Domain**: Уведомления

**Агрегаты:**
- **Order Aggregate**: Order, OrderItem, OrderStatus
- **Worker Aggregate**: Worker, WorkerStatus

### 3. CQRS Pattern (Command Query Responsibility Segregation)

**Commands (Изменения состояния):**
- CreateOrder
- UpdateOrderStatus
- RegisterWorker

**Queries (Чтение данных):**
- GetOrderStatus
- GetOrderHistory
- GetWorkersStatus

## Детальная архитектура компонентов

### Order Service Architecture

```go
// internal/services/order/service.go
type Service struct {
    repo      Repository
    publisher MessagePublisher
    validator Validator
    logger    Logger
}

type Repository interface {
    CreateOrder(ctx context.Context, order *Order) error
    CreateOrderItems(ctx context.Context, orderID int, items []OrderItem) error
    LogOrderStatus(ctx context.Context, orderID int, status OrderStatus) error
    GetNextOrderNumber(ctx context.Context) (string, error)
    WithTransaction(ctx context.Context, fn func(tx Transaction) error) error
}

type MessagePublisher interface {
    PublishOrder(ctx context.Context, order OrderMessage) error
}

type Validator interface {
    ValidateCreateOrderRequest(req CreateOrderRequest) error
}
```

### Kitchen Worker Architecture

```go
// internal/services/kitchen/worker.go
type Worker struct {
    name       string
    orderTypes []string
    repo       Repository
    consumer   MessageConsumer
    publisher  MessagePublisher
    logger     Logger
    shutdown   chan struct{}
}

type Repository interface {
    RegisterWorker(ctx context.Context, worker *Worker) error
    UpdateWorkerStatus(ctx context.Context, name string, status WorkerStatus) error
    UpdateOrderStatus(ctx context.Context, orderNumber string, status OrderStatus) error
    LogOrderStatus(ctx context.Context, orderID int, status OrderStatus) error
}

type MessageConsumer interface {
    ConsumeOrders(ctx context.Context, handler OrderHandler) error
}

type OrderHandler interface {
    HandleOrder(ctx context.Context, order OrderMessage) error
}
```

### Messaging Architecture

```go
// internal/messaging/rabbitmq.go
type RabbitMQClient struct {
    connection *amqp.Connection
    channel    *amqp.Channel
    config     RabbitMQConfig
    logger     Logger
}

// Exchanges
const (
    OrdersTopicExchange      = "orders_topic"
    NotificationsFanoutExchange = "notifications_fanout"
)

// Queues
const (
    KitchenQueue        = "kitchen_queue"
    KitchenDineInQueue  = "kitchen_dine_in_queue"
    KitchenTakeoutQueue = "kitchen_takeout_queue"
    KitchenDeliveryQueue = "kitchen_delivery_queue"
    NotificationsQueue  = "notifications_queue"
    DeadLetterQueue     = "dead_letter_queue"
)

// Routing Keys
const (
    KitchenDineInRouting   = "kitchen.dine_in.*"
    KitchenTakeoutRouting  = "kitchen.takeout.*"
    KitchenDeliveryRouting = "kitchen.delivery.*"
)
```

## Паттерны обработки ошибок

### 1. Circuit Breaker Pattern

```go
type CircuitBreaker struct {
    maxFailures int
    timeout     time.Duration
    failures    int
    lastFailure time.Time
    state       CircuitState
}

type CircuitState int

const (
    Closed CircuitState = iota
    Open
    HalfOpen
)
```

### 2. Retry Pattern с экспоненциальным backoff

```go
type RetryConfig struct {
    MaxRetries      int
    InitialInterval time.Duration
    MaxInterval     time.Duration
    Multiplier      float64
}

func RetryWithBackoff(ctx context.Context, config RetryConfig, fn func() error) error {
    // Реализация retry логики
}
```

### 3. Graceful Shutdown Pattern

```go
type GracefulShutdown struct {
    timeout time.Duration
    signals []os.Signal
    cleanup []func() error
}

func (gs *GracefulShutdown) Wait() {
    // Ожидание сигналов и выполнение cleanup
}
```

## Конфигурация и DI

### Dependency Injection Container

```go
type Container struct {
    config     *Config
    logger     Logger
    db         *pgx.Conn
    rabbitmq   *RabbitMQClient
    services   map[string]interface{}
}

func NewContainer(configPath string) (*Container, error) {
    // Инициализация всех зависимостей
}

func (c *Container) GetOrderService() *order.Service {
    // Возвращает настроенный Order Service
}
```

### Configuration Management

```go
type Config struct {
    Database struct {
        Host     string `yaml:"host"`
        Port     int    `yaml:"port"`
        User     string `yaml:"user"`
        Password string `yaml:"password"`
        Database string `yaml:"database"`
    } `yaml:"database"`
    
    RabbitMQ struct {
        Host     string `yaml:"host"`
        Port     int    `yaml:"port"`
        User     string `yaml:"user"`
        Password string `yaml:"password"`
    } `yaml:"rabbitmq"`
}
```

## Мониторинг и наблюдаемость

### Structured Logging

```go
type Logger interface {
    Info(ctx context.Context, action string, message string, fields ...Field)
    Debug(ctx context.Context, action string, message string, fields ...Field)
    Error(ctx context.Context, action string, message string, err error, fields ...Field)
}

type Field struct {
    Key   string
    Value interface{}
}
```

### Request Tracing

```go
type RequestContext struct {
    RequestID string
    StartTime time.Time
    Service   string
    Action    string
}

func NewRequestContext(service, action string) RequestContext {
    return RequestContext{
        RequestID: generateRequestID(),
        StartTime: time.Now(),
        Service:   service,
        Action:    action,
    }
}
```

## Дополнительные компоненты для production

### 1. Health Check System

```go
// internal/health/checker.go
type HealthChecker struct {
    checks map[string]CheckFunc
}

type CheckFunc func(ctx context.Context) error

type HealthStatus struct {
    Status    string            `json:"status"`
    Timestamp time.Time         `json:"timestamp"`
    Checks    map[string]string `json:"checks"`
    Version   string            `json:"version"`
}

// Endpoints для каждого сервиса
// GET /health - общий статус
// GET /health/ready - готовность к работе
// GET /health/live - проверка живости
```

### 2. Metrics and Monitoring

```go
// internal/metrics/metrics.go
type Metrics struct {
    ordersCreated    counter
    ordersProcessed  counter
    processingTime   histogram
    activeWorkers    gauge
    queueSize        gauge
    errorRate        counter
}

type MetricsCollector interface {
    IncrementOrdersCreated()
    IncrementOrdersProcessed()
    RecordProcessingTime(duration time.Duration)
    SetActiveWorkers(count int)
    SetQueueSize(size int)
    IncrementErrors(errorType string)
}
```

### 3. Rate Limiting

```go
// internal/middleware/ratelimit.go
type RateLimiter struct {
    requests map[string]*tokenBucket
    mu       sync.RWMutex
    rate     int
    burst    int
}

type tokenBucket struct {
    tokens    int
    lastRefill time.Time
}

func (rl *RateLimiter) Allow(clientID string) bool {
    // Реализация token bucket алгоритма
}
```

### 4. Connection Pooling

```go
// internal/database/pool.go
type ConnectionPool struct {
    pool *pgxpool.Pool
    cfg  pgxpool.Config
}

func NewConnectionPool(cfg DatabaseConfig) (*ConnectionPool, error) {
    config, err := pgxpool.ParseConfig(cfg.DSN())
    if err != nil {
        return nil, err
    }
    
    // Настройка пула соединений
    config.MaxConns = cfg.MaxConnections
    config.MinConns = cfg.MinConnections
    config.MaxConnLifetime = cfg.MaxConnLifetime
    config.MaxConnIdleTime = cfg.MaxConnIdleTime
    
    pool, err := pgxpool.ConnectConfig(context.Background(), config)
    return &ConnectionPool{pool: pool, cfg: *config}, err
}
```

### 5. Dead Letter Queue Handler

```go
// internal/messaging/dlq.go
type DeadLetterHandler struct {
    consumer MessageConsumer
    logger   Logger
    storage  DLQStorage
}

type DLQStorage interface {
    SaveFailedMessage(ctx context.Context, msg FailedMessage) error
    GetFailedMessages(ctx context.Context, limit int) ([]FailedMessage, error)
    RetryMessage(ctx context.Context, id string) error
}

type FailedMessage struct {
    ID          string    `json:"id"`
    OriginalMsg []byte    `json:"original_message"`
    Error       string    `json:"error"`
    Timestamp   time.Time `json:"timestamp"`
    RetryCount  int       `json:"retry_count"`
}
```

### 6. Security Layer

```go
// internal/security/auth.go
type AuthMiddleware struct {
    secret []byte
    logger Logger
}

func (am *AuthMiddleware) ValidateToken(token string) (*Claims, error) {
    // JWT validation logic
}

type Claims struct {
    UserID   string `json:"user_id"`
    Role     string `json:"role"`
    ExpireAt int64  `json:"exp"`
}

// internal/security/validator.go
type InputValidator struct {
    sanitizer *html.Sanitizer
}

func (iv *InputValidator) SanitizeInput(input string) string {
    // XSS protection
}

func (iv *InputValidator) ValidateSQL(query string) error {
    // SQL injection protection
}
```

### 7. Cache Layer

```go
// internal/cache/redis.go
type CacheService struct {
    client redis.Client
    ttl    time.Duration
}

func (cs *CacheService) GetOrderStatus(orderNumber string) (*OrderStatus, error) {
    // Cache lookup
}

func (cs *CacheService) SetOrderStatus(orderNumber string, status *OrderStatus) error {
    // Cache write with TTL
}
```

### 8. Background Job Scheduler

```go
// internal/scheduler/scheduler.go
type JobScheduler struct {
    jobs   map[string]Job
    ticker *time.Ticker
    ctx    context.Context
    cancel context.CancelFunc
}

type Job interface {
    Name() string
    Schedule() time.Duration
    Execute(ctx context.Context) error
}

// Jobs examples:
// - CleanupExpiredOrders
// - SendHeartbeat
// - CollectMetrics
// - RotateLogs
```

### 9. Configuration Management

```go
// internal/config/env.go
type Environment string

const (
    Development Environment = "development"
    Testing     Environment = "testing"
    Staging     Environment = "staging"
    Production  Environment = "production"
)

type Config struct {
    Environment Environment `yaml:"environment"`
    
    // Feature flags
    Features struct {
        EnableMetrics     bool `yaml:"enable_metrics"`
        EnableRateLimit   bool `yaml:"enable_rate_limit"`
        EnableCache       bool `yaml:"enable_cache"`
        EnableAuth        bool `yaml:"enable_auth"`
    } `yaml:"features"`
    
    // Performance settings
    Performance struct {
        MaxConcurrentOrders int           `yaml:"max_concurrent_orders"`
        RequestTimeout      time.Duration `yaml:"request_timeout"`
        ShutdownTimeout     time.Duration `yaml:"shutdown_timeout"`
    } `yaml:"performance"`
}
```

### 10. API Documentation

```go
// internal/docs/swagger.go
// Swagger/OpenAPI documentation
type APIDoc struct {
    Title       string `json:"title"`
    Version     string `json:"version"`
    Description string `json:"description"`
    Paths       map[string]PathItem `json:"paths"`
}

// Генерация документации через go:generate
//go:generate swagger generate spec -o ./docs/swagger.json
```

## Улучшенная структура проекта

```
restaurant-system/
├── cmd/
│   └── main.go
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   └── env.go
│   ├── health/
│   │   └── checker.go
│   ├── metrics/
│   │   ├── metrics.go
│   │   └── collector.go
│   ├── middleware/
│   │   ├── ratelimit.go
│   │   ├── auth.go
│   │   └── logging.go
│   ├── security/
│   │   ├── auth.go
│   │   └── validator.go
│   ├── cache/
│   │   └── redis.go
│   ├── scheduler/
│   │   ├── scheduler.go
│   │   └── jobs.go
│   ├── database/
│   │   ├── connection.go
│   │   ├── pool.go
│   │   └── migrations.go
│   ├── messaging/
│   │   ├── rabbitmq.go
│   │   ├── dlq.go
│   │   └── reconnect.go
│   └── docs/
│       └── swagger.go
├── configs/
│   ├── config.yaml
│   ├── config.dev.yaml
│   ├── config.staging.yaml
│   └── config.prod.yaml
├── deployments/
│   ├── docker-compose.yml
│   ├── k8s/
│   └── helm/
├── monitoring/
│   ├── grafana/
│   └── prometheus/
└── scripts/
    ├── migrate.sh
    └── test.sh
```

## Что НЕ нужно добавлять для данного проекта

❌ **Избыточные компоненты:**
1. **Service Mesh** (Istio/Linkerd) - слишком сложно для учебного проекта
2. **Event Sourcing** - не требуется по условию
3. **SAGA Pattern** - простые операции, достаточно транзакций
4. **GraphQL** - REST API по условию
5. **Kubernetes Operators** - избыточно
6. **Distributed Tracing** (Jaeger/Zipkin) - хорошо иметь, но не критично

## Приоритеты реализации

### 🔥 Критично (Must Have)
1. Health Checks - для мониторинга
2. Connection Pooling - для производительности
3. Graceful Shutdown - для надежности
4. Dead Letter Queue - для обработки ошибок

### 🟡 Важно (Should Have)
1. Rate Limiting - для защиты от перегрузок
2. Metrics - для мониторинга производительности
3. Caching - для производительности
4. Background Jobs - для maintenance

### 🔵 Желательно (Nice to Have)
1. Security Layer - для production
2. API Documentation - для разработчиков
3. Configuration Management - для разных окружений

## Итоговая оценка

Представленная архитектура теперь **готова для production**, но для учебного проекта можно реализовать по приоритетам:

1. **Минимальная версия**: Основные сервисы + Health Checks + Connection Pooling
2. **Расширенная версия**: + Rate Limiting + Metrics + DLQ
3. **Production-ready**: + Security + Caching + Background Jobs

Это позволит постепенно наращивать сложность системы и изучать новые паттерны.