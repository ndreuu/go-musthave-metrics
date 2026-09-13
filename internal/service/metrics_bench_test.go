package service

import (
	"context"
	"fmt"
	"testing"

	models "go-musthave-metrics/internal/model"
	"go-musthave-metrics/internal/repository"
)

func BenchmarkMetricsService_UpdateMetric(b *testing.B) {
	storage := repository.NewMemStorage("")
	service := NewMetricsService(storage)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := &UpdateMetricResult{
			MType: "gauge",
			Name:  fmt.Sprintf("test_gauge_%d", i),
			Value: fmt.Sprintf("%d.%d", i, i%100),
		}
		_ = service.UpdateMetric(ctx, result)
	}
}

func BenchmarkMetricsService_GetMetricValue(b *testing.B) {
	storage := repository.NewMemStorage("")
	service := NewMetricsService(storage)
	ctx := context.Background()

	_ = storage.SetGauge(ctx, "test_gauge", 123.456)
	_ = storage.AddCounter(ctx, "test_counter", 100)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			_, _ = service.GetMetricValue(ctx, "gauge", "test_gauge")
		} else {
			_, _ = service.GetMetricValue(ctx, "counter", "test_counter")
		}
	}
}

func BenchmarkMetricsService_GetAllMetrics(b *testing.B) {
	storage := repository.NewMemStorage("")
	service := NewMetricsService(storage)
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		_ = storage.SetGauge(ctx, fmt.Sprintf("gauge_%d", i), float64(i))
		_ = storage.AddCounter(ctx, fmt.Sprintf("counter_%d", i), int64(i))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.GetAllMetrics()
	}
}

func BenchmarkMetricsService_UpdateMetricFromJSON(b *testing.B) {
	storage := repository.NewMemStorage("")
	service := NewMetricsService(storage)
	ctx := context.Background()

	value := 123.456
	delta := int64(100)

	b.ReportAllocs()
	b.Run("Gauge", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			metric := &models.Metrics{
				ID:    fmt.Sprintf("gauge_%d", i),
				MType: models.Gauge,
				Value: &value,
			}
			_ = service.UpdateMetricFromJSON(ctx, metric)
		}
	})

	b.Run("Counter", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			metric := &models.Metrics{
				ID:    fmt.Sprintf("counter_%d", i),
				MType: models.Counter,
				Delta: &delta,
			}
			_ = service.UpdateMetricFromJSON(ctx, metric)
		}
	})
}

func BenchmarkMetricsService_GetMetricFromJSON(b *testing.B) {
	storage := repository.NewMemStorage("")
	service := NewMetricsService(storage)
	ctx := context.Background()

	_ = storage.SetGauge(ctx, "test_gauge", 123.456)
	_ = storage.AddCounter(ctx, "test_counter", 100)

	b.ReportAllocs()
	b.Run("Gauge", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			metric := &models.Metrics{
				ID:    "test_gauge",
				MType: models.Gauge,
			}
			_, _ = service.GetMetricFromJSON(ctx, metric)
		}
	})

	b.Run("Counter", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			metric := &models.Metrics{
				ID:    "test_counter",
				MType: models.Counter,
			}
			_, _ = service.GetMetricFromJSON(ctx, metric)
		}
	})
}

func BenchmarkMetricsService_UpdateMetricsBatch(b *testing.B) {
	storage := repository.NewMemStorage("")
	service := NewMetricsService(storage)
	ctx := context.Background()

	metrics := make([]models.Metrics, 10)
	value := 123.456
	delta := int64(100)
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			metrics[i] = models.Metrics{ID: fmt.Sprintf("gauge_%d", i), MType: models.Gauge, Value: &value}
		} else {
			metrics[i] = models.Metrics{ID: fmt.Sprintf("counter_%d", i), MType: models.Counter, Delta: &delta}
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.UpdateMetricsBatch(ctx, metrics)
	}
}

func BenchmarkMetricsService_ParseUpdatePath(b *testing.B) {
	storage := repository.NewMemStorage("")
	service := NewMetricsService(storage)

	b.ReportAllocs()
	b.Run("Gauge", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = service.ParseUpdatePath("/update/gauge/test_gauge/123.456")
		}
	})

	b.Run("Counter", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = service.ParseUpdatePath("/update/counter/test_counter/100")
		}
	})
}
