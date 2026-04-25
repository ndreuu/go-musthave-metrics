package repository

import (
	"fmt"
	"sync"

	models "go-musthave-metrics/internal/model"
)

type MetricsStorage interface {
	SetGauge(name string, value float64) error
	AddCounter(name string, value int64) error
	GetGauge(name string) (float64, error)
	GetCounter(name string) (int64, error)
	GetAll() []models.Metrics
}

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
		return 0, fmt.Errorf("gauge metric %%q not found", name)
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
		return 0, fmt.Errorf("counter metric %%q not found", name)
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
