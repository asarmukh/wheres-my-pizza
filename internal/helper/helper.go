package helper

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"wheres-my-pizza/internal/core/domain"
)

var customerNameRegexp = regexp.MustCompile(`^[a-zA-Z\s'-]{1,100}$`)

func ValidateOrder(order *domain.Order) error {
	if strings.TrimSpace(order.CustomerName) == "" {
		return errors.New("customer name is required")
	}

	if !customerNameRegexp.MatchString(order.CustomerName) {
		return errors.New("customer name must be 1-100 characters and contain only letters, spaces, hyphens, or apostrophes")
	}

	if order.Type != "dine-in" && order.Type != "delivery" && order.Type != "takeout" {
		return errors.New("order type must be either 'dine-in', 'delivery' or `takeout`")
	}

	switch order.Type {
	case "dine-in":
		if order.TableNumber == nil || *order.TableNumber < 1 || *order.TableNumber > 100 {
			return errors.New("table number is required for dine-in and must be between 1 and 100")
		}
		if order.DeliveryAddress != nil && strings.TrimSpace(*order.DeliveryAddress) != "" {
			return errors.New("delivery address must not be present for dine-in")
		}
	case "delivery":
		if order.DeliveryAddress == nil || len(strings.TrimSpace(*order.DeliveryAddress)) < 10 {
			return errors.New("delivery address is required for delivery and must be at leat 10 characters")
		}
		if order.TableNumber != nil {
			return errors.New("table number must not be present for delivery")
		}
	}

	if len(order.Items) == 0 || len(order.Items) > 20 {
		return errors.New("order must containt between 1 and 20 items")
	}

	for i, item := range order.Items {
		if strings.TrimSpace(item.Name) == "" || len(item.Name) > 20 {
			return fmt.Errorf("item %d name must be 1-20 characters", i+1)
		}
		if item.Quantity < 1 || item.Quantity > 10 {
			return fmt.Errorf("item %d quantity must be between 1 and 10", i+1)
		}
		if item.Price < 0.01 || item.Price > 999.99 {
			return fmt.Errorf("item %d price must be between 0.01 and 999.99", i+1)
		}
	}

	return nil
}

func ResponseInJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, `{Error: failed to encode JSON}`, http.StatusInternalServerError)
	}
}

func ResponswWithError(w http.ResponseWriter, statusCode int, message string) {
	ResponseInJSON(w, statusCode, map[string]string{"Error": message})
}
