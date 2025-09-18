package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"wheres-my-pizza/internal/config"
	"wheres-my-pizza/internal/db"
	"wheres-my-pizza/internal/logger"
	"wheres-my-pizza/internal/rabbitmq"

	orderhttp "wheres-my-pizza/internal/order/adapter/http"
	ordermq "wheres-my-pizza/internal/order/adapter/mq"
	orderrepo "wheres-my-pizza/internal/order/adapter/repo"
	orderapp "wheres-my-pizza/internal/order/app"

	kitchenmq "wheres-my-pizza/internal/kitchen/adapter/mq"
	kitchenrepo "wheres-my-pizza/internal/kitchen/adapter/repo"
	kitchenapp "wheres-my-pizza/internal/kitchen/app"

	trackhttp "wheres-my-pizza/internal/tracking/adapter/http"
	trackrepo "wheres-my-pizza/internal/tracking/adapter/repo"
	trackapp "wheres-my-pizza/internal/tracking/app"

	notimq "wheres-my-pizza/internal/notification/adapter/mq"
	notiapp "wheres-my-pizza/internal/notification/app"

	demo "wheres-my-pizza/internal/demo"
)

func main() {
	mode := flag.String("mode", "", "Service mode: order-service | kitchen-worker | tracking-service | notification-subscriber")
	port := flag.Int("port", 0, "HTTP port (for HTTP services)")
	maxConcurrent := flag.Int("max-concurrent", 50, "Max concurrent orders to process (order-service)")
	workerName := flag.String("worker-name", "", "Unique worker name (kitchen-worker)")
	orderTypes := flag.String("order-types", "", "Comma-separated order types to handle (kitchen-worker)")
	heartbeatInterval := flag.Int("heartbeat-interval", 30, "Heartbeat interval seconds (kitchen-worker)")
	prefetch := flag.Int("prefetch", 1, "RabbitMQ prefetch (kitchen-worker)")
	cfgPath := flag.String("config", "config.yaml", "Path to YAML config file")
	flag.Parse()

	if *mode == "" {
		fmt.Fprintln(os.Stderr, "--mode is required")
		os.Exit(2)
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(2)
	}

	host, _ := os.Hostname()
	log := logger.New(*mode, host)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Shared DB (for services that need it)
	var pool *db.Pool
	var amqp *rabbitmq.Client

	// Initialize per-mode resources and start service
	switch *mode {
	case "demo-ui":
		if *port == 0 {
			*port = 3005
		}
		pool, err = db.Connect(ctx, cfg.Database)
		if err != nil {
			log.Error(ctx, "db_connection_failed", "Failed to connect to database", err)
			os.Exit(1)
		}
		if err := db.RunMigrations(ctx, pool); err != nil {
			log.Error(ctx, "db_migration_failed", "Failed to run migrations", err)
			os.Exit(1)
		}
		amqp = rabbitmq.NewClient(cfg.RabbitMQ, log)
		if err := amqp.ConnectWithRetry(ctx); err != nil {
			log.Error(ctx, "rabbitmq_connection_failed", "Failed to connect to RabbitMQ", err)
			os.Exit(1)
		}
		if err := amqp.EnsureOrderTopology(ctx); err != nil {
			log.Error(ctx, "rabbitmq_declare_failed", "Failed to declare order topology", err)
			os.Exit(1)
		}
		if err := amqp.EnsureNotificationTopology(ctx); err != nil {
			log.Error(ctx, "rabbitmq_declare_failed", "Failed to declare notification topology", err)
			os.Exit(1)
		}
		h, err := demo.New(ctx, log, pool, amqp, *maxConcurrent)
		if err != nil {
			log.Error(ctx, "demo_failed", "Failed to init demo ui", err)
			os.Exit(1)
		}
		httpSrv := &http.Server{Addr: fmt.Sprintf(":%d", *port), Handler: h}
		go func() {
			log.Info(ctx, "service_started", fmt.Sprintf("Demo UI started on port %d", *port), logger.M{"port": *port})
			if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error(ctx, "http_server_failed", "HTTP server error", err)
				stop()
			}
		}()
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	case "order-service":
		if *port == 0 {
			*port = 3000
		}
		pool, err = db.Connect(ctx, cfg.Database)
		if err != nil {
			log.Error(ctx, "db_connection_failed", "Failed to connect to database", err)
			os.Exit(1)
		}
		log.Info(ctx, "db_connected", "Connected to PostgreSQL database", logger.M{"duration_ms": 0})

		if err := db.RunMigrations(ctx, pool); err != nil {
			log.Error(ctx, "db_migration_failed", "Failed to run migrations", err)
			os.Exit(1)
		}

		amqp = rabbitmq.NewClient(cfg.RabbitMQ, log)
		if err := amqp.ConnectWithRetry(ctx); err != nil {
			log.Error(ctx, "rabbitmq_connection_failed", "Failed to connect to RabbitMQ", err)
			os.Exit(1)
		}
		if err := amqp.EnsureOrderTopology(ctx); err != nil {
			log.Error(ctx, "rabbitmq_declare_failed", "Failed to declare RabbitMQ topology", err)
			os.Exit(1)
		}
		log.Info(ctx, "rabbitmq_connected", "Connected to RabbitMQ exchange 'orders_topic'", nil)

		repo := orderrepo.NewPostgres(pool)
		pub := ordermq.NewPublisher(amqp)
		app := orderapp.NewService(repo, pub, log)
		srv := orderhttp.NewServer(app, log, *maxConcurrent)
		httpSrv := &http.Server{Addr: fmt.Sprintf(":%d", *port), Handler: srv}
		go func() {
			log.Info(ctx, "service_started", fmt.Sprintf("Order Service started on port %d", *port), logger.M{"port": *port, "max_concurrent": *maxConcurrent})
			if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error(ctx, "http_server_failed", "HTTP server error", err)
				stop()
			}
		}()

		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)

	case "kitchen-worker":
		if *workerName == "" {
			fmt.Fprintln(os.Stderr, "--worker-name is required for kitchen-worker")
			os.Exit(2)
		}
		pool, err = db.Connect(ctx, cfg.Database)
		if err != nil {
			log.Error(ctx, "db_connection_failed", "Failed to connect to database", err)
			os.Exit(1)
		}
		if err := db.RunMigrations(ctx, pool); err != nil {
			log.Error(ctx, "db_migration_failed", "Failed to run migrations", err)
			os.Exit(1)
		}
		amqp = rabbitmq.NewClient(cfg.RabbitMQ, log)
		if err := amqp.ConnectWithRetry(ctx); err != nil {
			log.Error(ctx, "rabbitmq_connection_failed", "Failed to connect to RabbitMQ", err)
			os.Exit(1)
		}
		if err := amqp.EnsureKitchenTopology(ctx, *orderTypes); err != nil {
			log.Error(ctx, "rabbitmq_declare_failed", "Failed to declare RabbitMQ kitchen topology", err)
			os.Exit(1)
		}
		cons := kitchenmq.NewConsumer(amqp)
		repo := kitchenrepo.New(pool)
		pub := kitchenmq.NewPublisher(amqp)
		runner := kitchenapp.NewRunner(*workerName, *orderTypes, *prefetch, *heartbeatInterval, cons, repo, pub, log)
		if err := runner.Run(ctx); err != nil {
			// already logged inside
			os.Exit(1)
		}

	case "tracking-service":
		if *port == 0 {
			*port = 3002
		}
		pool, err = db.Connect(ctx, cfg.Database)
		if err != nil {
			log.Error(ctx, "db_connection_failed", "Failed to connect to database", err)
			os.Exit(1)
		}
		if err := db.RunMigrations(ctx, pool); err != nil {
			log.Error(ctx, "db_migration_failed", "Failed to run migrations", err)
			os.Exit(1)
		}
		repo := trackrepo.New(pool)
		app := trackapp.New(repo, 2*30)
		srv := trackhttp.New(app, log)
		httpSrv := &http.Server{Addr: fmt.Sprintf(":%d", *port), Handler: srv}
		go func() {
			log.Info(ctx, "service_started", fmt.Sprintf("Tracking Service started on port %d", *port), logger.M{"port": *port})
			if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error(ctx, "http_server_failed", "HTTP server error", err)
				stop()
			}
		}()
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)

	case "notification-subscriber":
		amqp = rabbitmq.NewClient(cfg.RabbitMQ, log)
		if err := amqp.ConnectWithRetry(ctx); err != nil {
			log.Error(ctx, "rabbitmq_connection_failed", "Failed to connect to RabbitMQ", err)
			os.Exit(1)
		}
		if err := amqp.EnsureNotificationTopology(ctx); err != nil {
			log.Error(ctx, "rabbitmq_declare_failed", "Failed to declare notification topology", err)
			os.Exit(1)
		}
		cons := notimq.NewConsumer(amqp)
		sub := notiapp.New(cons, log)
		if err := sub.Run(ctx); err != nil {
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown --mode: %s\n", *mode)
		os.Exit(2)
	}

	if amqp != nil {
		amqp.Close()
	}
	if pool != nil {
		pool.Close()
	}
}
