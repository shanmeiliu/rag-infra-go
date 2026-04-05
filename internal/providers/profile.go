package providers

import (
	"fmt"
	"regexp"
	"strings"
)

type EmbeddingProfile struct {
	Provider   string
	Model      string
	Dimension  int
	StorageKey string
}

func NewEmbeddingProfile(provider, model string, dim int) EmbeddingProfile {
	return EmbeddingProfile{
		Provider:   provider,
		Model:      model,
		Dimension:  dim,
		StorageKey: sanitizeStorageKey(provider + "_" + model),
	}
}

func (p EmbeddingProfile) TableName() string {
	return "chunk_embeddings_" + p.StorageKey
}

func sanitizeStorageKey(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	re := regexp.MustCompile(`[^a-z0-9_]+`)
	s = re.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	return s
}

func (p EmbeddingProfile) Validate() error {
	if p.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if p.Model == "" {
		return fmt.Errorf("model is required")
	}
	if p.Dimension <= 0 {
		return fmt.Errorf("dimension must be > 0")
	}
	if p.StorageKey == "" {
		return fmt.Errorf("storage key is required")
	}
	return nil
}