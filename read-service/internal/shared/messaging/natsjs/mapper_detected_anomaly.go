package natsjs

import (
	"encoding/json"
	"errors"
)

var ErrInvalidEnvelope = errors.New("invalid detected anomaly envelope")

// Envelope matches your pattern-service schema: V2 Meta/Data + flat fallback.
type DetectedAnomalyEnvelope struct {
	Meta *DetectedAnomalyMeta `json:"meta,omitempty"`
	Data *DetectedAnomalyData `json:"data,omitempty"`

	Source     string  `json:"source,omitempty"`
	MetricName string  `json:"metric_name,omitempty"`
	Env        string  `json:"env,omitempty"`
	Region     string  `json:"region,omitempty"`
	InstanceID string  `json:"instance_id,omitempty"`
	Timestamp  int64   `json:"timestamp,omitempty"`
	Value      float64 `json:"value,omitempty"`

	Baseline  float64 `json:"baseline,omitempty"`
	MAD       float64 `json:"mad,omitempty"`
	Threshold float64 `json:"threshold,omitempty"`

	WindowSize int     `json:"window_size,omitempty"`
	ThresholdK float64 `json:"threshold_k,omitempty"`
	Detector   string  `json:"detector,omitempty"`
}

type DetectedAnomalyMeta struct {
	EventID      string `json:"event_id"`
	EventType    string `json:"event_type"`
	EventVersion int    `json:"event_version"`
	Producer     string `json:"producer"`
	OccurredAt   string `json:"occurred_at"`
	ProducedAt   string `json:"produced_at"`
	TraceID      string `json:"trace_id,omitempty"`
}

type DetectedAnomalyData struct {
	Source     string `json:"source"`
	MetricName string `json:"metric_name"`
	Env        string `json:"env,omitempty"`
	Region     string `json:"region,omitempty"`
	InstanceID string `json:"instance_id,omitempty"`
	Timestamp  int64  `json:"timestamp,omitempty"`

	Value     float64 `json:"value"`
	Baseline  float64 `json:"baseline"`
	MAD       float64 `json:"mad"`
	Threshold float64 `json:"threshold"`

	WindowSize int     `json:"window_size"`
	ThresholdK float64 `json:"threshold_k"`
	Detector   string  `json:"detector"`
}

// DetectedAnomalyDTO is what projector consumes (transport-agnostic internal DTO).
type DetectedAnomalyDTO struct {
	Source, MetricName, Env, Region, InstanceID string
	Timestamp                                   int64
	Value, Baseline, MAD, Threshold             float64
	WindowSize                                  int32
	ThresholdK                                  float64
	Detector                                    string

	EventID, EventType, Producer, OccurredAt, ProducedAt, TraceID string
	EventVersion                                                  int32
}

func DecodeDetectedAnomaly(payload []byte) (DetectedAnomalyDTO, error) {
	var env DetectedAnomalyEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return DetectedAnomalyDTO{}, ErrorMalformedPayload
	}

	// Prefer V2
	if env.Meta != nil && env.Data != nil {
		if env.Data.Source == "" || env.Data.MetricName == "" {
			return DetectedAnomalyDTO{}, ErrInvalidEnvelope
		}
		return DetectedAnomalyDTO{
			Source:     env.Data.Source,
			MetricName: env.Data.MetricName,
			Env:        env.Data.Env,
			Region:     env.Data.Region,
			InstanceID: env.Data.InstanceID,
			Timestamp:  env.Data.Timestamp,

			Value:     env.Data.Value,
			Baseline:  env.Data.Baseline,
			MAD:       env.Data.MAD,
			Threshold: env.Data.Threshold,

			WindowSize: int32(env.Data.WindowSize),
			ThresholdK: env.Data.ThresholdK,
			Detector:   env.Data.Detector,

			EventID:      env.Meta.EventID,
			EventType:    env.Meta.EventType,
			Producer:     env.Meta.Producer,
			OccurredAt:   env.Meta.OccurredAt,
			ProducedAt:   env.Meta.ProducedAt,
			TraceID:      env.Meta.TraceID,
			EventVersion: int32(env.Meta.EventVersion),
		}, nil
	}

	// Fallback flat
	if env.Source == "" || env.MetricName == "" {
		return DetectedAnomalyDTO{}, ErrInvalidEnvelope
	}
	return DetectedAnomalyDTO{
		Source:     env.Source,
		MetricName: env.MetricName,
		Env:        env.Env,
		Region:     env.Region,
		InstanceID: env.InstanceID,
		Timestamp:  env.Timestamp,

		Value:     env.Value,
		Baseline:  env.Baseline,
		MAD:       env.MAD,
		Threshold: env.Threshold,

		WindowSize: int32(env.WindowSize),
		ThresholdK: env.ThresholdK,
		Detector:   env.Detector,
	}, nil
}
