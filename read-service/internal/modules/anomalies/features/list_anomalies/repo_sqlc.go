package list_anomalies

import (
	"context"
	"fmt"
	"github.com/sergii-gagauz/anomaly_detection_platform/read-service/internal/shared/postgres/dbutil"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/sergii-gagauz/anomaly_detection_platform/read-service/internal/shared/postgres/db"
)

// Repo is a sqlc-backed repository implementation for anomaly events.
type Repo struct {
	q *db.Queries
}

// NewRepo creates a new instance of the anomaly repository.
func NewRepo(q *db.Queries) *Repo {
	return &Repo{q: q}
}

// List retrieves a filtered list of anomalies from the database.
func (r *Repo) List(ctx context.Context, f Filter) ([]Row, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	rows, err := r.q.ListAnomalies(ctx, db.ListAnomaliesParams{
		FromTs:     dbutil.Int8Ptr(f.FromTS),
		ToTs:       dbutil.Int8Ptr(f.ToTS),
		Source:     dbutil.TextPtr(f.Source),
		MetricName: dbutil.TextPtr(f.MetricName),
		Env:        dbutil.TextPtr(f.Env),
		Region:     dbutil.TextPtr(f.Region),
		InstanceID: dbutil.TextPtr(f.InstanceID),
		Limit:      limit,
		Offset:     f.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list anomalies: %w", err)
	}

	out := make([]Row, 0, len(rows))
	for _, it := range rows {
		out = append(out, Row{
			ID: it.ID,

			Source:     it.Source,
			MetricName: it.MetricName,
			Env:        it.Env,
			Region:     it.Region,
			InstanceID: it.InstanceID,

			TS:        it.Ts,
			Value:     it.Value,
			Baseline:  it.Baseline,
			MAD:       it.Mad,
			Threshold: it.Threshold,

			WindowSize: it.WindowSize,
			ThresholdK: it.ThresholdK,
			Detector:   it.Detector,

			Producer: it.Producer,
			TraceID:  it.TraceID,
			EventID:  it.EventID,

			CreatedAt: pgTime(it.CreatedAt),
		})
	}

	return out, nil
}

// pgTime converts pgtype.Timestamptz to time.Time.
// Keep it internal to repo (still not string formatting).
func pgTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}
