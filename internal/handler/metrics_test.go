package handler

import (
	"go-musthave-metrics/internal/repository"
	"go-musthave-metrics/internal/service"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewMetricsHandler(t *testing.T) {
	storage := repository.NewMemStorage()
	metricsService := service.NewMetricsService(storage)
	handler := NewMetricsHandler(metricsService)

	if handler == nil {
		t.Fatal("Expected handler to be non-nil")
	}
	if handler.service == nil {
		t.Fatal("Expected service to be set")
	}
}

func TestMetricsHandler_UpdateMetricHandler_MethodNotAllowed(t *testing.T) {
	storage := repository.NewMemStorage()
	metricsService := service.NewMetricsService(storage)
	handler := NewMetricsHandler(metricsService)

	req := httptest.NewRequest(http.MethodGet, "/update/gauge/TestMetric/100", nil)
	w := httptest.NewRecorder()

	handler.UpdateMetricHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestMetricsHandler_UpdateMetricHandler_Success_Gauge(t *testing.T) {
	storage := repository.NewMemStorage()
	metricsService := service.NewMetricsService(storage)
	handler := NewMetricsHandler(metricsService)

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/TestMetric/100.5", nil)
	w := httptest.NewRecorder()

	handler.UpdateMetricHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	value, err := storage.GetGauge("TestMetric")
	if err != nil {
		t.Errorf("Expected no error getting gauge, got %v", err)
	}
	if value != 100.5 {
		t.Errorf("Expected gauge value 100.5, got %f", value)
	}
}

func TestMetricsHandler_UpdateMetricHandler_Success_Counter(t *testing.T) {
	storage := repository.NewMemStorage()
	metricsService := service.NewMetricsService(storage)
	handler := NewMetricsHandler(metricsService)

	req := httptest.NewRequest(http.MethodPost, "/update/counter/TestCounter/50", nil)
	w := httptest.NewRecorder()

	handler.UpdateMetricHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	value, err := storage.GetCounter("TestCounter")
	if err != nil {
		t.Errorf("Expected no error getting counter, got %v", err)
	}
	if value != 50 {
		t.Errorf("Expected counter value 50, got %d", value)
	}
}

func TestMetricsHandler_UpdateMetricHandler_InvalidPath_TooShort(t *testing.T) {
	storage := repository.NewMemStorage()
	metricsService := service.NewMetricsService(storage)
	handler := NewMetricsHandler(metricsService)

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/Metric", nil)
	w := httptest.NewRecorder()

	handler.UpdateMetricHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestMetricsHandler_UpdateMetricHandler_EmptyMetricName(t *testing.T) {
	storage := repository.NewMemStorage()
	metricsService := service.NewMetricsService(storage)
	handler := NewMetricsHandler(metricsService)

	req := httptest.NewRequest(http.MethodPost, "/update/gauge//100", nil)
	w := httptest.NewRecorder()

	handler.UpdateMetricHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Not found") {
		t.Errorf("Expected 'Not found' in response, got: %s", body)
	}
}

func TestMetricsHandler_UpdateMetricHandler_InvalidMetricType(t *testing.T) {
	storage := repository.NewMemStorage()
	metricsService := service.NewMetricsService(storage)
	handler := NewMetricsHandler(metricsService)

	req := httptest.NewRequest(http.MethodPost, "/update/histogram/TestMetric/100", nil)
	w := httptest.NewRecorder()

	handler.UpdateMetricHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestMetricsHandler_UpdateMetricHandler_InvalidGaugeValue(t *testing.T) {
	storage := repository.NewMemStorage()
	metricsService := service.NewMetricsService(storage)
	handler := NewMetricsHandler(metricsService)

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/TestMetric/not-a-number", nil)
	w := httptest.NewRecorder()

	handler.UpdateMetricHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestMetricsHandler_UpdateMetricHandler_InvalidCounterValue(t *testing.T) {
	storage := repository.NewMemStorage()
	metricsService := service.NewMetricsService(storage)
	handler := NewMetricsHandler(metricsService)

	req := httptest.NewRequest(http.MethodPost, "/update/counter/TestMetric/1.5", nil)
	w := httptest.NewRecorder()

	handler.UpdateMetricHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestMetricsHandler_UpdateMetricHandler_Counter_Accumulate(t *testing.T) {
	storage := repository.NewMemStorage()
	metricsService := service.NewMetricsService(storage)
	handler := NewMetricsHandler(metricsService)

	req := httptest.NewRequest(http.MethodPost, "/update/counter/TestCounter/10", nil)
	w := httptest.NewRecorder()
	handler.UpdateMetricHandler(w, req)

	req = httptest.NewRequest(http.MethodPost, "/update/counter/TestCounter/20", nil)
	w = httptest.NewRecorder()
	handler.UpdateMetricHandler(w, req)

	value, err := storage.GetCounter("TestCounter")
	if err != nil {
		t.Errorf("Expected no error getting counter, got %v", err)
	}
	if value != 30 {
		t.Errorf("Expected counter value 30, got %d", value)
	}
}

func TestMetricsHandler_UpdateMetricHandler_Gauge_Overwrite(t *testing.T) {
	storage := repository.NewMemStorage()
	metricsService := service.NewMetricsService(storage)
	handler := NewMetricsHandler(metricsService)

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/TestGauge/100", nil)
	w := httptest.NewRecorder()
	handler.UpdateMetricHandler(w, req)

	req = httptest.NewRequest(http.MethodPost, "/update/gauge/TestGauge/200", nil)
	w = httptest.NewRecorder()
	handler.UpdateMetricHandler(w, req)

	value, err := storage.GetGauge("TestGauge")
	if err != nil {
		t.Errorf("Expected no error getting gauge, got %v", err)
	}
	if value != 200 {
		t.Errorf("Expected gauge value 200, got %f", value)
	}
}

func TestMetricsHandler_UpdateMetricHandler_PathWithoutLeadingSlash(t *testing.T) {
	storage := repository.NewMemStorage()
	metricsService := service.NewMetricsService(storage)
	handler := NewMetricsHandler(metricsService)

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/TestMetric/123.456", nil)
	w := httptest.NewRecorder()

	handler.UpdateMetricHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}
