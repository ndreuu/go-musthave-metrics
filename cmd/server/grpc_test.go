package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go-musthave-metrics/internal/agent"
	serverconfig "go-musthave-metrics/internal/config/server"
	pb "go-musthave-metrics/internal/proto"
	"go-musthave-metrics/internal/service"
	"go-musthave-metrics/internal/testutil"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

type lifecycleMetricsServer struct {
	pb.UnimplementedMetricsServer
	update func(context.Context, *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error)
}

func (s *lifecycleMetricsServer) UpdateMetrics(ctx context.Context, request *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
	return s.update(ctx, request)
}

func listenGRPCLifecycle(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	return listener
}

func waitGRPCLifecycleResult(t *testing.T, result <-chan error, description string) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(3 * time.Second):
		t.Fatalf("%s did not finish within 3s", description)
		return nil
	}
}

func lifecycleTLS(t *testing.T) (grpc.ServerOption, grpc.DialOption) {
	t.Helper()
	certFile, keyFile := testutil.NewTLSFiles(t)
	serverCredentials, err := credentials.NewServerTLSFromFile(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	clientCredentials, err := credentials.NewClientTLSFromFile(certFile, "")
	if err != nil {
		t.Fatal(err)
	}
	return grpc.Creds(serverCredentials), grpc.WithTransportCredentials(clientCredentials)
}

func TestRunGRPCServerWaitsForActiveRequest(t *testing.T) {
	listener := listenGRPCLifecycle(t)
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	serverTLS, clientTLS := lifecycleTLS(t)
	server := grpc.NewServer(serverTLS)
	pb.RegisterMetricsServer(server, &lifecycleMetricsServer{
		update: func(ctx context.Context, _ *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
			close(started)
			select {
			case <-release:
				return pb.UpdateMetricsResponse_builder{}.Build(), nil
			case <-ctx.Done():
				return nil, status.FromContextError(ctx.Err()).Err()
			}
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		releaseOnce.Do(func() { close(release) })
		server.Stop()
	})
	stopped := make(chan error, 1)
	go func() { stopped <- runGRPCServer(ctx, server, listener) }()
	conn, err := grpc.NewClient(listener.Addr().String(), clientTLS)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	rpcCtx, cancelRPC := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelRPC()
	requestDone := make(chan error, 1)
	go func() {
		_, err := pb.NewMetricsClient(conn).UpdateMetrics(rpcCtx, pb.UpdateMetricsRequest_builder{}.Build())
		requestDone <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("RPC did not reach the server")
	}
	cancel()
	select {
	case err := <-stopped:
		t.Fatalf("server stopped before the active RPC finished: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	releaseOnce.Do(func() { close(release) })
	if err := waitGRPCLifecycleResult(t, requestDone, "active RPC"); err != nil {
		t.Fatalf("active RPC failed during graceful shutdown: %v", err)
	}
	if err := waitGRPCLifecycleResult(t, stopped, "gRPC shutdown"); err != nil {
		t.Fatalf("graceful shutdown failed: %v", err)
	}
}

func TestRunGRPCServerReturnsStartupError(t *testing.T) {
	listener := listenGRPCLifecycle(t)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	t.Cleanup(server.Stop)
	stopped := make(chan error, 1)
	go func() { stopped <- runGRPCServer(context.Background(), server, listener) }()
	if err := waitGRPCLifecycleResult(t, stopped, "gRPC startup failure"); err == nil || !strings.Contains(err.Error(), "serve gRPC") {
		t.Fatalf("expected gRPC startup error, got %v", err)
	}
}

func TestShutdownGRPCServerBoundsHandlerIgnoringCancellation(t *testing.T) {
	listener := listenGRPCLifecycle(t)
	started := make(chan struct{})
	release := make(chan struct{})
	handlerDone := make(chan struct{})
	var releaseOnce sync.Once
	serverTLS, clientTLS := lifecycleTLS(t)
	server := grpc.NewServer(serverTLS)
	pb.RegisterMetricsServer(server, &lifecycleMetricsServer{
		update: func(context.Context, *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
			close(started)
			defer close(handlerDone)
			<-release
			return pb.UpdateMetricsResponse_builder{}.Build(), nil
		},
	})
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		server.Stop()
		// Wait for the timed-out GracefulStop goroutine to finish after the
		// deliberately uncooperative handler is released.
		finished := make(chan error, 1)
		go func() { server.GracefulStop(); finished <- nil }()
		_ = waitGRPCLifecycleResult(t, finished, "released handler shutdown")
	})
	conn, err := grpc.NewClient(listener.Addr().String(), clientTLS)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	rpcCtx, cancelRPC := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelRPC()
	requestDone := make(chan error, 1)
	go func() {
		_, err := pb.NewMetricsClient(conn).UpdateMetrics(rpcCtx, pb.UpdateMetricsRequest_builder{}.Build())
		requestDone <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("RPC did not reach the server")
	}
	stopped := make(chan error, 1)
	go func() { stopped <- shutdownGRPCServer(server, 10*time.Millisecond) }()
	err = waitGRPCLifecycleResult(t, stopped, "forced gRPC shutdown")
	if err == nil || !strings.Contains(err.Error(), "shutdown gRPC") || !strings.Contains(err.Error(), "10ms") {
		t.Fatalf("expected shutdown timeout error, got %v", err)
	}
	select {
	case <-handlerDone:
		t.Fatal("handler finished before being released; cancellation was not ignored")
	default:
	}
	if err := waitGRPCLifecycleResult(t, requestDone, "cancelled RPC"); err == nil {
		t.Fatal("RPC unexpectedly succeeded after forced shutdown")
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case <-handlerDone:
	case <-time.After(3 * time.Second):
		t.Fatal("released handler did not finish")
	}
	if err := waitGRPCLifecycleResult(t, served, "gRPC Serve completion"); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		t.Fatalf("Serve failed: %v", err)
	}
}

func TestRunServersStopsGRPCOnHTTPStartupError(t *testing.T) {
	occupiedHTTP := listenGRPCLifecycle(t)
	grpcListener := listenGRPCLifecycle(t)
	grpcServer := grpc.NewServer()
	t.Cleanup(grpcServer.Stop)
	httpServer := &http.Server{Addr: occupiedHTTP.Addr().String()}
	stopped := make(chan error, 1)
	go func() { stopped <- runServers(context.Background(), httpServer, grpcServer, grpcListener) }()
	if err := waitGRPCLifecycleResult(t, stopped, "combined server startup failure"); err == nil || !strings.Contains(err.Error(), "serve HTTP") {
		t.Fatalf("expected HTTP startup error, got %v", err)
	}
	accepted := make(chan error, 1)
	go func() {
		conn, err := grpcListener.Accept()
		if conn != nil {
			_ = conn.Close()
		}
		accepted <- err
	}()
	if err := waitGRPCLifecycleResult(t, accepted, "gRPC listener closure"); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("gRPC listener was not closed after HTTP failure: %v", err)
	}
}

