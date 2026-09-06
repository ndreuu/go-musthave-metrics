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
	"go-musthave-metrics/pkg/retry"
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

func (p *PostgresStorage) SetGauge(ctx context.Context, name string, value float64) error {
	if name == "" {
		return fmt.Errorf("metric name cannot be empty")
	}

	cfg := retry.DefaultConfig()

	return retry.Do(ctx, cfg, func() error {
		query := `
			INSERT INTO metrics (id, type, value, delta, updated_at)
			VALUES ($1, 'gauge', $2, NULL, NOW())
			ON CONFLICT (id) DO UPDATE SET
				type = 'gauge',
				value = EXCLUDED.value,
				delta = NULL,
				updated_at = NOW()
		`

		_, err := p.db.ExecContext(ctx, query, name, value)
		if err != nil {
			return fmt.Errorf("set gauge: %w", err)
		}

		return nil
	})
}

func (p *PostgresStorage) AddCounter(ctx context.Context, name string, delta int64) error {
	if name == "" {
		return fmt.Errorf("metric name cannot be empty")
	}

	cfg := retry.DefaultConfig()

	return retry.Do(ctx, cfg, func() error {
		query := `
			INSERT INTO metrics (id, type, delta, value, updated_at)
			VALUES ($1, 'counter', $2, NULL, NOW())
			ON CONFLICT (id) DO UPDATE SET
				type = 'counter',
				delta = COALESCE(metrics.delta, 0) + EXCLUDED.delta,
				value = NULL,
				updated_at = NOW()
		`

		_, err := p.db.ExecContext(ctx, query, name, delta)
		if err != nil {
			return fmt.Errorf("add counter: %w", err)
		}

		return nil
	})
}

func (p *PostgresStorage) GetGauge(ctx context.Context, name string) (float64, error) {
	if name == "" {
		return 0, fmt.Errorf("metric name cannot be empty")
	}

	var result float64
	cfg := retry.DefaultConfig()

	err := retry.Do(ctx, cfg, func() error {
		err := p.db.QueryRowContext(ctx,
			`SELECT value FROM metrics WHERE id = $1 AND type = 'gauge'`,
			name,
		).Scan(&result)

		if err == sql.ErrNoRows {
			return fmt.Errorf("gauge metric not found: %s", name)
		}
		if err != nil {
			return fmt.Errorf("get gauge: %w", err)
		}

		return nil
	})

	return result, err
}

func (p *PostgresStorage) GetCounter(ctx context.Context, name string) (int64, error) {
	if name == "" {
		return 0, fmt.Errorf("metric name cannot be empty")
	}

	var result int64
	cfg := retry.DefaultConfig()

	err := retry.Do(ctx, cfg, func() error {
		err := p.db.QueryRowContext(ctx,
			`SELECT delta FROM metrics WHERE id = $1 AND type = 'counter'`,
			name,
		).Scan(&result)

		if err == sql.ErrNoRows {
			return fmt.Errorf("counter metric not found: %s", name)
		}
		if err != nil {
			return fmt.Errorf("get counter: %w", err)
		}

		return nil
	})

	return result, err
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

	if err := rows.Err(); err != nil {
		return nil
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

func (p *PostgresStorage) UpdateMetricsBatch(ctx context.Context, metrics []models.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}

	cfg := retry.DefaultConfig()

	return retry.Do(ctx, cfg, func() error {
		txCtx, txCancel := context.WithTimeout(ctx, 10*time.Second)
		defer txCancel()

		tx, err := p.db.BeginTx(txCtx, nil)
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}
		defer tx.Rollback()

		for _, metric := range metrics {
			switch metric.MType {
			case models.Gauge:
				if metric.Value == nil {
					continue
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
				if _, err := tx.ExecContext(txCtx, query, metric.ID, *metric.Value); err != nil {
					return fmt.Errorf("set gauge %s: %w", metric.ID, err)
				}
			case models.Counter:
				if metric.Delta == nil {
					continue
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
				if _, err := tx.ExecContext(txCtx, query, metric.ID, *metric.Delta); err != nil {
					return fmt.Errorf("add counter %s: %w", metric.ID, err)
				}
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit transaction: %w", err)
		}

		return nil
	})
}
