package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"math"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	pb "go-musthave-metrics/internal/proto"
	"go-musthave-metrics/internal/testutil"

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

func startGRPCSenderTestServer(t *testing.T, network, address string, update func(context.Context, *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error)) (string, string) {
	t.Helper()
	listener, err := net.Listen(network, address)
	if err != nil {
		if network == "tcp6" {
			t.Skipf("IPv6 is unavailable: %v", err)
		}
		t.Fatalf("listen: %v", err)
	}
	certFile, keyFile := testutil.NewTLSFiles(t)
	serverCredentials, err := credentials.NewServerTLSFromFile(certFile, keyFile)
	if err != nil {
		t.Fatalf("server TLS credentials: %v", err)
	}
	server := grpc.NewServer(grpc.Creds(serverCredentials))
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
	return listener.Addr().String(), certFile
}

func newGRPCSenderForTest(t *testing.T, address, caFile string) *GRPCSender {
	t.Helper()
	sender, err := NewGRPCSender(address, caFile)
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
	tlsInfo, ok := remote.AuthInfo.(credentials.TLSInfo)
	if !ok || tlsInfo.State.Version < tls.VersionTLS12 {
		return status.Error(codes.Unauthenticated, "TLS 1.2 or newer is required")
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
	address, caFile := startGRPCSenderTestServer(t, "tcp4", "127.0.0.1:0", func(ctx context.Context, request *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
		if err := checkGRPCSenderRealIP(ctx); err != nil {
			return nil, err
		}
		deadline, ok := ctx.Deadline()
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "request deadline is missing")
		}
		deadlines <- time.Until(deadline)
		requests <- request
		return pb.UpdateMetricsResponse_builder{}.Build(), nil
	})
	sender := newGRPCSenderForTest(t, address, caFile)
	metrics := []*Metric{
		{Name: "Alloc", MType: "gauge", Value: 123.456},
		{Name: "PollCount", MType: "counter", Value: 42},
		{Name: "NegativeCounter", MType: "counter", Value: -3.8},
	}
	if err := sender.SendBatch(metrics); err != nil {
		t.Fatalf("send batch: %v", err)
	}
	request := <-requests
	if len(request.GetMetrics()) != len(metrics) {
		t.Fatalf("received %d metrics, want %d", len(request.GetMetrics()), len(metrics))
	}
	for i, metric := range metrics {
		got := request.GetMetrics()[i]
		if got.GetId() != metric.Name {
			t.Errorf("metric %d: name = %q, want %q", i, got.GetId(), metric.Name)
		}
		switch metric.MType {
		case "gauge":
			if got.GetType() != pb.Metric_GAUGE || got.GetValue() != metric.Value || got.GetDelta() != 0 {
				t.Errorf("gauge mapping: %v", got)
			}
		case "counter":
			if got.GetType() != pb.Metric_COUNTER || got.GetDelta() != int64(metric.Value) || got.GetValue() != 0 {
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
			address, caFile := startGRPCSenderTestServer(t, tt.network, tt.address, func(ctx context.Context, request *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
				if err := checkGRPCSenderRealIP(ctx); err != nil {
					return nil, err
				}
				remote, _ := peer.FromContext(ctx)
				peers <- remote.Addr.String()
				return pb.UpdateMetricsResponse_builder{}.Build(), nil
			})
			sender := newGRPCSenderForTest(t, address, caFile)
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
	address, caFile := startGRPCSenderTestServer(t, "tcp4", "127.0.0.1:0", func(context.Context, *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
		requests <- struct{}{}
		return pb.UpdateMetricsResponse_builder{}.Build(), nil
	})
	sender := newGRPCSenderForTest(t, address, caFile)
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
	address, caFile := startGRPCSenderTestServer(t, "tcp4", "127.0.0.1:0", func(context.Context, *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
		return nil, status.Error(codes.PermissionDenied, "untrusted subnet")
	})
	sender := newGRPCSenderForTest(t, address, caFile)
	err := sender.SendBatch([]*Metric{{Name: "Alloc", MType: "gauge", Value: 1}})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("status = %s, want PermissionDenied; error: %v", status.Code(err), err)
	}
}

func TestGRPCSenderClose(t *testing.T) {
	address, caFile := startGRPCSenderTestServer(t, "tcp4", "127.0.0.1:0", func(context.Context, *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
		return pb.UpdateMetricsResponse_builder{}.Build(), nil
	})
	sender, err := NewGRPCSender(address, caFile)
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
		if sender, err := NewGRPCSender(address, ""); err == nil || sender != nil {
			t.Errorf("address %q: sender = %v, error = %v", address, sender, err)
		}
	}
}

