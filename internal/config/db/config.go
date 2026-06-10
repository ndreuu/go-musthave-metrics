package db

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	DSN string

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func NewConfig() *Config {
	return &Config{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}
}

func (c *Config) LoadFromEnv() {
	if dsn := os.Getenv("DATABASE_DSN"); dsn != "" {
		c.DSN = dsn
	}
}

func (c *Config) LoadFromFlags(dsn string) {
	if dsn != "" {
		c.DSN = dsn
	}
}

func (c *Config) Validate() error {
	if c.DSN == "" {
		return fmt.Errorf("database DSN is required")
	}
	return nil
}
