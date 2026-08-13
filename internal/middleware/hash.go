// Package middleware предоставляет HTTP middleware для Gin.
package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HashSHA256 возвращает middleware для проверки хеша SHA256 запроса.
// key - секретный ключ для вычисления хеша.
// Сравнивает хеш из заголовка HashSHA256 с вычисленным хешем тела запроса.
// Возвращает ошибку 400 при несовпадении хешей.
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
		if receivedHash != "" {
			expectedHash := calculateHash(body, key)
			if receivedHash != expectedHash {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid hash"})
				return
			}
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
