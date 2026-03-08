package anomaly_detected

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sergii-gagauz/anomaly_detection_platform/read-service/internal/shared/postgres/db"
)

// Projector persists anomaly.detected events into read models.
// Responsibilities:
// - idempotency (processed_events)
// - append anomaly event (anomaly_events)
// - upsert current series state (series_state)
type Projector struct {
	pool *pgxpool.Pool
	db   *db.Queries
}

func NewProjector(pool *pgxpool.Pool) *Projector {
	return &Projector{
		pool: pool,
		db:   db.New(pool),
	}
}

// Input is a stable internal representation of detected anomaly payload.
type Input struct {
	DeduplicateKey string
	Subject        string
	StreamSeq      int64

	Source, MetricName, Env, Region, InstanceID string
	Timestamp                                   int64
	Value, Baseline, MAD, Threshold             float64
	WindowSize                                  int32
	ThresholdK                                  float64
	Detector                                    string

	EventID, EventType, Producer, OccurredAt, ProducedAt, TraceID string
	EventVersion                                                  int32
}

// Project persists the detected anomaly into the read-side database.
// It executes the following steps in a single transaction:
// 1. Idempotency check: ensures the same event is not processed twice.
// 2. Insert Anomaly Event: stores the immutable event record.
// 3. Upsert Series State: updates the mutable current state of the time series.
func (projector *Projector) Project(ctx context.Context, in Input) error {
	tx, err := projector.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := projector.db.WithTx(tx)

	// Idempotency: insert dedup key once.
	// If it already exists, we consider the message processed and do nothing.
	_, err = qtx.TryMarkProcessed(ctx, db.TryMarkProcessedParams{
		DedupKey:  in.DeduplicateKey,
		Subject:   in.Subject,
		StreamSeq: in.StreamSeq,
	})
	if err != nil {
		// sqlc returns error on no rows if ON CONFLICT happened (since RETURNING).
		// We intentionally treat "no rows" as "already processed".
		if isNoRows(err) {
			slog.InfoContext(ctx, "Anomaly event already processed (idempotent skip)",
				slog.String("dedup_key", in.DeduplicateKey),
			)
			return tx.Commit(ctx)
		}
		return fmt.Errorf("idempotency: %w", err)
	}

	// Insert the historical record of the anomaly.
	if err := qtx.InsertAnomalyEvent(ctx, db.InsertAnomalyEventParams{
		Source:     in.Source,
		MetricName: in.MetricName,
		Env:        in.Env,
		Region:     in.Region,
		InstanceID: in.InstanceID,

		Ts:    in.Timestamp,
		Value: in.Value,

		Baseline:  in.Baseline,
		Mad:       in.MAD,
		Threshold: in.Threshold,

		WindowSize:   in.WindowSize,
		ThresholdK:   in.ThresholdK,
		Detector:     in.Detector,
		Producer:     in.Producer,
		TraceID:      in.TraceID,
		EventID:      in.EventID,
		EventType:    in.EventType,
		EventVersion: in.EventVersion,
		OccurredAt:   in.OccurredAt,
		ProducedAt:   in.ProducedAt,
	}); err != nil {
		return fmt.Errorf("insert anomaly event: %w", err)
	}

	// Update the latest known state for this series (source/metric/env/region).
	// This supports the "GetSeriesState" view.
	if err := qtx.UpsertSeriesState(ctx, db.UpsertSeriesStateParams{
		Source:     in.Source,
		MetricName: in.MetricName,
		Env:        in.Env,
		Region:     in.Region,

		LastTs:    in.Timestamp,
		LastValue: in.Value,

		Baseline:  in.Baseline,
		Mad:       in.MAD,
		Threshold: in.Threshold,

		WindowSize:   in.WindowSize,
		ThresholdK:   in.ThresholdK,
		Detector:     in.Detector,
		Producer:     in.Producer,
		TraceID:      in.TraceID,
		EventID:      in.EventID,
		EventVersion: in.EventVersion,
		OccurredAt:   in.OccurredAt,
		ProducedAt:   in.ProducedAt,
	}); err != nil {
		return fmt.Errorf("upsert series state: %w", err)
	}

	return tx.Commit(ctx)
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
