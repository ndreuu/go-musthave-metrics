package service

import (
	"context"
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

func (m *MockStorage) SetGauge(ctx context.Context, name string, value float64) error {
	if name == "" {
		return nil
	}
	m.gauges[name] = value
	return nil
}

func (m *MockStorage) AddCounter(ctx context.Context, name string, value int64) error {
	if name == "" {
		return nil
	}
	m.counters[name] += value
	return nil
}

func (m *MockStorage) GetGauge(ctx context.Context, name string) (float64, error) {
	if name == "" {
		return 0, nil
	}
	value, exists := m.gauges[name]
	if !exists {
		return 0, nil
	}
	return value, nil
}

func (m *MockStorage) GetCounter(ctx context.Context, name string) (int64, error) {
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

func (m *MockStorage) UpdateMetricsBatch(ctx context.Context, metrics []models.Metrics) error {
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
		name       string
		expectName string
		expectType string
		path       string
		expectErr  bool
	}{
		{"valid gauge", "TestMetric", "gauge", "/update/gauge/TestMetric/123.456", false},
		{"valid counter", "TestCounter", "counter", "/update/counter/TestCounter/100", false},
		{"valid with leading slash", "Metric", "gauge", "update/gauge/Metric/1.0", false},
		{"invalid - too short", "", "", "/update/gauge/Metric", true},
		{"invalid - wrong prefix", "", "", "/wrong/gauge/Metric/1.0", true},
		{"invalid - wrong type", "", "", "/update/wrong/Metric/1.0", true},
		{"invalid - empty name", "", "", "/update/gauge//1.0", true},
		{"invalid - empty value", "", "", "/update/gauge/Metric/", true},
		{"invalid - gauge with int", "", "", "/update/gauge/Metric/abc", true},
		{"invalid - counter with float", "", "", "/update/counter/Metric/1.5", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.ParseUpdatePath(tt.path)

			if tt.expectErr && err == nil {
				t.Errorf("Expected error for path %s", tt.path)
			}

			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error for path %s, got %v", tt.path, err)
			}

			if !tt.expectErr && result != nil {
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

	err := service.UpdateMetric(context.Background(), result)
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

	err := service.UpdateMetric(context.Background(), result)
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

	err := service.UpdateMetric(context.Background(), result)
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

	err := service.UpdateMetric(context.Background(), result)
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

	err := service.UpdateMetric(context.Background(), result)
	if err == nil {
		t.Fatal("Expected error for invalid counter value")
	}
}

func TestMetricsService_GetMetricValue_Gauge(t *testing.T) {
	storage := NewMockStorage()
	storage.gauges["TestGauge"] = 123.456
	service := NewMetricsService(storage)

	value, err := service.GetMetricValue(context.Background(), "gauge", "TestGauge")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if value != "123.456" {
		t.Errorf("Expected '123.456', got %s", value)
	}
}

func TestMetricsService_GetMetricValue_Counter(t *testing.T) {
	storage := NewMockStorage()
	storage.counters["TestCounter"] = 42
	service := NewMetricsService(storage)

	value, err := service.GetMetricValue(context.Background(), "counter", "TestCounter")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if value != "42" {
		t.Errorf("Expected '42', got %s", value)
	}
}

func TestMetricsService_GetMetricValue_EmptyName(t *testing.T) {
	service := NewMetricsService(NewMockStorage())

	_, err := service.GetMetricValue(context.Background(), "gauge", "")
	if err == nil {
		t.Fatal("Expected error for empty name")
	}
}

func TestMetricsService_GetMetricValue_InvalidType(t *testing.T) {
	service := NewMetricsService(NewMockStorage())

	_, err := service.GetMetricValue(context.Background(), "histogram", "Metric")
	if err == nil {
		t.Fatal("Expected error for invalid type")
	}
	if !strings.Contains(err.Error(), "invalid metric type") {
		t.Errorf("Expected 'invalid metric type' error, got: %v", err)
	}
}

func TestMetricsService_GetAllMetrics(t *testing.T) {
	storage := NewMockStorage()
	storage.gauges["Gauge1"] = 1.5
	storage.counters["Counter1"] = 10
	service := NewMetricsService(storage)

	metrics := service.GetAllMetrics()
	if metrics != nil {
		t.Fatal("Expected nil metrics from mock storage")
	}
}

func TestMetricsService_UpdateMetricFromJSON_Gauge(t *testing.T) {
	storage := NewMockStorage()
	service := NewMetricsService(storage)

	value := 3.14
	m := &models.Metrics{ID: "Gauge", MType: models.Gauge, Value: &value}

	err := service.UpdateMetricFromJSON(context.Background(), m)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if storage.gauges["Gauge"] != 3.14 {
		t.Errorf("Expected gauge 3.14, got %f", storage.gauges["Gauge"])
	}
}

func TestMetricsService_UpdateMetricFromJSON_Counter(t *testing.T) {
	storage := NewMockStorage()
	service := NewMetricsService(storage)

	delta := int64(5)
	m := &models.Metrics{ID: "Counter", MType: models.Counter, Delta: &delta}

	err := service.UpdateMetricFromJSON(context.Background(), m)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if storage.counters["Counter"] != 5 {
		t.Errorf("Expected counter 5, got %d", storage.counters["Counter"])
	}
}

func TestMetricsService_UpdateMetricFromJSON_EmptyID(t *testing.T) {
	service := NewMetricsService(NewMockStorage())
	value := 1.0
	m := &models.Metrics{ID: "", MType: models.Gauge, Value: &value}

	err := service.UpdateMetricFromJSON(context.Background(), m)
	if err == nil {
		t.Fatal("Expected error for empty ID")
	}
}

func TestMetricsService_UpdateMetricFromJSON_EmptyType(t *testing.T) {
	service := NewMetricsService(NewMockStorage())
	value := 1.0
	m := &models.Metrics{ID: "Gauge", MType: "", Value: &value}

	err := service.UpdateMetricFromJSON(context.Background(), m)
	if err == nil {
		t.Fatal("Expected error for empty type")
	}
}

func TestMetricsService_UpdateMetricFromJSON_MissingGaugeValue(t *testing.T) {
	service := NewMetricsService(NewMockStorage())
	m := &models.Metrics{ID: "Gauge", MType: models.Gauge}

	err := service.UpdateMetricFromJSON(context.Background(), m)
	if err == nil {
		t.Fatal("Expected error for missing gauge value")
	}
}

func TestMetricsService_UpdateMetricFromJSON_MissingCounterDelta(t *testing.T) {
	service := NewMetricsService(NewMockStorage())
	m := &models.Metrics{ID: "Counter", MType: models.Counter}

	err := service.UpdateMetricFromJSON(context.Background(), m)
	if err == nil {
		t.Fatal("Expected error for missing counter delta")
	}
}

func TestMetricsService_UpdateMetricFromJSON_InvalidType(t *testing.T) {
	service := NewMetricsService(NewMockStorage())
	value := 1.0
	m := &models.Metrics{ID: "Gauge", MType: "histogram", Value: &value}

	err := service.UpdateMetricFromJSON(context.Background(), m)
	if err == nil {
		t.Fatal("Expected error for invalid type")
	}
}

func TestMetricsService_GetMetricFromJSON_Gauge(t *testing.T) {
	storage := NewMockStorage()
	storage.gauges["Gauge"] = 9.99
	service := NewMetricsService(storage)

	m := &models.Metrics{ID: "Gauge", MType: models.Gauge}

	result, err := service.GetMetricFromJSON(context.Background(), m)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result.Value == nil || *result.Value != 9.99 {
		t.Errorf("Expected value 9.99, got %v", result.Value)
	}
}

func TestMetricsService_GetMetricFromJSON_Counter(t *testing.T) {
	storage := NewMockStorage()
	storage.counters["Counter"] = 77
	service := NewMetricsService(storage)

	m := &models.Metrics{ID: "Counter", MType: models.Counter}

	result, err := service.GetMetricFromJSON(context.Background(), m)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result.Delta == nil || *result.Delta != 77 {
		t.Errorf("Expected delta 77, got %v", result.Delta)
	}
}

func TestMetricsService_GetMetricFromJSON_EmptyID(t *testing.T) {
	service := NewMetricsService(NewMockStorage())
	m := &models.Metrics{ID: "", MType: models.Gauge}

	_, err := service.GetMetricFromJSON(context.Background(), m)
	if err == nil {
		t.Fatal("Expected error for empty ID")
	}
}

func TestMetricsService_GetMetricFromJSON_InvalidType(t *testing.T) {
	service := NewMetricsService(NewMockStorage())
	m := &models.Metrics{ID: "Gauge", MType: "histogram"}

	_, err := service.GetMetricFromJSON(context.Background(), m)
	if err == nil {
		t.Fatal("Expected error for invalid type")
	}
}

func TestMetricsService_UpdateMetricsBatch(t *testing.T) {
	storage := NewMockStorage()
	service := NewMetricsService(storage)

	value := 1.5
	delta := int64(10)
	metrics := []models.Metrics{
		{ID: "Gauge", MType: models.Gauge, Value: &value},
		{ID: "Counter", MType: models.Counter, Delta: &delta},
	}

	err := service.UpdateMetricsBatch(context.Background(), metrics)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if storage.gauges["Gauge"] != 1.5 {
		t.Errorf("Expected gauge 1.5, got %f", storage.gauges["Gauge"])
	}
	if storage.counters["Counter"] != 10 {
		t.Errorf("Expected counter 10, got %d", storage.counters["Counter"])
	}
}
