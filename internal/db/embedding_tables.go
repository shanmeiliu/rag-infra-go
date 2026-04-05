package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/shanmeiliu/rag-infra-go/internal/providers"
)

func EnsureEmbeddingTable(ctx context.Context, db *sql.DB, profile providers.EmbeddingProfile) error {
	if err := profile.Validate(); err != nil {
		return err
	}

	query := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    chunk_id TEXT PRIMARY KEY REFERENCES chunks(chunk_id) ON DELETE CASCADE,
    embedding VECTOR(%d) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`, profile.TableName(), profile.Dimension)

	_, err := db.ExecContext(ctx, query)
	return err
}