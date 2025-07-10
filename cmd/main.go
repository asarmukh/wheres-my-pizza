package cmd

import (
	"fmt"
	"wheresmypizza/internal/logger"
)

func main() {
	reqID := logger.NewRequestID()

	logger.Info("order-service", "service-started", "Order service started on port 3000", reqID, map[string]any{
		"port": 3000,
		"env":  "dev",
	})

	logger.Error("order-service", "db_connection_failed", "Cannot connect to PostgreSQL", reqID, fmt.Errorf("timeout after 5s"))
}
