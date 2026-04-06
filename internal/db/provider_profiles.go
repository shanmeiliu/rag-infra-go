package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/shanmeiliu/rag-infra-go/internal/providers"
)

func EnsureProfileSchema(ctx context.Context, db *sql.DB) error {
	query := `
CREATE TABLE IF NOT EXISTS embedding_profiles (
    id BIGSERIAL PRIMARY KEY,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    dimension INTEGER NOT NULL,
    storage_key TEXT UNIQUE NOT NULL,
    table_name TEXT UNIQUE NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	_, err := db.ExecContext(ctx, query)
	return err
}

func UpsertEmbeddingProfile(ctx context.Context, db *sql.DB, profile providers.EmbeddingProfile, setActive bool) error {
	query := `
INSERT INTO embedding_profiles (
    provider, model, dimension, storage_key, table_name, is_active, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
ON CONFLICT (storage_key)
DO UPDATE SET
    provider = EXCLUDED.provider,
    model = EXCLUDED.model,
    dimension = EXCLUDED.dimension,
    table_name = EXCLUDED.table_name,
    is_active = EXCLUDED.is_active,
    updated_at = NOW();
`
	if setActive {
		if _, err := db.ExecContext(ctx, `UPDATE embedding_profiles SET is_active = FALSE;`); err != nil {
			return err
		}
	}

	_, err := db.ExecContext(
		ctx,
		query,
		profile.Provider,
		profile.Model,
		profile.Dimension,
		profile.StorageKey,
		profile.TableName(),
		setActive,
	)
	return err
}

func GetEmbeddingProfileByStorageKey(ctx context.Context, db *sql.DB, storageKey string) (*providers.EmbeddingProfile, error) {
	query := `
SELECT provider, model, dimension, storage_key
FROM embedding_profiles
WHERE storage_key = $1
LIMIT 1;
`
	var provider, model, key string
	var dimension int

	err := db.QueryRowContext(ctx, query, storageKey).Scan(&provider, &model, &dimension, &key)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	profile := providers.NewEmbeddingProfile(provider, model, dimension)
	if profile.StorageKey != key {
		return nil, fmt.Errorf("storage key mismatch for profile %s", storageKey)
	}

	return &profile, nil
}

func ListEmbeddingProfiles(ctx context.Context, db *sql.DB) ([]providers.EmbeddingProfile, error) {
	query := `
SELECT provider, model, dimension, storage_key
FROM embedding_profiles
ORDER BY updated_at DESC;
`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []providers.EmbeddingProfile
	for rows.Next() {
		var provider, model, key string
		var dimension int
		if err := rows.Scan(&provider, &model, &dimension, &key); err != nil {
			return nil, err
		}
		p := providers.NewEmbeddingProfile(provider, model, dimension)
		out = append(out, p)
	}
	return out, rows.Err()
}

var _ = time.Now
