package retrieval

import (
	"context"

	"github.com/shanmeiliu/rag-infra-go/internal/chat"
	"github.com/shanmeiliu/rag-infra-go/pkg/embedding"
	"github.com/shanmeiliu/rag-infra-go/pkg/vectorstore"
)

type PGVectorRetriever struct {
	embedder embedding.Client
	store    vectorstore.Store
	topK     int
}

func NewPGVectorRetriever(embedder embedding.Client, store vectorstore.Store, topK int) *PGVectorRetriever {
	if topK <= 0 {
		topK = 5
	}

	return &PGVectorRetriever{
		embedder: embedder,
		store:    store,
		topK:     topK,
	}
}

func (r *PGVectorRetriever) Retrieve(ctx context.Context, query string) ([]chat.Document, error) {
	queryEmbedding, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	results, err := r.store.Search(ctx, queryEmbedding, r.topK, nil)
	if err != nil {
		return nil, err
	}

	docs := make([]chat.Document, 0, len(results))
	for _, res := range results {
		docs = append(docs, chat.Document{
			ID:      res.ChunkID,
			Content: res.Content,
			Source:  res.DocID,
		})
	}

	return docs, nil
}
