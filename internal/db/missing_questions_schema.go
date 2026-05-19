package db

import (
	"context"
	"database/sql"
)

func EnsureMissingQuestionsSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS missing_questions (
	id BIGSERIAL PRIMARY KEY,
	session_id TEXT,
	mode TEXT,
	question TEXT NOT NULL,
	rewritten_query TEXT,
	reason TEXT NOT NULL DEFAULT 'no_relevant_chunks',
	filters JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_missing_questions_created_at
	ON missing_questions(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_missing_questions_reason
	ON missing_questions(reason);
`)
	return err
}
