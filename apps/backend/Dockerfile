# GetOJobu Go API Dockerfile
# Multi-stage build: build -> runtime
#
# Build stage
FROM golang:1.27.3-alpine AS builder

WORKDIR /src

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the API binary
RUN go build -o /bin/getojobu-api ./cmd/api

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy the built binary from builder
COPY --from=builder /bin/getojobu-api /bin/getojobu-api

EXPOSE 8080

# Entry point
CMD ["/bin/getojobu-api"]
