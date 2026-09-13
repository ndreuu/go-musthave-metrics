package middleware

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"go-musthave-metrics/internal/crypto"
)

func TestDecrypt_NoKey_Passes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var body []byte
	r := gin.New()
	r.Use(Decrypt(nil))
	r.POST("/update", func(c *gin.Context) {
		body, _ = io.ReadAll(c.Request.Body)
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/update", strings.NewReader("plain body"))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if string(body) != "plain body" {
		t.Errorf("body was modified without key: got %q", body)
	}
}

func TestDecrypt_ValidEncryptedBody_Passes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	plain := []byte(`{"id":"gauge","type":"gauge","value":1.5}`)
	encrypted, err := crypto.Encrypt(&priv.PublicKey, plain)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	var body []byte
	r := gin.New()
	r.Use(Decrypt(priv))
	r.POST("/update", func(c *gin.Context) {
		body, _ = io.ReadAll(c.Request.Body)
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(encrypted))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if string(body) != string(plain) {
		t.Errorf("decrypted body mismatch: got %q, want %q", body, plain)
	}
}

func TestDecrypt_InvalidEncryptedBody_Rejected(t *testing.T) {
	gin.SetMode(gin.TestMode)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	r := gin.New()
	r.Use(Decrypt(priv))
	r.POST("/update", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/update", strings.NewReader("not-encrypted"))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}