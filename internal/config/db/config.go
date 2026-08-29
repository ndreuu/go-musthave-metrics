package db

import (
	"fmt"
	"os"
	"time"
)

// Config представляет конфигурацию подключения к базе данных.
type Config struct {
	DSN string

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// NewConfig создает конфигурацию с параметрами по умолчанию.
func NewConfig() *Config {
	return &Config{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}
}

// LoadFromEnv загружает DSN из переменной окружения DATABASE_DSN.
func (c *Config) LoadFromEnv() {
	if dsn := os.Getenv("DATABASE_DSN"); dsn != "" {
		c.DSN = dsn
	}
}

// LoadFromFlags загружает DSN из аргументов командной строки.
func (c *Config) LoadFromFlags(dsn string) {
	if dsn != "" {
		c.DSN = dsn
	}
}

// Validate проверяет корректность конфигурации.
// Возвращает ошибку, если DSN не указан.
func (c *Config) Validate() error {
	if c.DSN == "" {
		return fmt.Errorf("database DSN is required")
	}
	return nil
}
