---
description: Generate Go code from SQL queries using sqlc
---
# Code Generation (sqlc)

This workflow regenerates the Go data access layer in the `read-service` whenever the SQL queries or the schema definition changes.

1. Navigate to the `read-service` directory.
   ```bash
   cd /Users/sergiigagauz/Public/DevelopmentProjects/anomaly_detection_platform/read-service
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
