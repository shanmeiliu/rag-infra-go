package db

import (
	"context"
	"database/sql"
)

func EnsureSourceSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sources (
			id BIGSERIAL PRIMARY KEY,
			source_key TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			source_type TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'ready',
			origin TEXT,
			file_path TEXT,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_sources_source_type ON sources(source_type);`,
		`CREATE INDEX IF NOT EXISTS idx_sources_status ON sources(status);`,
		`CREATE INDEX IF NOT EXISTS idx_sources_created_at ON sources(created_at);`,
	}

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}
