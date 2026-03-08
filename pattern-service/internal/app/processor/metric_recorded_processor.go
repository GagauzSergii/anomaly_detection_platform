package processor

import (
	"context"
	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/domain/metric"
)

type MetricRecordedProcessor interface {
	Process(ctx context.Context, event metric.MetricRecorded) error
}
