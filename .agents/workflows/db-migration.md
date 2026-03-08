---
description: Apply database migrations for read-service using goose
---
# Database Migrations

This workflow is used when updating database schemas for the `read-service`. The service uses `goose` for handling migrations against a PostgreSQL database.

1. Make sure the local database is running (usually via docker-compose).
   ```bash
   docker compose up -d postgres
   ```

2. Navigate to the `read-service` directory.
   ```bash
   cd /Users/sergiigagauz/Public/DevelopmentProjects/anomaly_detection_platform/read-service
   ```

3. Export the standard local Database URL (adjust if the password/db changes).
   ```bash
   export POSTGRES_DSN="postgres://postgres:postgres@localhost:5432/read?sslmode=disable"
   ```

// turbo
4. Run the 'migrate-up' make target to apply migrations natively, or execute goose directly.
   ```bash
   make migrate-up
   ```

5. When creating a new migration, use:
   ```bash
   go run github.com/pressly/goose/v3/cmd/goose@latest -dir db/migrations create <migration_name> sql
   ```
