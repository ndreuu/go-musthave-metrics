// Package crypto предоставляет вспомогательные функции для
// асимметричного шифрования на основе RSA.
package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// LoadPublicKey читает публичный ключ из PEM-файла и возвращает *rsa.PublicKey.
// Поддерживаются форматы PKIX (SubjectPublicKeyInfo) и PKCS#1.
func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read public key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("public key: failed to decode PEM block")
	}

	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		rk, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("public key: not an RSA key")
		}
		return rk, nil
	}

	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return key, nil
	}

	return nil, errors.New("public key: unsupported PEM format")
}

// LoadPrivateKey читает приватный ключ из PEM-файла и возвращает *rsa.PrivateKey.
// Поддерживаются форматы PKCS#1 и PKCS#8.
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("private key: failed to decode PEM block")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rk, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key: not an RSA key")
		}
		return rk, nil
	}

	return nil, errors.New("private key: unsupported PEM format")
}

// Encrypt шифрует данные с помощью RSA-OAEP на основе SHA-256.
func Encrypt(pub *rsa.PublicKey, data []byte) ([]byte, error) {
	return rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, data, nil)
}

// Decrypt расшифровывает данные с помощью RSA-OAEP на основе SHA-256.
func Decrypt(priv *rsa.PrivateKey, data []byte) ([]byte, error) {
	return rsa.DecryptOAEP(sha256.New(), rand.Reader, priv, data, nil)
}