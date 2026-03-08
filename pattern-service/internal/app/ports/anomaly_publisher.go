package ports

import (
	"context"

	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/domain/anomaly"
)

// AnomalyPublisher publishes anomaly domain events to external systems (JetStream, Kafka, etc.).
type AnomalyPublisher interface {
	PublishAnomalyDetected(ctx context.Context, anomalyEvent anomaly.DetectedAnomaly) error
}
