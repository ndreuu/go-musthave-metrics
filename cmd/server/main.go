package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"go-musthave-metrics/internal/handler"
	"go-musthave-metrics/internal/repository"
	"go-musthave-metrics/internal/service"
)

var (
	flagRunAddr string
)

func parseFlags() {
	flag.StringVar(&flagRunAddr, "a", ":8080", "address and port to run server")
	flag.Parse()

	if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
		flagRunAddr = envAddr
	}
}

func main() {
	parseFlags()

	storage := repository.NewMemStorage()

	metricsService := service.NewMetricsService(storage)

	metricsHandler := handler.NewMetricsHandler(metricsService)

	r := gin.Default()

	r.POST("/update/:type/:name/:value", metricsHandler.UpdateMetricHandler)
	r.GET("/value/:type/:name", metricsHandler.GetMetricHandler)
	r.GET("/", metricsHandler.ListMetricsHandler)

	fmt.Printf("Server starting on %s\n", flagRunAddr)
	if err := r.Run(flagRunAddr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
