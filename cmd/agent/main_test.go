package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"go-musthave-metrics/internal/agent"

	"go.uber.org/zap"
)

type snapshotCollector struct {
	snapshots [][]*agent.Metric
	drained   chan struct{}
	mu        sync.Mutex
}

func (c *snapshotCollector) Collect()         {}
func (c *snapshotCollector) CollectGopsutil() {}
func (c *snapshotCollector) DrainMetrics() []*agent.Metric {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.snapshots) == 0 {
		return nil
	}
	metrics := c.snapshots[0]
	c.snapshots = c.snapshots[1:]
	if c.drained != nil {
		close(c.drained)
		c.drained = nil
	}
	return metrics
}

type sendFunc func(*agent.Metric) error

func (f sendFunc) Send(metric *agent.Metric) error { return f(metric) }

func awaitAgent(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("agent did not drain its queue and stop")
		return nil
	}
}

func TestRunAgentFinishesBatchOnShutdown(t *testing.T) {
	var batch []*agent.Metric
	for i := 0; i < 8; i++ {
		batch = append(batch, &agent.Metric{Name: fmt.Sprintf("gauge%d", i), MType: "gauge", Value: float64(i)})
	}
	batch = append(batch, &agent.Metric{Name: "PollCount", MType: "counter", Value: 3})
	drained := make(chan struct{})
	collector := &snapshotCollector{
		snapshots: [][]*agent.Metric{batch, {{Name: "PollCount", MType: "counter", Value: 2}}},
		drained:   drained,
	}
	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce, releaseOnce sync.Once
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		releaseOnce.Do(func() { close(release) })
	})
	var sent []*agent.Metric
	sender := sendFunc(func(metric *agent.Metric) error {
		startedOnce.Do(func() { close(started) })
		<-release
		sent = append(sent, metric)
		return nil
	})
	cfg := &agent.Config{PollInterval: time.Hour, ReportInterval: time.Millisecond, RateLimit: 1}
	done := make(chan error, 1)
	go func() { done <- runAgent(ctx, cfg, collector, sender, zap.NewNop()) }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not receive the first metric")
	}
	<-drained
	cancel()
	releaseOnce.Do(func() { close(release) })
	if err := awaitAgent(t, done); err != nil {
		t.Fatal(err)
	}
	if len(sent) != len(batch)+1 {
		t.Fatalf("sent %d metrics, want %d including the final snapshot", len(sent), len(batch)+1)
	}
	var pollCount float64
	for _, metric := range sent {
		if metric.Name == "PollCount" {
			pollCount += metric.Value
		}
	}
	if pollCount != 5 {
		t.Fatalf("PollCount = %v, want 5 without lost increments", pollCount)
	}
}

func TestRunAgentDrainsQueueAfterSendError(t *testing.T) {
	wantErr := errors.New("server rejected metric")
	var batch []*agent.Metric
	for i := 0; i < 10; i++ {
		batch = append(batch, &agent.Metric{Name: fmt.Sprintf("metric%d", i), MType: "gauge"})
	}
	collector := &snapshotCollector{snapshots: [][]*agent.Metric{batch}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var mu sync.Mutex
	var sent int
	sender := sendFunc(func(*agent.Metric) error {
		mu.Lock()
		sent++
		mu.Unlock()
		return wantErr
	})
	cfg := &agent.Config{PollInterval: time.Hour, ReportInterval: time.Hour, RateLimit: 2}
	done := make(chan error, 1)
	go func() { done <- runAgent(ctx, cfg, collector, sender, zap.NewNop()) }()
	if err := awaitAgent(t, done); !errors.Is(err, wantErr) {
		t.Fatalf("got %v, want the send error from errgroup", err)
	}
	if sent != len(batch) {
		t.Fatalf("sent %d metrics, want all %d attempted", sent, len(batch))
	}
}
