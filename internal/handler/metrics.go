package handler

import (
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"
	models "go-musthave-metrics/internal/model"
	"go-musthave-metrics/internal/repository"
	"go-musthave-metrics/internal/service"
)

type MetricsHandler struct {
	service  *service.MetricsService
	storage  repository.Storage
	filePath string
}

func NewMetricsHandler(service *service.MetricsService, storage repository.Storage, filePath string) *MetricsHandler {
	return &MetricsHandler{
		service:  service,
		storage:  storage,
		filePath: filePath,
	}
}

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

	if err := h.service.UpdateMetric(result); err != nil {
		c.String(http.StatusBadRequest, "Bad request: "+err.Error())
		return
	}

	if memStorage, ok := h.storage.(*repository.MemStorage); ok && h.filePath != "" {
		if err := memStorage.SaveToFile(); err != nil {
			c.String(http.StatusInternalServerError, "Failed to save metrics: "+err.Error())
			return
		}
	}

	c.String(http.StatusOK, "OK")
}

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

	value, err := h.service.GetMetricValue(mType, name)
	if err != nil {
		c.String(http.StatusNotFound, "Not found")
		return
	}

	c.String(http.StatusOK, value)
}

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
}

func (h *MetricsHandler) UpdateMetricJSONHandler(c *gin.Context) {
	var m models.Metrics
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	if err := h.service.UpdateMetricFromJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if memStorage, ok := h.storage.(*repository.MemStorage); ok && h.filePath != "" {
		if err := memStorage.SaveToFile(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save metrics: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "OK"})
}

func (h *MetricsHandler) GetMetricJSONHandler(c *gin.Context) {
	var m models.Metrics
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	result, err := h.service.GetMetricFromJSON(&m)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

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

	if err := h.service.UpdateMetricsBatch(metrics); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if memStorage, ok := h.storage.(*repository.MemStorage); ok && h.filePath != "" {
		if err := memStorage.SaveToFile(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save metrics: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "OK"})
}
