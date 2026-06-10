package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	*sql.DB
}

func NewDB(dsn string) (*DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("database DSN is not configured")
	}

	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{DB: sqlDB}, nil
}

func (db *DB) Ping(ctx context.Context) error {
	if db == nil || db.DB == nil {
		return fmt.Errorf("database is not configured")
	}

	return db.PingContext(ctx)
}

func (db *DB) Close() error {
	if db == nil || db.DB == nil {
		return nil
	}

	return db.DB.Close()
}