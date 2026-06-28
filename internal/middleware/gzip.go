package middleware

import (
	"compress/gzip"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func Gzip() gin.HandlerFunc {
	return func(c *gin.Context) {
		acceptEncoding := c.GetHeader("Accept-Encoding")
		supportsGzip := strings.Contains(acceptEncoding, "gzip")

		if !supportsGzip {
			c.Next()
			return
		}

		gzWriter := gzip.NewWriter(c.Writer)
		defer gzWriter.Close()

		c.Writer = &gzipResponseWriter{
			ResponseWriter: c.Writer,
			Writer:         gzWriter,
		}

		c.Next()
	}
}

type gzipResponseWriter struct {
	gin.ResponseWriter
	Writer      *gzip.Writer
	compressing bool
	checked     bool
	statusCode  int
}

func (w *gzipResponseWriter) shouldCompress() bool {
	contentType := w.Header().Get("Content-Type")

	return strings.HasPrefix(contentType, "application/json") ||
		strings.HasPrefix(contentType, "text/html")
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.checked {
		w.checked = true
		w.compressing = w.shouldCompress()

		if w.compressing {
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Vary", "Accept-Encoding")
			w.Header().Del("Content-Length")
		}

		if w.statusCode != 0 {
			w.ResponseWriter.WriteHeader(w.statusCode)
		}
	}

	if !w.compressing {
		return w.ResponseWriter.Write(b)
	}

	return w.Writer.Write(b)
}

func (w *gzipResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func GzipUnmarshal() gin.HandlerFunc {
	return func(c *gin.Context) {
		contentEncoding := c.GetHeader("Content-Encoding")

		if contentEncoding != "gzip" {
			c.Next()
			return
		}

		gzReader, err := gzip.NewReader(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid gzip data"})
			return
		}
		defer gzReader.Close()

		c.Request.Body = gzReader

		c.Next()
	}
}
