package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand/v2"
	"os/signal"
	"syscall"
	"time"

	metricsv1 "github.com/sergii-gagauz/anomaly_detection_platform/api/gen/metrics/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Parse CLI arguments
	target := flag.String("target", "localhost:50051", "gRPC server address (host:port)")
	flag.Parse()

	// Initialize a production-ready structured logger (Zap).
	// Bypasses the standard 'log' package to enforce observability standards.
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sugar := logger.Sugar()

	// Run the main application logic and handle fatal errors gracefully
	if err := run(*target, sugar); err != nil {
		sugar.Fatalw("metrics-feeder shutdown with error", "error", err)
	}
}

func run(target string, logger *zap.SugaredLogger) error {
	// 1. Establish a single root context for Graceful Shutdown.
	// It listens for SIGINT (Ctrl+C) and SIGTERM (Kubernetes pod termination).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 2. Initialize the gRPC client connection.
	logger.Infow("dialing grpc server", "target", target)

	// Note: insecure.NewCredentials() is used here for local development.
	// In production, mTLS or standard TLS credentials should be injected.
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("grpc dial %s: %w", target, err)
	}
	// Ensure the connection is closed when the service stops to prevent resource leaks
	defer func() {
		if err := conn.Close(); err != nil {
			logger.Errorw("failed to close grpc connection", "error", err)
		}
	}()

	client := metricsv1.NewMetricsServiceClient(conn)
	logger.Infow("metrics-feeder connected, starting simulation loop", "interval_ms", 500)

	// 3. Simulation Loop
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	seq := 0
	for {
		select {
		case <-ctx.Done():
			// Shutdown signal received; exit the loop cleanly
			logger.Info("shutdown signal received, stopping feeder")
			return nil

		case <-ticker.C:
			seq++

			var value float64

			// =====================================================================
			// DEMO MODE (Like the curl script: flatline at ~100, spikes to ~1000)
			// =====================================================================

			// nolint:gosec // acceptable for simulation
			value = 100.0 + (rand.Float64() * 2.0) // Baseline: 100.0 - 102.0

			// With a probability of 5%, we generate a huge spike
			// nolint:gosec // acceptable for simulation
			if rand.Float64() < 0.05 {
				value = 1000.0 + (rand.Float64() * 50.0) // Anomaly: 1000.0 - 1050.0
				logger.Warnw("injecting HUGE demo anomaly spike", "seq", seq, "value", value)
			}

			// =====================================================================
			// REALISTIC NOISE MODE (Working logic for normal testing)
			// =====================================================================
			/*
				// nolint:gosec
				value = 30.0 + rand.Float64()*10.0 // Baseline: 30.0 - 40.0

				// nolint:gosec
				if rand.Float64() < 0.05 {
					value = 90.0 + rand.Float64()*10.0 // Anomaly: 90.0 - 100.0
					logger.Warnw("injecting realistic anomaly spike", "seq", seq, "value", value)
				}
			*/
			// =====================================================================

			req := &metricsv1.RecordMetricRequest{
				Source:     "metrics-feeder",
				MetricName: "cpu_usage_percent",
				Value:      value,
				Env:        "dev",
				Region:     "local",
				InstanceId: "feeder-001",
				Timestamp:  time.Now().Unix(), // FIXED: time.Now().UnixMilli() -> Unix()
			}

			// Apply a strict timeout to the network call.
			// Never pass the root background context to external RPCs to prevent hanging goroutines.
			reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			resp, err := client.RecordMetric(reqCtx, req)
			cancel() // Release context resources immediately after the RPC completes

			if err != nil {
				logger.Errorw("RecordMetric RPC failed",
					"seq", seq,
					"error", err,
				)
				continue
			}

			logger.Infow("metric recorded",
				"seq", seq,
				"success", resp.GetAccepted(), // FIXED: resp.GetSuccess() -> GetAccepted()
				"value", fmt.Sprintf("%.2f", value),
			)
		}
	}
}
