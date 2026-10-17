package main

import (
	"crypto/rsa"

	serverconfig "go-musthave-metrics/internal/config/server"
	"go-musthave-metrics/internal/handler"
	"go-musthave-metrics/internal/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func newRouter(cfg *serverconfig.Config, log *zap.Logger, privateKey *rsa.PrivateKey,
	metricsHandler *handler.MetricsHandler, pingHandler *handler.PingHandler) (*gin.Engine, error) {
	r := gin.New()
	r.RedirectTrailingSlash = false
	r.Use(gin.Recovery(), middleware.RequestLogger(log))

	updates := r.Group("/")
	if cfg.TrustedSubnet != "" {
		checkSubnet, err := middleware.TrustedSubnet(cfg.TrustedSubnet)
		if err != nil {
			return nil, err
		}
		updates.Use(checkSubnet)
	}
	updates.Use(middleware.GzipUnmarshal(), middleware.Gzip())
	updates.POST("/update/:type/:name/:value", metricsHandler.UpdateMetricHandler)
	jsonUpdates := updates.Group("/", middleware.Decrypt(privateKey), middleware.HashSHA256(cfg.Key))
	jsonUpdates.POST("/update", metricsHandler.UpdateMetricJSONHandler)
	jsonUpdates.POST("/update/", metricsHandler.UpdateMetricJSONHandler)
	jsonUpdates.POST("/updates", metricsHandler.UpdateMetricsBatchHandler)
	jsonUpdates.POST("/updates/", metricsHandler.UpdateMetricsBatchHandler)

	reads := r.Group("/", middleware.GzipUnmarshal(), middleware.Gzip())
	reads.GET("/value/:type/:name", metricsHandler.GetMetricHandler)
	reads.GET("/", metricsHandler.ListMetricsHandler)
	jsonReads := reads.Group("/", middleware.Decrypt(privateKey), middleware.HashSHA256(cfg.Key))
	jsonReads.POST("/value", metricsHandler.GetMetricJSONHandler)
	jsonReads.POST("/value/", metricsHandler.GetMetricJSONHandler)
	if pingHandler != nil {
		reads.GET("/ping", pingHandler.PingHandler)
	}

	return r, nil
}
