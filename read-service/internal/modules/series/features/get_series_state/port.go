package get_series_state

import "context"

type State struct {
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

type Repository interface {
	Get(ctx context.Context, key Key) (State, bool, error)
}

type Key struct {
	Source     string
	MetricName string
	Env        string
	Region     string
}
