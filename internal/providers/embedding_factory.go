package providers

import (
	"fmt"

	"github.com/shanmeiliu/rag-infra-go/pkg/embedding"
)

func NewEmbeddingClient(cfg ProviderConfig) (embedding.Client, error) {
	switch cfg.EmbeddingProvider {
	case "openai":
		return NewOpenAIEmbeddingClient(cfg), nil
	case "local":
		return NewLocalEmbeddingClient(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", cfg.EmbeddingProvider)
	}
}