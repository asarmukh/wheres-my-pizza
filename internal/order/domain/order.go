package domain

import (
	"errors"
	"regexp"
	"strings"
)

var reCustomer = regexp.MustCompile(`^[A-Za-z\s\-']{1,100}$`)

type Item struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

type OrderRequest struct {
	CustomerName    string  `json:"customer_name"`
	OrderType       string  `json:"order_type"`
	TableNumber     *int    `json:"table_number"`
	DeliveryAddress *string `json:"delivery_address"`
	Items           []Item  `json:"items"`
}

func (r OrderRequest) Validate() error {
	if !reCustomer.MatchString(r.CustomerName) {
		return errors.New("invalid customer_name")
	}
	switch r.OrderType {
	case "dine_in", "takeout", "delivery":
	default:
		return errors.New("invalid order_type")
	}
	if len(r.Items) < 1 || len(r.Items) > 20 {
		return errors.New("items must contain 1-20 elements")
	}
	for _, it := range r.Items {
		if len(strings.TrimSpace(it.Name)) < 1 || len(it.Name) > 50 {
			return errors.New("invalid item.name")
		}
		if it.Quantity < 1 || it.Quantity > 10 {
			return errors.New("invalid item.quantity")
		}
		if it.Price < 0.01 || it.Price > 999.99 {
			return errors.New("invalid item.price")
		}
	}
	if r.OrderType == "dine_in" {
		if r.TableNumber == nil || *r.TableNumber < 1 || *r.TableNumber > 100 {
			return errors.New("table_number required 1-100 for dine_in")
		}
		if r.DeliveryAddress != nil {
			return errors.New("delivery_address must not be present for dine_in")
		}
	}
	if r.OrderType == "delivery" {
		if r.DeliveryAddress == nil || len(strings.TrimSpace(*r.DeliveryAddress)) < 10 {
			return errors.New("delivery_address required with min 10 chars for delivery")
		}
		if r.TableNumber != nil {
			return errors.New("table_number must not be present for delivery")
		}
	}
	return nil
}

func (r OrderRequest) TotalAmount() float64 {
	total := 0.0
	for _, it := range r.Items {
		total += float64(it.Quantity) * it.Price
	}
	return total
}

func PriorityForTotal(total float64) int {
	if total > 100.0 {
		return 10
	}
	if total >= 50.0 {
		return 5
	}
	return 1
}
