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

	"go-musthave-metrics/internal/buildinfo"
	"go-musthave-metrics/internal/config/db"
	"go-musthave-metrics/internal/handler"
	"go-musthave-metrics/internal/logger"
	"go-musthave-metrics/internal/middleware"
	"go-musthave-metrics/internal/repository"
	"go-musthave-metrics/internal/service"
	"go-musthave-metrics/internal/service/audit"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var (
	flagRunAddr       string
	flagLogLevel      string
	flagStoreInterval int
	flagFilePath      string
	flagRestore       bool
	flagDBDSN         string
	flagKey           string
	flagAuditFile     string
	flagAuditURL      string
	filePathSet       bool
)

func parseFlags() error {
	flag.StringVar(&flagRunAddr, "a", ":8080", "address and port to run server")
	flag.StringVar(&flagLogLevel, "l", "info", "log level")
	flag.IntVar(&flagStoreInterval, "i", 300, "store interval in seconds")
	flag.StringVar(&flagFilePath, "f", "", "path to metrics file")
	flag.BoolVar(&flagRestore, "r", false, "restore metrics from file")
	flag.StringVar(&flagDBDSN, "d", "", "database DSN")
	flag.StringVar(&flagKey, "k", "", "key for signing data")
	flag.StringVar(&flagAuditFile, "audit-file", "", "path to audit log file")
	flag.StringVar(&flagAuditURL, "audit-url", "", "URL to send audit logs")
	flag.Parse()

	flag.Visit(func(f *flag.Flag) {
		if f.Name == "f" {
			filePathSet = true
		}
	})

	if envAddr, ok := os.LookupEnv("ADDRESS"); ok && envAddr != "" {
		flagRunAddr = envAddr
	}
	if envLogLevel, ok := os.LookupEnv("LOG_LEVEL"); ok && envLogLevel != "" {
		flagLogLevel = envLogLevel
	}
	if envInterval, ok := os.LookupEnv("STORE_INTERVAL"); ok && envInterval != "" {
		v, err := strconv.Atoi(envInterval)
		if err != nil {
			return fmt.Errorf("invalid STORE_INTERVAL value: %s", envInterval)
		}
		flagStoreInterval = v
	}
	if envPath, ok := os.LookupEnv("FILE_STORAGE_PATH"); ok && envPath != "" {
		flagFilePath = envPath
		filePathSet = true
	}
	if envRestore, ok := os.LookupEnv("RESTORE"); ok && envRestore != "" {
		flagRestore = envRestore == "true"
	}
	if envKey, ok := os.LookupEnv("KEY"); ok && envKey != "" {
		flagKey = envKey
	}
	if envAuditFile, ok := os.LookupEnv("AUDIT_FILE"); ok && envAuditFile != "" {
		flagAuditFile = envAuditFile
	}
	if envAuditURL, ok := os.LookupEnv("AUDIT_URL"); ok && envAuditURL != "" {
		flagAuditURL = envAuditURL
	}

	return nil
}

func main() {
	buildinfo.Print()

	if err := parseFlags(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse flags: %v\n", err)
		return
	}

	log, err := logger.NewLogger(flagLogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		return
	}
	defer func() {
		_ = log.Sync()
	}()

	dbConfig := db.NewConfig()
	dbConfig.LoadFromEnv()
	dbConfig.LoadFromFlags(flagDBDSN)

	var storage repository.Storage
	var dbConn *repository.PostgresStorage

	if dbConfig.DSN != "" {
		storage, err = repository.NewPostgresStorage(dbConfig.DSN)
		if err != nil {
			log.Fatal("Failed to connect to database", zap.Error(err))
		}
		dbConn = storage.(*repository.PostgresStorage)
		log.Info("Connected to PostgreSQL")
	} else if filePathSet && flagFilePath != "" {
		storage = repository.NewMemStorage(flagFilePath)
		if flagRestore {
			log.Info("Metrics restored from file", zap.String("file", flagFilePath))
		}
		log.Info("Using file storage", zap.String("file", flagFilePath))
	} else {
		storage = repository.NewMemStorage("")
		log.Info("Using in-memory storage")
	}

	metricsService := service.NewMetricsService(storage)

	auditService := audit.NewAuditService()

	if flagAuditFile != "" {
		fileObserver, err := audit.NewFileObserver(flagAuditFile)
		if err != nil {
			log.Fatal("Failed to create file audit observer", zap.Error(err))
		}
		auditService.AddObserver(fileObserver)
		log.Info("Audit to file enabled", zap.String("file", flagAuditFile))
	}

	if flagAuditURL != "" {
		urlObserver := audit.NewURLObserver(flagAuditURL)
		auditService.AddObserver(urlObserver)
		log.Info("Audit to URL enabled", zap.String("url", flagAuditURL))
	}

	var syncFilePath string
	if flagStoreInterval == 0 && flagFilePath != "" && dbConn == nil {
		syncFilePath = flagFilePath
	}

	metricsHandler := handler.NewMetricsHandler(
		metricsService,
		storage,
		syncFilePath,
		flagKey,
		auditService,
	)

	var pingHandler *handler.PingHandler
	if dbConn != nil {
		pingHandler = handler.NewPingHandler(dbConn)
	}

	r := gin.New()
	r.RedirectTrailingSlash = false

	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger(log))
	r.Use(middleware.GzipUnmarshal())
	r.Use(middleware.Gzip())

	r.POST("/update/:type/:name/:value", metricsHandler.UpdateMetricHandler)
	r.GET("/value/:type/:name", metricsHandler.GetMetricHandler)
	r.GET("/", metricsHandler.ListMetricsHandler)

	jsonRoutes := r.Group("/")
	jsonRoutes.Use(middleware.HashSHA256(flagKey))
	{
		jsonRoutes.POST("/update", metricsHandler.UpdateMetricJSONHandler)
		jsonRoutes.POST("/update/", metricsHandler.UpdateMetricJSONHandler)

		jsonRoutes.POST("/updates", metricsHandler.UpdateMetricsBatchHandler)
		jsonRoutes.POST("/updates/", metricsHandler.UpdateMetricsBatchHandler)

		jsonRoutes.POST("/value", metricsHandler.GetMetricJSONHandler)
		jsonRoutes.POST("/value/", metricsHandler.GetMetricJSONHandler)
	}

	if pingHandler != nil {
		r.GET("/ping", pingHandler.PingHandler)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if flagStoreInterval > 0 && flagFilePath != "" && dbConn == nil {
		go func(ctx context.Context) {
			ticker := time.NewTicker(
				time.Duration(flagStoreInterval) * time.Second,
			)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					if err := storage.(*repository.MemStorage).SaveToFile(); err != nil {
						log.Error(
							"Failed to save metrics to file",
							zap.String("file", flagFilePath),
							zap.Error(err),
						)
					} else {
						log.Info(
							"Metrics saved to file",
							zap.String("file", flagFilePath),
						)
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

	ctxShutdown, cancelShutdown := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancelShutdown()

	if err := server.Shutdown(ctxShutdown); err != nil {
		log.Fatal("Server forced to shutdown", zap.Error(err))
	}

	cancel()

	if err := auditService.Close(); err != nil {
		log.Error("Failed to close audit service", zap.Error(err))
	}

	if dbConn != nil {
		if err := dbConn.Close(); err != nil {
			log.Error("Failed to close database connection", zap.Error(err))
		}
	}

	log.Info("Server stopped")
}
