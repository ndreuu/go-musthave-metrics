package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	models "go-musthave-metrics/internal/model"
	"go-musthave-metrics/pkg/retry"
)

type Sender struct {
	serverAddress string
	client        *http.Client
	key           string
}

func NewSender(serverAddress string, key string) *Sender {
	return &Sender{
		serverAddress: serverAddress,
		client:        &http.Client{},
		key:           key,
	}
}

func calculateHash(data []byte, key string) string {
	h := sha256.New()
	h.Write(data)
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Sender) Send(metric *Metric) error {
	return s.sendWithRetry(metric)
}

func (s *Sender) sendWithRetry(metric *Metric) error {
	cfg := retry.DefaultConfig()
	ctx := context.Background()

	return retry.Do(ctx, cfg, func() error {
		return s.sendOnce(metric, "/update")
	})
}

func (s *Sender) sendOnce(metric *Metric, endpoint string) error {
	m := models.Metrics{
		ID:    metric.Name,
		MType: metric.MType,
	}

	switch metric.MType {
	case "gauge":
		m.Value = &metric.Value
	case "counter":
		delta := int64(metric.Value)
		m.Delta = &delta
	}

	jsonData, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	buf := &bytes.Buffer{}
	gzWriter := gzip.NewWriter(buf)
	if _, err := gzWriter.Write(jsonData); err != nil {
		return fmt.Errorf("failed to compress data: %w", err)
	}
	if err := gzWriter.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, s.serverAddress+endpoint, buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	if s.key != "" {
		hash := calculateHash(buf.Bytes(), s.key)
		req.Header.Set("HashSHA256", hash)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil
}

func (s *Sender) SendAll(metrics []*Metric) []error {
	errors := make([]error, 0)
	for _, metric := range metrics {
		if err := s.Send(metric); err != nil {
			errors = append(errors, fmt.Errorf("failed to send metric %s: %w", metric.Name, err))
		}
	}
	return errors
}

func (s *Sender) SendBatch(metrics []*Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	cfg := retry.DefaultConfig()
	ctx := context.Background()

	return retry.Do(ctx, cfg, func() error {
		return s.sendBatchOnce(metrics)
	})
}

func (s *Sender) sendBatchOnce(metrics []*Metric) error {
	batch := make([]models.Metrics, 0, len(metrics))
	for _, metric := range metrics {
		m := models.Metrics{
			ID:    metric.Name,
			MType: metric.MType,
		}
		switch metric.MType {
		case "gauge":
			m.Value = &metric.Value
		case "counter":
			delta := int64(metric.Value)
			m.Delta = &delta
		}
		batch = append(batch, m)
	}

	jsonData, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	buf := &bytes.Buffer{}
	gzWriter := gzip.NewWriter(buf)
	if _, err := gzWriter.Write(jsonData); err != nil {
		return fmt.Errorf("failed to compress data: %w", err)
	}
	if err := gzWriter.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, s.serverAddress+"/updates/", buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	if s.key != "" {
		hash := calculateHash(buf.Bytes(), s.key)
		req.Header.Set("HashSHA256", hash)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil
}

type MetricGetter interface {
	GetMetric(name string) *Metric
}

func (s *Sender) SendMetricByName(getter MetricGetter, name string) error {
	metric := getter.GetMetric(name)
	if metric == nil {
		return fmt.Errorf("metric %s not found", name)
	}
	return s.Send(metric)
}

func FormatMetricValue(mType string, value float64) string {
	if mType == "counter" {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}
