package demo

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"wheres-my-pizza/internal/db"
	"wheres-my-pizza/internal/logger"
	ordermq "wheres-my-pizza/internal/order/adapter/mq"
	orderrepo "wheres-my-pizza/internal/order/adapter/repo"
	orderapp "wheres-my-pizza/internal/order/app"
	"wheres-my-pizza/internal/order/domain"
	"wheres-my-pizza/internal/rabbitmq"
	trackrepo "wheres-my-pizza/internal/tracking/adapter/repo"
	trackapp "wheres-my-pizza/internal/tracking/app"
)

//go:embed web/*
var webFS embed.FS

type sseHub struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

func newHub() *sseHub { return &sseHub{subs: make(map[chan []byte]struct{})} }
func (h *sseHub) subscribe() chan []byte {
	ch := make(chan []byte, 32)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}
func (h *sseHub) unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.subs, ch)
	close(ch)
	h.mu.Unlock()
}
func (h *sseHub) broadcast(msg []byte) {
	h.mu.Lock()
	for ch := range h.subs {
		select {
		case ch <- msg:
		default:
		}
	}
	h.mu.Unlock()
}

type Server struct {
	mux *http.ServeMux
}

func New(ctx context.Context, log *logger.Logger, pool *db.Pool, amqpClient *rabbitmq.Client, maxConcurrent int) (http.Handler, error) {
	mux := http.NewServeMux()
	// Static UI
	mux.Handle("/", http.FileServerFS(webFS))

	// App services
	oRepo := orderrepo.NewPostgres(pool)
	oPub := ordermq.NewPublisher(amqpClient)
	oApp := orderapp.NewService(oRepo, oPub, log)

	tRepo := trackrepo.New(pool)
	tApp := trackapp.New(tRepo, 60)

	// Simple API using app directly
	mux.HandleFunc("/api/orders", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req domain.OrderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", 400)
			return
		}
		resp, err := oApp.Create(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/api/orders/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/orders/"), "/")
		if len(parts) < 2 {
			http.NotFound(w, r)
			return
		}
		on := parts[0]
		switch parts[1] {
		case "status":
			resp, err := tApp.Status(r.Context(), on)
			if err != nil {
				http.Error(w, http.StatusText(404), 404)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		case "history":
			resp, err := tApp.History(r.Context(), on)
			if err != nil {
				http.Error(w, http.StatusText(500), 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.NotFound(w, r)
		}
	})

	mux.HandleFunc("/api/workers/status", func(w http.ResponseWriter, r *http.Request) {
		resp, err := tApp.Workers(r.Context())
		if err != nil {
			http.Error(w, http.StatusText(500), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// SSE stream of notifications
	hub := newHub()
	mux.HandleFunc("/api/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "stream unsupported", 500)
			return
		}
		ch := hub.subscribe()
		defer hub.unsubscribe(ch)
		// initial ping
		fmt.Fprintf(w, ":ok\n\n")
		flusher.Flush()
		for {
			select {
			case <-r.Context().Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", msg)
				flusher.Flush()
			}
		}
	})

	// background consumer
	go func() {
		for {
			deliveries, err := amqpClient.ConsumeNotifications(ctx)
			if err != nil {
				return
			}
			for m := range deliveries {
				hub.broadcast(m.Body)
				_ = m.Ack(false)
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()

	return &Server{mux: mux}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }
