package vectorstore

import "context"

type Chunk struct {
	ChunkID   string
	DocID     string
	Content   string
	Metadata  map[string]any
	Embedding []float32
}

type SearchResult struct {
	ChunkID  string
	DocID    string
	Content  string
	Metadata map[string]any
	Score    float64
}

type Store interface {
	Upsert(ctx context.Context, chunks []Chunk) error
	Search(ctx context.Context, embedding []float32, topK int, filters map[string]any) ([]SearchResult, error)
}