package handler

import (
	"net/http"
	"strings"

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

func (h *MetricsHandler) UpdateMetricHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	result, err := h.service.ParseUpdatePath(r.URL.Path)
	if err != nil {
		if strings.Contains(err.Error(), "metric name cannot be empty") ||
			strings.Contains(err.Error(), "invalid path format") {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateMetric(result); err != nil {
		http.Error(w, "Bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}
