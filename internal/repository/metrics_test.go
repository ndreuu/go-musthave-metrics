package repository

import (
	"context"
	"os"
	"strings"
	"testing"

	models "go-musthave-metrics/internal/model"
)

func TestNewMemStorage(t *testing.T) {
	storage := NewMemStorage("")

	if storage == nil {
		t.Fatal("Expected storage to be non-nil")
	}
	if storage.gauges == nil {
		t.Fatal("Expected gauges map to be initialized")
	}
	if storage.counters == nil {
		t.Fatal("Expected counters map to be initialized")
	}
}

func TestMemStorage_SetGauge(t *testing.T) {
	storage := NewMemStorage("")

	err := storage.SetGauge(context.Background(), "TestGauge", 123.456)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	value, err := storage.GetGauge(context.Background(), "TestGauge")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if value != 123.456 {
		t.Errorf("Expected value 123.456, got %f", value)
	}
}

func TestMemStorage_SetGauge_EmptyName(t *testing.T) {
	storage := NewMemStorage("")

	err := storage.SetGauge(context.Background(), "", 123.456)
	if err == nil {
		t.Fatal("Expected error for empty name")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Expected error about empty name, got %v", err)
	}
}

func TestMemStorage_SetGauge_Overwrite(t *testing.T) {
	storage := NewMemStorage("")

	err := storage.SetGauge(context.Background(), "TestGauge", 100.0)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = storage.SetGauge(context.Background(), "TestGauge", 200.0)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	value, err := storage.GetGauge(context.Background(), "TestGauge")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if value != 200.0 {
		t.Errorf("Expected value 200.0, got %f", value)
	}
}

func TestMemStorage_AddCounter(t *testing.T) {
	storage := NewMemStorage("")

	err := storage.AddCounter(context.Background(), "TestCounter", 10)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	value, err := storage.GetCounter(context.Background(), "TestCounter")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if value != 10 {
		t.Errorf("Expected value 10, got %d", value)
	}
}

func TestMemStorage_AddCounter_EmptyName(t *testing.T) {
	storage := NewMemStorage("")

	err := storage.AddCounter(context.Background(), "", 10)
	if err == nil {
		t.Fatal("Expected error for empty name")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Expected error about empty name, got %v", err)
	}
}

func TestMemStorage_AddCounter_Accumulate(t *testing.T) {
	storage := NewMemStorage("")

	err := storage.AddCounter(context.Background(), "TestCounter", 10)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = storage.AddCounter(context.Background(), "TestCounter", 20)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = storage.AddCounter(context.Background(), "TestCounter", 30)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	value, err := storage.GetCounter(context.Background(), "TestCounter")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if value != 60 {
		t.Errorf("Expected value 60, got %d", value)
	}
}

func TestMemStorage_GetGauge_NotFound(t *testing.T) {
	storage := NewMemStorage("")

	_, err := storage.GetGauge(context.Background(), "NonExisting")
	if err == nil {
		t.Fatal("Expected error for non-existing gauge")
	}
}

func TestMemStorage_GetGauge_EmptyName(t *testing.T) {
	storage := NewMemStorage("")

	_, err := storage.GetGauge(context.Background(), "")
	if err == nil {
		t.Fatal("Expected error for empty name")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Expected error about empty name, got %v", err)
	}
}

func TestMemStorage_GetCounter_NotFound(t *testing.T) {
	storage := NewMemStorage("")

	_, err := storage.GetCounter(context.Background(), "NonExisting")
	if err == nil {
		t.Fatal("Expected error for non-existing counter")
	}
}

func TestMemStorage_GetCounter_EmptyName(t *testing.T) {
	storage := NewMemStorage("")

	_, err := storage.GetCounter(context.Background(), "")
	if err == nil {
		t.Fatal("Expected error for empty name")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Expected error about empty name, got %v", err)
	}
}

func TestMemStorage_GetAll(t *testing.T) {
	storage := NewMemStorage("")

	_ = storage.SetGauge(context.Background(), "Gauge1", 1.0)
	_ = storage.SetGauge(context.Background(), "Gauge2", 2.0)

	_ = storage.AddCounter(context.Background(), "Counter1", 10)
	_ = storage.AddCounter(context.Background(), "Counter2", 20)

	metrics := storage.GetAll()

	if len(metrics) != 4 {
		t.Errorf("Expected 4 metrics, got %d", len(metrics))
	}

	foundMetrics := make(map[string]bool)
	for _, m := range metrics {
		foundMetrics[m.ID] = true
	}

	expectedMetrics := []string{"Gauge1", "Gauge2", "Counter1", "Counter2"}
	for _, expected := range expectedMetrics {
		if !foundMetrics[expected] {
			t.Errorf("Expected metric %s to be present", expected)
		}
	}
}

func TestMemStorage_GetAll_Empty(t *testing.T) {
	storage := NewMemStorage("")

	metrics := storage.GetAll()
	if len(metrics) != 0 {
		t.Errorf("Expected 0 metrics for empty storage, got %d", len(metrics))
	}
}

