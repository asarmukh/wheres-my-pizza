package app

import (
	"context"
	"encoding/json"
	"strconv"

	"wheres-my-pizza/internal/logger"
	"wheres-my-pizza/internal/order/domain"
)

type OrderRepository interface {
	CreateOrder(ctx context.Context, req domain.OrderRequest, total float64, priority int) (orderNumber string, orderID int64, err error)
}

type OrderPublisher interface {
	Publish(ctx context.Context, routingKey string, priority uint8, payload []byte) error
}

type Service struct {
	repo OrderRepository
	pub  OrderPublisher
	log  *logger.Logger
}

func NewService(repo OrderRepository, pub OrderPublisher, log *logger.Logger) *Service {
	return &Service{repo: repo, pub: pub, log: log}
}

type CreateResponse struct {
	OrderNumber string  `json:"order_number"`
	Status      string  `json:"status"`
	TotalAmount float64 `json:"total_amount"`
}

func (s *Service) Create(ctx context.Context, req domain.OrderRequest) (CreateResponse, error) {
	if err := req.Validate(); err != nil {
		return CreateResponse{}, err
	}
	total := req.TotalAmount()
	priority := domain.PriorityForTotal(total)

	orderNumber, orderID, err := s.repo.CreateOrder(ctx, req, total, priority)
	if err != nil {
		return CreateResponse{}, err
	}
	s.log.Debug(ctx, "order_received", "Order created", logger.M{"order_number": orderNumber, "order_id": orderID})

	// publish
	payload := map[string]any{
		"order_number":     orderNumber,
		"customer_name":    req.CustomerName,
		"order_type":       req.OrderType,
		"table_number":     req.TableNumber,
		"delivery_address": req.DeliveryAddress,
		"items":            req.Items,
		"total_amount":     round2(total),
		"priority":         priority,
	}
	body, _ := json.Marshal(payload)
	rk := "kitchen." + req.OrderType + "." + strconv.Itoa(priority)
	if err := s.pub.Publish(ctx, rk, uint8(priority), body); err != nil {
		return CreateResponse{}, err
	}
	s.log.Debug(ctx, "order_published", "Order published", logger.M{"routing_key": rk})
	return CreateResponse{OrderNumber: orderNumber, Status: "received", TotalAmount: round2(total)}, nil
}

func round2(v float64) float64 { return float64(int(v*100+0.5)) / 100 }
