package main

import (
	"flag"
	"os"

	"github.com/gin-gonic/gin"
	"go-musthave-metrics/internal/handler"
	"go-musthave-metrics/internal/logger"
	"go-musthave-metrics/internal/middleware"
	"go-musthave-metrics/internal/repository"
	"go-musthave-metrics/internal/service"
	"go.uber.org/zap"
)

var (
	flagRunAddr  string
	flagLogLevel string
)

func parseFlags() {
	flag.StringVar(&flagRunAddr, "a", ":8080", "address and port to run server")
	flag.StringVar(&flagLogLevel, "l", "info", "log level")
	flag.Parse()

	if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
		flagRunAddr = envAddr
	}
	if envLogLevel := os.Getenv("LOG_LEVEL"); envLogLevel != "" {
		flagLogLevel = envLogLevel
	}
}

func main() {
	parseFlags()

	if err := logger.Initialize(flagLogLevel); err != nil {
		logger.Log.Fatal("Failed to initialize logger", zap.Error(err))
	}
	defer logger.Log.Sync()

	storage := repository.NewMemStorage()

	metricsService := service.NewMetricsService(storage)

	metricsHandler := handler.NewMetricsHandler(metricsService)

	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(logger.RequestLogger())
	r.Use(middleware.GzipUnmarshal())
	r.Use(middleware.Gzip())

	r.POST("/update/:type/:name/:value", metricsHandler.UpdateMetricHandler)
	r.GET("/value/:type/:name", metricsHandler.GetMetricHandler)
	r.GET("/", metricsHandler.ListMetricsHandler)
	
	r.POST("/update", metricsHandler.UpdateMetricJSONHandler)
	r.POST("/value", metricsHandler.GetMetricJSONHandler)

	logger.Log.Info("Server starting", zap.String("address", flagRunAddr))

	if err := r.Run(flagRunAddr); err != nil {
		logger.Log.Fatal("Failed to start server", zap.Error(err))
	}
}
