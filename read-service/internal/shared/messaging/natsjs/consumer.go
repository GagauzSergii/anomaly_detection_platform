package natsjs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	waitStreamMaxAttempts = 30
	waitStreamDelay       = 2 * time.Second

	shutdownWaitTimeout   = 15 * time.Second
	messageProcessTimeout = 30 * time.Second
)

type Handler interface {
	Handle(ctx context.Context, subject string, streamSeq uint64, payload []byte) error
}

var ErrorMalformedPayload = errors.New("malformed payload")

// JetStreamConsumer is a production-grade consumer:
// - waits for stream readiness
// - graceful shutdown (Drain + wait in-flight)
// - message-level timeout
// - Term() for malformed payload, Nak() for temporary failures.
type JetStreamConsumer struct {
	conn *nats.Conn
	js   nats.JetStreamContext

	streamName string
	subject    string
	durable    string

	handler Handler

	mu         sync.Mutex
	consumeCtx context.Context

	wg sync.WaitGroup

	shutdownWaitTimeout   time.Duration
	messageProcessTimeout time.Duration
}

// NewJetStreamConsumer creates a configured JetStream consumer.
func NewJetStreamConsumer(natsURL, streamName, subject, durable string, handler Handler) (*JetStreamConsumer, error) {
	opts := []nats.Option{
		nats.Name(durable),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
		nats.RetryOnFailedConnect(true),
	}

	conn, err := nats.Connect(natsURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("connect nats: %w", err)
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("connect nats: %w", err)
	}

	return &JetStreamConsumer{
		conn:                  conn,
		js:                    js,
		streamName:            streamName,
		subject:               subject,
		durable:               durable,
		handler:               handler,
		shutdownWaitTimeout:   shutdownWaitTimeout,
		messageProcessTimeout: messageProcessTimeout,
	}, nil
}

func (consumer *JetStreamConsumer) Start(ctx context.Context) error {
	consumer.mu.Lock()
	consumer.consumeCtx = ctx
	consumer.mu.Unlock()

	if err := consumer.waitStreamReady(ctx); err != nil {
		// should be logged
		_ = fmt.Errorf("wait stream ready: %w", err)
		return err
	}

	subject, err := consumer.js.Subscribe(
		consumer.subject,
		consumer.handleMessage,
		nats.Durable(consumer.durable),
		nats.ManualAck(),
		nats.BindStream(consumer.streamName),
	)
	if err != nil {
		return fmt.Errorf("subscribe to nats error: %w", err)
	}

	<-ctx.Done()

	// Stop accepting new messages; let in-flight finish.
	_ = subject.Drain()

	waitDone := make(chan struct{})
	go func() {
		consumer.wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
	case <-time.After(consumer.shutdownWaitTimeout):
		// best-effort shutdown
	}

	_ = consumer.conn.Drain()
	consumer.conn.Close()
	return nil
}

func (consumer *JetStreamConsumer) waitStreamReady(ctx context.Context) error {
	var lastError error
	for attempt := 1; attempt <= waitStreamMaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		info, err := consumer.js.StreamInfo(consumer.streamName)
		if err == nil && info != nil {
			// should be logged
			_ = fmt.Errorf("error getting stream info: %w", err)
			return nil
		}
		lastError = err
		time.Sleep(waitStreamDelay)
	}
	return fmt.Errorf("steam %s not ready: %w", consumer.streamName, lastError)
}

func (consumer *JetStreamConsumer) handleMessage(message *nats.Msg) {
	consumer.wg.Add(1)
	defer consumer.wg.Done()

	metaData, err := message.Metadata()
	var seq uint64
	if err == nil && metaData != nil {
		seq = metaData.Sequence.Stream
	}

	// IMPORTANT: use independent ctx so shutdown doesn't cancel in-flight processing instantly.
	processCtx, cancel := context.WithTimeout(context.Background(), consumer.messageProcessTimeout)
	defer cancel()

	if err := consumer.handler.Handle(processCtx, message.Subject, seq, message.Data); err != nil {
		if errors.Is(err, ErrorMalformedPayload) {
			_ = message.Term()
			return
		}
		_ = message.Nak()
		return
	}
	_ = message.Ack()
}
