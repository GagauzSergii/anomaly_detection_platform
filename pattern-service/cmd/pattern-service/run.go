package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/app/command"
	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/app/processor"
	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/domain/detector"
	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/infra/config"
	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/infra/log"
	"github.com/sergii-gagauz/anomaly_detection_platform/pattern-service/internal/infra/natsjs"
)

func run() error {
	// root lifecycle context
	rootCtx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	cfg := config.Load()

	logger, err := log.New()
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer logger.Sync()

	// ---- NATS / JetStream ----
	jetStreamClient, err := natsjs.NewJetStreamClient(logger, cfg.NATSUrl)
	if err != nil {
		return fmt.Errorf("init jetstream client: %w", err)
	}
	defer jetStreamClient.Close()

	jetStreamContext, err := jetStreamClient.Stream()
	if err != nil {
		return fmt.Errorf("get jetstream context: %w", err)
	}

	anomalyPublisher, err := natsjs.NewJetStreamAnomalyPublisher(logger, jetStreamContext)
	if err != nil {
		return fmt.Errorf("init anomaly publisher: %w", err)
	}

	// ---- Detector ----
	madDetector := detector.NewMADDetector(detector.Config{
		WindowSize: cfg.MADWindowSize,
		Warmup:     cfg.MADWarmup,
		ThresholdK: cfg.MADThresholdK,
		Mode:       detector.DetectionMode(cfg.MADMode),

		MinMad:   cfg.MinMAD,
		MinDelta: cfg.MinDelta,
	})

	// ---- Command handler ----
	processMetricHandler := command.NewProcessMetricRecordHandler(
		madDetector,
		anomalyPublisher,
		command.HandlerConfig{
			DetectorName: "MAD",
			ThresholdK:   cfg.MADThresholdK,
			Producer:     "pattern-service",
		},
	)

	metricProcessor := processor.NewMADMetricRecordedProcessor(processMetricHandler)

	// ---- Consumer ----
	metricsConsumer, err := natsjs.NewJetStreamConsumer(
		logger,
		cfg.NATSUrl,
		metricProcessor,
	)
	if err != nil {
		return fmt.Errorf("init metrics consumer: %w", err)
	}

	// ---- HTTP (health only) ----
	httpServer := startHTTPServer(logger, cfg.HTTPPort)

	// ---- Run background workers ----
	go func() {
		if err := metricsConsumer.Start(rootCtx); err != nil && rootCtx.Err() == nil {
			logger.Errorw("metrics consumer stopped with error", "error", err)
			stop()
		}
	}()

	// ---- Cleanup worker ----
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-rootCtx.Done():
				return
			case <-ticker.C:
				// Cleanup series not seen for 1 hour
				cleaned := madDetector.Cleanup(1 * time.Hour)
				if cleaned > 0 {
					logger.Infow("cleaned up stale series", "count", cleaned)
				}
			}
		}
	}()

	<-rootCtx.Done()
	logger.Infow("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = httpServer.Shutdown(shutdownCtx)

	logger.Infow("pattern-service stopped gracefully")
	return nil
}

func startHTTPServer(logger log.Interface, port string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		logger.Infow("starting http server", "port", port, "service", "pattern-service")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorw("http server error", "error", err)
		}
	}()

	return server
}
