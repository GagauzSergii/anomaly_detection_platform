# Build stage
FROM golang:1.24.11-alpine AS builder

WORKDIR /app

# Dependencies cashing
COPY go.mod ./
RUN go mod download

# Code copy
COPY . .

# binary buils
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o pattern-service ./cmd/pattern-service

# Runtime stage
FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /app

# image without compiler
COPY --from=builder /app/pattern-service /usr/local/bin/pattern-service

ENV PATTERN_SERVICE_PORT=8082

EXPOSE 8082

CMD ["pattern-service"]
