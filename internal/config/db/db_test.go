package db

import (
	"testing"

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
	db, err := NewDB("")
	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "database DSN is not configured")
}

func TestDB_InvalidDSN(t *testing.T) {
	db, err := NewDB("invalid-dsn")
	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "invalid-dsn")
}

func TestDB_UnreachableHost(t *testing.T) {
	dsn := "postgres://user:pass@192.0.2.1:5432/db?sslmode=disable&connect_timeout=1"
	db, err := NewDB(dsn)
	assert.Error(t, err)
	assert.Nil(t, db)
}

func TestDB_Close(t *testing.T) {
	// Test closing nil DB
	var db *DB
	err := db.Close()
	assert.NoError(t, err)
}

func TestDB_ConcurrentPing(t *testing.T) {
	t.Skip("Skipping test that requires database connection")
}
