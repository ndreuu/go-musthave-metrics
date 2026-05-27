package agent

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	models "go-musthave-metrics/internal/model"
)

type Sender struct {
	serverAddress string
	client        *http.Client
}

func NewSender(serverAddress string) *Sender {
	return &Sender{
		serverAddress: serverAddress,
		client:        &http.Client{},
	}
}

func (s *Sender) Send(metric *Metric) error {
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

	var bodyReader io.Reader
	var contentEncoding string

	buf := &strings.Builder{}
	gzWriter := gzip.NewWriter(buf)
	if _, err := gzWriter.Write(jsonData); err != nil {
		return fmt.Errorf("failed to compress data: %w", err)
	}
	if err := gzWriter.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}
	bodyReader = strings.NewReader(buf.String())
	contentEncoding = "gzip"

	req, err := http.NewRequest(http.MethodPost, s.serverAddress+"/update", bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", contentEncoding)

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
