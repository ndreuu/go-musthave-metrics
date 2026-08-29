// Package handler предоставляет HTTP-обработчики для управления метриками.
package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	models "go-musthave-metrics/internal/model"
	"go-musthave-metrics/internal/repository"
	"go-musthave-metrics/internal/service"
	"go-musthave-metrics/internal/service/audit"
)

// MetricsHandler обрабатывает HTTP-запросы для операций с метриками.
// Предоставляет методы для обновления, получения и списка метрик через REST API.
type MetricsHandler struct {
	service      *service.MetricsService
	storage      repository.Storage
	filePath     string
	key          string
	auditService *audit.AuditService
}

// NewMetricsHandler создает новый экземпляр MetricsHandler.
// service - сервис для работы с метриками.
// storage - хранилище метрик (в памяти или PostgreSQL).
// filePath - путь к файлу для сохранения метрик (пустая строка отключает сохранение).
// key - секретный ключ для вычисления хеша (пустая строка отключает проверку).
// auditService - сервис аудита для логирования событий (может быть nil).
func NewMetricsHandler(service *service.MetricsService, storage repository.Storage, filePath string, key string, auditService *audit.AuditService) *MetricsHandler {
	return &MetricsHandler{
		service:      service,
		storage:      storage,
		filePath:     filePath,
		key:          key,
		auditService: auditService,
	}
}

func calculateHash(data []byte, key string) string {
	h := sha256.New()
	h.Write(data)
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}

func (h *MetricsHandler) setResponseHash(c *gin.Context, data []byte) {
	if h.key == "" {
		return
	}
	hash := calculateHash(data, h.key)
	c.Header("HashSHA256", hash)
}

func (h *MetricsHandler) sendAudit(c *gin.Context, metrics []string) {
	if h.auditService == nil {
		return
	}

	event := &models.AuditEvent{
		Timestamp: time.Now().Unix(),
		Metrics:   metrics,
		IPAddress: c.ClientIP(),
	}

	go h.auditService.NotifyObservers(event)
}

// UpdateMetricHandler обрабатывает POST-запросы для обновления метрики через URL.
// Формат URL: /update/{type}/{name}/{value}
// Пример: POST /update/gauge/MyGauge/3.14
// type - тип метрики ("gauge" или "counter").
// name - имя метрики.
// value - значение метрики.
// Возвращает HTTP 200 при успехе, 400 при ошибке валидации, 404 если метрика не найдена.
func (h *MetricsHandler) UpdateMetricHandler(c *gin.Context) {
	mType := c.Param("type")
	name := c.Param("name")
	value := c.Param("value")

	if name == "" {
		c.String(http.StatusNotFound, "Not found")
		return
	}

	if mType != "gauge" && mType != "counter" {
		c.String(http.StatusBadRequest, "Invalid metric type: must be 'gauge' or 'counter'")
		return
	}

	result := &service.UpdateMetricResult{
		MType: mType,
		Name:  name,
		Value: value,
	}

	if err := h.service.UpdateMetric(c.Request.Context(), result); err != nil {
		c.String(http.StatusBadRequest, "Bad request: "+err.Error())
		return
	}

	if memStorage, ok := h.storage.(*repository.MemStorage); ok && h.filePath != "" {
		if err := memStorage.SaveToFile(); err != nil {
			c.String(http.StatusInternalServerError, "Failed to save metrics: "+err.Error())
			return
		}
	}

	h.sendAudit(c, []string{name})

	c.String(http.StatusOK, "OK")
}

// GetMetricHandler обрабатывает GET-запросы для получения значения метрики.
// Формат URL: /update/{type}/{name}
// Пример: GET /update/gauge/MyGauge
// type - тип метрики ("gauge" или "counter").
// name - имя метрики.
// Возвращает HTTP 200 со значением метрики, 404 если метрика не найдена.
func (h *MetricsHandler) GetMetricHandler(c *gin.Context) {
	mType := c.Param("type")
	name := c.Param("name")

	if name == "" {
		c.String(http.StatusNotFound, "Not found")
		return
	}

	if mType != "gauge" && mType != "counter" {
		c.String(http.StatusBadRequest, "Invalid metric type: must be 'gauge' or 'counter'")
		return
	}

	value, err := h.service.GetMetricValue(c.Request.Context(), mType, name)
	if err != nil {
		c.String(http.StatusNotFound, "Not found")
		return
	}

	h.sendAudit(c, []string{name})

	c.String(http.StatusOK, value)
}

