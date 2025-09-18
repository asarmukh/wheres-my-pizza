package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"time"
)

type M map[string]any

type Logger struct {
	service  string
	hostname string
}

func New(service, hostname string) *Logger {
	return &Logger{service: service, hostname: hostname}
}

type logEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Service   string `json:"service"`
	Hostname  string `json:"hostname"`
	RequestID string `json:"request_id"`
	Action    string `json:"action"`
	Message   string `json:"message"`
	Details   any    `json:"details,omitempty"`
	Error     *struct {
		Msg   string `json:"msg"`
		Stack string `json:"stack"`
	} `json:"error,omitempty"`
}

type ctxKey string

const requestIDKey ctxKey = "request_id"

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestIDFromContext(ctx context.Context) string {
	v := ctx.Value(requestIDKey)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func (l *Logger) write(le logEntry) {
	b, _ := json.Marshal(le)
	fmt.Fprintln(os.Stdout, string(b))
}

func (l *Logger) Info(ctx context.Context, action, message string, details any) {
	l.write(logEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     "INFO",
		Service:   l.service,
		Hostname:  l.hostname,
		RequestID: RequestIDFromContext(ctx),
		Action:    action,
		Message:   message,
		Details:   details,
	})
}

func (l *Logger) Debug(ctx context.Context, action, message string, details any) {
	l.write(logEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     "DEBUG",
		Service:   l.service,
		Hostname:  l.hostname,
		RequestID: RequestIDFromContext(ctx),
		Action:    action,
		Message:   message,
		Details:   details,
	})
}

func (l *Logger) Error(ctx context.Context, action, message string, err error) {
	l.write(logEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     "ERROR",
		Service:   l.service,
		Hostname:  l.hostname,
		RequestID: RequestIDFromContext(ctx),
		Action:    action,
		Message:   message,
		Error: &struct {
			Msg   string `json:"msg"`
			Stack string `json:"stack"`
		}{Msg: err.Error(), Stack: string(debug.Stack())},
	})
}
