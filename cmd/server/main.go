package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"go-musthave-metrics/internal/handler"
	"go-musthave-metrics/internal/repository"
	"go-musthave-metrics/internal/service"
)

func main() {
	storage := repository.NewMemStorage()

	metricsService := service.NewMetricsService(storage)

	metricsHandler := handler.NewMetricsHandler(metricsService)

	r := gin.Default()

	r.POST("/update/:type/:name/:value", metricsHandler.UpdateMetricHandler)
	r.GET("/value/:type/:name", metricsHandler.GetMetricHandler)
	r.GET("/", metricsHandler.ListMetricsHandler)

	addr := "localhost:8080"
	fmt.Printf("Server starting on %s\n", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
