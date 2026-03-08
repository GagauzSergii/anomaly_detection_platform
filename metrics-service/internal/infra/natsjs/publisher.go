package natsjs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/sergii-gagauz/anomaly_detection_platform/metrics-service/internal/domain/metric"
	"github.com/sergii-gagauz/anomaly_detection_platform/metrics-service/internal/infra/events/schema"
	"github.com/sergii-gagauz/anomaly_detection_platform/metrics-service/internal/infra/log"
)

const (
	streamName = "METRICS_RECORDED"
	subject    = "metrics.recorded"
)

// NatsEventPublisher event publisher realization on NATS, JetStream
type NatsEventPublisher struct {
	logger  log.Interface
	conn    *nats.Conn
	js      nats.JetStreamContext
	subject string
}

func NewNatsEventPublisher(logger log.Interface, natsURL string) (*NatsEventPublisher, error) {
	opts := []nats.Option{
		nats.Name("metrics-service-publisher"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
		nats.RetryOnFailedConnect(true),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			logger.Errorw("NATS disconnected", "err", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Infow("NATS reconnected", "url", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			logger.Infow("NATS connection closed")
		}),
	}

	nc, err := nats.Connect(natsURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("nats connection error: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("nats jetstream creation error: %w", err)
	}

	// try to create a stream, if exist - logging
	if _, err := js.StreamInfo(streamName); err != nil {
		if errors.Is(err, nats.ErrStreamNotFound) {
			if _, err := js.AddStream(&nats.StreamConfig{
				Name:     streamName,
				Subjects: []string{subject},
				Storage:  nats.FileStorage,
				//TODO Retention, MaxMsgs, MaxBytes
			}); err != nil {
				nc.Close()
				return nil, fmt.Errorf("add jetstream stream %s: %w", streamName, err)
			}
			logger.Infow("jetstream stream created",
				"name", streamName,
				"subject", streamName)
		} else {
			nc.Close()
			return nil, fmt.Errorf("jetstream stream info %s: %w", streamName, err)
		}
	} else {
		logger.Infow("jetstream stream exists",
			"name", streamName,
			"subject", streamName)
	}

	logger.Infow("NATS JetStream publisher started",
		"streamName", streamName,
		"subject", subject,
	)

	logger.Infow("connected to NATS JetStream",
		"url", natsURL,
		"subject", subject,
		"streamName", streamName,
	)

	return &NatsEventPublisher{
		logger:  logger,
		conn:    nc,
		js:      js,
		subject: subject,
	}, nil
}

// PublishMetricRecorded is the realization of command.EventPublisher
func (publisher *NatsEventPublisher) PublishMetricRecorded(ctx context.Context, event metric.MetricRecorded) error {
	dto := schema.MetricRecordedDto{
		Source:     event.Source,
		MetricName: event.MetricName,
		Value:      event.Value,
		Env:        event.Env,
		Region:     event.Region,
		InstanceID: event.InstanceID,
		Timestamp:  event.Timestamp,
	}
	data, err := json.Marshal(dto)
	if err != nil {
		return fmt.Errorf("marshal metric recorded event: %w", err)
	}

	publisherContext, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	ack, err := publisher.js.Publish(publisher.subject, data, nats.Context(publisherContext))
	if err != nil {
		return fmt.Errorf("publish metric recorded event: %w", err)
	}

	publisher.logger.Infow("metric recorded event is published to JetStream",
		"subject", publisher.subject,
		"streamName", ack.Stream,
		"seq", ack.Sequence,
		"source", event.Source,
		"metric", event.MetricName,
		"value", event.Value,
		"env", event.Env,
		"region", event.Region,
		"instance", event.InstanceID,
		"timestamp", event.Timestamp)

	return nil
}

func (publisher *NatsEventPublisher) Close() error {
	if publisher.conn != nil {
		publisher.logger.Infow("closing connection to NATS JetStream")
		err := publisher.conn.Drain()
		if err != nil {
			return err
		}
		publisher.conn.Close()
	}
	return nil
}
