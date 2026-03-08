package dto

import "time"

// AnomalyResponse represents a single anomaly event detected by the system.
// @Description A detected anomaly with its metric context and statistical boundaries.
type AnomalyResponse struct {
	ID         int64     `json:"id" example:"1024"`
	Source     string    `json:"source" example:"payment-gateway"`
	MetricName string    `json:"metric_name" example:"latency"`
	Env        string    `json:"env" example:"production"`
	Region     string    `json:"region" example:"us-east-1"`
	InstanceID string    `json:"instance_id" example:"i-0abcd1234efgh5678"`
	Ts         int64     `json:"ts" example:"1709400000"`
	Value      float64   `json:"value" example:"350.5"`
	Baseline   float64   `json:"baseline" example:"120.0"`
	Mad        float64   `json:"mad" example:"15.5"`
	Threshold  float64   `json:"threshold" example:"300.0"`
	WindowSize int32     `json:"window_size" example:"60"`
	ThresholdK float64   `json:"threshold_k" example:"3.0"`
	Detector   string    `json:"detector" example:"mad_detector"`
	Producer   string    `json:"producer" example:"pattern-service"`
	TraceID    string    `json:"trace_id" example:"trace-550e8400-e29b-41d4-a716-446655440000"`
	EventID    string    `json:"event_id" example:"evt-12345"`
	CreatedAt  time.Time `json:"created_at" example:"2026-03-08T15:00:00Z"`
}

// SeriesStateResponse represents the current baseline state of a metric series.
// @Description The latest statistical state for a specific metric series.
type SeriesStateResponse struct {
	Source     string    `json:"source" example:"payment-gateway"`
	MetricName string    `json:"metric_name" example:"latency"`
	Env        string    `json:"env" example:"production"`
	Region     string    `json:"region" example:"us-east-1"`
	LastTs     int64     `json:"last_ts" example:"1709400000"`
	LastValue  float64   `json:"last_value" example:"125.5"`
	Baseline   float64   `json:"baseline" example:"120.0"`
	Mad        float64   `json:"mad" example:"15.5"`
	Threshold  float64   `json:"threshold" example:"300.0"`
	WindowSize int32     `json:"window_size" example:"60"`
	ThresholdK float64   `json:"threshold_k" example:"3.0"`
	Detector   string    `json:"detector" example:"mad_detector"`
	Producer   string    `json:"producer" example:"ingestion-service"`
	TraceID    string    `json:"trace_id" example:"trace-550e8400-e29b-41d4-a716-446655440000"`
	UpdatedAt  time.Time `json:"updated_at" example:"2026-03-08T15:00:00Z"`
}
