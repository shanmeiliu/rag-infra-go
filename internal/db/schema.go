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
			tsv tsvector,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,

		`ALTER TABLE chunks ADD COLUMN IF NOT EXISTS tsv tsvector;`,

		`CREATE INDEX IF NOT EXISTS idx_chunks_doc_id ON chunks(doc_id);`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_metadata ON chunks USING GIN (metadata);`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_tsv ON chunks USING GIN (tsv);`,

		`CREATE OR REPLACE FUNCTION chunks_tsv_trigger() RETURNS trigger AS $$
		begin
		  new.tsv := to_tsvector('english', coalesce(new.content, ''));
		  return new;
		end
		$$ LANGUAGE plpgsql;`,

		`DROP TRIGGER IF EXISTS tsv_update ON chunks;`,

		`CREATE TRIGGER tsv_update
		BEFORE INSERT OR UPDATE ON chunks
		FOR EACH ROW
		EXECUTE FUNCTION chunks_tsv_trigger();`,

		`UPDATE chunks
		SET tsv = to_tsvector('english', coalesce(content, ''))
		WHERE tsv IS NULL;`,
	}

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}
