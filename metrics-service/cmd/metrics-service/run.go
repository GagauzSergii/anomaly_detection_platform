package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"

	metricsv1 "github.com/sergii-gagauz/anomaly_detection_platform/api/gen/metrics/v1"
	"github.com/sergii-gagauz/anomaly_detection_platform/metrics-service/internal/app/command"
	"github.com/sergii-gagauz/anomaly_detection_platform/metrics-service/internal/infra/config"
	mlog "github.com/sergii-gagauz/anomaly_detection_platform/metrics-service/internal/infra/log"
	"github.com/sergii-gagauz/anomaly_detection_platform/metrics-service/internal/infra/natsjs"
	metricsgrpc "github.com/sergii-gagauz/anomaly_detection_platform/metrics-service/internal/interfaces/grpc"
)

func run() error {
	// Context that can be canceled by SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	logger, err := mlog.New()
	if err != nil {
		// Nothing to log with zap if those one was not created
		// Lets return error in main
		return fmt.Errorf("failed to init logger: %w", err)
	}

	// lets flush logger buffer during run out
	defer func() {
		_ = logger.Sync()
	}()

	// Application wiring
	natsPublisher, err := natsjs.NewNatsEventPublisher(logger, cfg.NATSUrl)
	if err != nil {
		return fmt.Errorf("failed to init nats publisher: %w", err)
	}
	defer func(natsPublisher *natsjs.NatsEventPublisher) {
		err := natsPublisher.Close()
		if err != nil {
			fmt.Printf("failed to close nats publisher from run(): %v", err)
		}
	}(natsPublisher)

	recordMetricHandler := command.NewRecordMetricCommandHandler(natsPublisher)

	// ── gRPC server ─────────────────────────────────────────────────────
	grpcSrv := grpc.NewServer()
	metricsServer := metricsgrpc.NewMetricsServer(recordMetricHandler, logger)
	metricsv1.RegisterMetricsServiceServer(grpcSrv, metricsServer)

	grpcLis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("failed to listen on grpc port %s: %w", cfg.GRPCPort, err)
	}

	go func() {
		logger.Infow("starting grpc server",
			"port", cfg.GRPCPort,
			"service", "metrics-service")

		if err := grpcSrv.Serve(grpcLis); err != nil {
			logger.Errorw("grpc server error", "error", err)
			stop()
		}
	}()

	// ── HTTP server (Gin) ───────────────────────────────────────────────
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Standard middleware : logging of requests and recover from panic
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// healthcheck
	router.GET("/v1/health", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "OK")
	})

	// temporary debug endpoint for RecordMetric Call thought HTTP+JSON
	router.POST("/v1/debug/recordMetric", func(ctx *gin.Context) {
		var request struct {
			Source     string  `json:"source"`
			MetricName string  `json:"metric_name"`
			Value      float64 `json:"value"`
			Env        string  `json:"env"`
			Region     string  `json:"region"`
			InstanceID string  `json:"instance_id"`
			Timestamp  int64   `json:"timestamp"`
		}

		if err := ctx.ShouldBindJSON(&request); err != nil {
			logger.Errorw("failed to bind request", "error", err)
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		cmd := command.RecordMetricCommand{
			Source:     request.Source,
			MetricName: request.MetricName,
			Value:      request.Value,
			Env:        request.Env,
			Region:     request.Region,
			InstanceID: request.InstanceID,
			Timestamp:  request.Timestamp,
		}

		if err := recordMetricHandler.Handle(ctx, cmd); err != nil {
			logger.Errorw("failed to handle request", "error", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"message": "OK"})
	})

	srv := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: router,
	}

	// HTTP server start on separate goroutine
	go func() {
		logger.Infow("starting http server",
			"port", cfg.HTTPPort,
			"service", "metrics-service")

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorw("http server error", "error", err)
			// Context break and graceful shutdown
			stop()
		}
	}()

	// ── Graceful shutdown ───────────────────────────────────────────────
	<-ctx.Done()
	logger.Infow("shutting down — signal received, stopping servers gracefully")

	// 1. Stop accepting new gRPC calls, drain in-flight.
	grpcSrv.GracefulStop()
	logger.Infow("grpc server stopped")

	// 2. Shut down HTTP with a 10s deadline.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Errorw("http server shutdown", "error", err)
	}

	logger.Infow("metrics-service stopped gracefully")
	return nil
}
