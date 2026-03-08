.PHONY: proto

PROTO_DIR  := api/proto
GEN_DIR    := api/gen

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
