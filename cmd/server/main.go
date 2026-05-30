package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
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

	if envAddr, ok := os.LookupEnv("ADDRESS"); ok && envAddr != "" {
		flagRunAddr = envAddr
	}
	if envLogLevel, ok := os.LookupEnv("LOG_LEVEL"); ok && envLogLevel != "" {
		flagLogLevel = envLogLevel
	}
	if envInterval, ok := os.LookupEnv("STORE_INTERVAL"); ok && envInterval != "" {
		v, err := strconv.Atoi(envInterval)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid STORE_INTERVAL value: %s\n", envInterval)
			os.Exit(1)
		}
		flagStoreInterval = v
	}
	if envPath, ok := os.LookupEnv("FILE_STORAGE_PATH"); ok && envPath != "" {
		flagFilePath = envPath
	}
	if envRestore, ok := os.LookupEnv("RESTORE"); ok && envRestore != "" {
		flagRestore = envRestore == "true"
	}
}

func main() {
	parseFlags()

	log, err := logger.NewLogger(flagLogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	storage := repository.NewMemStorage(flagFilePath)
	if flagRestore && flagFilePath != "" {
		log.Info("Metrics restored from file", zap.String("file", flagFilePath))
	}

	metricsService := service.NewMetricsService(storage)

	var syncFilePath string
	if flagStoreInterval == 0 {
		syncFilePath = flagFilePath
	}
	metricsHandler := handler.NewMetricsHandler(metricsService, storage, syncFilePath)

	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger(log))
	r.Use(middleware.GzipUnmarshal())
	r.Use(middleware.Gzip())

	r.POST("/update/:type/:name/:value", metricsHandler.UpdateMetricHandler)
	r.GET("/value/:type/:name", metricsHandler.GetMetricHandler)
	r.GET("/", metricsHandler.ListMetricsHandler)
	
	r.POST("/update", metricsHandler.UpdateMetricJSONHandler)
	r.POST("/value", metricsHandler.GetMetricJSONHandler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if flagStoreInterval > 0 && flagFilePath != "" {
		go func(ctx context.Context) {
			ticker := time.NewTicker(time.Duration(flagStoreInterval) * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if err := storage.SaveToFile(); err != nil {
						log.Error("Failed to save metrics to file", zap.String("file", flagFilePath), zap.Error(err))
					} else {
						log.Info("Metrics saved to file", zap.String("file", flagFilePath))
					}
				case <-ctx.Done():
					log.Info("Store ticker stopped")
					return
				}
			}
		}(ctx)
	}

	log.Info("Server starting", zap.String("address", flagRunAddr))

	server := &http.Server{
		Addr:    flagRunAddr,
		Handler: r,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(ctxShutdown); err != nil {
		log.Fatal("Server forced to shutdown", zap.Error(err))
	}

	cancel()
	log.Info("Server stopped")
}
