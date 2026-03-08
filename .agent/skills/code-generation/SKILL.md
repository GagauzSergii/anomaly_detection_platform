---
name: Code Generation (sqlc)
description: Generate Go code from SQL queries using sqlc
---
# Code Generation (sqlc)

## Context
This skill regenerates the Go data access layer in the `read-service` whenever the SQL queries or the schema definition changes.

## Guidelines
1. Navigate to the `read-service` directory (assuming execution from project root).
   ```bash
   cd read-service
   ```

// turbo
2. Execute the `sqlc generate` command (or the `generate` Make target) to regenerate code in the internal db package.
   ```bash
   make generate
   ```

3. Verify there are no compilation issues after generating the new files.
   ```bash
   go build ./...
   ```

## Verification
- Code successfully generates and compiles.
