package main

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-musthave-metrics/internal/buildinfo"
	serverconfig "go-musthave-metrics/internal/config/server"
	"go-musthave-metrics/internal/crypto"
	"go-musthave-metrics/internal/handler"
	"go-musthave-metrics/internal/logger"
	"go-musthave-metrics/internal/middleware"
	"go-musthave-metrics/internal/repository"
	"go-musthave-metrics/internal/service"
	"go-musthave-metrics/internal/service/audit"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func main() {
	buildinfo.Print()

	cfg, err := serverconfig.GetConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		return
	}

	log, err := logger.NewLogger(cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		return
	}
	defer func() {
		_ = log.Sync()
	}()

	var privateKey *rsa.PrivateKey
	if cfg.CryptoKey != "" {
		privateKey, err = crypto.LoadPrivateKey(cfg.CryptoKey)
		if err != nil {
			log.Fatal("Failed to load private key", zap.Error(err))
		}
		log.Info("Private key loaded", zap.String("path", cfg.CryptoKey))
	}

	var storage repository.Storage
	var dbConn *repository.PostgresStorage

	if cfg.DatabaseDSN != "" {
		storage, err = repository.NewPostgresStorage(cfg.DatabaseDSN)
		if err != nil {
			log.Fatal("Failed to connect to database", zap.Error(err))
		}
		dbConn = storage.(*repository.PostgresStorage)
		log.Info("Connected to PostgreSQL")
	} else if cfg.FilePath != "" {
		storage = repository.NewMemStorage(cfg.FilePath)
		if cfg.Restore {
			log.Info("Metrics restored from file", zap.String("file", cfg.FilePath))
		}
		log.Info("Using file storage", zap.String("file", cfg.FilePath))
	} else {
		storage = repository.NewMemStorage("")
		log.Info("Using in-memory storage")
	}

	metricsService := service.NewMetricsService(storage)

	auditService := audit.NewAuditService()

	if cfg.AuditFile != "" {
		fileObserver, err := audit.NewFileObserver(cfg.AuditFile)
		if err != nil {
			log.Fatal("Failed to create file audit observer", zap.Error(err))
		}
		auditService.AddObserver(fileObserver)
		log.Info("Audit to file enabled", zap.String("file", cfg.AuditFile))
	}

	if cfg.AuditURL != "" {
		urlObserver := audit.NewURLObserver(cfg.AuditURL)
		auditService.AddObserver(urlObserver)
		log.Info("Audit to URL enabled", zap.String("url", cfg.AuditURL))
	}

	var syncFilePath string
	if cfg.StoreInterval == 0 && cfg.FilePath != "" && dbConn == nil {
		syncFilePath = cfg.FilePath
	}

	metricsHandler := handler.NewMetricsHandler(
		metricsService,
		storage,
		syncFilePath,
		cfg.Key,
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
	jsonRoutes.Use(middleware.Decrypt(privateKey))
	jsonRoutes.Use(middleware.HashSHA256(cfg.Key))
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	storeCtx, stopStore := context.WithCancel(context.Background())
	defer stopStore()
	var storeGroup errgroup.Group

	if cfg.StoreInterval > 0 && cfg.FilePath != "" && dbConn == nil {
		storeGroup.Go(func() error {
			ticker := time.NewTicker(
				time.Duration(cfg.StoreInterval) * time.Second,
			)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					if err := storage.(*repository.MemStorage).SaveToFile(); err != nil {
						log.Error(
							"Failed to save metrics to file",
							zap.String("file", cfg.FilePath),
							zap.Error(err),
						)
					} else {
						log.Info(
							"Metrics saved to file",
							zap.String("file", cfg.FilePath),
						)
					}

				case <-storeCtx.Done():
					log.Info("Store ticker stopped")
					return nil
				}
			}
		})
	}

	log.Info("Server starting", zap.String("address", cfg.Address))

	server := &http.Server{
		Addr:    cfg.Address,
		Handler: r,
	}

	if err := runHTTPServer(ctx, server); err != nil {
		log.Error("Server stopped with an error", zap.Error(err))
	}

	stopStore()
	if err := storeGroup.Wait(); err != nil {
		log.Error("Failed to stop metrics store", zap.Error(err))
	}
	if cfg.FilePath != "" && dbConn == nil {
		if memStorage, ok := storage.(*repository.MemStorage); ok {
			if err := memStorage.SaveToFile(); err != nil {
				log.Error("Failed to save metrics on shutdown", zap.Error(err))
			} else {
				log.Info("Metrics saved on shutdown", zap.String("file", cfg.FilePath))
			}
		}
	}

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

func runHTTPServer(ctx context.Context, server *http.Server) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	group, serverCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		defer cancel()
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	})
	group.Go(func() error {
		<-serverCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return errors.Join(fmt.Errorf("shutdown HTTP: %w", err), server.Close())
		}
		return nil
	})
	return group.Wait()
}
