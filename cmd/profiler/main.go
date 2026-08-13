package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"

	models "go-musthave-metrics/internal/model"
	"go-musthave-metrics/internal/repository"
	"go-musthave-metrics/internal/service"
)

const (
	metricsCount    = 1000
	operationsCount = 50000
)

func main() {
	profilePath := flag.String("out", "profiles/base.pprof", "output heap profile")
	flag.Parse()

	fmt.Println("Starting memory profiling for metrics service...")

	storage := repository.NewMemStorage("")
	metricsService := service.NewMetricsService(storage)

	metrics := generateMetrics(metricsCount)

	ctx := context.Background()

	for i := 0; i < operationsCount; i++ {
		metric := metrics[i%len(metrics)]

		_ = metricsService.UpdateMetricFromJSON(ctx, &metric)
		_, _ = metricsService.GetMetricFromJSON(ctx, &metric)

		if i%100 == 0 {
			_ = metricsService.GetAllMetrics()
		}
	}

	f, err := os.Create(*profilePath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	runtime.GC()

	if err := pprof.WriteHeapProfile(f); err != nil {
		panic(err)
	}

	fmt.Printf("Memory profile saved to %s\n", *profilePath)

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	fmt.Printf("Alloc = %v KB\n", memStats.Alloc/1024)
	fmt.Printf("TotalAlloc = %v KB\n", memStats.TotalAlloc/1024)
	fmt.Printf("Sys = %v KB\n", memStats.Sys/1024)
	fmt.Printf("NumGC = %v\n", memStats.NumGC)
}

func generateMetrics(count int) []models.Metrics {
	metrics := make([]models.Metrics, count)
	for i := 0; i < count; i++ {
		value := float64(i) * 1.5
		delta := int64(i * 10)
		if i%2 == 0 {
			metrics[i] = models.Metrics{
				ID:    fmt.Sprintf("gauge_metric_%d", i),
				MType: models.Gauge,
				Value: &value,
			}
		} else {
			metrics[i] = models.Metrics{
				ID:    fmt.Sprintf("counter_metric_%d", i),
				MType: models.Counter,
				Delta: &delta,
			}
		}
	}
	return metrics
}
