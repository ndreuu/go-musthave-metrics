package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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

	var sb strings.Builder
	sb.WriteString("<!DOCTYPE html>\n")
	sb.WriteString("<html>\n<head>\n")
	sb.WriteString("<title>Metrics</title>\n")
	sb.WriteString("<style>\n")
	sb.WriteString("body { font-family: Arial, sans-serif; margin: 20px; }\n")
	sb.WriteString("table { border-collapse: collapse; width: 100%; }\n")
	sb.WriteString("th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }\n")
	sb.WriteString("th { background-color: #4CAF50; color: white; }\n")
	sb.WriteString("tr:nth-child(even) { background-color: #f2f2f2; }\n")
	sb.WriteString("</style>\n")
	sb.WriteString("</head>\n<body>\n")
	sb.WriteString("<h1>Metrics</h1>\n")

	if len(metrics) == 0 {
		sb.WriteString("<p>No metrics available</p>\n")
	} else {
		sb.WriteString("<table>\n")
		sb.WriteString("<tr><th>Type</th><th>Name</th><th>Value</th></tr>\n")

		for _, m := range metrics {
			sb.WriteString("<tr>")
			sb.WriteString(fmt.Sprintf("<td>%s</td>", m.MType))
			sb.WriteString(fmt.Sprintf("<td>%s</td>", m.ID))
			if m.MType == "gauge" && m.Value != nil {
				sb.WriteString(fmt.Sprintf("<td>%.2f</td>", *m.Value))
			} else if m.MType == "counter" && m.Value != nil {
				sb.WriteString(fmt.Sprintf("<td>%d</td>", int64(*m.Value)))
			} else {
				sb.WriteString("<td>-</td>")
			}
			sb.WriteString("</tr>\n")
		}

		sb.WriteString("</table>\n")
	}

	sb.WriteString("</body>\n</html>")

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(sb.String()))
}
