package ingestion

import (
	"context"
	"fmt"

	"github.com/shanmeiliu/rag-infra-go/pkg/embedding"
	"github.com/shanmeiliu/rag-infra-go/pkg/vectorstore"
)

type Service struct {
	embedder embedding.Client
	store    vectorstore.Store
}

type InputChunk struct {
	ChunkID  string         `json:"chunk_id"`
	DocID    string         `json:"doc_id"`
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata"`
}

func NewService(embedder embedding.Client, store vectorstore.Store) *Service {
	return &Service{
		embedder: embedder,
		store:    store,
	}
}

func (s *Service) Ingest(ctx context.Context, chunks []InputChunk) error {
	if len(chunks) == 0 {
		return fmt.Errorf("no chunks provided")
	}

	items := make([]vectorstore.Chunk, 0, len(chunks))
	for _, ch := range chunks {
		if ch.ChunkID == "" {
			return fmt.Errorf("chunk_id is required")
		}
		if ch.DocID == "" {
			return fmt.Errorf("doc_id is required")
		}
		if ch.Content == "" {
			return fmt.Errorf("content is required")
		}

		emb, err := s.embedder.Embed(ctx, ch.Content)
		if err != nil {
			return fmt.Errorf("embed chunk %s: %w", ch.ChunkID, err)
		}

		items = append(items, vectorstore.Chunk{
			ChunkID:   ch.ChunkID,
			DocID:     ch.DocID,
			Content:   ch.Content,
			Metadata:  ch.Metadata,
			Embedding: emb,
		})
	}

	return s.store.Upsert(ctx, items)
}
