package schema

// DetectedAnomalyEnvelope is a backward-compatible message format.
// - New consumers should prefer Meta/Data.
// - Old consumers may still read flat fields.
type DetectedAnomalyEnvelope struct {
	// --- V2 meta ---
	Meta *DetectedAnomalyMeta `json:"meta,omitempty"`
	// --- V2 data ---
	Data *DetectedAnomalyData `json:"data,omitempty"`

	// --- Backward-compatible flat fields (V1-like) ---
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
