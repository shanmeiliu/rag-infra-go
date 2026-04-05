package retrieval

import (
	"context"

	"github.com/yourname/rag-infra-go/internal/chat"
)

type MockRetriever struct{}

func NewMockRetriever() *MockRetriever {
	return &MockRetriever{}
}

func (r *MockRetriever) Retrieve(ctx context.Context, query string) ([]chat.Document, error) {
	return []chat.Document{
		{
			ID:      "doc-1",
			Source:  "knowledge-base",
			Content: "RAG combines retrieval with generation by fetching relevant context before prompting the model.",
		},
		{
			ID:      "doc-2",
			Source:  "architecture-notes",
			Content: "A production RAG system usually includes query rewriting, retrieval, reranking, memory, and model fallback.",
		},
	}, nil
}
