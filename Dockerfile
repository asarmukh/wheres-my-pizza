FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o wheres-my-pizza ./cmd/main.go

FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache ca-certificates

# Copy the binary
COPY --from=builder /app/wheres-my-pizza /app/wheres-my-pizza

# Copy configs directory (this will be overridden by volume mount, but good as fallback)
COPY --from=builder /app/configs /app/configs

# Make binary executable
RUN chmod +x /app/wheres-my-pizza

# Set default environment variable
ENV CONFIG_PATH=/app/configs/config.yaml

CMD ["/app/wheres-my-pizza"]