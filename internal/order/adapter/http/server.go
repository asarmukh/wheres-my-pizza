package httpadapter

import (
	"encoding/json"
	"net/http"

	"wheres-my-pizza/internal/logger"
	orderapp "wheres-my-pizza/internal/order/app"
	"wheres-my-pizza/internal/order/domain"
)

type Server struct {
	app *orderapp.Service
	log *logger.Logger
	sem chan struct{}
}

func NewServer(app *orderapp.Service, log *logger.Logger, maxConcurrent int) http.Handler {
	if maxConcurrent <= 0 {
		maxConcurrent = 50
	}
	s := &Server{app: app, log: log, sem: make(chan struct{}, maxConcurrent)}
	mux := http.NewServeMux()
	mux.HandleFunc("/orders", s.createOrder)
	return mux
}

func (s *Server) createOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.sem <- struct{}{}
	defer func() { <-s.sem }()
	var req domain.OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	resp, err := s.app.Create(r.Context(), req)
	if err != nil {
		s.log.Error(r.Context(), "validation_failed", "Create failed", err)
		http.Error(w, err.Error(), 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