// ListMetricsHandler обрабатывает GET-запросы для получения списка всех метрик.
// Формат URL: /metrics/
// Пример: GET /metrics/
// Возвращает HTML-страницу с таблицей всех метрик (ID и тип).
// Content-Type: text/html; charset=utf-8
func (h *MetricsHandler) ListMetricsHandler(c *gin.Context) {
	metrics := h.service.GetAllMetrics()

	c.Header("Content-Type", "text/html; charset=utf-8")

	tmpl := `
<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><title>Metrics</title></head>
<body>
<table border="1">
<tr><th>ID</th><th>Type</th></tr>
{{range .}}<tr><td>{{.ID}}</td><td>{{.MType}}</td></tr>{{end}}
</table>
</body>
</html>
`

	t, err := template.New("metrics").Parse(tmpl)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to parse template")
		return
	}

	if err := t.Execute(c.Writer, metrics); err != nil {
		c.String(http.StatusInternalServerError, "Failed to render template")
		return
	}

	metricNames := make([]string, len(metrics))
	for i, m := range metrics {
		metricNames[i] = m.ID
	}
	h.sendAudit(c, metricNames)
}

// UpdateMetricJSONHandler обрабатывает POST-запросы для обновления метрики в формате JSON.
// Формат URL: /update/
// Тело запроса: JSON объект с полями id, type, value (для gauge) или delta (для counter).
// Пример: POST /update/ {"id":"MyGauge","type":"gauge","value":3.14}
// При включенном ключе добавляет заголовок HashSHA256 с хешем ответа.
// Возвращает HTTP 200 {"status":"OK"} при успехе, 400 при ошибке.
func (h *MetricsHandler) UpdateMetricJSONHandler(c *gin.Context) {
	var m models.Metrics
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	if err := h.service.UpdateMetricFromJSON(c.Request.Context(), &m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if memStorage, ok := h.storage.(*repository.MemStorage); ok && h.filePath != "" {
		if err := memStorage.SaveToFile(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save metrics: " + err.Error()})
			return
		}
	}

	h.sendAudit(c, []string{m.ID})

	resp := gin.H{"status": "OK"}
	respData, _ := json.Marshal(resp)
	h.setResponseHash(c, respData)
	c.JSON(http.StatusOK, resp)
}

// GetMetricJSONHandler обрабатывает POST-запросы для получения метрики в формате JSON.
// Формат URL: /value/
// Тело запроса: JSON объект с полями id и type.
// Пример: POST /value/ {"id":"MyGauge","type":"gauge"}
// Возвращает JSON объект с данными метрики (id, type, value или delta).
// При включенном ключе добавляет заголовок HashSHA256 с хешем ответа.
// Возвращает HTTP 200 при успехе, 404 если метрика не найдена.
func (h *MetricsHandler) GetMetricJSONHandler(c *gin.Context) {
	var m models.Metrics
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	result, err := h.service.GetMetricFromJSON(c.Request.Context(), &m)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	h.sendAudit(c, []string{m.ID})

	respData, _ := json.Marshal(result)
	h.setResponseHash(c, respData)
	c.JSON(http.StatusOK, result)
}

// UpdateMetricsBatchHandler обрабатывает POST-запросы для пакетного обновления метрик.
// Формат URL: /updates/
// Тело запроса: JSON массив объектов с полями id, type, value/delta.
// Пример: POST /updates/ [{"id":"MyGauge","type":"gauge","value":3.14},{"id":"MyCounter","type":"counter","delta":1}]
// При включенном ключе добавляет заголовок HashSHA256 с хешем ответа.
// Возвращает HTTP 200 {"status":"OK"} при успехе, 400 при ошибке.
func (h *MetricsHandler) UpdateMetricsBatchHandler(c *gin.Context) {
	var metrics []models.Metrics
	if err := c.ShouldBindJSON(&metrics); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	if len(metrics) == 0 {
		c.JSON(http.StatusOK, gin.H{"status": "OK"})
		return
	}

	if err := h.service.UpdateMetricsBatch(c.Request.Context(), metrics); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if memStorage, ok := h.storage.(*repository.MemStorage); ok && h.filePath != "" {
		if err := memStorage.SaveToFile(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save metrics: " + err.Error()})
			return
		}
	}

	metricNames := make([]string, len(metrics))
	for i, m := range metrics {
		metricNames[i] = m.ID
	}
	h.sendAudit(c, metricNames)

	resp := gin.H{"status": "OK"}
	respData, _ := json.Marshal(resp)
	h.setResponseHash(c, respData)
	c.JSON(http.StatusOK, resp)
}
