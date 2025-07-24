package helper

import (
	"errors"
	"fmt"
	"strings"
	"wheres-my-pizza/internal/core/domain"
)

func ValidateOrder(order *domain.Order) error {
	if strings.TrimSpace(order.CustomerName) == "" {
		return errors.New("customer name is required")
	}

	if order.Type != "dine-in" && order.Type != "delivery" {
		return errors.New("order type must be either 'dine-in' or 'delivery'")
	}

	if order.Type == "dine-in" && order.TableNumber == nil {
		return errors.New("table number is required for dine-in orders")
	}

	if order.Type == "delivery" && (order.DeliveryAddress == nil || strings.TrimSpace(*order.DeliveryAddress) == "") {
		return errors.New("delivery address is required for delivery orders")
	}

	if order.TotalAmount < 0 {
		return errors.New("total amount cannot be negative")
	}

	if order.Priority < 0 || order.Priority > 5 {
		return errors.New("priority must be between 0 and 5")
	}

	if len(order.Items) == 0 {
		return errors.New("order must containt at leat one item")
	}

	calculatedTotal := 0.0
	for i, item := range order.Items {
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("item %d name is required", i+1)
		}
		if item.Quantity <= 0 {
			return fmt.Errorf("item %d quantity must be at least 1", i+1)
		}
		if item.Price < 0 {
			return fmt.Errorf("item %d price cannot be negative", i+1)
		}
		calculatedTotal += float64(item.Quantity) * item.Price
	}

	if order.TotalAmount != calculatedTotal {
		return fmt.Errorf("total amount mismatch: expected %.2f but got %.2f", calculatedTotal, order.TotalAmount)
	}

	return nil
}
