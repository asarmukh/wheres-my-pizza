package config

import (
	"net/http"
	"wheres-my-pizza/internal/adapters/http/handlers"
)

func RegisterRoutes(mux *http.ServeMux, orderHandler *handlers.OrderHandler) {
	mux.HandleFunc("/orders", orderHandler.CreateOrder)
	mux.HandleFunc("GET /orders/{number}", orderHandler.GetOrderByNumber)
}
