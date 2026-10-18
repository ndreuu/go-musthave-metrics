package grpcmetrics_test

import (
	"context"
	"testing"

	"go-musthave-metrics/internal/grpcmetrics"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestTrustedSubnetInterceptor(t *testing.T) {
	for _, tt := range []struct {
		name   string
		cidr   string
		values []string
		allow  bool
	}{
		{name: "disabled without IP", allow: true},
		{name: "disabled with invalid IP", values: []string{"invalid"}, allow: true},
		{name: "trusted IPv4", cidr: "192.0.2.0/24", values: []string{"192.0.2.42"}, allow: true},
		{name: "trusted mapped IPv4", cidr: "192.0.2.0/24", values: []string{"::ffff:192.0.2.42"}, allow: true},
		{name: "outside IPv4", cidr: "192.0.2.0/24", values: []string{"198.51.100.42"}},
		{name: "missing IP", cidr: "192.0.2.0/24"},
		{name: "empty IP", cidr: "192.0.2.0/24", values: []string{""}},
		{name: "invalid IP", cidr: "192.0.2.0/24", values: []string{"invalid"}},
		{name: "IP with port", cidr: "192.0.2.0/24", values: []string{"192.0.2.42:1234"}},
		{name: "multiple IPs", cidr: "192.0.2.0/24", values: []string{"192.0.2.42", "198.51.100.42"}},
		{name: "comma separated IPs", cidr: "192.0.2.0/24", values: []string{"192.0.2.42,198.51.100.42"}},
		{name: "trusted IPv6", cidr: "2001:db8:1::/48", values: []string{"2001:db8:1::42"}, allow: true},
		{name: "outside IPv6", cidr: "2001:db8:1::/48", values: []string{"2001:db8:2::42"}},
		{name: "wrong address family", cidr: "2001:db8:1::/48", values: []string{"192.0.2.42"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			interceptor, err := grpcmetrics.TrustedSubnetInterceptor(tt.cidr)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if tt.values != nil {
				ctx = metadata.NewIncomingContext(ctx, metadata.MD{"x-real-ip": tt.values})
			}
			called := false
			response, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{FullMethod: "/metrics.Metrics/UpdateMetrics"}, func(gotCtx context.Context, req any) (any, error) {
				called = true
				if gotCtx != ctx || req != "request" {
					t.Fatal("interceptor changed context or request")
				}
				return "response", nil
			})
			if tt.allow {
				if !called || err != nil || response != "response" {
					t.Fatalf("allowed request = %v, %v; handler called = %v", response, err, called)
				}
			} else if called || response != nil || status.Code(err) != codes.PermissionDenied {
				t.Fatalf("rejected request = %v, %v; handler called = %v", response, err, called)
			}
		})
	}
}

func TestTrustedSubnetInterceptorInvalidCIDR(t *testing.T) {
	if interceptor, err := grpcmetrics.TrustedSubnetInterceptor("192.0.2.0/99"); err == nil || interceptor != nil {
		t.Fatalf("invalid CIDR = %v, %v; want nil interceptor and error", interceptor, err)
	}
}
