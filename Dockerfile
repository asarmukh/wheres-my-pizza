FROM golang:1.22 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o wheres-my-pizza ./cmd/main.go

FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/wheres-my-pizza /app/wheres-my-pizza
RUN chmod +x /app/wheres-my-pizza

COPY configs /app/configs

CMD ["/app/wheres-my-pizza"]
