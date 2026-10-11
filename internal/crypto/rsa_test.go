package crypto

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

// generateTestKeys генерирует пару RSA-ключей и записывает их в PEM-файлы.
// Возвращает пути к файлам публичного и приватного ключей.
func generateTestKeys(t *testing.T, keySize int) (pubPath, privPath string) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	dir := t.TempDir()

	pubPath = filepath.Join(dir, "public.pem")
	privPath = filepath.Join(dir, "private.pem")

	pubDER := x509.MarshalPKCS1PublicKey(&priv.PublicKey)
	pubBlock := &pem.Block{Type: "RSA PUBLIC KEY", Bytes: pubDER}
	if err := os.WriteFile(pubPath, pem.EncodeToMemory(pubBlock), 0600); err != nil {
		t.Fatalf("failed to write public key: %v", err)
	}

	privDER := x509.MarshalPKCS1PrivateKey(priv)
	privBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privDER}
	if err := os.WriteFile(privPath, pem.EncodeToMemory(privBlock), 0600); err != nil {
		t.Fatalf("failed to write private key: %v", err)
	}

	return pubPath, privPath
}

func TestLoadPublicKey(t *testing.T) {
	pubPath, _ := generateTestKeys(t, 2048)

	pub, err := LoadPublicKey(pubPath)
	if err != nil {
		t.Fatalf("LoadPublicKey returned error: %v", err)
	}
	if pub == nil {
		t.Fatal("LoadPublicKey returned nil key")
	}
	if pub.Size() == 0 {
		t.Fatal("public key has invalid size")
	}
}

func TestLoadPublicKey_NonExistent(t *testing.T) {
	if _, err := LoadPublicKey(filepath.Join(t.TempDir(), "missing.pem")); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadPrivateKey(t *testing.T) {
	_, privPath := generateTestKeys(t, 2048)

	priv, err := LoadPrivateKey(privPath)
	if err != nil {
		t.Fatalf("LoadPrivateKey returned error: %v", err)
	}
	if priv == nil {
		t.Fatal("LoadPrivateKey returned nil key")
	}
}

func TestLoadPrivateKey_NonExistent(t *testing.T) {
	if _, err := LoadPrivateKey(filepath.Join(t.TempDir(), "missing.pem")); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	pubPath, privPath := generateTestKeys(t, 2048)

	pub, err := LoadPublicKey(pubPath)
	if err != nil {
		t.Fatalf("failed to load public key: %v", err)
	}
	priv, err := LoadPrivateKey(privPath)
	if err != nil {
		t.Fatalf("failed to load private key: %v", err)
	}

	message := []byte("hello, encrypted world")
	ciphertext, err := Encrypt(pub, message)
	if err != nil {
		t.Fatalf("Encrypt returned error: %v", err)
	}
	if bytes.Equal(ciphertext, message) {
		t.Fatal("ciphertext must differ from plaintext")
	}

	plaintext, err := Decrypt(priv, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt returned error: %v", err)
	}
	if !bytes.Equal(plaintext, message) {
		t.Errorf("decrypted message mismatch: got %q, want %q", plaintext, message)
	}
}