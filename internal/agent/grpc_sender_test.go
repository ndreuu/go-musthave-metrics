package agent

import (
	"context"
	"math"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	pb "go-musthave-metrics/internal/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type testGRPCSenderService struct {
	pb.UnimplementedMetricsServer
	update func(context.Context, *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error)
}

func (s *testGRPCSenderService) UpdateMetrics(ctx context.Context, request *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
	return s.update(ctx, request)
}

func startGRPCSenderTestServer(t *testing.T, network, address string, update func(context.Context, *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error)) string {
	t.Helper()
	listener, err := net.Listen(network, address)
	if err != nil {
		if network == "tcp6" {
			t.Skipf("IPv6 is unavailable: %v", err)
		}
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	pb.RegisterMetricsServer(server, &testGRPCSenderService{update: update})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		<-done
	})
	return listener.Addr().String()
}

func newGRPCSenderForTest(t *testing.T, address string) *GRPCSender {
	t.Helper()
	sender, err := NewGRPCSender(address)
	if err != nil {
		t.Fatalf("create sender: %v", err)
	}
	t.Cleanup(func() { _ = sender.Close() })
	return sender
}

func checkGRPCSenderRealIP(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok || len(md.Get("x-real-ip")) != 1 {
		return status.Error(codes.InvalidArgument, "expected exactly one x-real-ip metadata value")
	}
	actual, err := netip.ParseAddr(md.Get("x-real-ip")[0])
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid x-real-ip: %v", err)
	}
	remote, ok := peer.FromContext(ctx)
	if !ok {
		return status.Error(codes.InvalidArgument, "missing connection peer")
	}
	remoteIP, err := netip.ParseAddrPort(remote.Addr.String())
	if err != nil || actual.Unmap() != remoteIP.Addr().WithZone("").Unmap() {
		return status.Errorf(codes.InvalidArgument, "x-real-ip %s does not match peer %s", actual, remote.Addr)
	}
	return nil
}

func TestGRPCSenderSendBatch(t *testing.T) {
	requests := make(chan *pb.UpdateMetricsRequest, 2)
	deadlines := make(chan time.Duration, 2)
	address := startGRPCSenderTestServer(t, "tcp4", "127.0.0.1:0", func(ctx context.Context, request *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
		if err := checkGRPCSenderRealIP(ctx); err != nil {
			return nil, err
		}
		deadline, ok := ctx.Deadline()
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "request deadline is missing")
		}
		deadlines <- time.Until(deadline)
		requests <- request
		return &pb.UpdateMetricsResponse{}, nil
	})
	sender := newGRPCSenderForTest(t, address)
	metrics := []*Metric{
		{Name: "Alloc", MType: "gauge", Value: 123.456},
		{Name: "PollCount", MType: "counter", Value: 42},
		{Name: "NegativeCounter", MType: "counter", Value: -3.8},
	}
	if err := sender.SendBatch(metrics); err != nil {
		t.Fatalf("send batch: %v", err)
	}
	request := <-requests
	if len(request.Metrics) != len(metrics) {
		t.Fatalf("received %d metrics, want %d", len(request.Metrics), len(metrics))
	}
	for i, metric := range metrics {
		got := request.Metrics[i]
		if got.Id != metric.Name {
			t.Errorf("metric %d: name = %q, want %q", i, got.Id, metric.Name)
		}
		switch metric.MType {
		case "gauge":
			if got.Type != pb.Metric_GAUGE || got.Value != metric.Value || got.Delta != 0 {
				t.Errorf("gauge mapping: %v", got)
			}
		case "counter":
			if got.Type != pb.Metric_COUNTER || got.Delta != int64(metric.Value) || got.Value != 0 {
				t.Errorf("counter mapping: %v", got)
			}
		}
	}
	if remaining := <-deadlines; remaining <= 0 || remaining > requestTimeout+time.Second {
		t.Errorf("unexpected deadline duration: %s", remaining)
	}
	select {
	case <-requests:
		t.Error("one batch resulted in multiple RPCs")
	default:
	}
}

