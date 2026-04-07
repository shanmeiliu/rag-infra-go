package retrieval

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type KeywordResult struct {
	ChunkID string
	DocID   string
	Content string
	Score   float64
}

func KeywordSearch(ctx context.Context, db *sql.DB, query string, limit int, filters map[string]any) ([]KeywordResult, error) {
	sqlText := `
SELECT
    chunk_id,
    doc_id,
    content,
    ts_rank_cd(tsv, plainto_tsquery('english', $1)) AS score
FROM chunks
`
	args := []any{query}
	whereParts := []string{
		"tsv @@ plainto_tsquery('english', $1)",
	}

	nextArg := 2

	if docID, ok := filters["doc_id"].(string); ok && strings.TrimSpace(docID) != "" {
		whereParts = append(whereParts, fmt.Sprintf("doc_id = $%d", nextArg))
		args = append(args, docID)
		nextArg++
	}

	if metadataRaw, ok := filters["metadata"].(map[string]any); ok {
		for k, v := range metadataRaw {
			whereParts = append(whereParts, fmt.Sprintf("metadata->>'%s' = $%d", escapeKeywordField(k), nextArg))
			args = append(args, fmt.Sprintf("%v", v))
			nextArg++
		}
	}

	sqlText += " WHERE " + strings.Join(whereParts, " AND ")
	sqlText += fmt.Sprintf(" ORDER BY score DESC LIMIT $%d", nextArg)
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, sqlText, args...)
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

func escapeKeywordField(s string) string {
	s = strings.ReplaceAll(s, `'`, `''`)
	return s
}
