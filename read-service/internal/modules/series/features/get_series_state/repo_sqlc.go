package get_series_state

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sergii-gagauz/anomaly_detection_platform/read-service/internal/shared/postgres/db"
)

type Repo struct{ query *db.Queries }

func NewRepo(query *db.Queries) *Repo { return &Repo{query: query} }

func (r *Repo) Get(ctx context.Context, key Key) (State, bool, error) {
	row, err := r.query.GetSeriesState(ctx, db.GetSeriesStateParams{
		Source:     key.Source,
		MetricName: key.MetricName,
		Env:        key.Env,
		Region:     key.Region,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return State{}, false, nil
		}
		return State{}, false, err
	}

	return State{
		Source:     row.Source,
		MetricName: row.MetricName,
		Env:        row.Env,
		Region:     row.Region,

		LastTS:    row.LastTs,
		LastValue: row.LastValue,
		Baseline:  row.Baseline,
		MAD:       row.Mad,
		Threshold: row.Threshold,

		WindowSize: row.WindowSize,
		ThresholdK: row.ThresholdK,
		Detector:   row.Detector,

		Producer:  row.Producer,
		TraceID:   row.TraceID,
		UpdatedAt: row.UpdatedAt.Time.Format(time.RFC3339),
	}, true, nil
}
