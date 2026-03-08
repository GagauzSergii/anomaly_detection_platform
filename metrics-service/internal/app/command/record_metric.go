package command

import (
	"context"
	"github.com/sergii-gagauz/anomaly_detection_platform/metrics-service/internal/domain/metric"
)

// EventPublisher is an abstraction over any transport event (NATS, Kafka, etc)
type EventPublisher interface {
	PublishMetricRecorded(ctx context.Context, event metric.MetricRecorded) error
}

// RecordMetricCommand is command (DTO) on application level, not on gRPC
type RecordMetricCommand struct {
	Source     string
	MetricName string
	Value      float64
	Env        string
	Region     string
	InstanceID string
	Timestamp  int64
}

// RecordMetricCommandHandler is an interface which contract use case
type RecordMetricCommandHandler interface {
	Handle(ctx context.Context, command RecordMetricCommand) error
}

type recordMetricCommandHandler struct {
	publisher EventPublisher
}

func (h *recordMetricCommandHandler) Handle(ctx context.Context, command RecordMetricCommand) error {
	// application level validation should be added (like range)
	event, err := metric.NewMetricRecorded(
		command.Source,
		command.MetricName,
		command.Value,
		command.Env,
		command.Region,
		command.InstanceID,
		command.Timestamp,
	)

	if err != nil {
		return err
	}

	// everything handler know handler must publish domain event
	return h.publisher.PublishMetricRecorded(ctx, event)
}

func NewRecordMetricCommandHandler(publisher EventPublisher) RecordMetricCommandHandler {
	return &recordMetricCommandHandler{
		publisher: publisher,
	}
}
