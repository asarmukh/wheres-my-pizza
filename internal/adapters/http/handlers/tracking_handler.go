package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"wheres-my-pizza/internal/core/services"
)

type TrackingHandler struct {
	service *services.TrackingService
}

func NewTrackingHandler(service *services.TrackingService) *TrackingHandler {
	return &TrackingHandler{service: service}
}

func (h *TrackingHandler) HandlerOrderStatus(w http.ResponseWriter, r *http.Request) {
	orderID, err := strconv.Atoi(r.URL.Path[len("/orders/"):])
	if err != nil {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	order, err := h.service.GetOrderStatus(orderID)
	if err != nil {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(order)
}

func (h *TrackingHandler) HandleWorkerStatus(w http.ResponseWriter, r *http.Request) {
	workerID, err := strconv.Atoi(r.URL.Path[len("/workers"):])
	if err != nil {
		http.Error(w, "Invalid worker ID", http.StatusBadRequest)
		return
	}

	worker, err := h.service.GetWorkerStatus(workerID)
	if err != nil {
		http.Error(w, "Worker not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(worker)
}
