package services

import (
	"wheres-my-pizza/internal/core/domain"
	"wheres-my-pizza/internal/core/ports"
)

type TrackingService struct {
	trackingRepo ports.TrackingService
}

func NewTrackingService(trackingRepo ports.TrackingRepo) *TrackingService {
	return &TrackingService{
		trackingRepo: trackingRepo,
	}
}

func (s *TrackingService) GetOrderStatus(orderID int) (*domain.Order, error) {
	order, err := s.trackingRepo.GetOrderStatus(orderID)
	if err != nil {
		return nil, err
	}
	return order, nil
}

func (s *TrackingService) GetWorkerStatus(workerID int) (*domain.Worker, error) {
	worker, err := s.trackingRepo.GetWorkerStatus(workerID)
	if err != nil {
		return nil, err
	}
	return worker, nil
}
