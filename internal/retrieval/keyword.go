package retrieval

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

type KeywordResult struct {
	ChunkID  string
	DocID    string
	Content  string
	Metadata map[string]any
	Score    float64
}

func KeywordSearch(ctx context.Context, db *sql.DB, query string, limit int, filters map[string]any) ([]KeywordResult, error) {
	sqlText := `
SELECT
	chunk_id,
	doc_id,
	content,
	metadata,
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
			if k == "source_groups" {
				groups := normalizeKeywordStringSlice(v)
				if len(groups) > 0 {
					placeholders := make([]string, 0, len(groups))
					for _, group := range groups {
						placeholders = append(placeholders, fmt.Sprintf("$%d", nextArg))
						args = append(args, group)
						nextArg++
					}
					whereParts = append(whereParts, fmt.Sprintf("metadata->>'source_group' IN (%s)", strings.Join(placeholders, ",")))
				}
				continue
			}

			if k == "source_group" {
				group := strings.TrimSpace(fmt.Sprintf("%v", v))
				if group != "" {
					whereParts = append(whereParts, fmt.Sprintf("metadata->>'source_group' = $%d", nextArg))
					args = append(args, group)
					nextArg++
				}
				continue
			}

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
		var metadataBytes []byte

		if err := rows.Scan(&r.ChunkID, &r.DocID, &r.Content, &metadataBytes, &r.Score); err != nil {
			return nil, err
		}

		if len(metadataBytes) > 0 {
			_ = json.Unmarshal(metadataBytes, &r.Metadata)
		}

		results = append(results, r)
	}

	return results, rows.Err()
}

func normalizeKeywordStringSlice(v any) []string {
	var out []string

	switch raw := v.(type) {
	case []string:
		for _, item := range raw {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
	case []any:
		for _, item := range raw {
			s := strings.TrimSpace(fmt.Sprintf("%v", item))
			if s != "" {
				out = append(out, s)
			}
		}
	case string:
		for _, item := range strings.Split(raw, ",") {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
	}

	return out
}

func escapeKeywordField(s string) string {
	s = strings.ReplaceAll(s, `'`, `''`)
	return s
}
