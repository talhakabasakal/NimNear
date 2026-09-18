.PHONY: build run test test-postgres lint migrate migrate-test migrate-down docker-up docker-down clean help seed seed-dev-places seed-production-events

APP_NAME := nimnear-api
BUILD_DIR := bin
MAIN_PATH := ./cmd/server
MIGRATION_DIR := internal/infrastructure/postgres/migrations
# Default local Nimnear DSN. Override with DB_DSN. Never point this at MasterFabric.
DB_DSN ?= postgres://nimnear:nimnear@localhost:5432/nimnear?sslmode=disable

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

## build: Build the application binary
build:
	@echo "Building $(APP_NAME)..."
	go build -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)

## run: Run the application
run:
	@echo "Running $(APP_NAME)..."
	go run $(MAIN_PATH)

## test-postgres: Run isolated PostgreSQL integration tests against nimnear_test
test-postgres:
	@echo "Running isolated PostgreSQL tests..."
	@if [ -z "$(NIMNEAR_TEST_DATABASE_URL)" ]; then \
		echo "NIMNEAR_TEST_DATABASE_URL is required, e.g. postgres://nimnear:nimnear@localhost:5432/nimnear_test?sslmode=disable"; \
		exit 1; \
	fi
	NIMNEAR_TEST_DATABASE_URL="$(NIMNEAR_TEST_DATABASE_URL)" NIMNEAR_TEST_DB_ISOLATED=true go test -count=1 ./internal/infrastructure/postgres/... ./internal/gateway/...

## test: Run all tests
test:
	@echo "Running tests..."
	go test -v -race -count=1 ./...

## test-cover: Run tests with coverage
test-cover:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## lint: Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

## migrate: Apply goose migrations to DB_DSN (local Nimnear by default)
migrate:
	@echo "Running goose migrations against the configured Nimnear DSN..."
	goose -dir $(MIGRATION_DIR) postgres "$(DB_DSN)" up

## migrate-test: Apply goose migrations to the isolated nimnear_test database
migrate-test:
	@if [ -z "$(NIMNEAR_TEST_DATABASE_URL)" ]; then \
		echo "NIMNEAR_TEST_DATABASE_URL is required"; \
		exit 1; \
	fi
	goose -dir $(MIGRATION_DIR) postgres "$(NIMNEAR_TEST_DATABASE_URL)" up

## migrate-down: Rollback last migration
migrate-down:
	@echo "Rolling back last migration..."
	goose -dir $(MIGRATION_DIR) postgres "$(DB_DSN)" down

## migrate-status: Show migration status
migrate-status:
	@echo "Migration status..."
	goose -dir $(MIGRATION_DIR) postgres "$(DB_DSN)" status

## docker-up: Start Docker Compose services
docker-up:
	@echo "Starting Docker services..."
	docker compose -f deployments/docker-compose.yml up -d

## docker-down: Stop Docker Compose services
docker-down:
	@echo "Stopping Docker services..."
	docker compose -f deployments/docker-compose.yml down

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -f deployments/Dockerfile -t $(APP_NAME):latest .

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR) coverage.out coverage.html tmp

## seed: Manually bootstrap RBAC roles and permissions
seed:
	@echo "Bootstrapping RBAC roles and permissions..."
	go run ./scripts/seed.go

## seed-dev-places: Insert fictional development places only (never production)
seed-dev-places:
	@echo "Seeding fictional development places..."
	go run ./cmd/seed-dev-places

## seed-production-events: Idempotent production demo events via EventUseCase.Create
seed-production-events:
	@echo "Seeding production demo events..."
	go run ./cmd/seed-production-events $(ARGS)
