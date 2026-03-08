package events

import (
	"context"

	"github.com/sergii-gagauz/anomaly_detection_platform/metrics-service/internal/app/command"
	"github.com/sergii-gagauz/anomaly_detection_platform/metrics-service/internal/domain/metric"
	"github.com/sergii-gagauz/anomaly_detection_platform/metrics-service/internal/infra/log"
)

// StubPublisher is temporary realization of EventPublisher which log event, later should be moved to NATS JetStream.
type StubPublisher struct {
	logger log.Interface
}

func NewStubPublisher(logger log.Interface) command.EventPublisher {
	return &StubPublisher{
		logger: logger,
	}
}

func (p *StubPublisher) PublishMetricRecorded(_ context.Context, event metric.MetricRecorded) error {
	p.logger.Infow("MetricRecorded event (stub publish)",
		"source", event.Source,
		"metric", event.MetricName,
		"value", event.Value,
		"env", event.Env,
		"region", event.Region,
		"instance", event.InstanceID,
		"timestamp", event.Timestamp,
	)
	return nil
}
