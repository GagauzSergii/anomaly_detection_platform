package natsjs

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/app/processor"
	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/domain/metric"
	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/infra/log"
)

const (
	streamName = "METRICS_RECORDED"
	subject    = "metrics.recorded"

	waitStreamMaxAttempts = 20
	waitStreamDelay       = 1 * time.Second
	durableName           = "pattern-service-metrics-consumer"

	shutdownWaitTimeout   = 15 * time.Second
	messageProcessTimeout = 30 * time.Second
)

type JetStreamConsumer struct {
	logger log.Interface
	conn   *nats.Conn
	js     nats.JetStreamContext

	processor processor.MetricRecordedProcessor

	// protects consumeCtx access
	mu         sync.Mutex
	consumeCtx context.Context

	// graceful shutdown: wait for in-flight handlers
	wg sync.WaitGroup

	// time budget to finish processing of already received messages
	shutdownWaitTimeout time.Duration

	// time budget for each single message processing
	messageProcessTimeout time.Duration
}

// NewJetStreamConsumer creates JetStreamConsumer.
func NewJetStreamConsumer(
	logger log.Interface,
	natsURL string,
	processor processor.MetricRecordedProcessor) (*JetStreamConsumer, error) {
	opts := []nats.Option{
		nats.Name("pattern-service-jetstream-consumer"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
		nats.RetryOnFailedConnect(true),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			logger.Errorw("NATS connection disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Infow("NATS reconnected", "url", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			logger.Infow("NATS connection closed")
		}),
	}

	connection, err := nats.Connect(natsURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("error connecting to NATS JetStream: %w", err)
	}

	stream, err := connection.JetStream()
	if err != nil {
		connection.Close()
		return nil, fmt.Errorf("error connecting to NATS JetStream: %w", err)
	}

	return &JetStreamConsumer{
		logger:                logger,
		conn:                  connection,
		js:                    stream,
		processor:             processor,
		shutdownWaitTimeout:   shutdownWaitTimeout,
		messageProcessTimeout: messageProcessTimeout,
	}, nil
}

func (consumer *JetStreamConsumer) Start(ctx context.Context) error {
	var lastErr error

	consumer.mu.Lock()
	consumer.consumeCtx = ctx
	consumer.mu.Unlock()

	for attempt := 1; attempt <= waitStreamMaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		info, err := consumer.js.StreamInfo(streamName)
		if err == nil && info != nil {
			consumer.logger.Infow("jetstream stream found",
				"name", info.Config.Name,
				"subjects", info.Config.Subjects,
				"attempt", attempt)
			lastErr = nil
			break
		}

		lastErr = err
		consumer.logger.Errorw("waiting for jetstream stream",
			"stream", streamName,
			"attempt", attempt,
			"error", err)
		time.Sleep(waitStreamDelay)
	}

	if lastErr != nil {
		return fmt.Errorf("stream %s is not ready: %w", streamName, lastErr)
	}

	sub, err := consumer.js.Subscribe(
		subject,
		consumer.handleMessage,
		nats.Durable(durableName),
		nats.ManualAck(),
		nats.BindStream(streamName))
	if err != nil {
		return fmt.Errorf("error subscribing to NATS JetStream: %w", err)
	}

	consumer.logger.Infow("NATS JetStream consumer started",
		"stream", streamName,
		"subjects", streamName,
		"durable", durableName)

	// waiting for shutdown signals
	<-ctx.Done()
	consumer.logger.Infow("NATS JetStream consumer shutting down")

	// 1) Stop accepting new deliveries, let pending callbacks drain.
	if err := sub.Drain(); err != nil {
		consumer.logger.Errorw("error shutting down consumer", "error", err)
	}

	waitDone := make(chan struct{})
	go func() {
		consumer.wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		consumer.logger.Infow("all in-flight message handlers finished")
	case <-time.After(consumer.shutdownWaitTimeout):
		consumer.logger.Errorw("shutdown timeout waiting for in-flight handlers",
			"timeout", consumer.shutdownWaitTimeout.String())
	}

	if err := consumer.conn.Drain(); err != nil {
		return fmt.Errorf("error draining NATS connection: %w", err)
	}
	consumer.conn.Close()

	consumer.logger.Infow("NATS JetStream consumer stopped gracefully")
	return nil
}

// handleMessage processes one JetStream message.
// - If JSON is invalid, we Term() the message (no retries for malformed payload).
// - If processing fails (temporary infra issue), we Nak() to retry.
// - If processing succeeds, we Ack().
func (consumer *JetStreamConsumer) handleMessage(message *nats.Msg) {
	consumer.wg.Add(1)
	defer consumer.wg.Done()

	var metricRecordedEvent metric.MetricRecorded

	if err := json.Unmarshal(message.Data, &metricRecordedEvent); err != nil {
		consumer.logger.Errorw("failed to unmarshal MetricRecorded event", "error", err)
		_ = message.Term()
		return
	}

	// IMPORTANT:
	// Using independent context for message processing, so that service shutdown
	// doesn't instantly cancel in-flight business logic.
	processCtx, cancel := context.WithTimeout(context.Background(), consumer.messageProcessTimeout)
	defer cancel()

	if err := consumer.processor.Process(processCtx, metricRecordedEvent); err != nil {
		consumer.logger.Errorw("failed to process MetricRecorded event", "error", err)

		// For temporary errors (publish timeout etc.) we want retries.
		_ = message.Nak()
		return
	}

	if err := message.Ack(); err != nil {
		consumer.logger.Errorw("failed to ack MetricRecorded event", "error", err)
	}
}