func TestNewGRPCSenderInvalidCAFile(t *testing.T) {
	invalidCA := filepath.Join(t.TempDir(), "invalid.pem")
	if err := os.WriteFile(invalidCA, []byte("not a certificate"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, caFile := range []string{filepath.Join(t.TempDir(), "missing.pem"), invalidCA} {
		if sender, err := NewGRPCSender("localhost:8081", caFile); err == nil || sender != nil {
			t.Errorf("CA file %q: sender = %v, error = %v", caFile, sender, err)
		}
	}
}

func TestGRPCSenderRejectsUntrustedCertificate(t *testing.T) {
	address, _ := startGRPCSenderTestServer(t, "tcp4", "127.0.0.1:0", func(context.Context, *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
		t.Error("untrusted connection must not invoke the service")
		return pb.UpdateMetricsResponse_builder{}.Build(), nil
	})
	unrelatedCA, _ := testutil.NewTLSFiles(t)
	for _, tt := range []struct {
		name   string
		caFile string
	}{
		{name: "system roots"},
		{name: "unrelated CA", caFile: unrelatedCA},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sender := newGRPCSenderForTest(t, address, tt.caFile)
			err := sender.SendBatch([]*Metric{{Name: "Alloc", MType: "gauge", Value: 1}})
			if status.Code(err) != codes.Unavailable || !strings.Contains(err.Error(), "certificate") {
				t.Fatalf("expected certificate verification failure, got %v", err)
			}
		})
	}
}

func TestLocalIPCredentialsRejectsWrongHostname(t *testing.T) {
	address, caFile := startGRPCSenderTestServer(t, "tcp4", "127.0.0.1:0", func(context.Context, *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
		t.Error("invalid hostname must not invoke the service")
		return pb.UpdateMetricsResponse_builder{}.Build(), nil
	})
	certPEM, err := os.ReadFile(caFile)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certPEM) {
		t.Fatal("invalid test certificate")
	}
	transport := localIPCredentials{TransportCredentials: credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    roots,
	})}
	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, _, err := transport.ClientHandshake(ctx, "wrong.example:443", conn); err == nil || !strings.Contains(err.Error(), "wrong.example") {
		t.Fatalf("expected hostname verification failure, got %v", err)
	}
}

func TestGRPCSenderValidatesPerRPCAuthority(t *testing.T) {
	address, caFile := startGRPCSenderTestServer(t, "tcp4", "127.0.0.1:0", func(ctx context.Context, _ *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
		return pb.UpdateMetricsResponse_builder{}.Build(), checkGRPCSenderRealIP(ctx)
	})
	sender := newGRPCSenderForTest(t, address, caFile)
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	if _, err := sender.client.UpdateMetrics(ctx, pb.UpdateMetricsRequest_builder{}.Build(), grpc.CallAuthority("localhost")); err != nil {
		t.Fatalf("valid authority: %v", err)
	}
	if _, err := sender.client.UpdateMetrics(ctx, pb.UpdateMetricsRequest_builder{}.Build(), grpc.CallAuthority("wrong.example")); status.Code(err) != codes.Unavailable {
		t.Fatalf("expected rejection of unverified authority, got %v", err)
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
	if !(realIPCredentials{}).RequireTransportSecurity() {
		t.Error("real IP metadata must require TLS")
	}
}
