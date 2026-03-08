package command

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/app/ports"
	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/domain/anomaly"
	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/domain/detector"
	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/domain/metric"
	"time"
)

const (
	anomalyDetected = "anomaly.detected"
	eventVersion    = 2
)

// HandlerConfig configures detection and publishing.
type HandlerConfig struct {
	DetectorName string
	ThresholdK   float64
	Producer     string
}

// ProcessMetricRecordHandler processes MetricRecorded events and detects anomalies.
type ProcessMetricRecordHandler struct {
	madDetector      *detector.MADDetector
	anomalyPublisher ports.AnomalyPublisher
	config           HandlerConfig
}

// NewProcessMetricRecordHandler wires detector and publisher.
func NewProcessMetricRecordHandler(
	madDetector *detector.MADDetector,
	anomalyPublisher ports.AnomalyPublisher,
	config HandlerConfig,
) *ProcessMetricRecordHandler {
	return &ProcessMetricRecordHandler{
		madDetector:      madDetector,
		anomalyPublisher: anomalyPublisher,
		config:           config,
	}
}

// Handle processes a metric event, publishes anomaly.detected if needed, and returns (isAnomaly, error)
func (handler *ProcessMetricRecordHandler) Handle(ctx context.Context, recordedMetric metric.MetricRecorded) (bool, error) {
	seriesKey := detector.NewSeriesKey(
		recordedMetric.Source,
		recordedMetric.MetricName,
		recordedMetric.Env,
		recordedMetric.Region)

	// TODO handle currentWindowSize except _
	baselineMedian, deviationMAD, anomalyLowerThreshold, anomalyUpperThreshold, currentWindowSize,
		isAnomaly := handler.madDetector.Observe(seriesKey, recordedMetric.Value)

	if !isAnomaly {
		return false, nil
	}

	thresholdToStore := anomalyUpperThreshold
	if recordedMetric.Value < anomalyLowerThreshold {
		thresholdToStore = anomalyLowerThreshold
	}

	currentTimeRFC3339 := time.Now().UTC().Format(time.RFC3339)

	detectedAnomaly, err := anomaly.NewDetectedAnomaly(anomaly.DetectedAnomaly{
		Source:     recordedMetric.Source,
		MetricName: recordedMetric.MetricName,
		Env:        recordedMetric.Env,
		Region:     recordedMetric.Region,
		InstanceID: recordedMetric.InstanceID,
		Timestamp:  recordedMetric.Timestamp,

		Value:     recordedMetric.Value,
		Baseline:  baselineMedian,
		MAD:       deviationMAD,
		Threshold: thresholdToStore,

		WindowSize:   currentWindowSize,
		ThresholdK:   handler.config.ThresholdK,
		Detector:     handler.config.DetectorName,
		Producer:     handler.config.Producer,
		EventID:      uuid.NewString(),
		EventType:    anomalyDetected,
		EventVersion: eventVersion,
		OccurredAt:   currentTimeRFC3339,
		ProducedAt:   currentTimeRFC3339,
	})

	if err != nil {
		return true, fmt.Errorf("error creating detected anomaly: %w", err)
	}

	if err := handler.anomalyPublisher.PublishAnomalyDetected(ctx, detectedAnomaly); err != nil {
		return true, fmt.Errorf("error publishing detected anomaly: %w", err)
	}

	return true, nil
}
