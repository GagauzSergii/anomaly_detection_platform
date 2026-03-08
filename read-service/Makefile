# Database connection string
POSTGRES_DSN ?= "postgres://user:pass@localhost:5432/db?sslmode=disable"

.PHONY: generate migrate-up run docker-up docker-down build

# 1. Generate Go code from SQL queries using sqlc
generate:
	@echo "==> Generating code from SQL..."
	sqlc generate

# 2. Run the service locally (depends on code generation)
run: generate
	@echo "==> Starting the service..."
	go run ./cmd/read-service

# 3. Apply database migrations using goose
migrate-up:
	@echo "==> Applying migrations..."
	@if [ -z "$(POSTGRES_DSN)" ]; then echo "Error: POSTGRES_DSN is not set"; exit 1; fi
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir db/migrations postgres "$(POSTGRES_DSN)" up

# 4. Generate Swagger documentation
swagger:
	@echo "==> Generating Swagger docs..."
	@if ! command -v swag > /dev/null; then \
		echo "Installing swag..."; \
		go install github.com/swaggo/swag/cmd/swag@latest; \
	fi
	swag init -g cmd/read-service/main.go -o docs/

# 5. Build the service binary
build: generate swagger
	@echo "==> Building binary..."
	go build -o bin/read-service ./cmd/read-service

# 5. Docker management
docker-up:
	@echo "==> Scaling up containers..."
	docker compose up --build

docker-down:
	@echo "==> Cleaning up containers..."
	docker compose down