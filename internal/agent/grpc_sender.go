package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math"
	"net"
	"net/netip"
	"os"
	"strings"

	pb "go-musthave-metrics/internal/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// GRPCSender отправляет пакеты метрик через переиспользуемое gRPC-соединение.
type GRPCSender struct {
	conn   *grpc.ClientConn
	client pb.MetricsClient
}

// NewGRPCSender создает отправитель для заданного адреса gRPC-сервера.
// caFile задаёт доверенные сертификаты в PEM; пустое значение использует
// системное хранилище корневых сертификатов.
func NewGRPCSender(address, caFile string) (*GRPCSender, error) {
	if strings.TrimSpace(address) == "" {
		return nil, fmt.Errorf("gRPC server address is empty")
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if caFile != "" {
		certPEM, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read gRPC CA certificate: %w", err)
		}
		tlsConfig.RootCAs = x509.NewCertPool()
		if !tlsConfig.RootCAs.AppendCertsFromPEM(certPEM) {
			return nil, fmt.Errorf("gRPC CA file contains no valid certificates")
		}
	}
	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(localIPCredentials{TransportCredentials: credentials.NewTLS(tlsConfig)}),
		grpc.WithPerRPCCredentials(realIPCredentials{}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}
	return &GRPCSender{conn: conn, client: pb.NewMetricsClient(conn)}, nil
}

// SendBatch отправляет все метрики одним RPC с ограниченным временем ожидания.
func (s *GRPCSender) SendBatch(metrics []*Metric) error {
	if len(metrics) == 0 {
		return nil
	}
	batch := make([]*pb.Metric, 0, len(metrics))
	for i, metric := range metrics {
		if metric == nil {
			return fmt.Errorf("metric at index %d is nil", i)
		}
		if metric.Name == "" {
			return fmt.Errorf("metric at index %d has an empty name", i)
		}
		if math.IsNaN(metric.Value) || math.IsInf(metric.Value, 0) {
			return fmt.Errorf("metric %q has a non-finite value", metric.Name)
		}
		m := pb.Metric_builder{Id: metric.Name}.Build()
		switch metric.MType {
		case "gauge":
			m.SetType(pb.Metric_GAUGE)
			m.SetValue(metric.Value)
		case "counter":
			if metric.Value < -0x1p63 || metric.Value >= 0x1p63 {
				return fmt.Errorf("counter %q is outside the int64 range", metric.Name)
			}
			m.SetType(pb.Metric_COUNTER)
			m.SetDelta(int64(metric.Value))
		default:
			return fmt.Errorf("metric %q has unsupported type %q", metric.Name, metric.MType)
		}
		batch = append(batch, m)
	}
	request := pb.UpdateMetricsRequest_builder{Metrics: batch}.Build()

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	if _, err := s.client.UpdateMetrics(ctx, request); err != nil {
		return fmt.Errorf("failed to send gRPC metrics: %w", err)
	}
	return nil
}

// Close освобождает gRPC-соединение отправителя.
func (s *GRPCSender) Close() error {
	return s.conn.Close()
}

// localIPCredentials сохраняет адрес локального конца каждого соединения.
// AuthInfo привязан к соединению, поэтому параллельные RPC и переподключения
// не используют общий изменяемый адрес.
type localIPCredentials struct {
	credentials.TransportCredentials
}

func (c localIPCredentials) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	conn, authInfo, err := c.TransportCredentials.ClientHandshake(ctx, authority, rawConn)
	if err != nil {
		return nil, nil, err
	}
	tlsInfo, ok := authInfo.(credentials.TLSInfo)
	if !ok {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("gRPC connection did not provide TLS authentication information")
	}
	addr, err := netip.ParseAddrPort(conn.LocalAddr().String())
	if err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("failed to get gRPC connection local IP: %w", err)
	}
	return conn, localIPAuthInfo{
		TLSInfo: tlsInfo,
		ip:      addr.Addr().WithZone("").String(),
	}, nil
}

func (c localIPCredentials) Clone() credentials.TransportCredentials {
	return localIPCredentials{TransportCredentials: c.TransportCredentials.Clone()}
}

type localIPAuthInfo struct {
	credentials.TLSInfo
	ip string
}

// realIPCredentials добавляет в метаданные IP соединения, выбранного для RPC.
type realIPCredentials struct{}

func (realIPCredentials) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	info, ok := credentials.RequestInfoFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("gRPC connection information is unavailable")
	}
	authInfo, ok := info.AuthInfo.(localIPAuthInfo)
	if !ok || authInfo.ip == "" {
		return nil, fmt.Errorf("gRPC connection local IP is unavailable")
	}
	return map[string]string{"x-real-ip": authInfo.ip}, nil
}

func (realIPCredentials) RequireTransportSecurity() bool {
	return true
}
