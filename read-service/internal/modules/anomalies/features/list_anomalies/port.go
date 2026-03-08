package list_anomalies

import (
	"context"
	"time"
)

// Row is a read-model DTO for application layer.
// Keep it transport-agnostic: no string formatting here.
type Row struct {
	ID int64

	Source     string
	MetricName string
	Env        string
	Region     string
	InstanceID string

	TS        int64
	Value     float64
	Baseline  float64
	MAD       float64
	Threshold float64

	WindowSize int32
	ThresholdK float64
	Detector   string

	Producer  string
	TraceID   string
	EventID   string
	CreatedAt time.Time
}

type Filter struct {
	FromTS     *int64
	ToTS       *int64
	Source     *string
	MetricName *string
	Env        *string
	Region     *string
	InstanceID *string
	Limit      int32
	Offset     int32
}

type Repository interface {
	List(ctx context.Context, f Filter) ([]Row, error)
}
