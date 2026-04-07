package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/shanmeiliu/rag-infra-go/internal/providers"
)

func EnsureEmbeddingTable(
	ctx context.Context,
	db *sql.DB,
	profile providers.EmbeddingProfile,
	enableHNSW bool,
) error {
	if err := profile.Validate(); err != nil {
		return err
	}

	tableName := profile.TableName()
	indexName := tableName + "_hnsw_idx"

	createTableQuery := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    chunk_id TEXT PRIMARY KEY REFERENCES chunks(chunk_id) ON DELETE CASCADE,
    embedding VECTOR(%d) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`, tableName, profile.Dimension)

	if _, err := db.ExecContext(ctx, createTableQuery); err != nil {
		return err
	}

	// 👇 Only create HNSW index if enabled
	if enableHNSW {
		createIndexQuery := fmt.Sprintf(`
CREATE INDEX IF NOT EXISTS %s
ON %s
USING hnsw (embedding vector_l2_ops);`, indexName, tableName)

		if _, err := db.ExecContext(ctx, createIndexQuery); err != nil {
			return err
		}
	}

	return nil
}
