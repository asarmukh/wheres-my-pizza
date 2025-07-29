package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
)

type LogLevel string

const (
	LevelInfo  LogLevel = "INFO"
	LevelDebug LogLevel = "DEBUG"
	LevelError LogLevel = "ERROR"
)

type LogEntry struct {
	Timestamp  string      `yaml:"timestamp"`
	Level      LogLevel    `yaml:"level"`
	Service    string      `yaml:"service"`
	Action     string      `yaml:"action"`
	Message    string      `yaml:"message"`
	Hostname   string      `yaml:"hostname"`
	RequestID  string      `yaml:"request_id"`
	DurationMs int64       `json:"duration_ms,omitempty"`
	Error      *ErrorEntry `yaml:"error,omitempty"`
	Details    interface{} `yaml:"details,omitempty"`
}

type ErrorEntry struct {
	Msg   string `yaml:"msg"`
	Stack string `yaml:"stack"`
}

func log(level LogLevel, service, action, message, requestID string, err error, details interface{}, durationMs int64) {
	hostName, _ := os.Hostname()

	entry := LogEntry{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Level:      level,
		Service:    service,
		Action:     action,
		Message:    message,
		Hostname:   hostName,
		RequestID:  requestID,
		Details:    details,
		DurationMs: durationMs,
	}

	if err != nil {
		entry.Error = &ErrorEntry{
			Msg:   err.Error(),
			Stack: fmt.Sprintf("%+v", err),
		}
	}

	bytes, _ := json.Marshal(entry)
	fmt.Println(string(bytes))
}

func Info(service, action, message string, requestID string, details interface{}, durationMs int64) {
	log(LevelInfo, service, action, message, requestID, nil, details, durationMs)
}

func Debug(service, action, message string, requestID string, details interface{}, durationMs int64) {
	log(LevelDebug, service, action, message, requestID, nil, details, durationMs)
}

func Error(service, action, message string, requestID string, err error) {
	log(LevelError, service, action, message, requestID, err, nil, 0)
}

func NewRequestID() string {
	return uuid.New().String()
}
