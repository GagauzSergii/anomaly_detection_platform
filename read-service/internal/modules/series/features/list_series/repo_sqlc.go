package list_series

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sergii-gagauz/anomaly_detection_platform/read-service/internal/shared/postgres/db"
)

// Repo is sqlc-backed repository implementation.
type Repo struct {
	query *db.Queries
}

func NewRepo(queries *db.Queries) *Repo {
	return &Repo{query: queries}
}

func (repo *Repo) List(ctx context.Context, filter Filter) ([]Row, error) {
	// 1. Обработка лимита
	limit := int32(filter.Limit)
	if limit <= 0 {
		limit = 200
	}

	// 2. Query call
	rows, err := repo.query.ListSeries(ctx, db.ListSeriesParams{
		Source:     toText(filter.Source),
		MetricName: toText(filter.MetricName),
		Env:        toText(filter.Env),
		Region:     toText(filter.Region),
		LimitVal:   limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list series query failed: %w", err)
	}

	// 3. Results mapping to business structure Row
	out := make([]Row, 0, len(rows))
	for _, it := range rows {
		out = append(out, Row{
			Source:     it.Source,
			MetricName: it.MetricName,
			Env:        it.Env,
			Region:     it.Region,

			LastTS:    it.LastTs,
			LastValue: it.LastValue,
			Baseline:  it.Baseline,
			MAD:       it.Mad,
			Threshold: it.Threshold,

			WindowSize: it.WindowSize,
			ThresholdK: it.ThresholdK,
			Detector:   it.Detector,

			Producer:  it.Producer,
			TraceID:   it.TraceID,
			UpdatedAt: formatTime(it.UpdatedAt),
		})
	}
	return out, nil
}

// toText *string to pgtype.Text
func toText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// formatTime convert pgtype.Timestamptz to RFC3339 string
func formatTime(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format(time.RFC3339)
}
