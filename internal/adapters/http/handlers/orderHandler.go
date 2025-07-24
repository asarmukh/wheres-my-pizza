package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"wheres-my-pizza/internal/core/domain"
	"wheres-my-pizza/internal/core/ports"
)

type OrderHandler struct {
	service ports.OrderService
}

func NewOrderHandler(service ports.OrderService) *OrderHandler {
	return &OrderHandler{service: service}
}

func (h *OrderHandler) GetOrderByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid order ID", http.StatusBadRequest)
		return
	}

	order, err := h.service.GetOrderByID(r.Context(), id)
	if err != nil {
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var order domain.Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if err := h.service.CreateOrder(r.Context(), &order); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
}