func TestGRPCSenderRealIPConcurrentAndReusedConnection(t *testing.T) {
	for _, tt := range []struct {
		name    string
		network string
		address string
	}{
		{name: "IPv4", network: "tcp4", address: "127.0.0.1:0"},
		{name: "IPv6", network: "tcp6", address: "[::1]:0"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			const calls = 16
			peers := make(chan string, calls)
			address := startGRPCSenderTestServer(t, tt.network, tt.address, func(ctx context.Context, request *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
				if err := checkGRPCSenderRealIP(ctx); err != nil {
					return nil, err
				}
				remote, _ := peer.FromContext(ctx)
				peers <- remote.Addr.String()
				return &pb.UpdateMetricsResponse{}, nil
			})
			sender := newGRPCSenderForTest(t, address)
			var wg sync.WaitGroup
			errors := make(chan error, calls)
			for range calls {
				wg.Go(func() {
					errors <- sender.SendBatch([]*Metric{{Name: "Alloc", MType: "gauge", Value: 1}})
				})
			}
			wg.Wait()
			close(errors)
			for err := range errors {
				if err != nil {
					t.Fatalf("concurrent batch: %v", err)
				}
			}
			close(peers)
			var connection string
			for remote := range peers {
				if connection == "" {
					connection = remote
				} else if remote != connection {
					t.Errorf("connection was not reused: %s and %s", connection, remote)
				}
			}
		})
	}
}

func TestGRPCSenderRejectsInvalidBatchWithoutRPC(t *testing.T) {
	requests := make(chan struct{}, 32)
	address := startGRPCSenderTestServer(t, "tcp4", "127.0.0.1:0", func(context.Context, *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
		requests <- struct{}{}
		return &pb.UpdateMetricsResponse{}, nil
	})
	sender := newGRPCSenderForTest(t, address)
	for _, tt := range []struct {
		metric *Metric
		name   string
	}{
		{name: "nil"},
		{name: "empty name", metric: &Metric{MType: "gauge", Value: 1}},
		{name: "unknown type", metric: &Metric{Name: "Alloc", MType: "unknown", Value: 1}},
		{name: "NaN", metric: &Metric{Name: "Alloc", MType: "gauge", Value: math.NaN()}},
		{name: "infinite gauge", metric: &Metric{Name: "Alloc", MType: "gauge", Value: math.Inf(1)}},
		{name: "infinite counter", metric: &Metric{Name: "PollCount", MType: "counter", Value: math.Inf(-1)}},
		{name: "counter overflow", metric: &Metric{Name: "PollCount", MType: "counter", Value: 0x1p63}},
		{name: "counter underflow", metric: &Metric{Name: "PollCount", MType: "counter", Value: -0x1p64}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// A valid prefix must not be sent when another metric is invalid.
			metrics := []*Metric{{Name: "Alloc", MType: "gauge", Value: 1}, tt.metric}
			if err := sender.SendBatch(metrics); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	if err := sender.SendBatch(nil); err != nil {
		t.Errorf("empty batch: %v", err)
	}
	select {
	case <-requests:
		t.Error("invalid or empty batch caused an RPC")
	default:
	}
}

func TestGRPCSenderPreservesStatusError(t *testing.T) {
	address := startGRPCSenderTestServer(t, "tcp4", "127.0.0.1:0", func(context.Context, *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
		return nil, status.Error(codes.PermissionDenied, "untrusted subnet")
	})
	sender := newGRPCSenderForTest(t, address)
	err := sender.SendBatch([]*Metric{{Name: "Alloc", MType: "gauge", Value: 1}})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("status = %s, want PermissionDenied; error: %v", status.Code(err), err)
	}
}

func TestGRPCSenderClose(t *testing.T) {
	address := startGRPCSenderTestServer(t, "tcp4", "127.0.0.1:0", func(context.Context, *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
		return &pb.UpdateMetricsResponse{}, nil
	})
	sender, err := NewGRPCSender(address)
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := sender.SendBatch([]*Metric{{Name: "Alloc", MType: "gauge", Value: 1}}); err == nil {
		t.Fatal("send after close unexpectedly succeeded")
	}
	if err := sender.SendBatch(nil); err != nil {
		t.Errorf("empty batch after close: %v", err)
	}
}

func TestNewGRPCSenderEmptyAddress(t *testing.T) {
	for _, address := range []string{"", "   "} {
		if sender, err := NewGRPCSender(address); err == nil || sender != nil {
			t.Errorf("address %q: sender = %v, error = %v", address, sender, err)
		}
	}
}

func TestRealIPCredentialsMissingConnectionInfo(t *testing.T) {
	for _, ctx := range []context.Context{
		context.Background(),
		credentials.NewContextWithRequestInfo(context.Background(), credentials.RequestInfo{}),
	} {
		if _, err := (realIPCredentials{}).GetRequestMetadata(ctx); err == nil {
			t.Error("missing connection IP must return an error")
		}
	}
	if (realIPCredentials{}).RequireTransportSecurity() {
		t.Error("real IP metadata must support the configured insecure transport")
	}
}
