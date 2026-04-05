package vectorstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shanmeiliu/rag-infra-go/internal/providers"
	pkgvector "github.com/shanmeiliu/rag-infra-go/pkg/vectorstore"
)

type PGVectorStore struct {
	db      *sql.DB
	profile providers.EmbeddingProfile
}

func NewPGVectorStore(db *sql.DB, profile providers.EmbeddingProfile) *PGVectorStore {
	return &PGVectorStore{
		db:      db,
		profile: profile,
	}
}

func (s *PGVectorStore) Upsert(ctx context.Context, chunks []pkgvector.Chunk) error {
	for _, ch := range chunks {
		metaJSON, err := json.Marshal(ch.Metadata)
		if err != nil {
			return err
		}

		if _, err := s.db.ExecContext(ctx, `
INSERT INTO chunks (chunk_id, doc_id, content, metadata)
VALUES ($1, $2, $3, $4::jsonb)
ON CONFLICT (chunk_id)
DO UPDATE SET
	doc_id = EXCLUDED.doc_id,
	content = EXCLUDED.content,
	metadata = EXCLUDED.metadata;
`, ch.ChunkID, ch.DocID, ch.Content, string(metaJSON)); err != nil {
			return err
		}

		query := fmt.Sprintf(`
INSERT INTO %s (chunk_id, embedding)
VALUES ($1, $2::vector)
ON CONFLICT (chunk_id)
DO UPDATE SET
	embedding = EXCLUDED.embedding;
`, s.profile.TableName())

		if _, err := s.db.ExecContext(ctx, query, ch.ChunkID, toVectorLiteral(ch.Embedding)); err != nil {
			return err
		}
	}

	return nil
}

func (s *PGVectorStore) Search(ctx context.Context, embedding []float32, topK int, filters map[string]any) ([]pkgvector.SearchResult, error) {
	if topK <= 0 {
		topK = 5
	}

	query := fmt.Sprintf(`
SELECT
	c.chunk_id,
	c.doc_id,
	c.content,
	c.metadata,
	e.embedding <-> $1::vector AS score
FROM %s e
JOIN chunks c ON c.chunk_id = e.chunk_id
ORDER BY e.embedding <-> $1::vector
LIMIT $2;
`, s.profile.TableName())

	rows, err := s.db.QueryContext(ctx, query, toVectorLiteral(embedding), topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []pkgvector.SearchResult
	for rows.Next() {
		var r pkgvector.SearchResult
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

func (s *PGVectorStore) DeleteAll(ctx context.Context) error {
	query := fmt.Sprintf(`TRUNCATE TABLE %s;`, s.profile.TableName())
	_, err := s.db.ExecContext(ctx, query)
	return err
}

func toVectorLiteral(v []float32) string {
	parts := make([]string, len(v))
	for i, x := range v {
		parts[i] = fmt.Sprintf("%f", x)
	}
	return "[" + strings.Join(parts, ",") + "]"
}