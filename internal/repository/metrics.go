package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	models "go-musthave-metrics/internal/model"
)

type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (m *MemStorage) SetGauge(name string, value float64) error {
	if name == "" {
		return fmt.Errorf("metric name cannot be empty")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges[name] = value
	return nil
}

func (m *MemStorage) AddCounter(name string, value int64) error {
	if name == "" {
		return fmt.Errorf("metric name cannot be empty")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name] += value
	return nil
}

func (m *MemStorage) GetGauge(name string) (float64, error) {
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

func (m *MemStorage) GetCounter(name string) (int64, error) {
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
	var metrics []models.Metrics
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

func (m *MemStorage) SaveToFile(filePath string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make([]models.Metrics, 0, len(m.gauges)+len(m.counters))
	for name, value := range m.gauges {
		metrics = append(metrics, models.Metrics{
			ID:    name,
			MType: models.Gauge,
			Value: &value,
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

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (m *MemStorage) LoadFromFile(filePath string) error {
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
				m.counters[metric.ID] = int64(*metric.Delta)
			}
		}
	}

	return nil
}
