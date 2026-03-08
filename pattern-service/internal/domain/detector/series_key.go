package detector

// SeriesKey uniquely identifies a metric time series for anomaly detection.
// We keep separate baselines for different service+metric+env+region combinations.
type SeriesKey struct {
	SourceService string
	MetricName    string
	Environment   string
	Region        string
}

func NewSeriesKey(sourceService, metricName, environment, region string) SeriesKey {
	return SeriesKey{
		SourceService: sourceService,
		MetricName:    metricName,
		Environment:   environment,
		Region:        region,
	}
}
