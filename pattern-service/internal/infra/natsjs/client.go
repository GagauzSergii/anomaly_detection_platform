package natsjs

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/infra/log"
)

// JetStreamClient owns a single NATS connection and provides a reusable JetStream stream context.
// It is responsible for connection lifecycle and graceful shutdown.
type JetStreamClient struct {
	logger log.Interface
	conn   *nats.Conn
	stream nats.JetStreamContext
}

// NewJetStreamClient connects to NATS and initializes JetStream context.
// This function should be called once in the service composition root.
func NewJetStreamClient(logger log.Interface, natsURL string) (*JetStreamClient, error) {
	options := []nats.Option{
		nats.Name("pattern-service-jetstream-client"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
		nats.RetryOnFailedConnect(true),

		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			logger.Errorw("NATS disconnected", "error", err)
		}),

		nats.ReconnectHandler(func(connection *nats.Conn) {
			logger.Errorw("NATS reconnected", "url", connection)
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			logger.Errorw("NATS connection closed", "url", natsURL)
		}),
	}

	connection, err := nats.Connect(natsURL, options...)
	if err != nil {
		return nil, fmt.Errorf("error connecting to NATS JetStream: %w", err)
	}

	jetStreamContext, err := connection.JetStream()
	if err != nil {
		connection.Close()
		return nil, fmt.Errorf("error init NATS JetStream: %w", err)
	}

	logger.Infow("Connected to NATS JetStream", "url", natsURL)

	return &JetStreamClient{
		logger: logger,
		conn:   connection,
		stream: jetStreamContext,
	}, nil
}

// Stream returns initialized JetStream stream context.
// It does not create new connections and is safe to reuse.
func (client *JetStreamClient) Stream() (nats.JetStreamContext, error) {
	if client.stream == nil {
		return nil, fmt.Errorf("NATS JetStream stream is not initialized")
	}
	return client.stream, nil
}

// Close gracefully drains and closes NATS connection.
func (client *JetStreamClient) Close() error {
	if client.conn == nil {
		return nil
	}

	client.logger.Infow("Closing NATS JetStream stream")

	if err := client.conn.Drain(); err != nil {
		return fmt.Errorf("error draining NATS JetStream stream: %w", err)
	}

	client.conn.Close()
	return nil
}
