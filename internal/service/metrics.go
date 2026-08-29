// Package service предоставляет бизнес-логику для работы с метриками.
package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	models "go-musthave-metrics/internal/model"
)

// MetricsStorage определяет интерфейс хранилища метрик.
// Реализуется в памяти (MemStorage) или PostgreSQL (PostgresStorage).
type MetricsStorage interface {
	SetGauge(ctx context.Context, name string, value float64) error
	AddCounter(ctx context.Context, name string, value int64) error
	GetGauge(ctx context.Context, name string) (float64, error)
	GetCounter(ctx context.Context, name string) (int64, error)
	GetAll() []models.Metrics
	UpdateMetricsBatch(ctx context.Context, metrics []models.Metrics) error
}

// MetricsService предоставляет бизнес-логику для операций с метриками.
type MetricsService struct {
	storage MetricsStorage
}

// NewMetricsService создает новый экземпляр MetricsService.
// storage - хранилище метрик, реализующее интерфейс MetricsStorage.
func NewMetricsService(storage MetricsStorage) *MetricsService {
	return &MetricsService{
		storage: storage,
	}
}

// UpdateMetricResult представляет результат разбора пути для обновления метрики.
type UpdateMetricResult struct {
	MType string
	Name  string
	Value string
}

// ParseUpdatePath разбирает URL путь и извлекает параметры метрики.
// Ожидает формат: /update/{type}/{name}/{value}
// Возвращает UpdateMetricResult с типом, именем и значением метрики, или ошибку при неверном формате.
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

// UpdateMetric обновляет метрику в хранилище на основе разобранных параметров.
// Для gauge устанавливает значение, для counter добавляет дельту.
func (s *MetricsService) UpdateMetric(ctx context.Context, result *UpdateMetricResult) error {
	if result.MType == "gauge" {
		value, err := strconv.ParseFloat(result.Value, 64)
		if err != nil {
			return err
		}
		return s.storage.SetGauge(ctx, result.Name, value)
	} else if result.MType == "counter" {
		value, err := strconv.ParseInt(result.Value, 10, 64)
		if err != nil {
			return err
		}
		return s.storage.AddCounter(ctx, result.Name, value)
	}
	return fmt.Errorf("unknown metric type: %s", result.MType)
}

// GetMetricValue получает значение метрики из хранилища и возвращает его как строку.
// mType - тип метрики ("gauge" или "counter").
// name - имя метрики.
// Возвращает строковое представление значения или ошибку, если метрика не найдена.
func (s *MetricsService) GetMetricValue(ctx context.Context, mType, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("metric name cannot be empty")
	}

	switch mType {
	case "gauge":
		value, err := s.storage.GetGauge(ctx, name)
		if err != nil {
			return "", err
		}
		return strconv.FormatFloat(value, 'f', -1, 64), nil
	case "counter":
		value, err := s.storage.GetCounter(ctx, name)
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(value, 10), nil
	default:
		return "", fmt.Errorf("invalid metric type: must be 'gauge' or 'counter'")
	}
}

// GetAllMetrics возвращает все метрики из хранилища.
func (s *MetricsService) GetAllMetrics() []models.Metrics {
	return s.storage.GetAll()
}

// UpdateMetricFromJSON обновляет метрику из JSON объекта.
// Для gauge ожидает поле Value, для counter - поле Delta.
func (s *MetricsService) UpdateMetricFromJSON(ctx context.Context, m *models.Metrics) error {
	if m.ID == "" {
		return fmt.Errorf("metric ID cannot be empty")
	}
	if m.MType == "" {
		return fmt.Errorf("metric type cannot be empty")
	}

	switch m.MType {
	case models.Gauge:
		if m.Value == nil {
			return fmt.Errorf("gauge metric must have value")
		}
		return s.storage.SetGauge(ctx, m.ID, *m.Value)
	case models.Counter:
		if m.Delta == nil {
			return fmt.Errorf("counter metric must have delta")
		}
		return s.storage.AddCounter(ctx, m.ID, *m.Delta)
	default:
		return fmt.Errorf("invalid metric type: must be 'gauge' or 'counter'")
	}
}

// GetMetricFromJSON получает метрику из JSON объекта и возвращает её с заполненным значением.
// Для gauge заполняет поле Value, для counter - поле Delta.
// Возвращает *models.Metrics с данными из хранилища или ошибку.
func (s *MetricsService) GetMetricFromJSON(ctx context.Context, m *models.Metrics) (*models.Metrics, error) {
	if m.ID == "" {
		return nil, fmt.Errorf("metric ID cannot be empty")
	}
	if m.MType == "" {
		return nil, fmt.Errorf("metric type cannot be empty")
	}

	result := m

	switch m.MType {
	case models.Gauge:
		value, err := s.storage.GetGauge(ctx, m.ID)
		if err != nil {
			return nil, err
		}
		result.Value = &value
		result.Delta = nil
	case models.Counter:
		value, err := s.storage.GetCounter(ctx, m.ID)
		if err != nil {
			return nil, err
		}
		result.Delta = &value
		result.Value = nil
	default:
		return nil, fmt.Errorf("invalid metric type: must be 'gauge' or 'counter'")
	}

	return result, nil
}

// UpdateMetricsBatch выполняет пакетное обновление метрик.
// metrics - срез метрик для обновления.
func (s *MetricsService) UpdateMetricsBatch(ctx context.Context, metrics []models.Metrics) error {
	return s.storage.UpdateMetricsBatch(ctx, metrics)
}
