# GetOJobu Go API Dockerfile
# Multi-stage build: build -> runtime
# Go 1.27.3 baseline as per project toolchain
#
# Build stage
FROM golang:1.27.3-alpine AS builder

WORKDIR /app

# Install build dependencies (git for swag, gcc for CGO)
RUN apk add --no-cache git gcc musl-dev

# Copy go module files first for caching
go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o /getojobu-api ./cmd/api

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /getojobu-api /getojobu-api

# Expose the API port
EXPOSE 8080

# Entry point
CMD ["/getojobu-api"]