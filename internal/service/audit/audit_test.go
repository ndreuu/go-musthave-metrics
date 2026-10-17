package audit

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	models "go-musthave-metrics/internal/model"
)

func testEvent() *models.AuditEvent {
	return &models.AuditEvent{
		IPAddress: "127.0.0.1",
		Metrics:   []string{"Alloc"},
		Timestamp: 1234567890,
	}
}

type recordingObserver struct {
	event *models.AuditEvent
}

func (o *recordingObserver) Notify(event *models.AuditEvent) error {
	o.event = event
	return nil
}

func (o *recordingObserver) Close() error { return nil }

func TestAuditService_AddNotify(t *testing.T) {
	s := NewAuditService()
	obs := &recordingObserver{}

	s.AddObserver(&FileObserver{})
	s.AddObserver(obs)
	if len(s.observers) != 2 {
		t.Fatalf("expected 2 observers, got %d", len(s.observers))
	}

	// Ошибка одного observer не должна мешать уведомлению остальных.
	event := testEvent()
	s.NotifyObservers(event)

	if obs.event != event {
		t.Fatal("observer should receive the event after another observer fails")
	}
}

func TestFileObserver_WriteAndClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	obs, err := NewFileObserver(path)
	if err != nil {
		t.Fatalf("NewFileObserver: %v", err)
	}

	if err = obs.Notify(testEvent()); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if err = obs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err = obs.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	if err = obs.Notify(testEvent()); err == nil {
		t.Fatal("expected error after close")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.Contains(string(data), "Alloc") {
		t.Fatalf("file should contain metric, got %s", data)
	}
}

func TestFileObserver_EmptyDir(t *testing.T) {
	_, err := NewFileObserver("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestURLObserver_NotifySuccess(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	obs := NewURLObserver(srv.URL)
	if err := obs.Notify(testEvent()); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if !strings.Contains(string(received), "Alloc") {
		t.Fatalf("server should receive event, got %s", received)
	}
	if err := obs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestURLObserver_NotifyServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	obs := NewURLObserver(srv.URL)
	if err := obs.Notify(testEvent()); err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestURLObserver_NotifyUnreachable(t *testing.T) {
	obs := NewURLObserver("http://127.0.0.1:1")
	if err := obs.Notify(testEvent()); err == nil {
		t.Fatal("expected error for unreachable server")
	}
}
