package service

import (
	"strings"
	"testing"

	models "go-musthave-metrics/internal/model"
)

type MockStorage struct {
	gauges   map[string]float64
	counters map[string]int64
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (m *MockStorage) SetGauge(name string, value float64) error {
	if name == "" {
		return nil
	}
	m.gauges[name] = value
	return nil
}

func (m *MockStorage) AddCounter(name string, value int64) error {
	if name == "" {
		return nil
	}
	m.counters[name] += value
	return nil
}

func (m *MockStorage) GetGauge(name string) (float64, error) {
	if name == "" {
		return 0, nil
	}
	value, exists := m.gauges[name]
	if !exists {
		return 0, nil
	}
	return value, nil
}

func (m *MockStorage) GetCounter(name string) (int64, error) {
	if name == "" {
		return 0, nil
	}
	value, exists := m.counters[name]
	if !exists {
		return 0, nil
	}
	return value, nil
}

func (m *MockStorage) GetAll() []models.Metrics {
	return nil
}

func (m *MockStorage) UpdateMetricsBatch(metrics []models.Metrics) error {
	for _, metric := range metrics {
		switch metric.MType {
		case models.Gauge:
			if metric.Value != nil {
				m.gauges[metric.ID] = *metric.Value
			}
		case models.Counter:
			if metric.Delta != nil {
				m.counters[metric.ID] += *metric.Delta
			}
		}
	}
	return nil
}

func TestNewMetricsService(t *testing.T) {
	storage := NewMockStorage()
	service := NewMetricsService(storage)

	if service == nil {
		t.Fatal("Expected service to be non-nil")
	}
	if service.storage == nil {
		t.Fatal("Expected storage to be set")
	}
}

func TestMetricsService_ParseUpdatePath(t *testing.T) {
	service := NewMetricsService(NewMockStorage())

	tests := []struct {
		name        string
		path        string
		expectError bool
		expectName  string
		expectType  string
	}{
		{"valid gauge", "/update/gauge/TestMetric/123.456", false, "TestMetric", "gauge"},
		{"valid counter", "/update/counter/TestCounter/100", false, "TestCounter", "counter"},
		{"valid with leading slash", "update/gauge/Metric/1.0", false, "Metric", "gauge"},
		{"invalid - too short", "/update/gauge/Metric", true, "", ""},
		{"invalid - wrong prefix", "/wrong/gauge/Metric/1.0", true, "", ""},
		{"invalid - wrong type", "/update/wrong/Metric/1.0", true, "", ""},
		{"invalid - empty name", "/update/gauge//1.0", true, "", ""},
		{"invalid - empty value", "/update/gauge/Metric/", true, "", ""},
		{"invalid - gauge with int", "/update/gauge/Metric/abc", true, "", ""},
		{"invalid - counter with float", "/update/counter/Metric/1.5", true, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.ParseUpdatePath(tt.path)

			if tt.expectError && err == nil {
				t.Errorf("Expected error for path %s", tt.path)
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error for path %s, got %v", tt.path, err)
			}

			if !tt.expectError && result != nil {
				if result.Name != tt.expectName {
					t.Errorf("Expected name %s, got %s", tt.expectName, result.Name)
				}
				if result.MType != tt.expectType {
					t.Errorf("Expected type %s, got %s", tt.expectType, result.MType)
				}
			}
		})
	}
}

func TestMetricsService_ParseUpdatePath_InvalidPathFormat(t *testing.T) {
	service := NewMetricsService(NewMockStorage())

	invalidPaths := []string{
		"",
		"/",
		"/update",
		"/update/",
		"/update/gauge",
		"/update/gauge/",
		"/update/gauge/Metric",
	}

	for _, path := range invalidPaths {
		_, err := service.ParseUpdatePath(path)
		if err == nil {
			t.Errorf("Expected error for invalid path: %s", path)
		}
		if !strings.Contains(err.Error(), "invalid path format") {
			t.Errorf("Expected 'invalid path format' error for path: %s, got: %v", path, err)
		}
	}
}

