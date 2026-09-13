package main

import (
	"context"
	"fmt"
	"go-musthave-metrics/internal/agent"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func printBuildInfo() {
	version := buildVersion
	if version == "" {
		version = "N/A"
	}
	date := buildDate
	if date == "" {
		date = "N/A"
	}
	commit := buildCommit
	if commit == "" {
		commit = "N/A"
	}

	fmt.Printf("Build version: %s\n", version)
	fmt.Printf("Build date: %s\n", date)
	fmt.Printf("Build commit: %s\n", commit)
}

func main() {
	printBuildInfo()

	cfg := agent.NewConfig()

	collector := agent.NewCollector()
	sender := agent.NewSender(cfg.ServerAddress, cfg.Key, cfg.CryptoKey)

	fmt.Printf("Agent starting with configuration:\n")
	fmt.Printf("  Poll Interval: %v\n", cfg.PollInterval)
	fmt.Printf("  Report Interval: %v\n", cfg.ReportInterval)
	fmt.Printf("  Server Address: %s\n", cfg.ServerAddress)
	fmt.Printf("  Rate Limit: %d\n", cfg.RateLimit)
	if cfg.CryptoKey != "" {
		fmt.Printf("  Crypto Key: %s\n", cfg.CryptoKey)
	}

	collectCtx, cancelCollectors := context.WithCancel(context.Background())
	defer cancelCollectors()

	var collectorsWG sync.WaitGroup
	var reporterWG sync.WaitGroup
	var workersWG sync.WaitGroup

	metricChan := make(chan *agent.Metric, cfg.RateLimit*2)

	for i := 0; i < cfg.RateLimit; i++ {
		workersWG.Add(1)
		go func(workerID int) {
			defer workersWG.Done()
			for metric := range metricChan {
				if err := sender.Send(metric); err != nil {
					log.Printf("Worker %d: error sending metric %s: %v", workerID, metric.Name, err)
				}
			}
		}(i)
	}

	collectorsWG.Add(1)
	go func() {
		defer collectorsWG.Done()
		ticker := time.NewTicker(cfg.PollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				collector.Collect()
				fmt.Printf("[%s] Runtime metrics collected\n", time.Now().Format(time.RFC3339))
			case <-collectCtx.Done():
				fmt.Println("Runtime collector shutting down")
				return
			}
		}
	}()

	collectorsWG.Add(1)
	go func() {
		defer collectorsWG.Done()
		ticker := time.NewTicker(cfg.PollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				collector.CollectGopsutil()
				fmt.Printf("[%s] Gopsutil metrics collected\n", time.Now().Format(time.RFC3339))
			case <-collectCtx.Done():
				fmt.Println("Gopsutil collector shutting down")
				return
			}
		}
	}()

	reporterWG.Add(1)
	go func() {
		defer reporterWG.Done()
		ticker := time.NewTicker(cfg.ReportInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				metrics := collector.DrainMetrics()
				if len(metrics) == 0 {
					continue
				}
				for _, metric := range metrics {
					select {
					case metricChan <- metric:
					case <-collectCtx.Done():
						return
					}
				}
				fmt.Printf("[%s] Sent %d metrics to worker pool\n", time.Now().Format(time.RFC3339), len(metrics))
			case <-collectCtx.Done():
				return
			}
		}
	}()

	fmt.Println("Agent is running. Press Ctrl+C to stop.")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(sigChan)

	<-sigChan
	fmt.Println("\nShutting down agent...")

	cancelCollectors()

	collectorsWG.Wait()

	reporterWG.Wait()

	metrics := collector.DrainMetrics()
	for _, metric := range metrics {
		metricChan <- metric
	}

	close(metricChan)

	workersWG.Wait()

	fmt.Println("Agent stopped")
}
