package schema

type MetricRecordedDto struct {
	Source     string  `json:"source"`
	MetricName string  `json:"metric_name"`
	Value      float64 `json:"value"`
	Env        string  `json:"env"`
	Region     string  `json:"region"`
	InstanceID string  `json:"instance_id"`
	Timestamp  int64   `json:"timestamp"`
}
