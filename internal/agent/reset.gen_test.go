package agent

import (
	"testing"
	"time"
)

func TestMetric_Reset(t *testing.T) {
	m := &Metric{
		MType: "gauge",
		Name:  "Alloc",
		Value: 12.5,
	}

	m.Reset()

	if m.MType != "" {
		t.Errorf("expected empty MType, got %q", m.MType)
	}
	if m.Name != "" {
		t.Errorf("expected empty Name, got %q", m.Name)
	}
	if m.Value != 0 {
		t.Errorf("expected zero Value, got %v", m.Value)
	}
}

func TestCollector_Reset(t *testing.T) {
	c := &Collector{
		pollCount: 42,
	}

	c.Reset()

	if c.pollCount != 0 {
		t.Errorf("expected zero pollCount, got %d", c.pollCount)
	}
	if len(c.metrics) != 0 {
		t.Errorf("expected empty metrics, got %d entries", len(c.metrics))
	}
}

func TestConfig_Reset(t *testing.T) {
	c := &Config{
		ServerAddress:  "localhost:8080",
		Key:            "secret",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		RateLimit:      5,
	}

	c.Reset()

	if c.ServerAddress != "" {
		t.Errorf("expected empty ServerAddress, got %q", c.ServerAddress)
	}
	if c.Key != "" {
		t.Errorf("expected empty Key, got %q", c.Key)
	}
	if c.PollInterval != 0 {
		t.Errorf("expected zero PollInterval, got %v", c.PollInterval)
	}
	if c.ReportInterval != 0 {
		t.Errorf("expected zero ReportInterval, got %v", c.ReportInterval)
	}
	if c.RateLimit != 0 {
		t.Errorf("expected zero RateLimit, got %d", c.RateLimit)
	}
}

func TestReset_NilReceiver(t *testing.T) {
	var metric *Metric
	var collector *Collector
	var config *Config

	metric.Reset()
	collector.Reset()
	config.Reset()
}
