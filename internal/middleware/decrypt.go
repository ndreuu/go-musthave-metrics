package middleware

import (
	"bytes"
	"crypto/rsa"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"go-musthave-metrics/internal/crypto"
)

// Decrypt возвращает middleware для расшифровки тела запроса
// с помощью RSA-OAEP. privateKey - приватный ключ сервера.
// Если privateKey равен nil, middleware пропускает запрос без изменений.
// Возвращает ошибку 400 при неудачной расшифровке.
func Decrypt(privateKey *rsa.PrivateKey) gin.HandlerFunc {
	return func(c *gin.Context) {
		if privateKey == nil {
			c.Next()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
			return
		}

		decrypted, err := crypto.Decrypt(privateKey, body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "failed to decrypt body"})
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(decrypted))
		c.Next()
	}
}