func TestMetricsService_ParseUpdatePath_InvalidMetricType(t *testing.T) {
	service := NewMetricsService(NewMockStorage())

	_, err := service.ParseUpdatePath("/update/histogram/Metric/1.0")
	if err == nil {
		t.Fatal("Expected error for invalid metric type")
	}
	if !strings.Contains(err.Error(), "invalid metric type") {
		t.Errorf("Expected 'invalid metric type' error, got: %v", err)
	}
}

func TestMetricsService_ParseUpdatePath_EmptyName(t *testing.T) {
	service := NewMetricsService(NewMockStorage())

	_, err := service.ParseUpdatePath("/update/gauge//1.0")
	if err == nil {
		t.Fatal("Expected error for empty metric name")
	}
	if !strings.Contains(err.Error(), "metric name cannot be empty") {
		t.Errorf("Expected 'metric name cannot be empty' error, got: %v", err)
	}
}

func TestMetricsService_ParseUpdatePath_InvalidGaugeValue(t *testing.T) {
	service := NewMetricsService(NewMockStorage())

	_, err := service.ParseUpdatePath("/update/gauge/Metric/not-a-number")
	if err == nil {
		t.Fatal("Expected error for invalid gauge value")
	}
	if !strings.Contains(err.Error(), "invalid gauge value") {
		t.Errorf("Expected 'invalid gauge value' error, got: %v", err)
	}
}

func TestMetricsService_ParseUpdatePath_InvalidCounterValue(t *testing.T) {
	service := NewMetricsService(NewMockStorage())

	_, err := service.ParseUpdatePath("/update/counter/Metric/1.5")
	if err == nil {
		t.Fatal("Expected error for invalid counter value")
	}
	if !strings.Contains(err.Error(), "invalid counter value") {
		t.Errorf("Expected 'invalid counter value' error, got: %v", err)
	}
}

func TestMetricsService_UpdateMetric_Gauge(t *testing.T) {
	storage := NewMockStorage()
	service := NewMetricsService(storage)

	result := &UpdateMetricResult{
		MType: "gauge",
		Name:  "TestGauge",
		Value: "123.456",
	}

	err := service.UpdateMetric(result)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if storage.gauges["TestGauge"] != 123.456 {
		t.Errorf("Expected gauge value 123.456, got %f", storage.gauges["TestGauge"])
	}
}

func TestMetricsService_UpdateMetric_Counter(t *testing.T) {
	storage := NewMockStorage()
	service := NewMetricsService(storage)

	result := &UpdateMetricResult{
		MType: "counter",
		Name:  "TestCounter",
		Value: "100",
	}

	err := service.UpdateMetric(result)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if storage.counters["TestCounter"] != 100 {
		t.Errorf("Expected counter value 100, got %d", storage.counters["TestCounter"])
	}
}

func TestMetricsService_UpdateMetric_UnknownType(t *testing.T) {
	storage := NewMockStorage()
	service := NewMetricsService(storage)

	result := &UpdateMetricResult{
		MType: "unknown",
		Name:  "TestMetric",
		Value: "100",
	}

	err := service.UpdateMetric(result)
	if err == nil {
		t.Fatal("Expected error for unknown metric type")
	}
	if !strings.Contains(err.Error(), "unknown metric type") {
		t.Errorf("Expected 'unknown metric type' error, got: %v", err)
	}
}

func TestMetricsService_UpdateMetric_InvalidGaugeValue(t *testing.T) {
	storage := NewMockStorage()
	service := NewMetricsService(storage)

	result := &UpdateMetricResult{
		MType: "gauge",
		Name:  "TestGauge",
		Value: "not-a-number",
	}

	err := service.UpdateMetric(result)
	if err == nil {
		t.Fatal("Expected error for invalid gauge value")
	}
}

func TestMetricsService_UpdateMetric_InvalidCounterValue(t *testing.T) {
	storage := NewMockStorage()
	service := NewMetricsService(storage)

	result := &UpdateMetricResult{
		MType: "counter",
		Name:  "TestCounter",
		Value: "not-a-number",
	}

	err := service.UpdateMetric(result)
	if err == nil {
		t.Fatal("Expected error for invalid counter value")
	}
}
