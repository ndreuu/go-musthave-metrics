package retry

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxAttempts != 4 {
		t.Errorf("expected MaxAttempts 4, got %d", cfg.MaxAttempts)
	}

	if len(cfg.Backoff) != 3 {
		t.Errorf("expected 3 backoff intervals, got %d", len(cfg.Backoff))
	}

	expectedBackoffs := []time.Duration{time.Second, 3 * time.Second, 5 * time.Second}
	for i, expected := range expectedBackoffs {
		if cfg.Backoff[i] != expected {
			t.Errorf("backoff[%d]: expected %v, got %v", i, expected, cfg.Backoff[i])
		}
	}
}

func TestIsRetriableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"context canceled", context.Canceled, false},
		{"context deadline exceeded", context.DeadlineExceeded, false},
		{"EOF error", io.EOF, true},
		{"connection refused", errors.New("connection refused"), true},
		{"connection reset", errors.New("connection reset"), true},
		{"broken pipe", errors.New("broken pipe"), true},
		{"no connection", errors.New("no connection"), true},
		{"i/o timeout", errors.New("i/o timeout"), true},
		{"unexpected EOF", errors.New("unexpected EOF"), true},
		{"permanent error", errors.New("permanent error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetriableError(tt.err)
			if result != tt.expected {
				t.Errorf("IsRetriableError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestIsRetriableDBError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"Class 08 - Connection Exception", &pgconn.PgError{Code: "08000"}, true},
		{"Class 08 - Connection Does Not Exist", &pgconn.PgError{Code: "08003"}, true},
		{"Class 08 - Connection Failure", &pgconn.PgError{Code: "08006"}, true},
		{"Class 08 - Client Unable To Establish Connection", &pgconn.PgError{Code: "08001"}, true},
		{"Class 23 - Unique Violation", &pgconn.PgError{Code: "23505"}, false},
		{"Class 40 - Transaction Rollback", &pgconn.PgError{Code: "40001"}, false},
		{"non-pg error", errors.New("some error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetriableDBError(tt.err)
			if result != tt.expected {
				t.Errorf("IsRetriableDBError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestDo_Success(t *testing.T) {
	cfg := Config{
		MaxAttempts: 3,
		Backoff:     []time.Duration{time.Millisecond},
	}

	attempts := 0
	fn := func() error {
		attempts++
		return nil
	}

	err := Do(context.Background(), cfg, fn)
	if err != nil {
		t.Errorf("Do() returned error: %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestDo_RetrySuccess(t *testing.T) {
	cfg := Config{
		MaxAttempts: 3,
		Backoff:     []time.Duration{time.Millisecond},
	}

	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 2 {
			return errors.New("connection refused")
		}
		return nil
	}

	err := Do(context.Background(), cfg, fn)
	if err != nil {
		t.Errorf("Do() returned error: %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestDo_Exhausted(t *testing.T) {
	cfg := Config{
		MaxAttempts: 3,
		Backoff:     []time.Duration{time.Millisecond},
	}

	attempts := 0
	fn := func() error {
		attempts++
		return errors.New("connection refused")
	}

	err := Do(context.Background(), cfg, fn)
	if err == nil {
		t.Error("Do() should return error when all attempts exhausted")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDo_NonRetriable(t *testing.T) {
	cfg := Config{
		MaxAttempts: 3,
		Backoff:     []time.Duration{time.Millisecond},
	}

	attempts := 0
	fn := func() error {
		attempts++
		return errors.New("permanent error")
	}

	err := Do(context.Background(), cfg, fn)
	if err == nil {
		t.Error("Do() should return error for non-retriable error")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt for non-retriable error, got %d", attempts)
	}
}

func TestDo_ContextCanceled(t *testing.T) {
	cfg := Config{
		MaxAttempts: 3,
		Backoff:     []time.Duration{100 * time.Millisecond},
	}

	ctx, cancel := context.WithCancel(context.Background())

	fn := func() error {
		cancel()
		return errors.New("connection refused")
	}

	err := Do(ctx, cfg, fn)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestIsRetriableError_NetworkTimeout(t *testing.T) {
	err := &net.OpError{
		Op:  "read",
		Net: "tcp",
		Err: &timeoutError{},
	}

	if !IsRetriableError(err) {
		t.Error("expected network timeout error to be retriable")
	}
}

type timeoutError struct{}

func (e *timeoutError) Error() string   { return "i/o timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }
