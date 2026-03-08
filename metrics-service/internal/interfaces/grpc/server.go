package grpc

import (
	"context"

	metricsv1 "github.com/sergii-gagauz/anomaly_detection_platform/api/gen/metrics/v1"
	"github.com/sergii-gagauz/anomaly_detection_platform/metrics-service/internal/app/command"
	"github.com/sergii-gagauz/anomaly_detection_platform/metrics-service/internal/infra/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MetricsServer implements the generated MetricsServiceServer interface.
// All dependencies are injected explicitly via the constructor — no global state.
type MetricsServer struct {
	metricsv1.UnimplementedMetricsServiceServer

	handler command.RecordMetricCommandHandler
	logger  log.Interface
}

// NewMetricsServer creates a MetricsServer with explicit dependency injection.
func NewMetricsServer(handler command.RecordMetricCommandHandler, logger log.Interface) *MetricsServer {
	return &MetricsServer{
		handler: handler,
		logger:  logger,
	}
}

// RecordMetric maps the gRPC request to the domain command and delegates to the handler.
func (s *MetricsServer) RecordMetric(ctx context.Context, req *metricsv1.RecordMetricRequest) (*metricsv1.RecordMetricResponse, error) {
	cmd := command.RecordMetricCommand{
		Source:     req.GetSource(),
		MetricName: req.GetMetricName(),
		Value:      req.GetValue(),
		Env:        req.GetEnv(),
		Region:     req.GetRegion(),
		InstanceID: req.GetInstanceId(),
		Timestamp:  req.GetTimestamp(),
	}

	if err := s.handler.Handle(ctx, cmd); err != nil {
		s.logger.Errorw("failed to handle RecordMetric RPC",
			"error", err,
			"source", req.GetSource(),
			"metric_name", req.GetMetricName(),
		)
		return nil, status.Errorf(codes.Internal, "failed to record metric: %v", err)
	}

	return &metricsv1.RecordMetricResponse{Accepted: true}, nil
}
