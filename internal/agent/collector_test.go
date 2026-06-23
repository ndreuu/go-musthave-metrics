package agent

import (
	"testing"
)

func TestNewCollector(t *testing.T) {
	collector := NewCollector(2)

	if collector == nil {
		t.Fatal("Expected collector to be non-nil")
	}
	if collector.metrics == nil {
		t.Fatal("Expected metrics map to be initialized")
	}
	if collector.pollCount != 0 {
		t.Fatalf("Expected pollCount to be 0, got %d", collector.pollCount)
	}
}

func TestCollector_Collect(t *testing.T) {
	collector := NewCollector(2)

	collector.Collect()

	metrics := collector.GetMetrics()
	if len(metrics) == 0 {
		t.Fatal("Expected metrics to be collected")
	}

	runtimeMetrics := []string{
		"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys",
		"HeapAlloc", "HeapIdle", "HeapInuse", "HeapObjects", "HeapReleased",
		"HeapSys", "LastGC", "Lookups", "MCacheInuse", "MCacheSys",
		"MSpanInuse", "MSpanSys", "Mallocs", "NextGC", "NumForcedGC",
		"NumGC", "OtherSys", "PauseTotalNs", "StackInuse", "StackSys",
		"Sys", "TotalAlloc",
	}

	collectedNames := make(map[string]bool)
	for _, m := range metrics {
		collectedNames[m.Name] = true
	}

	for _, expectedMetric := range runtimeMetrics {
		if !collectedNames[expectedMetric] {
			t.Errorf("Metric %s should be collected", expectedMetric)
		}
	}

	pollCountMetric := collector.GetMetric("PollCount")
	if pollCountMetric == nil {
		t.Fatal("PollCount metric should exist")
	}
	if pollCountMetric.MType != "counter" {
		t.Errorf("PollCount should be a counter, got %s", pollCountMetric.MType)
	}
	if pollCountMetric.Value != 1 {
		t.Errorf("PollCount should be 1 after first collection, got %f", pollCountMetric.Value)
	}

	randomValueMetric := collector.GetMetric("RandomValue")
	if randomValueMetric == nil {
		t.Fatal("RandomValue metric should exist")
	}
	if randomValueMetric.MType != "gauge" {
		t.Errorf("RandomValue should be a gauge, got %s", randomValueMetric.MType)
	}
	if randomValueMetric.Value < 0 || randomValueMetric.Value > 1000 {
		t.Errorf("RandomValue should be between 0 and 1000, got %f", randomValueMetric.Value)
	}
}

func TestCollector_Collect_MultipleTimes(t *testing.T) {
	collector := NewCollector(2)

	collector.Collect()
	collector.Collect()
	collector.Collect()

	pollCountMetric := collector.GetMetric("PollCount")
	if pollCountMetric == nil {
		t.Fatal("PollCount metric should exist")
	}
	if pollCountMetric.Value != 3 {
		t.Errorf("PollCount should be 3 after three collections, got %f", pollCountMetric.Value)
	}
}

func TestCollector_GetMetric(t *testing.T) {
	collector := NewCollector(2)
	collector.Collect()

	metric := collector.GetMetric("Alloc")
	if metric == nil {
		t.Fatal("Alloc metric should exist")
	}
	if metric.Name != "Alloc" {
		t.Errorf("Expected metric name to be Alloc, got %s", metric.Name)
	}
	if metric.MType != "gauge" {
		t.Errorf("Alloc should be a gauge, got %s", metric.MType)
	}

	nonExisting := collector.GetMetric("NonExistingMetric")
	if nonExisting != nil {
		t.Error("NonExistingMetric should return nil")
	}
}

func TestCollector_GetMetrics(t *testing.T) {
	collector := NewCollector(2)
	collector.Collect()

	metrics := collector.GetMetrics()
	if len(metrics) == 0 {
		t.Fatal("Expected metrics to be returned")
	}

	for _, m := range metrics {
		if m.MType != "gauge" && m.MType != "counter" {
			t.Errorf("Metric %s has invalid type: %s", m.Name, m.MType)
		}
		if m.Name == "" {
			t.Error("Metric name should not be empty")
		}
	}
}

func TestCollector_Collect_RuntimeMetricsTypes(t *testing.T) {
	collector := NewCollector(2)
	collector.Collect()

	runtimeGauges := []string{
		"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys",
		"HeapAlloc", "HeapIdle", "HeapInuse", "HeapObjects", "HeapReleased",
		"HeapSys", "LastGC", "Lookups", "MCacheInuse", "MCacheSys",
		"MSpanInuse", "MSpanSys", "Mallocs", "NextGC", "NumForcedGC",
		"NumGC", "OtherSys", "PauseTotalNs", "StackInuse", "StackSys",
		"Sys", "TotalAlloc", "RandomValue",
	}

	for _, metricName := range runtimeGauges {
		metric := collector.GetMetric(metricName)
		if metric == nil {
			t.Errorf("Metric %s should exist", metricName)
			continue
		}
		if metric.MType != "gauge" {
			t.Errorf("Metric %s should be a gauge, got %s", metricName, metric.MType)
		}
	}

	pollCount := collector.GetMetric("PollCount")
	if pollCount == nil {
		t.Fatal("PollCount metric should exist")
	}
	if pollCount.MType != "counter" {
		t.Errorf("PollCount should be a counter, got %s", pollCount.MType)
	}
}
