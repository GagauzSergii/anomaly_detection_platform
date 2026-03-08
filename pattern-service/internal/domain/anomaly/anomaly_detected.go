package anomaly

import (
	"errors"
)

var (
	// ErrInvalidAnomalyDetected is returned when a domain anomaly event is incomplete.
	ErrInvalidAnomalyDetected = errors.New("invalid anomaly detected")
)

// DetectedAnomaly represents a domain fact that an anomaly was detected.
// Domain objects must not depend on transport (JSON/Proto) concerns.
type DetectedAnomaly struct {
	Source     string
	MetricName string
	Env        string
	Region     string
	InstanceID string
	Timestamp  int64

	Value     float64
	Baseline  float64
	MAD       float64
	Threshold float64

	WindowSize int
	ThresholdK float64
	Detector   string
	Producer   string

	EventID      string
	EventType    string
	EventVersion int
	OccurredAt   string
	ProducedAt   string
	TraceId      string
}

func NewDetectedAnomaly(input DetectedAnomaly) (DetectedAnomaly, error) {
	if input.Source == "" || input.MetricName == "" {
		return DetectedAnomaly{}, ErrInvalidAnomalyDetected
	}
	if input.EventVersion == 0 {
		input.EventVersion = 2
	}
	return input, nil
}
