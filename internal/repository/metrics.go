// Package repository предоставляет реализации хранилищ для метрик.
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	models "go-musthave-metrics/internal/model"
)

// MemStorage реализует интерфейс MetricsStorage в памяти с сохранением в файл.
type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
	filePath string
}

// NewMemStorage создает новое хранилище метрик в памяти.
// filePath - путь к файлу для сохранения метрик (пустая строка отключает сохранение).
// При создании загружает существующие метрики из файла, если он существует.
func NewMemStorage(filePath string) *MemStorage {
	m := &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
		filePath: filePath,
	}
	if filePath != "" {
		if err := m.loadFromFileLocked(filePath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load metrics from file: %v\n", err)
		}
	}
	return m
}

// SetGauge устанавливает значение gauge метрики.
// name - имя метрики, value - значение.
// Сохраняет метрики в файл, если filePath указан.
func (m *MemStorage) SetGauge(ctx context.Context, name string, value float64) error {
	if name == "" {
		return fmt.Errorf("metric name cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.gauges[name] = value

	return m.saveToFileLocked()
}

func (m *MemStorage) AddCounter(ctx context.Context, name string, value int64) error {
	if name == "" {
		return fmt.Errorf("metric name cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.counters[name] += value

	return m.saveToFileLocked()
}

func (m *MemStorage) GetGauge(ctx context.Context, name string) (float64, error) {
	if name == "" {
		return 0, fmt.Errorf("metric name cannot be empty")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, exists := m.gauges[name]
	if !exists {
		return 0, fmt.Errorf("gauge metric not found: %s", name)
	}
	return value, nil
}

func (m *MemStorage) GetCounter(ctx context.Context, name string) (int64, error) {
	if name == "" {
		return 0, fmt.Errorf("metric name cannot be empty")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, exists := m.counters[name]
	if !exists {
		return 0, fmt.Errorf("counter metric not found: %s", name)
	}
	return value, nil
}

func (m *MemStorage) GetAll() []models.Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalCount := len(m.gauges) + len(m.counters)
	if totalCount == 0 {
		return make([]models.Metrics, 0)
	}

	metrics := make([]models.Metrics, 0, totalCount)
	for name, value := range m.gauges {
		metrics = append(metrics, models.Metrics{
			ID:    name,
			MType: models.Gauge,
			Value: &value,
		})
	}
	for name, value := range m.counters {
		v := float64(value)
		metrics = append(metrics, models.Metrics{
			ID:    name,
			MType: models.Counter,
			Value: &v,
		})
	}
	return metrics
}

func (m *MemStorage) saveToFileLocked() error {
	if m.filePath == "" {
		return nil
	}

	metrics := make([]models.Metrics, 0, len(m.gauges)+len(m.counters))

	for name, value := range m.gauges {
		v := value
		metrics = append(metrics, models.Metrics{
			ID:    name,
			MType: models.Gauge,
			Value: &v,
		})
	}

	for name, value := range m.counters {
		v := value
		metrics = append(metrics, models.Metrics{
			ID:    name,
			MType: models.Counter,
			Delta: &v,
		})
	}

	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	tmpFile := m.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpFile, m.filePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

func (m *MemStorage) SaveToFile() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.saveToFileLocked()
}

func (m *MemStorage) loadFromFileLocked(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	var metrics []models.Metrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return fmt.Errorf("failed to unmarshal metrics: %w", err)
	}

	for _, metric := range metrics {
		switch metric.MType {
		case models.Gauge:
			if metric.Value != nil {
				m.gauges[metric.ID] = *metric.Value
			}
		case models.Counter:
			if metric.Delta != nil {
				m.counters[metric.ID] = int64(*metric.Delta)
			}
		}
	}

	return nil
}

func (m *MemStorage) UpdateMetricsBatch(ctx context.Context, metrics []models.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, metric := range metrics {
		switch metric.MType {
		case models.Gauge:
			if metric.Value != nil {
				m.gauges[metric.ID] = *metric.Value
			}
		case models.Counter:
			if metric.Delta != nil {
				m.counters[metric.ID] += *metric.Delta
			}
		}
	}

	return m.saveToFileLocked()
}
