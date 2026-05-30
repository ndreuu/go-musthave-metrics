package middleware

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGzip_ResponseCompression(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(Gzip())
	router.GET("/test", func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.String(http.StatusOK, "test")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))
	assert.Equal(t, "Accept-Encoding", w.Header().Get("Vary"))
}

func TestGzip_NoCompressionWithoutHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

	handler := Gzip()
	handler(c)

	assert.Equal(t, "", w.Header().Get("Content-Encoding"))
}

func TestGzipUnmarshal_DecompressRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jsonData := `{"id":"test","type":"gauge","value":123}`

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	_, err := gzWriter.Write([]byte(jsonData))
	assert.NoError(t, err)
	assert.NoError(t, gzWriter.Close())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest(http.MethodPost, "/update", &buf)
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler := GzipUnmarshal()
	handler(c)

	body, err := io.ReadAll(c.Request.Body)
	assert.NoError(t, err)
	assert.Equal(t, jsonData, string(body))
}

func TestGzipUnmarshal_NoDecompressionWithoutHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jsonData := `{"id":"test","type":"gauge","value":123}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest(http.MethodPost, "/update", strings.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler := GzipUnmarshal()
	handler(c)

	body, err := io.ReadAll(c.Request.Body)
	assert.NoError(t, err)
	assert.Equal(t, jsonData, string(body))
}

func TestGzipUnmarshal_InvalidGzipData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest(http.MethodPost, "/update", strings.NewReader("invalid gzip data"))
	req.Header.Set("Content-Encoding", "gzip")
	c.Request = req

	handler := GzipUnmarshal()
	handler(c)

	assert.Equal(t, http.StatusBadRequest, c.Writer.Status())
}

func TestGzip_FullRequestResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(Gzip())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "hello"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))
	assert.Equal(t, "Accept-Encoding", w.Header().Get("Vary"))

	reader, err := gzip.NewReader(w.Body)
	assert.NoError(t, err)
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	assert.NoError(t, err)

	var result map[string]interface{}
	assert.NoError(t, json.Unmarshal(decompressed, &result))
	assert.Equal(t, "hello", result["message"])
}
