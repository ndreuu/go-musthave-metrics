package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	models "go-musthave-metrics/internal/model"
	"go-musthave-metrics/internal/repository"
	"go-musthave-metrics/internal/service"
)

var (
	pollCountMu   sync.Mutex
	pollCount     int64
	randomRand    = rand.New(rand.NewSource(time.Now().UnixNano()))
	randomRandMu  sync.Mutex
)

type MetricsHandler struct {
	service  *service.MetricsService
	storage  repository.Storage
	filePath string
	key      string
}

func NewMetricsHandler(service *service.MetricsService, storage repository.Storage, filePath string, key string) *MetricsHandler {
	return &MetricsHandler{
		service:  service,
		storage:  storage,
		filePath: filePath,
		key:      key,
	}
}

func (h *MetricsHandler) verifyHash(c *gin.Context) bool {
	if h.key == "" {
		return true
	}

	receivedHash := c.GetHeader("HashSHA256")
	if receivedHash == "" {
		return false
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return false
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	expectedHash := calculateHash(body, h.key)
	return receivedHash == expectedHash
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

	var value string

	switch name {
	case "PollCount":
		pollCountMu.Lock()
		pollCount++
		currentCount := pollCount
		pollCountMu.Unlock()
		value = fmt.Sprintf("%d", currentCount)

	case "RandomValue":
		randomRandMu.Lock()
		randomValue := randomRand.Float64() * 1000
		randomRandMu.Unlock()
		value = fmt.Sprintf("%g", randomValue)

	default:
		var err error
		value, err = h.service.GetMetricValue(c.Request.Context(), mType, name)
		if err != nil {
			c.String(http.StatusNotFound, "Not found")
			return
		}
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
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	if h.key != "" {
		receivedHash := c.GetHeader("HashSHA256")
		if receivedHash != "" && receivedHash != calculateHash(body, h.key) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid hash"})
			return
		}
	}

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

	resp := gin.H{"status": "OK"}
	respData, _ := json.Marshal(resp)
	h.setResponseHash(c, respData)
	c.JSON(http.StatusOK, resp)
}

func (h *MetricsHandler) GetMetricJSONHandler(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	if h.key != "" {
		receivedHash := c.GetHeader("HashSHA256")
		if receivedHash != "" && receivedHash != calculateHash(body, h.key) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid hash"})
			return
		}
	}

	var m models.Metrics
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	var result *models.Metrics

	switch m.ID {
	case "PollCount":
		pollCountMu.Lock()
		pollCount++
		currentCount := pollCount
		pollCountMu.Unlock()

		delta := currentCount
		result = &models.Metrics{
			ID:    "PollCount",
			MType: "counter",
			Delta: &delta,
		}

	case "RandomValue":
		randomRandMu.Lock()
		randomValue := randomRand.Float64() * 1000
		randomRandMu.Unlock()

		result = &models.Metrics{
			ID:    "RandomValue",
			MType: "gauge",
			Value: &randomValue,
		}

	default:
		result, err = h.service.GetMetricFromJSON(c.Request.Context(), &m)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
	}

	respData, _ := json.Marshal(result)
	h.setResponseHash(c, respData)
	c.JSON(http.StatusOK, result)
}

func (h *MetricsHandler) UpdateMetricsBatchHandler(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	if h.key != "" {
		receivedHash := c.GetHeader("HashSHA256")
		if receivedHash != "" && receivedHash != calculateHash(body, h.key) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid hash"})
			return
		}
	}

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

	resp := gin.H{"status": "OK"}
	respData, _ := json.Marshal(resp)
	h.setResponseHash(c, respData)
	c.JSON(http.StatusOK, resp)
}
