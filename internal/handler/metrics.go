package handler

import (
	"go-musthave-metrics/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
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
	c.JSON(http.StatusOK, gin.H{"metrics": metrics})
}
