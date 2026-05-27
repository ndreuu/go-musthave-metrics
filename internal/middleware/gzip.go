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

		gzResponseWriter := &gzipResponseWriter{
			ResponseWriter: c.Writer,
			Writer:         gzWriter,
		}

		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")
		c.Writer = gzResponseWriter

		c.Next()
	}
}

type gzipResponseWriter struct {
	gin.ResponseWriter
	Writer *gzip.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (w *gzipResponseWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(statusCode)
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
