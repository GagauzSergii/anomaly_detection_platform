package metric

import "errors"

var (
	// ErrInvalidMetricType ErrInvalidMetric is returned if metric don't pass basic validation
	ErrInvalidMetricType = errors.New("invalid metric")
)

// MetricRecorded is domain event "metric is recorded (fixed)"
type MetricRecorded struct {
	Source     string
	MetricName string
	Value      float64
	Env        string
	Region     string
	InstanceID string
	Timestamp  int64
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
		return MetricRecorded{}, ErrInvalidMetricType
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
