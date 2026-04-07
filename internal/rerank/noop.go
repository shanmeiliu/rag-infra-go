package rerank

import (
	"context"

	"github.com/shanmeiliu/rag-infra-go/pkg/reranker"
)

type NoopClient struct{}

func NewNoopClient() *NoopClient {
	return &NoopClient{}
}

func (c *NoopClient) Rerank(ctx context.Context, query string, docs []reranker.Candidate, topK int) ([]reranker.Candidate, error) {
	if topK <= 0 || topK >= len(docs) {
		return docs, nil
	}
	return docs[:topK], nil
}
