.PHONY: proto

PROTO_DIR  := api/proto
GEN_DIR    := api/gen

help:
	@echo "Usage:"
	@echo "  make proto   - Compile all .proto files into Go code"
	@echo "  make swagger - Generate Swagger documentation for read-service"
	@echo "  make build   - Build all services (depends on swagger)"

build: swagger
	@echo "==> Building read-service..."
	@$(MAKE) -C read-service build

# Compile all .proto files into Go code (requires protoc, protoc-gen-go, protoc-gen-go-grpc).
proto:
	@echo "==> Generating Go code from .proto files..."
	@mkdir -p $(GEN_DIR)
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(GEN_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/metrics/v1/metrics.proto
	@echo "==> Done. Generated files in $(GEN_DIR)/metrics/v1/"

# Generate Swagger documentation for read-service
swagger:
	@echo "==> Generating Swagger docs for read-service..."
	@if ! command -v swag > /dev/null; then \
		echo "swag not found. Please install it by running:"; \
		echo "go install github.com/swaggo/swag/cmd/swag@latest"; \
		exit 1; \
	fi
	cd read-service && swag init -g cmd/read-service/main.go -o docs/
