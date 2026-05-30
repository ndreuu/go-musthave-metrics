package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequestLogger_Logging(t *testing.T) {
	gin.SetMode(gin.TestMode)

	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	router := gin.New()
	router.Use(RequestLogger(logger))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.GreaterOrEqual(t, logs.Len(), 2)

	var requestLogged, responseLogged bool
	for i := 0; i < logs.Len(); i++ {
		entry := logs.All()[i]
		if entry.Message == "request" {
			requestLogged = true
			assert.Equal(t, "GET", entry.Context[0].String)
			assert.Equal(t, "/test", entry.Context[1].String)
		}
		if entry.Message == "response" {
			responseLogged = true
			assert.Equal(t, int64(200), entry.Context[0].Integer)
		}
	}

	assert.True(t, requestLogged, "Expected request to be logged")
	assert.True(t, responseLogged, "Expected response to be logged")
}

func TestRequestLogger_Duration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	router := gin.New()
	router.Use(RequestLogger(logger))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	foundDuration := false
	for i := 0; i < logs.Len(); i++ {
		entry := logs.All()[i]
		if entry.Message == "request" {
			for _, field := range entry.Context {
				if field.Key == "duration" {
					foundDuration = true
					break
				}
			}
		}
	}

	assert.True(t, foundDuration, "Expected duration field in request log")
}
