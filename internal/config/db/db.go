package db

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	*sql.DB
	mu  sync.RWMutex
	dsn string
}

func NewDB(dsn string) *DB {
	return &DB{
		dsn: dsn,
	}
}

func (db *DB) initialize(ctx context.Context) error {
	if db.dsn == "" {
		return fmt.Errorf("database DSN is not configured")
	}

	sqlDB, err := sql.Open("postgres", db.dsn)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	db.DB = sqlDB
	return nil
}

func (db *DB) Ping(ctx context.Context) error {
	db.mu.RLock()
	sqlDB := db.DB
	db.mu.RUnlock()

	if sqlDB != nil {
		return sqlDB.PingContext(ctx)
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	if db.DB == nil {
		initCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := db.initialize(initCtx); err != nil {
			return err
		}
	}

	return db.DB.PingContext(ctx)
}

func (db *DB) IsConfigured() bool {
	return db.dsn != ""
}

func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.DB == nil {
		return nil
	}

	err := db.DB.Close()
	db.DB = nil

	return err
}