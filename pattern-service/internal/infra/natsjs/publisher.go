package natsjs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nats-io/nats.go"

	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/app/ports"
	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/domain/anomaly"
	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/infra/log"
	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/infra/schema"
)

const (
	anomaliesStreamName    = "ANOMALIES"
	anomalyDetectedSubject = "anomaly.detected"
)

// JetStreamAnomalyPublisher publishes anomaly.detected into JetStream.
type JetStreamAnomalyPublisher struct {
	logger    log.Interface
	jetStream nats.JetStreamContext
}

// NewJetStreamAnomalyPublisher ensures the ANOMALIES stream exists and returns publisher.
func NewJetStreamAnomalyPublisher(logger log.Interface, jetStream nats.JetStreamContext) (ports.AnomalyPublisher, error) {
	streamInfo, err := jetStream.StreamInfo(anomaliesStreamName)
	if err != nil {
		if errors.Is(err, nats.ErrStreamNotFound) {
			if _, createErr := jetStream.AddStream(&nats.StreamConfig{
				Name:     anomaliesStreamName,
				Subjects: []string{anomalyDetectedSubject},
				Storage:  nats.FileStorage,
			}); createErr != nil {
				return nil, fmt.Errorf("create jetstream stream %s: %w", anomaliesStreamName, createErr)
			}
			logger.Infow("jetstream stream created", "name", anomaliesStreamName, "subject", anomalyDetectedSubject)
		} else {
			return nil, fmt.Errorf("jetstream stream info %s: %w", anomaliesStreamName, err)
		}
	} else {
		logger.Infow("jetstream stream found", "name", streamInfo.Config.Name, "subjects", streamInfo.Config.Subjects)
	}

	return &JetStreamAnomalyPublisher{
		logger:    logger,
		jetStream: jetStream,
	}, nil
}

// PublishAnomalyDetected serializes domain event into V2 envelope + flat backward-compat fields.
func (publisher *JetStreamAnomalyPublisher) PublishAnomalyDetected(ctx context.Context, detected anomaly.DetectedAnomaly) error {
	envelope := schema.DetectedAnomalyEnvelope{
		Meta: &schema.DetectedAnomalyMeta{
			EventID:      detected.EventID,
			EventType:    detected.EventType,
			EventVersion: detected.EventVersion,
			Producer:     detected.Producer,
			OccurredAt:   detected.OccurredAt,
			ProducedAt:   detected.ProducedAt,
			TraceID:      detected.TraceId,
		},
		Data: &schema.DetectedAnomalyData{
			Source:     detected.Source,
			MetricName: detected.MetricName,
			Env:        detected.Env,
			Region:     detected.Region,
			InstanceID: detected.InstanceID,
			Timestamp:  detected.Timestamp,

			Value:     detected.Value,
			Baseline:  detected.Baseline,
			MAD:       detected.MAD,
			Threshold: detected.Threshold,

			WindowSize: detected.WindowSize,
			ThresholdK: detected.ThresholdK,
			Detector:   detected.Detector,
		},

		// Flat fields (V1-like) for backward compatibility:
		Source:     detected.Source,
		MetricName: detected.MetricName,
		Env:        detected.Env,
		Region:     detected.Region,
		InstanceID: detected.InstanceID,
		Timestamp:  detected.Timestamp,

		Value:     detected.Value,
		Baseline:  detected.Baseline,
		MAD:       detected.MAD,
		Threshold: detected.Threshold,

		WindowSize: detected.WindowSize,
		ThresholdK: detected.ThresholdK,
		Detector:   detected.Detector,
	}

	payload, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal anomaly envelope: %w", err)
	}

	publishAck, err := publisher.jetStream.Publish(anomalyDetectedSubject, payload, nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("jetstream publish anomaly.detected: %w", err)
	}

	publisher.logger.Infow("anomaly detected event published",
		"stream", anomaliesStreamName,
		"subject", anomalyDetectedSubject,
		"seq", publishAck.Sequence,
		"source", detected.Source,
		"metric", detected.MetricName,
		"value", detected.Value,
		"baseline", detected.Baseline,
		"threshold", detected.Threshold,
	)

	return nil
}
