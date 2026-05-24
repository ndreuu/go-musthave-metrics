package agent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewSender(t *testing.T) {
	serverAddress := "http://localhost:8080"
	sender := NewSender(serverAddress)

	if sender == nil {
		t.Fatal("Expected sender to be non-nil")
	}
	if sender.serverAddress != serverAddress {
		t.Errorf("Expected serverAddress to be %s, got %s", serverAddress, sender.serverAddress)
	}
	if sender.client == nil {
		t.Fatal("Expected HTTP client to be initialized")
	}
}

func TestSender_Send_Gauge(t *testing.T) {
	var receivedURL string
	var receivedMethod string
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()
		receivedMethod = r.Method
		receivedContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSender(server.URL)

	metric := &Metric{
		MType: "gauge",
		Name:  "TestMetric",
		Value: 123.456,
	}

	err := sender.Send(metric)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if receivedMethod != http.MethodPost {
		t.Errorf("Expected POST method, got %s", receivedMethod)
	}

	if receivedContentType != "text/plain" {
		t.Errorf("Expected Content-Type to be text/plain, got %s", receivedContentType)
	}

	if !strings.Contains(receivedURL, "/update/gauge/TestMetric/") {
		t.Errorf("Expected URL to contain /update/gauge/TestMetric/, got %s", receivedURL)
	}
}

func TestSender_Send_Counter(t *testing.T) {
	var receivedURL string
	var receivedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()
		receivedMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSender(server.URL)

	metric := &Metric{
		MType: "counter",
		Name:  "TestCounter",
		Value: 42,
	}

	err := sender.Send(metric)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if receivedMethod != http.MethodPost {
		t.Errorf("Expected POST method, got %s", receivedMethod)
	}

	if !strings.Contains(receivedURL, "/update/counter/TestCounter/") {
		t.Errorf("Expected URL to contain /update/counter/TestCounter/, got %s", receivedURL)
	}
}

func TestSender_Send_UnknownType(t *testing.T) {
	sender := NewSender("http://localhost:8080")

	metric := &Metric{
		MType: "unknown",
		Name:  "TestMetric",
		Value: 123,
	}

	err := sender.Send(metric)
	if err == nil {
		t.Fatal("Expected error for unknown metric type")
	}
	if !strings.Contains(err.Error(), "unknown metric type") {
		t.Errorf("Expected error about unknown metric type, got %v", err)
	}
}

func TestSender_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	sender := NewSender(server.URL)

	metric := &Metric{
		MType: "gauge",
		Name:  "TestMetric",
		Value: 123,
	}

	err := sender.Send(metric)
	if err == nil {
		t.Fatal("Expected error for server error")
	}
	if !strings.Contains(err.Error(), "server returned status 500") {
		t.Errorf("Expected error about server status, got %v", err)
	}
}

func TestSender_SendAll(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSender(server.URL)

	metrics := []*Metric{
		{MType: "gauge", Name: "Metric1", Value: 1.0},
		{MType: "gauge", Name: "Metric2", Value: 2.0},
		{MType: "counter", Name: "Counter1", Value: 10},
	}

	errors := sender.SendAll(metrics)
	if len(errors) != 0 {
		t.Fatalf("Expected no errors, got %v", errors)
	}

	if requestCount != 3 {
		t.Errorf("Expected 3 requests, got %d", requestCount)
	}
}

func TestSender_SendAll_WithErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	sender := NewSender(server.URL)

	metrics := []*Metric{
		{MType: "gauge", Name: "Metric1", Value: 1.0},
		{MType: "gauge", Name: "Metric2", Value: 2.0},
	}

	errors := sender.SendAll(metrics)
	if len(errors) != 2 {
		t.Fatalf("Expected 2 errors, got %d", len(errors))
	}
}

func TestSender_SendMetricByName(t *testing.T) {
	var receivedURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSender(server.URL)
	collector := NewCollector()
	collector.Collect()

	err := sender.SendMetricByName(collector, "Alloc")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !strings.Contains(receivedURL, "/update/gauge/Alloc/") {
		t.Errorf("Expected URL to contain /update/gauge/Alloc/, got %s", receivedURL)
	}
}

func TestSender_SendMetricByName_NotFound(t *testing.T) {
	sender := NewSender("http://localhost:8080")
	collector := NewCollector()

	err := sender.SendMetricByName(collector, "NonExistingMetric")
	if err == nil {
		t.Fatal("Expected error for non-existing metric")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got %v", err)
	}
}

func TestFormatMetricValue(t *testing.T) {
	tests := []struct {
		name     string
		mType    string
		value    float64
		expected string
	}{
		{"counter integer", "counter", 42, "42"},
		{"counter large", "counter", 1000000, "1000000"},
		{"gauge integer", "gauge", 100, "100"},
		{"gauge float", "gauge", 123.456, "123.456"},
		{"gauge small", "gauge", 0.001, "0.001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMetricValue(tt.mType, tt.value)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}
