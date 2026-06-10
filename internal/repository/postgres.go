package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"

	models "go-musthave-metrics/internal/model"
)

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(dsn string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := runMigrations(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &PostgresStorage{db: db}, nil
}

func runMigrations(db *sql.DB) error {
	driver, err := migratepostgres.WithInstance(db, &migratepostgres.Config{})
	if err != nil {
		return fmt.Errorf("create migrate driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}

func (p *PostgresStorage) SetGauge(name string, value float64) error {
	if name == "" {
		return fmt.Errorf("metric name cannot be empty")
	}

	query := `
		INSERT INTO metrics (id, type, value, delta, updated_at)
		VALUES ($1, 'gauge', $2, NULL, NOW())
		ON CONFLICT (id) DO UPDATE SET
			type = 'gauge',
			value = EXCLUDED.value,
			delta = NULL,
			updated_at = NOW()
	`

	_, err := p.db.Exec(query, name, value)
	if err != nil {
		return fmt.Errorf("set gauge: %w", err)
	}

	return nil
}

func (p *PostgresStorage) AddCounter(name string, delta int64) error {
	if name == "" {
		return fmt.Errorf("metric name cannot be empty")
	}

	query := `
		INSERT INTO metrics (id, type, delta, value, updated_at)
		VALUES ($1, 'counter', $2, NULL, NOW())
		ON CONFLICT (id) DO UPDATE SET
			type = 'counter',
			delta = COALESCE(metrics.delta, 0) + EXCLUDED.delta,
			value = NULL,
			updated_at = NOW()
	`

	_, err := p.db.Exec(query, name, delta)
	if err != nil {
		return fmt.Errorf("add counter: %w", err)
	}

	return nil
}

func (p *PostgresStorage) GetGauge(name string) (float64, error) {
	if name == "" {
		return 0, fmt.Errorf("metric name cannot be empty")
	}

	var value float64
	err := p.db.QueryRow(
		`SELECT value FROM metrics WHERE id = $1 AND type = 'gauge'`,
		name,
	).Scan(&value)

	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("gauge metric not found: %s", name)
	}
	if err != nil {
		return 0, fmt.Errorf("get gauge: %w", err)
	}

	return value, nil
}

func (p *PostgresStorage) GetCounter(name string) (int64, error) {
	if name == "" {
		return 0, fmt.Errorf("metric name cannot be empty")
	}

	var delta int64
	err := p.db.QueryRow(
		`SELECT delta FROM metrics WHERE id = $1 AND type = 'counter'`,
		name,
	).Scan(&delta)

	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("counter metric not found: %s", name)
	}
	if err != nil {
		return 0, fmt.Errorf("get counter: %w", err)
	}

	return delta, nil
}

func (p *PostgresStorage) GetAll() []models.Metrics {
	rows, err := p.db.Query(`SELECT id, type, delta, value FROM metrics`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []models.Metrics

	for rows.Next() {
		var m models.Metrics
		var delta sql.NullInt64
		var value sql.NullFloat64

		if err := rows.Scan(&m.ID, &m.MType, &delta, &value); err != nil {
			continue
		}

		switch m.MType {
		case "gauge":
			if value.Valid {
				v := value.Float64
				m.Value = &v
				result = append(result, m)
			}
		case "counter":
			if delta.Valid {
				d := delta.Int64
				m.Delta = &d
				result = append(result, m)
			}
		}
	}

	return result
}

func (p *PostgresStorage) Close() error {
	return p.db.Close()
}

func (p *PostgresStorage) GetDB() *sql.DB {
	return p.db
}

func (p *PostgresStorage) Ping(ctx context.Context) error {
	if p == nil || p.db == nil {
		return fmt.Errorf("database is not configured")
	}

	return p.db.PingContext(ctx)
}
