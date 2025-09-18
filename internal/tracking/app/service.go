package app

import (
	"context"
	"time"
)

type Repo interface {
	GetOrderStatus(ctx context.Context, orderNumber string) (status, processedBy string, updatedAt time.Time, err error)
	GetOrderHistory(ctx context.Context, orderNumber string) ([]HistoryEntry, error)
	GetWorkers(ctx context.Context) ([]WorkerEntry, error)
}

type HistoryEntry struct {
	Status, ChangedBy string
	Timestamp         time.Time
}
type WorkerEntry struct {
	Name, Status    string
	OrdersProcessed int
	LastSeen        time.Time
}

type Service struct {
	repo                Repo
	offlineThresholdSec int
}

func New(repo Repo, offlineThresholdSec int) *Service {
	return &Service{repo: repo, offlineThresholdSec: offlineThresholdSec}
}

func (s *Service) Status(ctx context.Context, orderNumber string) (map[string]any, error) {
	st, by, at, err := s.repo.GetOrderStatus(ctx, orderNumber)
	if err != nil {
		return nil, err
	}
	return map[string]any{"order_number": orderNumber, "current_status": st, "updated_at": at.UTC().Format(time.RFC3339), "processed_by": by}, nil
}

func (s *Service) History(ctx context.Context, orderNumber string) ([]map[string]any, error) {
	h, err := s.repo.GetOrderHistory(ctx, orderNumber)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(h))
	for _, e := range h {
		out = append(out, map[string]any{"status": e.Status, "timestamp": e.Timestamp.UTC().Format(time.RFC3339), "changed_by": e.ChangedBy})
	}
	return out, nil
}

func (s *Service) Workers(ctx context.Context) ([]map[string]any, error) {
	ws, err := s.repo.GetWorkers(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	out := make([]map[string]any, 0, len(ws))
	for _, w := range ws {
		status := w.Status
		if now.Sub(w.LastSeen) > time.Duration(s.offlineThresholdSec)*time.Second {
			status = "offline"
		}
		out = append(out, map[string]any{"worker_name": w.Name, "status": status, "orders_processed": w.OrdersProcessed, "last_seen": w.LastSeen.UTC().Format(time.RFC3339)})
	}
	return out, nil
}
