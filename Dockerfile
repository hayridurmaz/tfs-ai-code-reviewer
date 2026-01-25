# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache build-base

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o build/reviewer ./cmd/reviewer/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Install CA certificates for HTTPS
RUN apk add --no-cache ca-certificates

COPY --from=builder /app/build/reviewer .
# No .env copy here, it should be mounted or provided via env vars

CMD ["./reviewer"]
