package db

import (
	"context"
	"database/sql"
)

func EnsureBaseSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE EXTENSION IF NOT EXISTS vector;`,
		`CREATE TABLE IF NOT EXISTS documents (
			id BIGSERIAL PRIMARY KEY,
			doc_id TEXT UNIQUE NOT NULL,
			title TEXT,
			source TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS chunks (
			id BIGSERIAL PRIMARY KEY,
			chunk_id TEXT UNIQUE NOT NULL,
			doc_id TEXT NOT NULL,
			content TEXT NOT NULL,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_doc_id ON chunks(doc_id);`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_metadata ON chunks USING GIN (metadata);`,
	}

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}