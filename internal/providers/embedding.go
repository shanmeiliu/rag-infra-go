package providers

import "context"

type MockEmbeddingClient struct{}

func NewMockEmbeddingClient() *MockEmbeddingClient {
	return &MockEmbeddingClient{}
}

func (c *MockEmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}
