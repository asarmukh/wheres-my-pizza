package httpadapter

import (
	"encoding/json"
	"net/http"
	"strings"

	"wheres-my-pizza/internal/logger"
	trackapp "wheres-my-pizza/internal/tracking/app"
)

type Server struct {
	app *trackapp.Service
	log *logger.Logger
}

func New(app *trackapp.Service, log *logger.Logger) http.Handler {
	s := &Server{app: app, log: log}
	mux := http.NewServeMux()
	mux.HandleFunc("/orders/", s.handleOrders)
	mux.HandleFunc("/workers/status", s.handleWorkers)
	return mux
}

func (s *Server) handleOrders(w http.ResponseWriter, r *http.Request) {
	s.log.Debug(r.Context(), "request_received", "tracking request", nil)
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/orders/"), "/")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}
	on := parts[0]
	switch parts[1] {
	case "status":
		resp, err := s.app.Status(r.Context(), on)
		if err != nil {
			http.Error(w, http.StatusText(404), 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	case "history":
		resp, err := s.app.History(r.Context(), on)
		if err != nil {
			http.Error(w, http.StatusText(500), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleWorkers(w http.ResponseWriter, r *http.Request) {
	resp, err := s.app.Workers(r.Context())
	if err != nil {
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
