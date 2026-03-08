---
description: Run standard unit tests for all microservices
---
# Run Unit Tests

This workflow executes the standard Go unit tests for validation of changes made to the domain logic or specific microservices.

// turbo-all

1. Run tests for the Pattern Service:
   ```bash
   cd /Users/sergiigagauz/Public/DevelopmentProjects/anomaly_detection_platform/pattern-service && go test -v -race ./...
   ```

2. Run tests for the Metrics Service:
   ```bash
   cd /Users/sergiigagauz/Public/DevelopmentProjects/anomaly_detection_platform/metrics-service && go test -v -race ./...
   ```

3. Run tests for the Read Service:
   ```bash
   cd /Users/sergiigagauz/Public/DevelopmentProjects/anomaly_detection_platform/read-service && go test -v -race ./...
   ```
