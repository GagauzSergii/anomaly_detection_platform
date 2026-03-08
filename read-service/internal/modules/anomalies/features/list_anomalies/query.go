package list_anomalies

type Query struct {
	FromTS *int64
	ToTS   *int64

	Source     *string
	MetricName *string
	Env        *string
	Region     *string
	InstanceID *string

	Limit  int32
	Offset int32
}
