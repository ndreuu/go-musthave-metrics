package logger

import "testing"

func TestNewLogger(t *testing.T) {
	log, err := NewLogger("info")
	if err != nil {
		t.Fatalf("NewLogger(info): %v", err)
	}
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNewLogger_InvalidLevel(t *testing.T) {
	if _, err := NewLogger("bogus"); err == nil {
		t.Fatal("expected error for invalid level")
	}
}

func TestNewLogger_DebugLevel(t *testing.T) {
	if _, err := NewLogger("debug"); err != nil {
		t.Fatalf("NewLogger(debug): %v", err)
	}
}