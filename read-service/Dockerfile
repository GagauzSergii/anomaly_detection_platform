FROM golang:1.24 AS build
WORKDIR /app

# Code copy
COPY . .

# binary buils
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/read-service ./cmd/read-service

# Runtime stage
FROM gcr.io/distroless/base-debian12

# image without compiler
COPY --from=build /bin/read-service /read-service

EXPOSE 8083


ENTRYPOINT ["/read-service"]
