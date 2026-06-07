package repository

import (
	"os"
	"strings"
	"testing"
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

	err := storage.SetGauge("TestGauge", 123.456)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	value, err := storage.GetGauge("TestGauge")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if value != 123.456 {
		t.Errorf("Expected value 123.456, got %f", value)
	}
}

func TestMemStorage_SetGauge_EmptyName(t *testing.T) {
	storage := NewMemStorage("")

	err := storage.SetGauge("", 123.456)
	if err == nil {
		t.Fatal("Expected error for empty name")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Expected error about empty name, got %v", err)
	}
}

func TestMemStorage_SetGauge_Overwrite(t *testing.T) {
	storage := NewMemStorage("")

	err := storage.SetGauge("TestGauge", 100.0)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = storage.SetGauge("TestGauge", 200.0)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	value, err := storage.GetGauge("TestGauge")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if value != 200.0 {
		t.Errorf("Expected value 200.0, got %f", value)
	}
}

func TestMemStorage_AddCounter(t *testing.T) {
	storage := NewMemStorage("")

	err := storage.AddCounter("TestCounter", 10)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	value, err := storage.GetCounter("TestCounter")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if value != 10 {
		t.Errorf("Expected value 10, got %d", value)
	}
}

func TestMemStorage_AddCounter_EmptyName(t *testing.T) {
	storage := NewMemStorage("")

	err := storage.AddCounter("", 10)
	if err == nil {
		t.Fatal("Expected error for empty name")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Expected error about empty name, got %v", err)
	}
}

func TestMemStorage_AddCounter_Accumulate(t *testing.T) {
	storage := NewMemStorage("")

	err := storage.AddCounter("TestCounter", 10)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = storage.AddCounter("TestCounter", 20)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = storage.AddCounter("TestCounter", 30)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	value, err := storage.GetCounter("TestCounter")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if value != 60 {
		t.Errorf("Expected value 60, got %d", value)
	}
}

func TestMemStorage_GetGauge_NotFound(t *testing.T) {
	storage := NewMemStorage("")

	_, err := storage.GetGauge("NonExisting")
	if err == nil {
		t.Fatal("Expected error for non-existing gauge")
	}
}

func TestMemStorage_GetGauge_EmptyName(t *testing.T) {
	storage := NewMemStorage("")

	_, err := storage.GetGauge("")
	if err == nil {
		t.Fatal("Expected error for empty name")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Expected error about empty name, got %v", err)
	}
}

func TestMemStorage_GetCounter_NotFound(t *testing.T) {
	storage := NewMemStorage("")

	_, err := storage.GetCounter("NonExisting")
	if err == nil {
		t.Fatal("Expected error for non-existing counter")
	}
}

func TestMemStorage_GetCounter_EmptyName(t *testing.T) {
	storage := NewMemStorage("")

	_, err := storage.GetCounter("")
	if err == nil {
		t.Fatal("Expected error for empty name")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Expected error about empty name, got %v", err)
	}
}

func TestMemStorage_GetAll(t *testing.T) {
	storage := NewMemStorage("")

	storage.SetGauge("Gauge1", 1.0)
	storage.SetGauge("Gauge2", 2.0)

	storage.AddCounter("Counter1", 10)
	storage.AddCounter("Counter2", 20)

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
			for j := 0; j < 100; j++ {
				storage.SetGauge("Gauge"+string(rune(id)), float64(j))
				storage.AddCounter("Counter"+string(rune(id)), int64(j))
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
	defer os.Remove("test_metrics.json")

	storage.SetGauge("TestGauge", 123.456)
	storage.AddCounter("TestCounter", 42)

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
	defer os.Remove(tmpFile)

	testData := `[
		{"id": "LoadedGauge", "type": "gauge", "value": 999.999},
		{"id": "LoadedCounter", "type": "counter", "delta": 123}
	]`

	err := os.WriteFile(tmpFile, []byte(testData), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	storage := NewMemStorage(tmpFile)

	gaugeVal, err := storage.GetGauge("LoadedGauge")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if gaugeVal != 999.999 {
		t.Errorf("Expected gauge value 999.999, got %f", gaugeVal)
	}

	counterVal, err := storage.GetCounter("LoadedCounter")
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
	defer os.Remove("test_roundtrip.json")

	storage.SetGauge("Gauge1", 1.5)
	storage.SetGauge("Gauge2", 2.5)
	storage.AddCounter("Counter1", 100)
	storage.AddCounter("Counter2", 200)

	err := storage.SaveToFile()
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	newStorage := NewMemStorage("test_roundtrip.json")

	g1, _ := newStorage.GetGauge("Gauge1")
	if g1 != 1.5 {
		t.Errorf("Expected Gauge1=1.5, got %f", g1)
	}

	g2, _ := newStorage.GetGauge("Gauge2")
	if g2 != 2.5 {
		t.Errorf("Expected Gauge2=2.5, got %f", g2)
	}

	c1, _ := newStorage.GetCounter("Counter1")
	if c1 != 100 {
		t.Errorf("Expected Counter1=100, got %d", c1)
	}

	c2, _ := newStorage.GetCounter("Counter2")
	if c2 != 200 {
		t.Errorf("Expected Counter2=200, got %d", c2)
	}
}
