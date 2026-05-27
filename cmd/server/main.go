package main

import (
	"flag"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go-musthave-metrics/internal/handler"
	"go-musthave-metrics/internal/logger"
	"go-musthave-metrics/internal/middleware"
	"go-musthave-metrics/internal/repository"
	"go-musthave-metrics/internal/service"
	"go.uber.org/zap"
)

var (
	flagRunAddr       string
	flagLogLevel      string
	flagStoreInterval int
	flagFilePath      string
	flagRestore       bool
)

func parseFlags() {
	flag.StringVar(&flagRunAddr, "a", ":8080", "address and port to run server")
	flag.StringVar(&flagLogLevel, "l", "info", "log level")
	flag.IntVar(&flagStoreInterval, "i", 300, "store interval in seconds")
	flag.StringVar(&flagFilePath, "f", "metrics.json", "path to metrics file")
	flag.BoolVar(&flagRestore, "r", false, "restore metrics from file")
	flag.Parse()

	if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
		flagRunAddr = envAddr
	}
	if envLogLevel := os.Getenv("LOG_LEVEL"); envLogLevel != "" {
		flagLogLevel = envLogLevel
	}
	if envInterval := os.Getenv("STORE_INTERVAL"); envInterval != "" {
		if v, err := strconv.Atoi(envInterval); err == nil {
			flagStoreInterval = v
		}
	}
	if envPath := os.Getenv("FILE_STORAGE_PATH"); envPath != "" {
		flagFilePath = envPath
	}
	if envRestore := os.Getenv("RESTORE"); envRestore != "" {
		flagRestore = envRestore == "true"
	}
}

func main() {
	parseFlags()

	if err := logger.Initialize(flagLogLevel); err != nil {
		logger.Log.Fatal("Failed to initialize logger", zap.Error(err))
	}
	defer logger.Log.Sync()

	storage := repository.NewMemStorage()

	if flagRestore && flagFilePath != "" {
		if err := storage.LoadFromFile(flagFilePath); err != nil {
			logger.Log.Warn("Failed to load metrics from file", zap.String("file", flagFilePath), zap.Error(err))
		} else {
			logger.Log.Info("Metrics restored from file", zap.String("file", flagFilePath))
		}
	}

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

	if flagStoreInterval > 0 {
		go func() {
			ticker := time.NewTicker(time.Duration(flagStoreInterval) * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				if err := storage.SaveToFile(flagFilePath); err != nil {
					logger.Log.Error("Failed to save metrics to file", zap.String("file", flagFilePath), zap.Error(err))
				} else {
					logger.Log.Info("Metrics saved to file", zap.String("file", flagFilePath))
				}
			}
		}()
	}

	logger.Log.Info("Server starting", zap.String("address", flagRunAddr))

	if err := r.Run(flagRunAddr); err != nil {
		logger.Log.Fatal("Failed to start server", zap.Error(err))
	}
}
