package retrieval

import (
	"context"
	"database/sql"
)

type KeywordResult struct {
	ChunkID string
	DocID   string
	Content string
	Score   float64
}

func KeywordSearch(ctx context.Context, db *sql.DB, query string, limit int) ([]KeywordResult, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
    chunk_id,
    doc_id,
    content,
    ts_rank_cd(tsv, plainto_tsquery('english', $1)) AS score
FROM chunks
WHERE tsv @@ plainto_tsquery('english', $1)
ORDER BY score DESC
LIMIT $2;
`, query, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []KeywordResult

	for rows.Next() {
		var r KeywordResult
		if err := rows.Scan(&r.ChunkID, &r.DocID, &r.Content, &r.Score); err != nil {
			return nil, err
		}
		results = append(results, r)
	}

	return results, rows.Err()
}