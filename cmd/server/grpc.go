package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	serverconfig "go-musthave-metrics/internal/config/server"
	"go-musthave-metrics/internal/grpcmetrics"
	pb "go-musthave-metrics/internal/proto"
	"go-musthave-metrics/internal/service"
	"go-musthave-metrics/internal/service/audit"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

func newGRPCServer(cfg *serverconfig.Config, metricsService *service.MetricsService, auditService *audit.AuditService) (*grpc.Server, error) {
	checkSubnet, err := grpcmetrics.TrustedSubnetInterceptor(cfg.TrustedSubnet)
	if err != nil {
		return nil, err
	}
	server := grpc.NewServer(grpc.UnaryInterceptor(checkSubnet))
	pb.RegisterMetricsServer(server, grpcmetrics.NewMetricsServer(metricsService, auditService))
	return server, nil
}

func runServers(ctx context.Context, httpServer *http.Server, grpcServer *grpc.Server, listener net.Listener) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	group, serverCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		defer cancel()
		return runHTTPServer(serverCtx, httpServer)
	})
	if grpcServer != nil {
		group.Go(func() error {
			defer cancel()
			return runGRPCServer(serverCtx, grpcServer, listener)
		})
	}
	return group.Wait()
}

func runGRPCServer(ctx context.Context, server *grpc.Server, listener net.Listener) error {
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	select {
	case err := <-served:
		shutdownErr := shutdownGRPCServer(server, 5*time.Second)
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return errors.Join(fmt.Errorf("serve gRPC: %w", err), shutdownErr)
		}
		return shutdownErr
	case <-ctx.Done():
		return shutdownGRPCServer(server, 5*time.Second)
	}
}

func shutdownGRPCServer(server *grpc.Server, timeout time.Duration) error {
	stopped := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(stopped)
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-stopped:
		return nil
	case <-timer.C:
		go server.Stop()
		return fmt.Errorf("shutdown gRPC: active requests did not finish within %s", timeout)
	}
}
