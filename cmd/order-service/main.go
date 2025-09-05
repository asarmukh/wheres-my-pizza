package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
	"wheres-my-pizza/internal/adapters/http/handlers"
	messaging "wheres-my-pizza/internal/adapters/messaging/rabbitmq"
	"wheres-my-pizza/internal/adapters/storage/postgres"
	"wheres-my-pizza/internal/config"
	"wheres-my-pizza/internal/core/services"
	"wheres-my-pizza/internal/database"
	"wheres-my-pizza/internal/logger"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	mode := flag.String("mode", "", "Mode to run: order-service | kitchen-worker | tracking-service | notification-subscriber")
	port := flag.Int("port", 3000, "HTTP port (only for services with HTTP API)")
	flag.Parse()

	if *mode == "" {
		log.Fatal("You must specify --mode")
	}

	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		log.Fatal("CONFIG_PATH env variable not set")
	}

	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	reqID := logger.NewRequestID()
	ctx := context.Background()

	dbStart := time.Now()
	db, err := database.Connect(cfg.Database)
	if err != nil {
		logger.Error(*mode, "db_connect", "cannot connect to DB", reqID, err)
		os.Exit(1)
	}
	defer db.Close()

	logger.Info("order-service", "db_connected", "Connected to PostgreSQL database", reqID, nil, time.Since(dbStart).Milliseconds())

	logger.Info("order-service", "service_started", "Order Service started", reqID, map[string]interface{}{
		"port":           *port,
		"max_concurrent": 50,
	}, 0)

	switch *mode {
	case "order-service":
		runOrderService(ctx, cfg, db, *port, reqID)
	case "kitchen-worker":
		runKitchenWorker(ctx, cfg, db)
	case "tracking-service":
		runTrackingService(ctx, cfg, db, *port)
	case "notification-subscriber":
		runNotificationService(ctx)
	default:
		log.Fatalf("Unknown mode: %s", *mode)
	}
}

func runOrderService(ctx context.Context, cfg *config.Config, db database.Pool, port int, reqID string) {
	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	if err != nil {
		logger.Error("order-service", "rabbitmq_connect", "failed to connect to RabbitMQ", reqID, err)
		os.Exit(1)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	err = ch.ExchangeDeclare(
		"orders_topic",
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)

	if err != nil {
		logger.Error("order-service", "rabbitmq_channel", "failed to open RabbitMQ channel", reqID, err)
		os.Exit(1)
	}

	publisher := messaging.NewRabbitMQPublisher(ch, "orders_topic")

	repo := postgres.NewOrderRepo(db)
	service := services.NewOrderService(repo, publisher)
	handler := handlers.NewOrderHandler(service)

	mux := http.NewServeMux()
	config.RegisterRoutes(mux, handler)

	addr := fmt.Sprintf(":%d", port)
	logger.Info("order-service", "http_server_started", "Starting HTTP server", reqID, map[string]interface{}{
		"address": addr,
	}, 0)

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("order-service", "http_server_failed", "ListenAndServe failed", reqID, err)
		os.Exit(1)
	}
}

func runKitchenWorker(ctx context.Context, cfg *config.Config, db database.Pool) {
	// TODO
}

func runTrackingService(ctx context.Context, cfg *config.Config, db database.Pool, port int) {
	// TODO
}

func runNotificationService(ctx context.Context) {
	// TODO
}
