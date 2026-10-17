package agent

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"go-musthave-metrics/internal/handler"
	"go-musthave-metrics/internal/middleware"
	"go-musthave-metrics/internal/repository"
	"go-musthave-metrics/internal/service"

	"github.com/gin-gonic/gin"
)

type encryptedRequestCounts struct {
	singles atomic.Int32
	batches atomic.Int32
}

func encryptedBatchTestServer(t *testing.T, privateKey *rsa.PrivateKey) (*Sender, *repository.MemStorage, *encryptedRequestCounts) {
	t.Helper()
	const hashKey = "encrypted-batch-test"
	storage := repository.NewMemStorage("")
	metricsHandler := handler.NewMetricsHandler(service.NewMetricsService(storage), storage, "", hashKey, nil)
	counts := &encryptedRequestCounts{}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		switch c.Request.URL.Path {
		case "/update":
			counts.singles.Add(1)
		case "/updates/":
			counts.batches.Add(1)
		}
		if c.GetHeader("Content-Encoding") != "gzip" || c.GetHeader("HashSHA256") == "" {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		c.Next()
	}, middleware.GzipUnmarshal(), middleware.Decrypt(privateKey), middleware.HashSHA256(hashKey))
	router.POST("/update", metricsHandler.UpdateMetricJSONHandler)
	router.POST("/updates/", metricsHandler.UpdateMetricsBatchHandler)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	publicKeyPath := filepath.Join(t.TempDir(), "public.pem")
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: x509.MarshalPKCS1PublicKey(&privateKey.PublicKey)})
	if err := os.WriteFile(publicKeyPath, publicKeyPEM, 0600); err != nil {
		t.Fatal(err)
	}
	sender := NewSender(server.URL, hashKey, publicKeyPath)
	t.Cleanup(sender.client.CloseIdleConnections)
	return sender, storage, counts
}

func TestSenderSendBatchEncryptedCompatibility(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	t.Run("runtime snapshot falls back without duplicate counters", func(t *testing.T) {
		sender, storage, counts := encryptedBatchTestServer(t, privateKey)
		collector := NewCollector()
		collector.Collect()
		metrics := append(collector.DrainMetrics(), &Metric{Name: "ExtraGauge", MType: "gauge", Value: 12.5})
		if len(metrics) != 30 {
			t.Fatalf("runtime snapshot has %d metrics, want 30", len(metrics))
		}
		if err := sender.SendBatch(metrics); err != nil {
			t.Fatal(err)
		}
		if counts.singles.Load() != 30 || counts.batches.Load() != 0 || len(storage.GetAll()) != 30 {
			t.Fatalf("single requests = %d, batch requests = %d, saved metrics = %d; want 30, 0, 30", counts.singles.Load(), counts.batches.Load(), len(storage.GetAll()))
		}
		if got, err := storage.GetCounter(ctx, "PollCount"); err != nil || got != 1 {
			t.Fatalf("PollCount = %d, %v; want 1", got, err)
		}
		if got, err := storage.GetGauge(ctx, "ExtraGauge"); err != nil || got != 12.5 {
			t.Fatalf("ExtraGauge = %v, %v; want 12.5", got, err)
		}
	})

	t.Run("small encrypted batch remains one request", func(t *testing.T) {
		sender, storage, counts := encryptedBatchTestServer(t, privateKey)
		if err := sender.SendBatch([]*Metric{{Name: "count", MType: "counter", Value: 2}, {Name: "gauge", MType: "gauge", Value: 3.5}}); err != nil {
			t.Fatal(err)
		}
		if counts.batches.Load() != 1 || counts.singles.Load() != 0 {
			t.Fatalf("batch requests = %d, single requests = %d; want 1, 0", counts.batches.Load(), counts.singles.Load())
		}
		if got, err := storage.GetCounter(ctx, "count"); err != nil || got != 2 {
			t.Fatalf("counter = %d, %v; want 2", got, err)
		}
		if got, err := storage.GetGauge(ctx, "gauge"); err != nil || got != 3.5 {
			t.Fatalf("gauge = %v, %v; want 3.5", got, err)
		}
	})

	t.Run("individual encryption failure is preserved", func(t *testing.T) {
		sender, storage, counts := encryptedBatchTestServer(t, privateKey)
		longName := strings.Repeat("large-metric", 30)
		err := sender.SendBatch([]*Metric{
			{Name: "count", MType: "counter", Value: 3},
			{Name: longName, MType: "gauge", Value: 1},
			{Name: "gauge", MType: "gauge", Value: 7},
		})
		if !errors.Is(err, rsa.ErrMessageTooLong) || !strings.Contains(err.Error(), longName) {
			t.Fatalf("error = %v, want individual RSA size error with metric name", err)
		}
		if counts.singles.Load() != 2 || counts.batches.Load() != 0 {
			t.Fatalf("single requests = %d, batch requests = %d; want 2, 0", counts.singles.Load(), counts.batches.Load())
		}
		if got, err := storage.GetCounter(ctx, "count"); err != nil || got != 3 {
			t.Fatalf("counter = %d, %v; want 3", got, err)
		}
		if got, err := storage.GetGauge(ctx, "gauge"); err != nil || got != 7 {
			t.Fatalf("gauge after failed metric = %v, %v; want 7", got, err)
		}
		if _, err := storage.GetGauge(ctx, longName); err == nil {
			t.Fatal("failed metric was saved")
		}
	})
}
