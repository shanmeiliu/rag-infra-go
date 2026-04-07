package rerank

import (
	"fmt"

	"github.com/shanmeiliu/rag-infra-go/pkg/reranker"
)

func NewClient(cfg Config) (reranker.Client, error) {
	switch cfg.Provider {
	case "", "none":
		return NewNoopClient(), nil
	case "http":
		if cfg.URL == "" {
			return nil, fmt.Errorf("RERANKER_URL is required when RERANKER_PROVIDER=http")
		}
		return NewHTTPClient(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported reranker provider: %s", cfg.Provider)
	}
}
