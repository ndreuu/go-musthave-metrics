// Package grpcmetrics предоставляет gRPC-сервис пакетного обновления метрик.
package grpcmetrics

import (
	"context"
	"errors"
	"math"
	"time"

	models "go-musthave-metrics/internal/model"
	"go-musthave-metrics/internal/proto"
	"go-musthave-metrics/internal/service"
	"go-musthave-metrics/internal/service/audit"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// MetricsServer принимает метрики через gRPC и сохраняет их общим сервисом.
type MetricsServer struct {
	proto.UnimplementedMetricsServer
	metricsService *service.MetricsService
	auditService   *audit.AuditService
}

// NewMetricsServer создает сервис; auditService может быть nil.
func NewMetricsServer(metricsService *service.MetricsService, auditService *audit.AuditService) *MetricsServer {
	return &MetricsServer{metricsService: metricsService, auditService: auditService}
}

// UpdateMetrics проверяет весь батч перед сохранением, чтобы некорректная метрика
// не приводила к частичному обновлению остальных метрик.
func (s *MetricsServer) UpdateMetrics(ctx context.Context, req *proto.UpdateMetricsRequest) (*proto.UpdateMetricsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "metrics request is missing")
	}

	metrics := make([]models.Metrics, len(req.Metrics))
	for i, metric := range req.Metrics {
		if metric == nil || metric.Id == "" {
			return nil, status.Errorf(codes.InvalidArgument, "metric %d must have an ID", i)
		}
		metrics[i].ID = metric.Id
		switch metric.Type {
		case proto.Metric_GAUGE:
			if math.IsNaN(metric.Value) || math.IsInf(metric.Value, 0) {
				return nil, status.Errorf(codes.InvalidArgument, "gauge metric %q must have a finite value", metric.Id)
			}
			value := metric.Value
			metrics[i].MType = models.Gauge
			metrics[i].Value = &value
		case proto.Metric_COUNTER:
			delta := metric.Delta
			metrics[i].MType = models.Counter
			metrics[i].Delta = &delta
		default:
			return nil, status.Errorf(codes.InvalidArgument, "metric %q has an unknown type", metric.Id)
		}
	}

	if len(metrics) == 0 {
		return &proto.UpdateMetricsResponse{}, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}
	if err := s.metricsService.UpdateMetricsBatch(ctx, metrics); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, status.FromContextError(err).Err()
		}
		return nil, status.Errorf(codes.Internal, "failed to store metrics: %v", err)
	}

	if s.auditService != nil {
		metricNames := make([]string, len(metrics))
		for i, metric := range metrics {
			metricNames[i] = metric.ID
		}
		md, _ := metadata.FromIncomingContext(ctx)
		var ip string
		if values := md.Get("x-real-ip"); len(values) > 0 {
			ip = values[0]
		}
		s.auditService.NotifyObservers(&models.AuditEvent{
			Timestamp: time.Now().Unix(),
			Metrics:   metricNames,
			IPAddress: ip,
		})
	}

	return &proto.UpdateMetricsResponse{}, nil
}
