package main

import (
	"context"
	"crypto/rsa"
	"encoding/json"
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
	"go-musthave-metrics/internal/crypto"
	"go-musthave-metrics/internal/handler"
	"go-musthave-metrics/internal/logger"
	"go-musthave-metrics/internal/middleware"
	"go-musthave-metrics/internal/repository"
	"go-musthave-metrics/internal/service"
	"go-musthave-metrics/internal/service/audit"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	defaultRunAddr       = ":8080"
	defaultLogLevel      = "info"
	defaultStoreInterval = 300
	defaultRestore       = false
)

type fileConfig struct {
	Address        string `json:"address"`
	Restore        *bool  `json:"restore"`
	StoreInterval  string `json:"store_interval"`
	StoreFile      string `json:"store_file"`
	DatabaseDSN    string `json:"database_dsn"`
	CryptoKey      string `json:"crypto_key"`
	Key            string `json:"key"`
	LogLevel       string `json:"log_level"`
	AuditFile      string `json:"audit_file"`
	AuditURL       string `json:"audit_url"`
}

var (
	flagRunAddr       string
	flagLogLevel      string
	flagStoreInterval int
	flagFilePath      string
	flagRestore       bool
	flagDBDSN         string
	flagKey           string
	flagCryptoKey     string
	flagAuditFile     string
	flagAuditURL      string
	flagConfig        string
	filePathSet       bool
)

func parseFlags() error {
	flag.StringVar(&flagRunAddr, "a", defaultRunAddr, "address and port to run server")
	flag.StringVar(&flagLogLevel, "l", defaultLogLevel, "log level")
	flag.IntVar(&flagStoreInterval, "i", defaultStoreInterval, "store interval in seconds")
	flag.StringVar(&flagFilePath, "f", "", "path to metrics file")
	flag.BoolVar(&flagRestore, "r", defaultRestore, "restore metrics from file")
	flag.StringVar(&flagDBDSN, "d", "", "database DSN")
	flag.StringVar(&flagKey, "k", "", "key for signing data")
	flag.StringVar(&flagCryptoKey, "crypto-key", "", "path to private key file for decryption")
	flag.StringVar(&flagAuditFile, "audit-file", "", "path to audit log file")
	flag.StringVar(&flagAuditURL, "audit-url", "", "URL to send audit logs")
	flag.StringVar(&flagConfig, "c", "", "path to JSON config file")
	flag.StringVar(&flagConfig, "config", "", "path to JSON config file")
	flag.Parse()

	setFlags := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })

	// Путь к конфигурации: флаг -c/-config имеет приоритет над CONFIG.
	cfgPath := flagConfig
	if !setFlags["c"] && !setFlags["config"] {
		if env, ok := os.LookupEnv("CONFIG"); ok && env != "" {
			cfgPath = env
		}
	}

	var fc fileConfig
	if cfgPath != "" {
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			return fmt.Errorf("failed to read config file %s: %w", cfgPath, err)
		}
		if err := json.Unmarshal(data, &fc); err != nil {
			return fmt.Errorf("failed to parse config file %s: %w", cfgPath, err)
		}
	}

	if v, ok := os.LookupEnv("ADDRESS"); ok && v != "" {
		flagRunAddr = v
	} else if !setFlags["a"] && fc.Address != "" {
		flagRunAddr = fc.Address
	}

	if v, ok := os.LookupEnv("LOG_LEVEL"); ok && v != "" {
		flagLogLevel = v
	} else if !setFlags["l"] && fc.LogLevel != "" {
		flagLogLevel = fc.LogLevel
	}

	if v, ok := os.LookupEnv("STORE_INTERVAL"); ok && v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid STORE_INTERVAL value: %s", v)
		}
		flagStoreInterval = n
	} else if !setFlags["i"] && fc.StoreInterval != "" {
		n, err := strconv.Atoi(fc.StoreInterval)
		if err != nil {
			return fmt.Errorf("invalid store_interval value in config: %s", fc.StoreInterval)
		}
		flagStoreInterval = n
	}

	if v, ok := os.LookupEnv("FILE_STORAGE_PATH"); ok && v != "" {
		flagFilePath = v
		filePathSet = true
	} else if !setFlags["f"] && fc.StoreFile != "" {
		flagFilePath = fc.StoreFile
		filePathSet = true
	}

	if v, ok := os.LookupEnv("RESTORE"); ok && v != "" {
		flagRestore = v == "true"
	} else if !setFlags["r"] && fc.Restore != nil {
		flagRestore = *fc.Restore
	}

	if v, ok := os.LookupEnv("DATABASE_DSN"); ok && v != "" {
		flagDBDSN = v
	} else if !setFlags["d"] && fc.DatabaseDSN != "" {
		flagDBDSN = fc.DatabaseDSN
	}

	if v, ok := os.LookupEnv("KEY"); ok && v != "" {
		flagKey = v
	} else if !setFlags["k"] && fc.Key != "" {
		flagKey = fc.Key
	}

	if v, ok := os.LookupEnv("CRYPTO_KEY"); ok && v != "" {
		flagCryptoKey = v
	} else if !setFlags["crypto-key"] && fc.CryptoKey != "" {
		flagCryptoKey = fc.CryptoKey
	}

	if v, ok := os.LookupEnv("AUDIT_FILE"); ok && v != "" {
		flagAuditFile = v
	} else if !setFlags["audit-file"] && fc.AuditFile != "" {
		flagAuditFile = fc.AuditFile
	}

	if v, ok := os.LookupEnv("AUDIT_URL"); ok && v != "" {
		flagAuditURL = v
	} else if !setFlags["audit-url"] && fc.AuditURL != "" {
		flagAuditURL = fc.AuditURL
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

	var privateKey *rsa.PrivateKey
	if flagCryptoKey != "" {
		privateKey, err = crypto.LoadPrivateKey(flagCryptoKey)
		if err != nil {
			log.Fatal("Failed to load private key", zap.Error(err))
		}
		log.Info("Private key loaded", zap.String("path", flagCryptoKey))
	}

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
	jsonRoutes.Use(middleware.Decrypt(privateKey))
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