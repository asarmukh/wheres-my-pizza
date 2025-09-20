package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"wheres-my-pizza/internal/adapters/http/handlers"
	"wheres-my-pizza/internal/adapters/storage/postgres"
	"wheres-my-pizza/internal/config"
	"wheres-my-pizza/internal/core/services"
	"wheres-my-pizza/internal/database"
)

func main() {
	cfgPath := os.Getenv("CONFIG_PATH") // пробуем взять из env
	// if cfgPath == "" {
	// 	cfgPath = *cfgPathFlag // если не задано, берём из флага
	// }
	if cfgPath == "" {
		fmt.Fprintln(os.Stderr, "CONFIG_PATH or --config is required")
		os.Exit(2)
	}

	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("unable to load config: %v", err)
	}

	pool, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}

	trackingRepo := postgres.NewTrackingRepo(pool)
	trackingService := services.NewTrackingService(trackingRepo)
	trackingHandler := handlers.NewTrackingHandler(trackingService)

	http.HandleFunc("/orders/", trackingHandler.HandlerOrderStatus)
	http.HandleFunc("/workers/", trackingHandler.HandleWorkerStatus)

	log.Printf("Tracking service is running")
	log.Fatal(http.ListenAndServe("", nil))
}
