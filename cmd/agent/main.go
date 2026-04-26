package main

import (
	"fmt"
	"log"
	"time"

	"go-musthave-metrics/internal/agent"
)

func main() {
	cfg := agent.NewConfig()

	collector := agent.NewCollector()
	sender := agent.NewSender(cfg.ServerAddress)

	fmt.Printf("Agent starting with configuration:\n")
	fmt.Printf("  Poll Interval: %v\n", cfg.PollInterval)
	fmt.Printf("  Report Interval: %v\n", cfg.ReportInterval)
	fmt.Printf("  Server Address: %s\n", cfg.ServerAddress)

	done := make(chan bool)

	go func() {
		ticker := time.NewTicker(cfg.PollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				collector.Collect()
				fmt.Printf("[%s] Metrics collected\n", time.Now().Format(time.RFC3339))
			case <-done:
				fmt.Println("Collector shutting down")
				return
			}
		}
	}()

	go func() {
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
			case <-done:
				fmt.Println("Sender shutting down")
				return
			}
		}
	}()

	fmt.Println("Agent is running. Press Ctrl+C to stop.")

	select {}
}
