package repository

import (
	models "go-musthave-metrics/internal/model"
)

type Storage interface {
	SetGauge(name string, value float64) error
	AddCounter(name string, delta int64) error
	GetGauge(name string) (float64, error)
	GetCounter(name string) (int64, error)
	GetAll() []models.Metrics
	UpdateMetricsBatch(metrics []models.Metrics) error
}
