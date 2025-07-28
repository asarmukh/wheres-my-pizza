package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"wheres-my-pizza/internal/core/domain"
	"wheres-my-pizza/internal/core/ports"
	"wheres-my-pizza/internal/helper"
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

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&order); err != nil {
		var syntaxErr *json.SyntaxError
		var unmarshalTypeErr *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxErr):
			helper.ResponswWithError(w, http.StatusBadRequest, fmt.Sprintf("Request body contains badly-formed JSON at position %d", syntaxErr.Offset))
		case errors.As(err, &unmarshalTypeErr):
			field := unmarshalTypeErr.Field
			helper.ResponswWithError(w, http.StatusBadRequest, fmt.Sprintf("Field '%s' expects value of type %s", field, unmarshalTypeErr.Type.String()))
		default:
			helper.ResponswWithError(w, http.StatusBadRequest, "Invalid request")
		}
		return
	}

	if err := h.service.CreateOrder(r.Context(), &order); err != nil {
		helper.ResponswWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	fmt.Println("status", order.Status)

	resp := domain.OrderResponse{
		OrderNumber: order.Number,
		Status:      order.Status,
		TotalAmount: order.TotalAmount,
	}

	helper.ResponseInJSON(w, http.StatusOK, resp)
}
