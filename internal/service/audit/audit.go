// Package audit предоставляет сервис аудита для логирования событий.
package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	models "go-musthave-metrics/internal/model"
)

const defaultTimeout = 5 * time.Second

// Observer определяет интерфейс наблюдателя для событий аудита.
type Observer interface {
	Notify(event *models.AuditEvent) error
	Close() error
}

// Subject определяет интерфейс субъекта для управления наблюдателями.
type Subject interface {
	AddObserver(observer Observer)
	RemoveObserver(observer Observer)
	NotifyObservers(event *models.AuditEvent)
}

// AuditService реализует паттерн Observer для аудита событий.
type AuditService struct {
	observers []Observer
	mu        sync.RWMutex
}

// NewAuditService создает новый экземпляр AuditService.
func NewAuditService() *AuditService {
	return &AuditService{
		observers: make([]Observer, 0),
	}
}

// AddObserver добавляет наблюдателя в список.
func (s *AuditService) AddObserver(observer Observer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observers = append(s.observers, observer)
}

// RemoveObserver удаляет наблюдателя из списка.
func (s *AuditService) RemoveObserver(observer Observer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, obs := range s.observers {
		if obs == observer {
			s.observers = append(s.observers[:i], s.observers[i+1:]...)
			break
		}
	}
}

// NotifyObservers уведомляет всех наблюдателей о событии.
func (s *AuditService) NotifyObservers(event *models.AuditEvent) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, observer := range s.observers {
		if err := observer.Notify(event); err != nil {
			continue
		}
	}
}

// Close закрывает всех наблюдателей.
func (s *AuditService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var lastErr error
	for _, observer := range s.observers {
		if err := observer.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// FileObserver реализует Observer для записи событий в файл.
type FileObserver struct {
	file *os.File
	mu   sync.Mutex
}

// NewFileObserver создает наблюдателя для записи в файл.
// filePath - путь к файлу для записи событий аудита.
func NewFileObserver(filePath string) (*FileObserver, error) {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory for audit file: %w", err)
	}

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit file: %w", err)
	}

	return &FileObserver{
		file: file,
	}, nil
}

func (o *FileObserver) Notify(event *models.AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	if o.file == nil {
		return fmt.Errorf("audit file is closed")
	}

	data = append(data, '\n')

	if _, err := o.file.Write(data); err != nil {
		return fmt.Errorf("failed to write audit event to file: %w", err)
	}

	return nil
}

func (o *FileObserver) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.file == nil {
		return nil
	}

	if err := o.file.Close(); err != nil {
		return fmt.Errorf("failed to close audit file: %w", err)
	}

	o.file = nil
	return nil
}
// URLObserver реализует Observer для отправки событий на URL.
type URLObserver struct {
	url    string
	client *http.Client
}

// NewURLObserver создает наблюдателя для отправки событий на URL.
// url - адрес сервера для получения событий аудита.
func NewURLObserver(url string) *URLObserver {
	return &URLObserver{
		url:    url,
		client: &http.Client{Timeout: defaultTimeout},
	}
}

func (o *URLObserver) Notify(event *models.AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create audit request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send audit event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("audit server returned status: %d", resp.StatusCode)
	}

	return nil
}

func (o *URLObserver) Close() error {
	o.client.CloseIdleConnections()
	return nil
}
