# Build stage
FROM golang:1.24.11-alpine AS builder

WORKDIR /app

# Copy shared api module first (needed for replace directive)
COPY api/ /api/

# Copy service source
COPY metrics-service/ ./

# Dependencies caching & build
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o metrics-service ./cmd/metrics-service

# Runtime stage
FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /app

# image without compiler
COPY --from=builder /app/metrics-service /usr/local/bin/metrics-service

ENV METRICS_SERVICE_PORT=8081

EXPOSE 8081 50051

CMD ["metrics-service"]
