FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy shared api module first (needed for replace directive)
COPY api/ /api/

# Copy service source
COPY metrics-feeder/ ./

# Dependencies caching & build
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o feeder ./cmd/feeder

# Runtime stage
FROM gcr.io/distroless/base-debian12:nonroot

COPY --from=builder /app/feeder /usr/local/bin/feeder

ENTRYPOINT ["feeder"]
