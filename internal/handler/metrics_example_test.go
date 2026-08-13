package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"

	"go-musthave-metrics/internal/handler"
	"go-musthave-metrics/internal/repository"
	"go-musthave-metrics/internal/service"
	"go-musthave-metrics/internal/service/audit"
)

// Пример использования UpdateMetricHandler для обновления gauge метрики через URL.
func ExampleMetricsHandler_UpdateMetricHandler_gauge() {
	storage := repository.NewMemStorage("")
	metricsService := service.NewMetricsService(storage)
	auditService := audit.NewAuditService()
	h := handler.NewMetricsHandler(metricsService, storage, "", "", auditService)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/update/gauge/MyGauge/3.14", nil)
	c.Params = []gin.Param{
		{Key: "type", Value: "gauge"},
		{Key: "name", Value: "MyGauge"},
		{Key: "value", Value: "3.14"},
	}

	h.UpdateMetricHandler(c)

	fmt.Println(w.Code)
	// Output: 200
}

// Пример использования UpdateMetricHandler для обновления counter метрики через URL.
func ExampleMetricsHandler_UpdateMetricHandler_counter() {
	storage := repository.NewMemStorage("")
	metricsService := service.NewMetricsService(storage)
	auditService := audit.NewAuditService()
	h := handler.NewMetricsHandler(metricsService, storage, "", "", auditService)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/update/counter/MyCounter/5", nil)
	c.Params = []gin.Param{
		{Key: "type", Value: "counter"},
		{Key: "name", Value: "MyCounter"},
		{Key: "value", Value: "5"},
	}

	h.UpdateMetricHandler(c)

	fmt.Println(w.Code)
	// Output: 200
}

// Пример использования GetMetricHandler для получения значения метрики.
func ExampleMetricsHandler_GetMetricHandler() {
	storage := repository.NewMemStorage("")
	metricsService := service.NewMetricsService(storage)
	auditService := audit.NewAuditService()
	h := handler.NewMetricsHandler(metricsService, storage, "", "", auditService)

	// Сначала установим значение
	storage.SetGauge(nil, "TestGauge", 42.5)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/update/gauge/TestGauge", nil)
	c.Params = []gin.Param{
		{Key: "type", Value: "gauge"},
		{Key: "name", Value: "TestGauge"},
	}

	h.GetMetricHandler(c)

	fmt.Println(w.Code)
	fmt.Println(w.Body.String())
	// Output:
	// 200
	// 42.5
}

// Пример использования UpdateMetricJSONHandler для обновления метрики через JSON.
func ExampleMetricsHandler_UpdateMetricJSONHandler() {
	storage := repository.NewMemStorage("")
	metricsService := service.NewMetricsService(storage)
	auditService := audit.NewAuditService()
	h := handler.NewMetricsHandler(metricsService, storage, "", "", auditService)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()

	body := bytes.NewBufferString(`{"id":"MyGauge","type":"gauge","value":3.14}`)
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/update/", body)
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateMetricJSONHandler(c)

	fmt.Println(w.Code)
	// Output: 200
}

// Пример использования GetMetricJSONHandler для получения метрики через JSON.
func ExampleMetricsHandler_GetMetricJSONHandler() {
	storage := repository.NewMemStorage("")
	metricsService := service.NewMetricsService(storage)
	auditService := audit.NewAuditService()
	h := handler.NewMetricsHandler(metricsService, storage, "", "", auditService)

	// Сначала установим значение
	storage.SetGauge(nil, "TestGauge", 42.5)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()

	body := bytes.NewBufferString(`{"id":"TestGauge","type":"gauge"}`)
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/value/", body)
	c.Request.Header.Set("Content-Type", "application/json")

	h.GetMetricJSONHandler(c)

	fmt.Println(w.Code)

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	fmt.Printf("id: %s, type: %s, value: %v\n", result["id"], result["type"], result["value"])
	// Output:
	// 200
	// id: TestGauge, type: gauge, value: 42.5
}

// Пример использования UpdateMetricsBatchHandler для пакетного обновления метрик.
func ExampleMetricsHandler_UpdateMetricsBatchHandler() {
	storage := repository.NewMemStorage("")
	metricsService := service.NewMetricsService(storage)
	auditService := audit.NewAuditService()
	h := handler.NewMetricsHandler(metricsService, storage, "", "", auditService)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()

	body := bytes.NewBufferString(`[
		{"id":"Gauge1","type":"gauge","value":1.5},
		{"id":"Counter1","type":"counter","delta":10}
	]`)
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/updates/", body)
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateMetricsBatchHandler(c)

	fmt.Println(w.Code)
	// Output: 200
}
