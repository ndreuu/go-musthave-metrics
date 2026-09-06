package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func BenchmarkRequestLogger(b *testing.B) {
	logger, _ := zap.NewProduction()
	handler := RequestLogger(logger)

	gin.SetMode(gin.TestMode)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = &http.Request{
			Method: "GET",
			URL:    &url.URL{Path: "/test"},
			Header: make(http.Header),
		}

		handler(c)
	}
}

func BenchmarkGzipMarshal(b *testing.B) {
	gin.SetMode(gin.TestMode)

	data := make([]byte, 1024) // 1KB
	for i := range data {
		data[i] = byte(i % 256)
	}

	r := gin.New()
	r.Use(Gzip())
	r.POST("/test", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", data)
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		r.ServeHTTP(w, req)
	}
}

func BenchmarkGzipUnmarshal(b *testing.B) {
	gin.SetMode(gin.TestMode)
	handler := GzipUnmarshal()

	data := make([]byte, 1024) // 1KB
	for i := range data {
		data[i] = byte(i % 256)
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		b.Fatal(err)
	}

	if err := gz.Close(); err != nil {
		b.Fatal(err)
	}

	compressedData := buf.Bytes()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = &http.Request{
			Method: "POST",
			URL:    &url.URL{Path: "/test"},
			Header: make(http.Header),
			Body:   io.NopCloser(bytes.NewReader(compressedData)),
		}
		c.Request.Header.Set("Content-Encoding", "gzip")

		handler(c)
	}
}

func BenchmarkHashSHA256(b *testing.B) {
	gin.SetMode(gin.TestMode)
	handler := HashSHA256("test_key")

	data := []byte(`{"id":"test","type":"gauge","value":123.456}`)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = &http.Request{
			Method: "POST",
			URL:    &url.URL{Path: "/test"},
			Header: make(http.Header),
			Body:   io.NopCloser(bytes.NewReader(data)),
		}

		handler(c)
		_ = w.Header().Get("HashSHA256")
	}
}
