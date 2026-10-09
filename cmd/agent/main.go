package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"go-musthave-metrics/internal/agent"
	"go-musthave-metrics/internal/logger"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func printBuildInfo(log *zap.Logger) {
	value := func(s string) string {
		if s == "" {
			return "N/A"
		}
		return s
	}
	log.Info("Build version: " + value(buildVersion))
	log.Info("Build date: " + value(buildDate))
	log.Info("Build commit: " + value(buildCommit))
}

func main() {
	log, err := logger.NewLogger("info")
	if err != nil {
		panic(err)
	}
	defer func() { _ = log.Sync() }()
	printBuildInfo(log)

	cfg, err := agent.GetConfig()
	if err != nil {
		log.Error("Failed to load configuration", zap.Error(err))
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	log.Info("Agent starting",
		zap.Duration("poll_interval", cfg.PollInterval),
		zap.Duration("report_interval", cfg.ReportInterval),
		zap.String("server_address", cfg.ServerAddress),
		zap.Int("rate_limit", cfg.RateLimit),
		zap.String("crypto_key", cfg.CryptoKey),
	)
	collector := agent.NewCollector()
	sender := agent.NewSender(cfg.ServerAddress, cfg.Key, cfg.CryptoKey)
	if err := runAgent(ctx, cfg, collector, sender, log); err != nil {
		log.Error("Agent stopped with an error", zap.Error(err))
		return
	}
	log.Info("Agent stopped")
}

type metricsCollector interface {
	Collect()
	CollectGopsutil()
	DrainMetrics() []*agent.Metric
}

type metricSender interface {
	Send(*agent.Metric) error
}

func runAgent(ctx context.Context, cfg *agent.Config, collector metricsCollector, sender metricSender, log *zap.Logger) error {
	var collectors errgroup.Group
	collectors.Go(func() error {
		return pollMetrics(ctx, cfg.PollInterval, collector.Collect, "Runtime", log)
	})
	collectors.Go(func() error {
		return pollMetrics(ctx, cfg.PollInterval, collector.CollectGopsutil, "Gopsutil", log)
	})

	metricChan := make(chan *agent.Metric, cfg.RateLimit*2)
	var pipeline errgroup.Group
	for workerID := 0; workerID < cfg.RateLimit; workerID++ {
		pipeline.Go(func() error {
			var firstErr error
			for metric := range metricChan {
				if err := sender.Send(metric); err != nil {
					log.Error("Failed to send metric", zap.Int("worker", workerID), zap.String("metric", metric.Name), zap.Error(err))
					if firstErr == nil {
						firstErr = fmt.Errorf("send metric %s: %w", metric.Name, err)
					}
				}
			}
			return firstErr
		})
	}

	pipeline.Go(func() error {
		defer close(metricChan)
		ticker := time.NewTicker(cfg.ReportInterval)
		defer ticker.Stop()

		enqueue := func() {
			metrics := collector.DrainMetrics()
			for _, metric := range metrics {
				metricChan <- metric
			}
			log.Debug("Metrics queued", zap.Int("count", len(metrics)))
		}
		for {
			select {
			case <-ctx.Done():
				log.Info("Shutting down agent")
				if err := collectors.Wait(); err != nil {
					return err
				}
				enqueue()
				return nil
			case <-ticker.C:
				enqueue()
			}
		}
	})

	return pipeline.Wait()
}

func pollMetrics(ctx context.Context, interval time.Duration, collect func(), name string, log *zap.Logger) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Debug("Collector stopped", zap.String("collector", name))
			return nil
		case <-ticker.C:
			collect()
			log.Debug("Metrics collected", zap.String("collector", name))
		}
	}
}
