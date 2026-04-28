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
	sender := agent.NewSender(cfg.ServerAddress)

	fmt.Printf("Agent starting with configuration:\n")
	fmt.Printf("  Poll Interval: %v\n", cfg.PollInterval)
	fmt.Printf("  Report Interval: %v\n", cfg.ReportInterval)
	fmt.Printf("  Server Address: %s\n", cfg.ServerAddress)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(cfg.PollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				collector.Collect()
				fmt.Printf("[%s] Metrics collected\n", time.Now().Format(time.RFC3339))
			case <-ctx.Done():
				fmt.Println("Collector shutting down")
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
				errors := sender.SendAll(metrics)
				if len(errors) > 0 {
					for _, err := range errors {
						log.Printf("Error sending metric: %v", err)
					}
				} else {
					fmt.Printf("[%s] Sent %d metrics to server\n", time.Now().Format(time.RFC3339), len(metrics))
				}
			case <-ctx.Done():
				fmt.Println("Sender shutting down")
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