func TestMemStorage_Concurrency(t *testing.T) {
	storage := NewMemStorage("")

	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			ctx := context.Background()
			for j := 0; j < 100; j++ {
				_ = storage.SetGauge(ctx, "Gauge"+string(rune(id)), float64(j))
				_ = storage.AddCounter(ctx, "Counter"+string(rune(id)), int64(j))
				storage.GetAll()
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestMemStorage_SaveToFile(t *testing.T) {
	storage := NewMemStorage("test_metrics.json")
	defer func() { _ = os.Remove("test_metrics.json") }()

	_ = storage.SetGauge(context.Background(), "TestGauge", 123.456)
	_ = storage.AddCounter(context.Background(), "TestCounter", 42)

	err := storage.SaveToFile()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	data, err := os.ReadFile("test_metrics.json")
	if err != nil {
		t.Fatalf("Expected no error reading file, got %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Expected non-empty file")
	}

	if !strings.Contains(string(data), "TestGauge") {
		t.Error("Expected file to contain TestGauge")
	}
	if !strings.Contains(string(data), "TestCounter") {
		t.Error("Expected file to contain TestCounter")
	}
}

func TestMemStorage_LoadFromFile(t *testing.T) {
	tmpFile := "test_metrics_load.json"
	defer func() { _ = os.Remove(tmpFile) }()

	testData := `[
		{"id": "LoadedGauge", "type": "gauge", "value": 999.999},
		{"id": "LoadedCounter", "type": "counter", "delta": 123}
	]`

	err := os.WriteFile(tmpFile, []byte(testData), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	storage := NewMemStorage(tmpFile)

	gaugeVal, err := storage.GetGauge(context.Background(), "LoadedGauge")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if gaugeVal != 999.999 {
		t.Errorf("Expected gauge value 999.999, got %f", gaugeVal)
	}

	counterVal, err := storage.GetCounter(context.Background(), "LoadedCounter")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if counterVal != 123 {
		t.Errorf("Expected counter value 123, got %d", counterVal)
	}
}

func TestMemStorage_LoadFromFile_NotExist(t *testing.T) {
	storage := NewMemStorage("non_existing_file.json")

	if storage == nil {
		t.Fatal("Expected storage to be non-nil")
	}
}

func TestMemStorage_SaveAndLoad_RoundTrip(t *testing.T) {
	storage := NewMemStorage("test_roundtrip.json")
	defer func() { _ = os.Remove("test_roundtrip.json") }()

	_ = storage.SetGauge(context.Background(), "Gauge1", 1.5)
	_ = storage.SetGauge(context.Background(), "Gauge2", 2.5)
	_ = storage.AddCounter(context.Background(), "Counter1", 100)
	_ = storage.AddCounter(context.Background(), "Counter2", 200)

	err := storage.SaveToFile()
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	newStorage := NewMemStorage("test_roundtrip.json")

	g1, _ := newStorage.GetGauge(context.Background(), "Gauge1")
	if g1 != 1.5 {
		t.Errorf("Expected Gauge1=1.5, got %f", g1)
	}

	g2, _ := newStorage.GetGauge(context.Background(), "Gauge2")
	if g2 != 2.5 {
		t.Errorf("Expected Gauge2=2.5, got %f", g2)
	}

	c1, _ := newStorage.GetCounter(context.Background(), "Counter1")
	if c1 != 100 {
		t.Errorf("Expected Counter1=100, got %d", c1)
	}

	c2, _ := newStorage.GetCounter(context.Background(), "Counter2")
	if c2 != 200 {
		t.Errorf("Expected Counter2=200, got %d", c2)
	}
}

func TestMemStorage_UpdateMetricsBatch(t *testing.T) {
	storage := NewMemStorage("")

	value := 1.5
	delta := int64(10)
	metrics := []models.Metrics{
		{ID: "Gauge", MType: models.Gauge, Value: &value},
		{ID: "Counter", MType: models.Counter, Delta: &delta},
	}

	err := storage.UpdateMetricsBatch(context.Background(), metrics)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	g, err := storage.GetGauge(context.Background(), "Gauge")
	if err != nil {
		t.Fatalf("GetGauge: %v", err)
	}
	if g != 1.5 {
		t.Errorf("Expected gauge 1.5, got %f", g)
	}

	c, err := storage.GetCounter(context.Background(), "Counter")
	if err != nil {
		t.Fatalf("GetCounter: %v", err)
	}
	if c != 10 {
		t.Errorf("Expected counter 10, got %d", c)
	}
}

func TestMemStorage_UpdateMetricsBatch_Empty(t *testing.T) {
	storage := NewMemStorage("")

	err := storage.UpdateMetricsBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Expected no error for empty batch, got %v", err)
	}
}

func TestMemStorage_UpdateMetricsBatch_NilFields(t *testing.T) {
	storage := NewMemStorage("")

	metrics := []models.Metrics{
		{ID: "GaugeNoValue", MType: models.Gauge},
		{ID: "CounterNoDelta", MType: models.Counter},
		{ID: "Unknown", MType: "histogram"},
	}

	err := storage.UpdateMetricsBatch(context.Background(), metrics)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(storage.GetAll()) != 0 {
		t.Errorf("Expected no metrics stored for nil fields, got %d", len(storage.GetAll()))
	}
}

func TestMemStorage_SaveToFile_NoFilePath(t *testing.T) {
	storage := NewMemStorage("")
	err := storage.SaveToFile()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestMemStorage_SetGauge_NoFilePath(t *testing.T) {
	storage := NewMemStorage("")
	err := storage.SetGauge(context.Background(), "Gauge", 1.0)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestMemStorage_AddCounter_NoFilePath(t *testing.T) {
	storage := NewMemStorage("")
	err := storage.AddCounter(context.Background(), "Counter", 1)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestMemStorage_LoadFromFile_Corrupt(t *testing.T) {
	tmpFile := "test_corrupt.json"
	defer func() { _ = os.Remove(tmpFile) }()

	err := os.WriteFile(tmpFile, []byte("not-json{{{"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	storage := NewMemStorage(tmpFile)
	if storage == nil {
		t.Fatal("Expected storage to be non-nil even on corrupt file")
	}
}
