package list_series

import "context"

// Row is a stable application DTO for the response layer (not DB schema leakage).
type Row struct {
	Source     string
	MetricName string
	Env        string
	Region     string

	LastTS    int64
	LastValue float64
	Baseline  float64
	MAD       float64
	Threshold float64

	WindowSize int32
	ThresholdK float64
	Detector   string

	Producer  string
	TraceID   string
	UpdatedAt string
}

// Repository defines the query port for listing series states.
type Repository interface {
	List(ctx context.Context, f Filter) ([]Row, error)
}

type Filter struct {
	Source     *string
	MetricName *string
	Env        *string
	Region     *string
	Limit      int32
}
