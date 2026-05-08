package sources

import (
	"context"
	"fmt"
)

type SourceStats struct {
	ChunkCount int `json:"chunk_count"`
}

func (r *Repository) GetSourceStats(
	ctx context.Context,
	sourceKey string,
) (*SourceStats, error) {
	query := `
SELECT COUNT(*)
FROM chunks
WHERE metadata->>'source_id' = $1
`

	var count int

	if err := r.db.QueryRowContext(
		ctx,
		query,
		sourceKey,
	).Scan(&count); err != nil {
		return nil, fmt.Errorf("count source chunks: %w", err)
	}

	return &SourceStats{
		ChunkCount: count,
	}, nil
}
