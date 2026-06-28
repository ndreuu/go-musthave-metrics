package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

func HashSHA256(key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if key == "" {
			c.Next()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		receivedHash := c.GetHeader("HashSHA256")
		expectedHash := calculateHash(body, key)

		if receivedHash == "" || receivedHash != expectedHash {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid hash"})
			return
		}

		c.Next()
	}
}

func calculateHash(data []byte, key string) string {
	h := sha256.New()
	h.Write(data)
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}
