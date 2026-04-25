package main

import (
	"fmt"
	"log"
	"net/http"

	"go-musthave-metrics/internal/handler"
	"go-musthave-metrics/internal/repository"
	"go-musthave-metrics/internal/service"
)

func main() {
	storage := repository.NewMemStorage()

	metricsService := service.NewMetricsService(storage)

	metricsHandler := handler.NewMetricsHandler(metricsService)

	http.HandleFunc("/update/", metricsHandler.UpdateMetricHandler)

	addr := "localhost:8080"
	fmt.Printf("Server starting on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
