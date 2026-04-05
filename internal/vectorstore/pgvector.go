package vectorstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shanmeiliu/rag-infra-go/pkg/vectorstore"
)

type PGVectorStore struct {
	db *sql.DB
}

func NewPGVectorStore(db *sql.DB) *PGVectorStore {
	return &PGVectorStore{db: db}
}

func (s *PGVectorStore) Upsert(ctx context.Context, chunks []vectorstore.Chunk) error {
	for _, ch := range chunks {
		metadataJSON, err := json.Marshal(ch.Metadata)
		if err != nil {
			return err
		}

		query := `
INSERT INTO chunks (chunk_id, doc_id, content, metadata, embedding)
VALUES ($1, $2, $3, $4::jsonb, $5::vector)
ON CONFLICT (chunk_id)
DO UPDATE SET
	content = EXCLUDED.content,
	metadata = EXCLUDED.metadata,
	embedding = EXCLUDED.embedding;
`
		_, err = s.db.ExecContext(
			ctx,
			query,
			ch.ChunkID,
			ch.DocID,
			ch.Content,
			string(metadataJSON),
			toVectorLiteral(ch.Embedding),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *PGVectorStore) Search(ctx context.Context, embedding []float32, topK int, filters map[string]any) ([]vectorstore.SearchResult, error) {
	if topK <= 0 {
		topK = 5
	}

	baseQuery := `
SELECT chunk_id, doc_id, content, metadata, embedding <-> $1::vector AS score
FROM chunks
`
	args := []any{toVectorLiteral(embedding)}
	whereParts := make([]string, 0)

	if len(filters) > 0 {
		for key, value := range filters {
			args = append(args, fmt.Sprintf("%v", value))
			whereParts = append(whereParts, fmt.Sprintf("metadata->>$%d = $%d", len(args), len(args)))
			_ = key
		}
	}

	if len(whereParts) > 0 {
		baseQuery += " WHERE " + strings.Join(whereParts, " AND ")
	}

	args = append(args, topK)
	baseQuery += fmt.Sprintf(" ORDER BY embedding <-> $1::vector LIMIT $%d", len(args))

	rows, err := s.db.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]vectorstore.SearchResult, 0)
	for rows.Next() {
		var r vectorstore.SearchResult
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

func toVectorLiteral(v []float32) string {
	parts := make([]string, len(v))
	for i, x := range v {
		parts[i] = fmt.Sprintf("%f", x)
	}
	return "[" + strings.Join(parts, ",") + "]"
}