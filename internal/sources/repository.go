package sources

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

var ErrSourceNotFound = errors.New("source not found")

type Source struct {
	ID              int64          `json:"id"`
	SourceKey       string         `json:"source_key"`
	Name            string         `json:"name"`
	SourceType      string         `json:"source_type"`
	Status          string         `json:"status"`
	Origin          *string        `json:"origin,omitempty"`
	FilePath        *string        `json:"file_path,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	CreatedByUserID *int64         `json:"created_by_user_id,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, s *Source) (int64, error) {
	metaJSON, err := json.Marshal(s.Metadata)
	if err != nil {
		return 0, err
	}

	row := r.db.QueryRowContext(ctx, `
INSERT INTO sources (
	source_key, name, source_type, status, origin, file_path, metadata, created_by_user_id
)
VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8)
RETURNING id
`,
		s.SourceKey,
		s.Name,
		s.SourceType,
		s.Status,
		s.Origin,
		s.FilePath,
		string(metaJSON),
		s.CreatedByUserID,
	)

	var id int64
	if err := row.Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*Source, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT
	id, source_key, name, source_type, status, origin, file_path, metadata, created_by_user_id, created_at, updated_at
FROM sources
WHERE id = $1
LIMIT 1
`, id)

	var s Source
	var metadataBytes []byte

	err := row.Scan(
		&s.ID,
		&s.SourceKey,
		&s.Name,
		&s.SourceType,
		&s.Status,
		&s.Origin,
		&s.FilePath,
		&metadataBytes,
		&s.CreatedByUserID,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSourceNotFound
		}
		return nil, err
	}

	if len(metadataBytes) > 0 {
		_ = json.Unmarshal(metadataBytes, &s.Metadata)
	}

	return &s, nil
}

func (r *Repository) List(ctx context.Context, limit int) ([]Source, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT
	id, source_key, name, source_type, status, origin, file_path, metadata, created_by_user_id, created_at, updated_at
FROM sources
ORDER BY created_at DESC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Source
	for rows.Next() {
		var s Source
		var metadataBytes []byte

		if err := rows.Scan(
			&s.ID,
			&s.SourceKey,
			&s.Name,
			&s.SourceType,
			&s.Status,
			&s.Origin,
			&s.FilePath,
			&metadataBytes,
			&s.CreatedByUserID,
			&s.CreatedAt,
			&s.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if len(metadataBytes) > 0 {
			_ = json.Unmarshal(metadataBytes, &s.Metadata)
		}

		out = append(out, s)
	}

	return out, rows.Err()
}

func (r *Repository) TouchUpdatedAt(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE sources
SET updated_at = NOW()
WHERE id = $1
`, id)
	return err
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM sources WHERE id = $1`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return ErrSourceNotFound
	}
	return err
}
