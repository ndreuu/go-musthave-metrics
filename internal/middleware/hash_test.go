package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateHash(t *testing.T) {
	data := []byte("hello")
	key := "secret"

	expected := sha256.Sum256(append(data, []byte(key)...))
	want := hex.EncodeToString(expected[:])

	got := calculateHash(data, key)
	if got != want {
		t.Errorf("Expected hash %s, got %s", want, got)
	}
}

func TestCalculateHash_DifferentKey(t *testing.T) {
	hash1 := calculateHash([]byte("data"), "key1")
	hash2 := calculateHash([]byte("data"), "key2")
	if hash1 == hash2 {
		t.Error("Expected different hashes for different keys")
	}
}

func TestHashSHA256_NoKey_Passes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(HashSHA256(""))
	r.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("body"))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHashSHA256_ValidHash_Passes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	key := "secret"
	body := `{"id":"gauge","type":"gauge","value":1.5}`
	expectedHash := calculateHash([]byte(body), key)

	r := gin.New()
	r.Use(HashSHA256(key))
	r.POST("/test", func(c *gin.Context) {
		data, _ := io.ReadAll(c.Request.Body)
		c.String(http.StatusOK, string(data))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("HashSHA256", expectedHash)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, body, w.Body.String())
}

func TestHashSHA256_InvalidHash_Rejected(t *testing.T) {
	gin.SetMode(gin.TestMode)

	key := "secret"
	body := `{"id":"gauge","type":"gauge","value":1.5}`

	r := gin.New()
	r.Use(HashSHA256(key))
	r.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("HashSHA256", "wrong-hash")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["error"], "invalid hash")
}