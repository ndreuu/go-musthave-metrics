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

func main() {
	cfg := agent.NewConfig()

	collector := agent.NewCollector()
	sender := agent.NewSender(cfg.ServerAddress, cfg.Key)

	fmt.Printf("Agent starting with configuration:\n")
	fmt.Printf("  Poll Interval: %v\n", cfg.PollInterval)
	fmt.Printf("  Report Interval: %v\n", cfg.ReportInterval)
	fmt.Printf("  Server Address: %s\n", cfg.ServerAddress)
	fmt.Printf("  Rate Limit: %d\n", cfg.RateLimit)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	metricChan := make(chan *agent.Metric, cfg.RateLimit*2)

	for i := 0; i < cfg.RateLimit; i++ {
		wg.Add(1)
		go func(workerId int) {
			defer wg.Done()
			for {
				select {
				case metric, ok := <-metricChan:
					if !ok {
						return
					}
					if err := sender.Send(metric); err != nil {
						log.Printf("Worker %d: Error sending metric %s: %v", workerId, metric.Name, err)
					}
				case <-ctx.Done():
					return
				}
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(cfg.PollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				collector.Collect()
				fmt.Printf("[%s] Runtime metrics collected\n", time.Now().Format(time.RFC3339))
			case <-ctx.Done():
				fmt.Println("Runtime collector shutting down")
				return
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(cfg.PollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				collector.CollectGopsutil()
				fmt.Printf("[%s] Gopsutil metrics collected\n", time.Now().Format(time.RFC3339))
			case <-ctx.Done():
				fmt.Println("Gopsutil collector shutting down")
				return
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(cfg.ReportInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				metrics := collector.GetMetrics()
				if len(metrics) > 0 {
					for _, metric := range metrics {
						select {
						case metricChan <- metric:
						case <-ctx.Done():
							return
						}
					}
					fmt.Printf("[%s] Sent %d metrics to worker pool\n", time.Now().Format(time.RFC3339), len(metrics))
				}
			case <-ctx.Done():
				fmt.Println("Sender shutting down")
				close(metricChan)
				return
			}
		}
	}()

	fmt.Println("Agent is running. Press Ctrl+C to stop.")

	<-sigChan
	fmt.Println("\nShutting down agent...")
	cancel()

	wg.Wait()
	fmt.Println("Agent stopped")
}
