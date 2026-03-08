package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/sergii-gagauz/anomaly_detection_platform/read-service/internal/modules/projections/anomaly_detected"
	"github.com/sergii-gagauz/anomaly_detection_platform/read-service/internal/shared/config"
	"github.com/sergii-gagauz/anomaly_detection_platform/read-service/internal/shared/logging"
	"github.com/sergii-gagauz/anomaly_detection_platform/read-service/internal/shared/messaging/natsjs"
	"github.com/sergii-gagauz/anomaly_detection_platform/read-service/internal/shared/postgres"
	"github.com/sergii-gagauz/anomaly_detection_platform/read-service/internal/transport/http/gin"
)

func main() {
	logging.Setup() // ATG Update: Initialize structured logging
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Postgres pool (runtime)
	pool, err := postgres.NewPool(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("postgres pool: %v", err)
	}
	defer pool.Close()

	// HTTP server
	router := gin.NewRouter(pool)
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Projector + consumer handler
	projector := anomaly_detected.NewProjector(pool)
	handler := anomaly_detected.NewHandler(projector)

	consumer, err := natsjs.NewJetStreamConsumer(
		cfg.NATSURL,
		cfg.AnomaliesStream,
		cfg.AnomaliesSubject,
		cfg.DurableName,
		handler,
	)
	if err != nil {
		log.Fatalf("nats consumer: %v", err)
	}

	// Start NATS consumer
	go func() {
		if err := consumer.Start(ctx); err != nil {
			log.Printf("consumer stopped: %v", err)
			stop()
		}
	}()

	// Start HTTP
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("http error: %v", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
