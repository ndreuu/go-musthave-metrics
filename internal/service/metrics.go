package service

import (
	"fmt"
	"strconv"
	"strings"

	models "go-musthave-metrics/internal/model"
)

type MetricsStorage interface {
	SetGauge(name string, value float64) error
	AddCounter(name string, value int64) error
	GetGauge(name string) (float64, error)
	GetCounter(name string) (int64, error)
	GetAll() []models.Metrics
}

type MetricsService struct {
	storage MetricsStorage
}

func NewMetricsService(storage MetricsStorage) *MetricsService {
	return &MetricsService{
		storage: storage,
	}
}

type UpdateMetricResult struct {
	MType string
	Name  string
	Value string
}

func (s *MetricsService) ParseUpdatePath(path string) (*UpdateMetricResult, error) {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid path format")
	}

	if parts[0] != "update" {
		return nil, fmt.Errorf("invalid path: must start with 'update'")
	}

	metricType := parts[1]
	if metricType != "gauge" && metricType != "counter" {
		return nil, fmt.Errorf("invalid metric type: must be 'gauge' or 'counter'")
	}

	metricName := parts[2]
	if metricName == "" {
		return nil, fmt.Errorf("metric name cannot be empty")
	}

	metricValue := parts[3]
	if metricValue == "" {
		return nil, fmt.Errorf("metric value cannot be empty")
	}

	if metricType == "gauge" {
		if _, err := strconv.ParseFloat(metricValue, 64); err != nil {
			return nil, fmt.Errorf("invalid gauge value: must be a float64")
		}
	} else if metricType == "counter" {
		if _, err := strconv.ParseInt(metricValue, 10, 64); err != nil {
			return nil, fmt.Errorf("invalid counter value: must be an int64")
		}
	}

	return &UpdateMetricResult{
		MType: metricType,
		Name:  metricName,
		Value: metricValue,
	}, nil
}

func (s *MetricsService) UpdateMetric(result *UpdateMetricResult) error {
	if result.MType == "gauge" {
		value, err := strconv.ParseFloat(result.Value, 64)
		if err != nil {
			return err
		}
		return s.storage.SetGauge(result.Name, value)
	} else if result.MType == "counter" {
		value, err := strconv.ParseInt(result.Value, 10, 64)
		if err != nil {
			return err
		}
		return s.storage.AddCounter(result.Name, value)
	}
	return fmt.Errorf("unknown metric type: %s", result.MType)
}

func (s *MetricsService) GetMetricValue(mType, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("metric name cannot be empty")
	}

	switch mType {
	case "gauge":
		value, err := s.storage.GetGauge(name)
		if err != nil {
			return "", err
		}
		return strconv.FormatFloat(value, 'f', -1, 64), nil
	case "counter":
		value, err := s.storage.GetCounter(name)
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(value, 10), nil
	default:
		return "", fmt.Errorf("invalid metric type: must be 'gauge' or 'counter'")
	}
}

func (s *MetricsService) GetAllMetrics() []models.Metrics {
	return s.storage.GetAll()
}
