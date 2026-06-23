CREATE TABLE IF NOT EXISTS metrics (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    delta BIGINT,
    value DOUBLE PRECISION,
    updated_at TIMESTAMP DEFAULT now()
);
