// Package models предоставляет модели данных для метрик и аудита.
package models

const (
	// Counter представляет тип метрики "counter".
	Counter = "counter"
	// Gauge представляет тип метрики "gauge".
	Gauge   = "gauge"
)

// Metrics представляет метрику с именем, типом и значением.
// Delta и Value объявлены через указатели, чтобы отличать значение "0" от незаданного значения.
//
// generate:reset
type Metrics struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
	Hash  string   `json:"hash,omitempty"`
}
