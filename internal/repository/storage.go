package repository

import (
	"context"
	models "go-musthave-metrics/internal/model"
)

type Storage interface {
	SetGauge(ctx context.Context, name string, value float64) error
	AddCounter(ctx context.Context, name string, delta int64) error
	GetGauge(ctx context.Context, name string) (float64, error)
	GetCounter(ctx context.Context, name string) (int64, error)
	GetAll() []models.Metrics
	UpdateMetricsBatch(ctx context.Context, metrics []models.Metrics) error
}
