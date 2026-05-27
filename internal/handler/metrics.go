package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	models "go-musthave-metrics/internal/model"
	"go-musthave-metrics/internal/service"
)

type MetricsHandler struct {
	service *service.MetricsService
}

func NewMetricsHandler(service *service.MetricsService) *MetricsHandler {
	return &MetricsHandler{
		service: service,
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

	html := "<html><body><table>"
	for _, m := range metrics {
		html += "<tr><td>" + m.ID + "</td><td>" + m.MType + "</td></tr>"
	}
	html += "</table></body></html>"

	c.String(http.StatusOK, html)
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
