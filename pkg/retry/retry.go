package retry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

var ErrRetryExhausted = errors.New("all retry attempts exhausted")

type Config struct {
	Backoff     []time.Duration
	MaxAttempts int
}

func DefaultConfig() Config {
	return Config{
		MaxAttempts: 4,
		Backoff: []time.Duration{
			time.Second,
			3 * time.Second,
			5 * time.Second,
		},
	}
}

func Do(ctx context.Context, cfg Config, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if attempt > 0 {
			delay := cfg.Backoff[min(attempt-1, len(cfg.Backoff)-1)]

			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		if !IsRetriableError(err) {
			return err
		}
	}

	return fmt.Errorf("%w: %v", ErrRetryExhausted, lastErr)
}

func IsRetriableError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	if IsRetriableDBError(err) {
		return true
	}

	if errors.Is(err, io.EOF) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	errStr := strings.ToLower(err.Error())

	retriableStrings := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"no connection",
		"i/o timeout",
		"unexpected eof",
	}

	for _, s := range retriableStrings {
		if strings.Contains(errStr, s) {
			return true
		}
	}

	return false
}

func IsRetriableDBError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return strings.HasPrefix(pgErr.Code, "08")
	}

	return false
}
