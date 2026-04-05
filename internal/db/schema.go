package db

import (
	"context"
	"database/sql"
)

func EnsureSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
	CREATE EXTENSION IF NOT EXISTS vector;
	CREATE TABLE IF NOT EXISTS chunks (
		id SERIAL PRIMARY KEY,
		content TEXT,
		embedding VECTOR(1536)
	);
	`)
	return err
}