package providers

import (
	"context"
	"hash/fnv"
)

type MockEmbeddingClient struct {
	dim int
}

func NewMockEmbeddingClient() *MockEmbeddingClient {
	return &MockEmbeddingClient{dim: 1536}
}

func (c *MockEmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	out := make([]float32, c.dim)

	h := fnv.New32a()
	_, _ = h.Write([]byte(text))
	seed := h.Sum32()

	for i := 0; i < c.dim; i++ {
		v := float32((int(seed)+i*31)%1000) / 1000.0
		out[i] = v
	}

	return out, nil
}
