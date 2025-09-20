package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"wheres-my-pizza/internal/config"
	"wheres-my-pizza/internal/database"
	"wheres-my-pizza/internal/handler"
	"wheres-my-pizza/internal/service"
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

	trackingService := service.NewTrackingService(pool)
	trackingHandler := handler.NewTrackingHandler(trackingService)

	http.HandleFunc("/orders/", trackingHandler.HandleOrderStatus)
	http.HandleFunc("/workers/", trackingHandler.HandleWorkerStatus)

	log.Printf("Tracking service is running on port %s", cfg.Server.Port)
	log.Fatal(http.ListenAndServe(cfg.Server.Port, nil))
}
