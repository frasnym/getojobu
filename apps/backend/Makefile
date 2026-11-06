# GetOJobu Go API Makefile
#
# Swagger/OpenAPI docs are GENERATED from code annotations via `swag init`.
# Never hand-edit docs/swagger.{json,yaml} or docs/docs.go — regenerate instead.
#
# Usage:
#   make generate-swagger   regenerate OpenAPI docs from swag annotations
#   make build              compile the API binary
#   make test               run all tests
#   make vet                run go vet
#   make fmt                gofmt all Go files
#   make run                run the API server (needs DATABASE_URL)
#   make clean              remove generated docs and binary

SWAG ?= swag
GO   ?= go

.PHONY: all generate-swagger build test vet fmt run clean

all: build

# Regenerate OpenAPI docs (docs/swagger.json, docs/swagger.yaml, docs/docs.go)
# from swag annotations in cmd/api/main.go and internal/deliveries/api/handler/*.go
generate-swagger:
	$(SWAG) init -g cmd/api/main.go -d . -o docs

# Compile the API binary
build:
	$(GO) build ./...

# Run all tests
test:
	$(GO) test ./...

# Run go vet
vet:
	$(GO) vet ./...

# Format all Go files
fmt:
	$(GO)fmt -w .

# Run the API server (requires DATABASE_URL or PG_* env vars)
run:
	@if [ -z "$$DATABASE_URL" ] && [ -z "$$PG_HOST" ]; then \
		echo "ERROR: set DATABASE_URL (or PG_HOST/PG_DATABASE/PG_USER/PG_PASSWORD) before running"; \
		exit 1; \
	fi
	$(GO) run ./cmd/api

# Remove generated docs and build cache
clean:
	rm -rf docs
	$(GO) clean -cache
