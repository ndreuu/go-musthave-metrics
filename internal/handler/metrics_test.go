package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go-musthave-metrics/internal/repository"
	"go-musthave-metrics/internal/service"
)

func setupTestRouter(storage *repository.MemStorage) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	metricsService := service.NewMetricsService(storage)
	handler := NewMetricsHandler(metricsService, storage, "")

	r.POST("/update/:type/:name/:value", handler.UpdateMetricHandler)
	r.GET("/value/:type/:name", handler.GetMetricHandler)
	r.GET("/", handler.ListMetricsHandler)

	return r
}

func TestNewMetricsHandler(t *testing.T) {
	storage := repository.NewMemStorage("")
	metricsService := service.NewMetricsService(storage)
	handler := NewMetricsHandler(metricsService, storage, "")

	if handler == nil {
		t.Fatal("Expected handler to be non-nil")
	}
	if handler.service == nil {
		t.Fatal("Expected service to be set")
	}
}

func TestMetricsHandler_UpdateMetricHandler_Success_Gauge(t *testing.T) {
	storage := repository.NewMemStorage("")
	r := setupTestRouter(storage)

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/TestMetric/100.5", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

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
	storage := repository.NewMemStorage("")
	r := setupTestRouter(storage)

	req := httptest.NewRequest(http.MethodPost, "/update/counter/TestCounter/50", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

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

func TestMetricsHandler_UpdateMetricHandler_InvalidMetricType(t *testing.T) {
	storage := repository.NewMemStorage("")
	r := setupTestRouter(storage)

	req := httptest.NewRequest(http.MethodPost, "/update/histogram/TestMetric/100", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestMetricsHandler_UpdateMetricHandler_EmptyName(t *testing.T) {
	storage := repository.NewMemStorage("")
	r := setupTestRouter(storage)

	req := httptest.NewRequest(http.MethodPost, "/update/gauge//100", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestMetricsHandler_GetMetricHandler_Success_Gauge(t *testing.T) {
	storage := repository.NewMemStorage("")
	storage.SetGauge("TestGauge", 123.456)
	r := setupTestRouter(storage)

	req := httptest.NewRequest(http.MethodGet, "/value/gauge/TestGauge", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	if w.Body.String() != "123.456" {
		t.Errorf("Expected body '123.456', got '%s'", w.Body.String())
	}
}

func TestMetricsHandler_GetMetricHandler_Success_Counter(t *testing.T) {
	storage := repository.NewMemStorage("")
	storage.AddCounter("TestCounter", 42)
	r := setupTestRouter(storage)

	req := httptest.NewRequest(http.MethodGet, "/value/counter/TestCounter", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	if w.Body.String() != "42" {
		t.Errorf("Expected body '42', got '%s'", w.Body.String())
	}
}

func TestMetricsHandler_GetMetricHandler_NotFound(t *testing.T) {
	storage := repository.NewMemStorage("")
	r := setupTestRouter(storage)

	req := httptest.NewRequest(http.MethodGet, "/value/gauge/NonExisting", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestMetricsHandler_GetMetricHandler_InvalidType(t *testing.T) {
	storage := repository.NewMemStorage("")
	r := setupTestRouter(storage)

	req := httptest.NewRequest(http.MethodGet, "/value/histogram/TestMetric", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestMetricsHandler_ListMetricsHandler_Empty(t *testing.T) {
	storage := repository.NewMemStorage("")
	r := setupTestRouter(storage)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	if !strings.Contains(w.Header().Get("Content-Type"), "text/html") {
		t.Errorf("Expected Content-Type text/html, got %s", w.Header().Get("Content-Type"))
	}
}

func TestMetricsHandler_ListMetricsHandler_WithMetrics(t *testing.T) {
	storage := repository.NewMemStorage("")
	storage.SetGauge("TestGauge", 100.5)
	storage.AddCounter("TestCounter", 50)
	r := setupTestRouter(storage)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "TestGauge") {
		t.Error("Expected body to contain 'TestGauge'")
	}
	if !strings.Contains(body, "TestCounter") {
		t.Error("Expected body to contain 'TestCounter'")
	}
}

func TestMetricsHandler_UpdateMetricHandler_Counter_Accumulate(t *testing.T) {
	storage := repository.NewMemStorage("")
	r := setupTestRouter(storage)

	req := httptest.NewRequest(http.MethodPost, "/update/counter/TestCounter/10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	req = httptest.NewRequest(http.MethodPost, "/update/counter/TestCounter/20", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	value, err := storage.GetCounter("TestCounter")
	if err != nil {
		t.Errorf("Expected no error getting counter, got %v", err)
	}
	if value != 30 {
		t.Errorf("Expected counter value 30, got %d", value)
	}
}

func TestMetricsHandler_UpdateMetricHandler_Gauge_Overwrite(t *testing.T) {
	storage := repository.NewMemStorage("")
	r := setupTestRouter(storage)

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/TestGauge/100", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	req = httptest.NewRequest(http.MethodPost, "/update/gauge/TestGauge/200", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	value, err := storage.GetGauge("TestGauge")
	if err != nil {
		t.Errorf("Expected no error getting gauge, got %v", err)
	}
	if value != 200 {
		t.Errorf("Expected gauge value 200, got %f", value)
	}
}
