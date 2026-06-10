package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfig_LoadFromEnv(t *testing.T) {
	t.Setenv("DATABASE_DSN", "postgres://test:test@localhost:5432/testdb?sslmode=disable")

	cfg := NewConfig()
	cfg.LoadFromEnv()

	assert.Equal(t, "postgres://test:test@localhost:5432/testdb?sslmode=disable", cfg.DSN)
}

func TestConfig_LoadFromFlags(t *testing.T) {
	cfg := NewConfig()
	cfg.LoadFromFlags("postgres://flag:flag@localhost:5432/flagdb?sslmode=disable")

	assert.Equal(t, "postgres://flag:flag@localhost:5432/flagdb?sslmode=disable", cfg.DSN)
}

func TestConfig_Validate(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database DSN is required")

	cfg.DSN = "postgres://test@localhost:5432/testdb"
	err = cfg.Validate()
	assert.NoError(t, err)
}

func TestDB_NoDSN(t *testing.T) {
	db := NewDB("")
	assert.False(t, db.IsConfigured())

	err := db.Ping(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database DSN is not configured")
}

func TestDB_InvalidDSN(t *testing.T) {
	db := NewDB("invalid-dsn")
	assert.True(t, db.IsConfigured())

	err := db.Ping(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid-dsn")
}

func TestDB_UnreachableHost(t *testing.T) {
	dsn := "postgres://user:pass@192.0.2.1:5432/db?sslmode=disable&connect_timeout=1"
	db := NewDB(dsn)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := db.Ping(ctx)
	assert.Error(t, err)
}

func TestDB_Close(t *testing.T) {
	db := NewDB("")
	err := db.Close()
	assert.NoError(t, err)

	db2 := NewDB("invalid-dsn")
	err = db2.Close()
	assert.NoError(t, err)
}

func TestDB_ConcurrentPing(t *testing.T) {
	dsn := "postgres://user:pass@192.0.2.1:5432/db?sslmode=disable&connect_timeout=1"
	db := NewDB(dsn)

	done := make(chan error, 3)

	for i := 0; i < 3; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			done <- db.Ping(ctx)
		}()
	}

	for i := 0; i < 3; i++ {
		err := <-done
		assert.Error(t, err)
	}
}
