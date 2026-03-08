package anomaly_detected

// ATG Update

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ATG Update: Integration test for the Projector.
// It requires a running Postgres instance (POSTGRES_DSN env var) to execute.
// If the env var is not set, the test is skipped (safe for CI without DB).
func TestProjector_Integration(t *testing.T) {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Skip("Skipping integration test: POSTGRES_DSN not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("Failed to connect to DB: %v", err)
	}
	defer pool.Close()

	// Ensure DB is reachable
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("DB not reachable: %v", err)
	}

	// Create projector
	projector := NewProjector(pool)

	// Create a unique input with random UUIDs to avoid collisions with previous runs.
	eventID := uuid.New().String()
	dedupKey := "test-dedup-" + eventID

	input := Input{
		DeduplicateKey: dedupKey,
		Subject:        "anomalies.detected",
		StreamSeq:      100,

		Source:     "integration_test",
		MetricName: "cpu_usage",
		Env:        "test",
		Region:     "us-east-1",
		InstanceID: "i-1234567890abcdef0",

		Timestamp: time.Now().Unix(),
		Value:     95.5,
		Baseline:  50.0,
		MAD:       10.0,
		Threshold: 3.0,

		WindowSize: 60,
		ThresholdK: 3.0,
		Detector:   "mad",

		EventID:      eventID,
		EventType:    "anomaly.detected",
		EventVersion: 1,
		Producer:     "pattern-service",
		OccurredAt:   time.Now().Format(time.RFC3339),
		ProducedAt:   time.Now().Format(time.RFC3339),
		TraceID:      uuid.New().String(),
	}

	// Test 1: First Projection (Success)
	// We expect the first event with a unique DedupKey to be inserted successfully.
	if err := projector.Project(ctx, input); err != nil {
		t.Fatalf("First projection failed: %v", err)
	}

	// Test 2: Idempotency (Second Projection with same key)
	// Should succeed (return nil) but validation logic inside Projector handles "no rows" as success.
	// This simulates a redelivery from the message queue.
	if err := projector.Project(ctx, input); err != nil {
		t.Fatalf("Idempotency check failed: %v", err)
	}
}
