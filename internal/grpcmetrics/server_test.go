package grpcmetrics_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"reflect"
	"testing"
	"time"

	"go-musthave-metrics/internal/grpcmetrics"
	models "go-musthave-metrics/internal/model"
	"go-musthave-metrics/internal/proto"
	"go-musthave-metrics/internal/repository"
	"go-musthave-metrics/internal/service"
	"go-musthave-metrics/internal/service/audit"
	"go-musthave-metrics/internal/testutil"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type batchStorage struct {
	*repository.MemStorage
	err     error
	metrics []models.Metrics
	calls   int
}

func (s *batchStorage) UpdateMetricsBatch(ctx context.Context, metrics []models.Metrics) error {
	s.calls++
	s.metrics = metrics
	if s.err != nil {
		return s.err
	}
	return s.MemStorage.UpdateMetricsBatch(ctx, metrics)
}

type auditObserver struct {
	events []*models.AuditEvent
}

func (o *auditObserver) Notify(event *models.AuditEvent) error {
	o.events = append(o.events, event)
	return nil
}

func (o *auditObserver) Close() error { return nil }

func TestUpdateMetricsBatch(t *testing.T) {
	storage := &batchStorage{MemStorage: repository.NewMemStorage("")}
	auditService := audit.NewAuditService()
	observer := &auditObserver{}
	auditService.AddObserver(observer)
	server := grpcmetrics.NewMetricsServer(service.NewMetricsService(storage), auditService)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-real-ip", "192.0.2.10"))

	for _, req := range []*proto.UpdateMetricsRequest{
		proto.UpdateMetricsRequest_builder{Metrics: []*proto.Metric{
			proto.Metric_builder{Id: "temperature", Type: proto.Metric_GAUGE, Value: 23.5}.Build(),
			proto.Metric_builder{Id: "requests", Type: proto.Metric_COUNTER, Delta: 3}.Build(),
			proto.Metric_builder{Id: "requests", Type: proto.Metric_COUNTER, Delta: 2}.Build(),
			proto.Metric_builder{Id: "zero", Type: proto.Metric_GAUGE}.Build(),
			proto.Metric_builder{Id: "zeroCounter", Type: proto.Metric_COUNTER}.Build(),
		}}.Build(),
		proto.UpdateMetricsRequest_builder{Metrics: []*proto.Metric{
			proto.Metric_builder{Id: "temperature", Type: proto.Metric_GAUGE, Value: -1}.Build(),
			proto.Metric_builder{Id: "requests", Type: proto.Metric_COUNTER, Delta: -1}.Build(),
		}}.Build(),
	} {
		resp, err := server.UpdateMetrics(ctx, req)
		if err != nil || resp == nil {
			t.Fatalf("UpdateMetrics() = %v, %v", resp, err)
		}
	}
	if storage.calls != 2 {
		t.Fatalf("storage calls = %d, want one call per batch", storage.calls)
	}
	if len(storage.metrics) != 2 || storage.metrics[0].Delta != nil || storage.metrics[1].Value != nil {
		t.Fatalf("incorrect model conversion: %+v", storage.metrics)
	}
	if got, err := storage.GetCounter(ctx, "requests"); err != nil || got != 4 {
		t.Fatalf("counter = %d, %v; want 4", got, err)
	}
	if got, err := storage.GetGauge(ctx, "temperature"); err != nil || got != -1 {
		t.Fatalf("gauge = %v, %v; want -1", got, err)
	}
	if got, err := storage.GetGauge(ctx, "zero"); err != nil || got != 0 {
		t.Fatalf("zero gauge = %v, %v; want 0", got, err)
	}
	if got, err := storage.GetCounter(ctx, "zeroCounter"); err != nil || got != 0 {
		t.Fatalf("zero counter = %v, %v; want 0", got, err)
	}
	if len(observer.events) != 2 {
		t.Fatalf("audit events = %d, want 2", len(observer.events))
	}
	event := observer.events[0]
	if event.IPAddress != "192.0.2.10" || event.Timestamp == 0 || !reflect.DeepEqual(event.Metrics, []string{"temperature", "requests", "requests", "zero", "zeroCounter"}) {
		t.Fatalf("incorrect audit event: %+v", event)
	}
}

func TestUpdateMetricsRejectsInvalidBatchBeforeStorage(t *testing.T) {
	tests := []struct {
		metric *proto.Metric
		name   string
	}{
		{name: "nil metric"},
		{name: "empty ID", metric: proto.Metric_builder{Type: proto.Metric_COUNTER, Delta: 1}.Build()},
		{name: "unknown type", metric: proto.Metric_builder{Id: "bad", Type: proto.Metric_MType(99)}.Build()},
		{name: "NaN", metric: proto.Metric_builder{Id: "bad", Value: math.NaN()}.Build()},
		{name: "positive infinity", metric: proto.Metric_builder{Id: "bad", Value: math.Inf(1)}.Build()},
		{name: "negative infinity", metric: proto.Metric_builder{Id: "bad", Value: math.Inf(-1)}.Build()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &batchStorage{MemStorage: repository.NewMemStorage("")}
			server := grpcmetrics.NewMetricsServer(service.NewMetricsService(storage), nil)
			_, err := server.UpdateMetrics(context.Background(), proto.UpdateMetricsRequest_builder{Metrics: []*proto.Metric{
				proto.Metric_builder{Id: "valid", Type: proto.Metric_COUNTER, Delta: 5}.Build(), tt.metric,
			}}.Build())
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("code = %v, want InvalidArgument; error: %v", status.Code(err), err)
			}
			if storage.calls != 0 || len(storage.GetAll()) != 0 {
				t.Fatal("invalid batch touched storage")
			}
		})
	}
}

