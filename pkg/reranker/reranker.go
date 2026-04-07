package reranker

import "context"

type Candidate struct {
	ID      string
	DocID   string
	Content string
	Score   float64
}

type Client interface {
	Rerank(ctx context.Context, query string, docs []Candidate, topK int) ([]Candidate, error)
}
