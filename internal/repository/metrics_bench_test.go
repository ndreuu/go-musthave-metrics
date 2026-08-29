package repository

import (
	"context"
	"fmt"
	"testing"

	models "go-musthave-metrics/internal/model"
)

func BenchmarkMemStorage_SetGauge(b *testing.B) {
	storage := NewMemStorage("")
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = storage.SetGauge(ctx, fmt.Sprintf("gauge_%d", i), float64(i))
	}
}

func BenchmarkMemStorage_AddCounter(b *testing.B) {
	storage := NewMemStorage("")
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = storage.AddCounter(ctx, "counter", 1)
	}
}

func BenchmarkMemStorage_GetGauge(b *testing.B) {
	storage := NewMemStorage("")
	ctx := context.Background()
	storage.SetGauge(ctx, "test_gauge", 123.456)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = storage.GetGauge(ctx, "test_gauge")
	}
}

func BenchmarkMemStorage_GetCounter(b *testing.B) {
	storage := NewMemStorage("")
	ctx := context.Background()
	storage.AddCounter(ctx, "test_counter", 100)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = storage.GetCounter(ctx, "test_counter")
	}
}

func BenchmarkMemStorage_GetAll(b *testing.B) {
	storage := NewMemStorage("")
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		storage.SetGauge(ctx, fmt.Sprintf("gauge_%d", i), float64(i))
		storage.AddCounter(ctx, fmt.Sprintf("counter_%d", i), int64(i))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = storage.GetAll()
	}
}

func BenchmarkMemStorage_UpdateMetricsBatch(b *testing.B) {
	storage := NewMemStorage("")
	ctx := context.Background()

	metrics := make([]models.Metrics, 10)
	for i := 0; i < 10; i++ {
		value := float64(i)
		delta := int64(i)
		if i%2 == 0 {
			metrics[i] = models.Metrics{ID: fmt.Sprintf("gauge_%d", i), MType: models.Gauge, Value: &value}
		} else {
			metrics[i] = models.Metrics{ID: fmt.Sprintf("counter_%d", i), MType: models.Counter, Delta: &delta}
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = storage.UpdateMetricsBatch(ctx, metrics)
	}
}

func BenchmarkMemStorage_Concurrent(b *testing.B) {
	storage := NewMemStorage("")
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			storage.SetGauge(ctx, fmt.Sprintf("gauge_%d", i), float64(i))
			storage.AddCounter(ctx, fmt.Sprintf("counter_%d", i), int64(i))
			storage.GetGauge(ctx, fmt.Sprintf("gauge_%d", i%10))
			storage.GetCounter(ctx, fmt.Sprintf("counter_%d", i%10))
			i++
		}
	})
}

func BenchmarkMemStorage_SaveToFile(b *testing.B) {
	storage := NewMemStorage("/tmp/bench_metrics.json")
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		storage.SetGauge(ctx, fmt.Sprintf("gauge_%d", i), float64(i))
		storage.AddCounter(ctx, fmt.Sprintf("counter_%d", i), int64(i))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = storage.SaveToFile()
	}
}
