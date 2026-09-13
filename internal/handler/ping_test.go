package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go-musthave-metrics/internal/repository"
)

func TestPingHandler_NoDBConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewPingHandler(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ping", nil)

	handler.PingHandler(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "database not configured")
}

func TestPingHandler_WithDB(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dsn := "postgres://postgres:postgres@localhost:5432/testdb?sslmode=disable"
	storage, err := repository.NewPostgresStorage(dsn)
	if err != nil {
		t.Skip("PostgreSQL not available, skipping test")
	}
	defer func() { _ = storage.Close() }()

	handler := NewPingHandler(storage)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ping", nil)

	handler.PingHandler(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}

type failingPinger struct{}

func (failingPinger) Ping(ctx context.Context) error {
	return errors.New("db unreachable")
}

func TestPingHandler_PingError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewPingHandler(failingPinger{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ping", nil)

	handler.PingHandler(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "database connection failed")
}
