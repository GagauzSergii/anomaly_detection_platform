package anomaly_detected

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sergii-gagauz/anomaly_detection_platform/read-service/internal/shared/messaging/natsjs"
)

// Handler adapts JetStream messages to projector input.
type Handler struct {
	projector *Projector
}

func NewHandler(projector *Projector) *Handler {
	return &Handler{projector: projector}
}

func (handler *Handler) Handle(ctx context.Context, subject string, streamSeq uint64, payload []byte) error {
	// ATG Update: Log incoming event processing start
	slog.InfoContext(ctx, "Processing anomaly detected event",
		slog.String("subject", subject),
		slog.Uint64("seq", streamSeq),
	)

	dto, err := natsjs.DecodeDetectedAnomaly(payload)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to decode event payload",
			slog.String("error", err.Error()),
		)
		// malformed JSON or invalid structure: no retries
		return natsjs.ErrorMalformedPayload
	}

	deduplication := dto.EventID
	if deduplication == "" {
		// fallback: subject:sequence
		deduplication = fmt.Sprintf("%s:%d", subject, streamSeq)
	}

	input := Input{
		DeduplicateKey: deduplication,
		Subject:        subject,
		StreamSeq:      int64(streamSeq),

		Source:     dto.Source,
		MetricName: dto.MetricName,
		Env:        dto.Env,
		Region:     dto.Region,
		InstanceID: dto.InstanceID,

		Timestamp: dto.Timestamp,
		Value:     dto.Value,
		Baseline:  dto.Baseline,
		MAD:       dto.MAD,
		Threshold: dto.Threshold,

		WindowSize: dto.WindowSize,
		ThresholdK: dto.ThresholdK,
		Detector:   dto.Detector,

		EventID:      dto.EventID,
		EventType:    dto.EventType,
		EventVersion: dto.EventVersion,
		Producer:     dto.Producer,
		OccurredAt:   dto.OccurredAt,
		ProducedAt:   dto.ProducedAt,
		TraceID:      dto.TraceID,
	}

	// Any infra/runtime error should be retried -> returning error will cause Nak().
	if err := handler.projector.Project(ctx, input); err != nil {
		slog.ErrorContext(ctx, "Failed to project event",
			slog.String("dedup_key", deduplication),
			slog.String("error", err.Error()),
		)
		return err
	}

	slog.InfoContext(ctx, "Successfully processed anomaly detected event",
		slog.String("dedup_key", deduplication),
	)

	return nil
}
