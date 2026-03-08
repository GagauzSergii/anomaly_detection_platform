---
name: Start Local Infrastructure
description: Start local infrastructure and services via Docker Compose
---
# Start Local Infrastructure

## Context
This skill is used to spin up the entire application stack or just the infrastructure dependencies (like NATS and PostgreSQL) locally for development and testing.

## Guidelines
1. Ensure you are in the project root directory where the `docker-compose.yml` file is located.

// turbo
2. Start the core infrastructure and services in detached mode.
   ```bash
   docker compose up -d
   ```

3. To view logs for a specific service (e.g., `read-service` or `nats`), use:
   ```bash
   docker compose logs -f <service_name>
   ```

4. To tear down the infrastructure and clean up containers when finished:
   ```bash
   docker compose down
   ```

## Verification
- Use `docker compose ps` to verify all required containers are running and healthy.
