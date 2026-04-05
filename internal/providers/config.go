package providers

import "os"

type ProviderConfig struct {
	EmbeddingProvider   string
	OpenAIAPIKey        string
	OpenAIBaseURL       string
	OpenAIEmbeddingModel string
	LocalEmbeddingBaseURL string
	LocalEmbeddingModel   string
}

func LoadProviderConfig() ProviderConfig {
	return ProviderConfig{
		EmbeddingProvider:     getEnv("EMBEDDING_PROVIDER", "openai"),
		OpenAIAPIKey:          os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL:         getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIEmbeddingModel:  getEnv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small"),
		LocalEmbeddingBaseURL: getEnv("LOCAL_EMBEDDING_BASE_URL", "http://localhost:11434"),
		LocalEmbeddingModel:   getEnv("LOCAL_EMBEDDING_MODEL", "nomic-embed-text"),
	}
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}