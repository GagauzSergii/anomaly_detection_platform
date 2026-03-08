package metric

import "errors"

var (
	ErrInvalidMetricRecorded = errors.New("invalid metric recorded")
)

type MetricRecorded struct {
	Source     string  `json:"source"`
	MetricName string  `json:"metric_name"`
	Value      float64 `json:"value"`
	Env        string  `json:"env"`
	Region     string  `json:"region"`
	InstanceID string  `json:"instance_id"`
	Timestamp  int64   `json:"timestamp"`
}

func NewMetricRecorded(
	source string,
	metricName string,
	value float64,
	env string,
	region string,
	instanceID string,
	timestamp int64,
) (MetricRecorded, error) {
	if source == "" || metricName == "" {
		return MetricRecorded{}, ErrInvalidMetricRecorded
	}

	return MetricRecorded{
		Source:     source,
		MetricName: metricName,
		Value:      value,
		Env:        env,
		Region:     region,
		InstanceID: instanceID,
		Timestamp:  timestamp,
	}, nil
}
