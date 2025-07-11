package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"wheres-my-pizza/internal/config"
	"wheres-my-pizza/internal/database"
	"wheres-my-pizza/internal/logger"
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

	db, err := database.Connect(cfg.Database)
	if err != nil {
		logger.Error(*mode, "db_connect", "cannot connect to DB", reqID, err)
		os.Exit(1)
	}
	defer db.Close()

	logger.Info(*mode, "startup", fmt.Sprintf("Service running in mode: %s", *mode), reqID, map[string]interface{}{
		"port": *port,
	})

	switch *mode {
	case "order-service":
		runOrderService(ctx, cfg, db, *port)
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

func runOrderService(ctx context.Context, cfg *config.Config, db database.Pool, port int) {
	// TODO
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