func TestUpdateMetricsEmptyAndMissingRequest(t *testing.T) {
	storage := &batchStorage{MemStorage: repository.NewMemStorage("")}
	server := grpcmetrics.NewMetricsServer(service.NewMetricsService(storage), nil)
	if _, err := server.UpdateMetrics(context.Background(), nil); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("nil request error = %v, want InvalidArgument", err)
	}
	if resp, err := server.UpdateMetrics(context.Background(), proto.UpdateMetricsRequest_builder{}.Build()); resp == nil || err != nil {
		t.Fatalf("empty batch = %v, %v", resp, err)
	}
	if storage.calls != 0 {
		t.Fatalf("storage calls = %d, want 0", storage.calls)
	}
}

func TestUpdateMetricsStorageErrors(t *testing.T) {
	for _, tt := range []struct {
		err  error
		name string
		code codes.Code
	}{
		{name: "storage error", err: errors.New("storage unavailable"), code: codes.Internal},
		{name: "cancelled", err: fmt.Errorf("store batch: %w", context.Canceled), code: codes.Canceled},
		{name: "deadline", err: fmt.Errorf("store batch: %w", context.DeadlineExceeded), code: codes.DeadlineExceeded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			storage := &batchStorage{MemStorage: repository.NewMemStorage(""), err: tt.err}
			auditService := audit.NewAuditService()
			observer := &auditObserver{}
			auditService.AddObserver(observer)
			server := grpcmetrics.NewMetricsServer(service.NewMetricsService(storage), auditService)
			_, err := server.UpdateMetrics(context.Background(), proto.UpdateMetricsRequest_builder{Metrics: []*proto.Metric{
				proto.Metric_builder{Id: "counter", Type: proto.Metric_COUNTER, Delta: 1}.Build(),
			}}.Build())
			if status.Code(err) != tt.code {
				t.Fatalf("code = %v, want %v; error: %v", status.Code(err), tt.code, err)
			}
			if storage.calls != 1 || len(observer.events) != 0 {
				t.Fatalf("storage calls = %d, audit events = %d", storage.calls, len(observer.events))
			}
		})
	}
}

func TestUpdateMetricsCancelledContext(t *testing.T) {
	storage := &batchStorage{MemStorage: repository.NewMemStorage("")}
	server := grpcmetrics.NewMetricsServer(service.NewMetricsService(storage), nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := server.UpdateMetrics(ctx, proto.UpdateMetricsRequest_builder{Metrics: []*proto.Metric{
		proto.Metric_builder{Id: "counter", Type: proto.Metric_COUNTER}.Build(),
	}}.Build())
	if status.Code(err) != codes.Canceled || storage.calls != 0 {
		t.Fatalf("error = %v, storage calls = %d; want Canceled, 0", err, storage.calls)
	}
}

func TestMetricsGRPCRoundTrip(t *testing.T) {
	storage := repository.NewMemStorage("")
	interceptor, err := grpcmetrics.TrustedSubnetInterceptor("192.0.2.0/24")
	if err != nil {
		t.Fatal(err)
	}
	certFile, keyFile := testutil.NewTLSFiles(t)
	serverCredentials, err := credentials.NewServerTLSFromFile(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	clientCredentials, err := credentials.NewClientTLSFromFile(certFile, "localhost")
	if err != nil {
		t.Fatal(err)
	}
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(grpc.Creds(serverCredentials), grpc.UnaryInterceptor(interceptor))
	proto.RegisterMetricsServer(server, grpcmetrics.NewMetricsServer(service.NewMetricsService(storage), nil))
	t.Cleanup(server.Stop)
	go func() { _ = server.Serve(listener) }()
	conn, err := grpc.NewClient("passthrough:///metrics", grpc.WithTransportCredentials(clientCredentials), grpc.WithAuthority("localhost"), grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return listener.DialContext(ctx)
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := proto.NewMetricsClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	trustedCtx := metadata.AppendToOutgoingContext(ctx, "x-real-ip", "192.0.2.12")
	req := proto.UpdateMetricsRequest_builder{Metrics: []*proto.Metric{
		proto.Metric_builder{Id: "gauge", Type: proto.Metric_GAUGE, Value: 12.25}.Build(),
		proto.Metric_builder{Id: "counter", Type: proto.Metric_COUNTER, Delta: 3}.Build(),
	}}.Build()
	if _, err := client.UpdateMetrics(trustedCtx, req); err != nil {
		t.Fatal(err)
	}
	if got, err := storage.GetGauge(ctx, "gauge"); err != nil || got != 12.25 {
		t.Fatalf("gauge = %v, %v; want 12.25", got, err)
	}
	if got, err := storage.GetCounter(ctx, "counter"); err != nil || got != 3 {
		t.Fatalf("counter = %v, %v; want 3", got, err)
	}
	if _, err := client.UpdateMetrics(metadata.AppendToOutgoingContext(ctx, "x-real-ip", "198.51.100.12"), req); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("untrusted request error = %v, want PermissionDenied", err)
	}
	if got, err := storage.GetCounter(ctx, "counter"); err != nil || got != 3 {
		t.Fatalf("untrusted request changed counter to %v, %v", got, err)
	}
}
