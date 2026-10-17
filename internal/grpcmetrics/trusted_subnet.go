package grpcmetrics

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TrustedSubnetInterceptor проверяет IP из метаданных x-real-ip.
// Пустая подсеть отключает проверку; CIDR разбирается при создании interceptor.
func TrustedSubnetInterceptor(cidr string) (grpc.UnaryServerInterceptor, error) {
	var subnet *net.IPNet
	if cidr != "" {
		var err error
		_, subnet, err = net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted subnet %q: %w", cidr, err)
		}
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if subnet != nil {
			md, _ := metadata.FromIncomingContext(ctx)
			values := md.Get("x-real-ip")
			if len(values) != 1 {
				return nil, status.Error(codes.PermissionDenied, "agent IP does not belong to the trusted subnet")
			}
			ip := net.ParseIP(values[0])
			if ip == nil || !subnet.Contains(ip) {
				return nil, status.Error(codes.PermissionDenied, "agent IP does not belong to the trusted subnet")
			}
		}
		return handler(ctx, req)
	}, nil
}
