---
name: API Documentation (Swagger)
description: Generate Swagger OpenAPI documentation for the read-service
---
# API Documentation (Swagger)

## Context
This skill regenerates the standard OpenAPI 3.0 documentation for the `read-service` API to ensure API discoverability is kept up to date with code changes.

## Guidelines
1. Ensure you are in the project root directory.

// turbo
2. Run the `make swagger` or `make build` command to parse the Go annotations, generate the `docs/` repository, and compile.
   ```bash
   make swagger
   ```

3. To view the generated swagger UI, start the service using `make run` inside the `read-service/` or via docker compose, and access `http://localhost:8080/swagger/index.html`.

## Verification
- Command outputs `Generate swagger docs....` without errors.
- Validate that the UI reflects the recent changes at `/swagger/index.html`.
