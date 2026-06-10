package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go-musthave-metrics/internal/config/db"
)

func TestPingHandler_NoDBConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbConn := db.NewDB("")
	handler := NewPingHandler(dbConn)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ping", nil)

	handler.PingHandler(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "database DSN is not configured")
}

func TestPingHandler_InvalidDSN(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbConn := db.NewDB("invalid-dsn")
	handler := NewPingHandler(dbConn)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ping", nil)

	handler.PingHandler(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "database connection failed")
}

func TestPingHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dsn := "postgres://postgres:postgres@localhost:5432/testdb?sslmode=disable"
	dbConn := db.NewDB(dsn)
	handler := NewPingHandler(dbConn)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ping", nil)

	handler.PingHandler(c)

	if w.Code == http.StatusOK {
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "ok")
	} else {
		t.Skip("PostgreSQL not available, skipping success test")
	}
}

func TestPingHandler_Timeout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dsn := "postgres://user:pass@192.0.2.1:5432/db?sslmode=disable&connect_timeout=1"
	dbConn := db.NewDB(dsn)
	handler := NewPingHandler(dbConn)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ping", nil)

	start := time.Now()
	handler.PingHandler(c)
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "database connection failed")
	assert.Less(t, elapsed, 5*time.Second, "Ping handler should timeout within reasonable time")
}

func TestPingHandler_ContextCancellation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dsn := "postgres://user:pass@192.0.2.1:5432/db?sslmode=disable"
	dbConn := db.NewDB(dsn)
	handler := NewPingHandler(dbConn)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	c.Request = req.WithContext(ctx)

	handler.PingHandler(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
