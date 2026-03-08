package processor

import (
	"context"

	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/app/command"
	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/domain/metric"
)

// MADMetricRecordedProcessor adapts MetricRecordedProcessor interface to command handler logic.
type MADMetricRecordedProcessor struct {
	processMetricRecordedHandler *command.ProcessMetricRecordHandler
}

// NewMADMetricRecordedProcessor wires processor to command handler.
func NewMADMetricRecordedProcessor(processMetricRecordedHandler *command.ProcessMetricRecordHandler) *MADMetricRecordedProcessor {
	return &MADMetricRecordedProcessor{processMetricRecordedHandler: processMetricRecordedHandler}
}

// Process applies anomaly detection logic and publishes AnomalyDetected when needed.
func (processor *MADMetricRecordedProcessor) Process(ctx context.Context, event metric.MetricRecorded) error {
	_, err := processor.processMetricRecordedHandler.Handle(ctx, event)
	return err
}
