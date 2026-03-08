package list_series

// Query is a CQRS query object.
type Query struct {
	Source     *string
	MetricName *string
	Env        *string
	Region     *string
	Limit      int32
}
