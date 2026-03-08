---
name: Run Unit Tests
description: Run standard unit tests for all microservices
---
# Run Unit Tests

## Context
This skill executes the standard Go unit tests for validation of changes made to the domain logic or specific microservices.

## Guidelines
// turbo-all

1. Run tests for the Pattern Service:
   ```bash
   cd pattern-service && go test -v -race ./...
   ```

2. Run tests for the Metrics Service:
   ```bash
   cd metrics-service && go test -v -race ./...
   ```

3. Run tests for the Read Service:
   ```bash
   cd read-service && go test -v -race ./...
   ```

## Verification
- All test suites should output `PASS`.