func TestAgentGRPCBatchUsesSharedStorageAndTrustedSubnet(t *testing.T) {
	for _, tt := range []struct {
		name   string
		subnet string
		allow  bool
	}{
		{name: "trusted loopback", subnet: "127.0.0.0/8", allow: true},
		{name: "untrusted loopback", subnet: "192.0.2.0/24"},
		{name: "empty subnet", allow: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			certFile, keyFile := testutil.NewTLSFiles(t)
			cfg := &serverconfig.Config{TrustedSubnet: tt.subnet, GRPCCertFile: certFile, GRPCKeyFile: keyFile}
			router, storage := testRouter(t, cfg)
			if err := storage.SetGauge(context.Background(), "Alloc", 1); err != nil {
				t.Fatal(err)
			}
			if err := storage.AddCounter(context.Background(), "PollCount", 2); err != nil {
				t.Fatal(err)
			}
			server, err := newGRPCServer(cfg, service.NewMetricsService(storage), nil)
			if err != nil {
				t.Fatal(err)
			}
			listener := listenGRPCLifecycle(t)
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(func() { cancel(); server.Stop() })
			stopped := make(chan error, 1)
			go func() { stopped <- runGRPCServer(ctx, server, listener) }()
			sender, err := agent.NewGRPCSender(listener.Addr().String(), certFile)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sender.Close() })
			sent := make(chan error, 1)
			go func() {
				sent <- sender.SendBatch([]*agent.Metric{
					{Name: "Alloc", MType: "gauge", Value: 23.5},
					{Name: "PollCount", MType: "counter", Value: 3},
				})
			}()
			err = waitGRPCLifecycleResult(t, sent, "agent batch")
			wantGauge, wantCounter := "1", "2"
			if tt.allow {
				if err != nil {
					t.Fatalf("send batch: %v", err)
				}
				wantGauge, wantCounter = "23.5", "5"
			} else if status.Code(err) != codes.PermissionDenied {
				t.Fatalf("untrusted batch error = %v, want PermissionDenied", err)
			}
			for _, metric := range []struct {
				path string
				want string
			}{
				{path: "/value/gauge/Alloc", want: wantGauge},
				{path: "/value/counter/PollCount", want: wantCounter},
			} {
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, metric.path, nil))
				if response.Code != http.StatusOK || response.Body.String() != metric.want {
					t.Errorf("HTTP read %s = %d %q, want 200 %q", metric.path, response.Code, response.Body.String(), metric.want)
				}
			}
			cancel()
			if err := waitGRPCLifecycleResult(t, stopped, "gRPC shutdown"); err != nil {
				t.Fatalf("shutdown: %v", err)
			}
		})
	}
}

func TestNewGRPCServerRejectsInvalidTrustedSubnet(t *testing.T) {
	server, err := newGRPCServer(&serverconfig.Config{TrustedSubnet: "invalid"}, nil, nil)
	if server != nil || err == nil {
		t.Fatalf("invalid subnet server = %v, error = %v; want nil server and error", server, err)
	}
}

func TestNewGRPCServerRequiresValidTLSFiles(t *testing.T) {
	certFile, keyFile := testutil.NewTLSFiles(t)
	_, otherKeyFile := testutil.NewTLSFiles(t)
	invalidFile := filepath.Join(t.TempDir(), "invalid.pem")
	if err := os.WriteFile(invalidFile, []byte("not a PEM certificate"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		cert string
		key  string
	}{
		{name: "no credentials"},
		{name: "missing certificate", key: keyFile},
		{name: "missing key", cert: certFile},
		{name: "unreadable certificate", cert: filepath.Join(t.TempDir(), "missing.pem"), key: keyFile},
		{name: "invalid certificate", cert: invalidFile, key: keyFile},
		{name: "invalid key", cert: certFile, key: invalidFile},
		{name: "mismatched key", cert: certFile, key: otherKeyFile},
	} {
		t.Run(test.name, func(t *testing.T) {
			server, err := newGRPCServer(&serverconfig.Config{
				GRPCCertFile: test.cert,
				GRPCKeyFile:  test.key,
			}, nil, nil)
			if server != nil {
				server.Stop()
				t.Fatal("server accepted invalid TLS configuration")
			}
			if err == nil || !strings.Contains(err.Error(), "TLS") {
				t.Fatalf("error = %v, want a TLS configuration error", err)
			}
		})
	}
}
