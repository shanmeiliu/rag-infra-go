package providers

import (
	"os"
	"strconv"
)

type ProviderConfig struct {
	EmbeddingProvider string

	OpenAIAPIKey         string
	OpenAIBaseURL        string
	OpenAIEmbeddingModel string

	LocalEmbeddingURL  string
	LocalEmbeddingAPIKey string
	LocalEmbeddingModel string

	LocalEmbeddingDimOverride int
	DisableEmbeddingProbe     bool
}

func LoadProviderConfig() ProviderConfig {
	return ProviderConfig{
		EmbeddingProvider:        getEnv("EMBEDDING_PROVIDER", "openai"),

		OpenAIAPIKey:             os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL:            getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIEmbeddingModel:     getEnv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small"),

		LocalEmbeddingURL:        getEnv("LOCAL_EMBEDDING_URL", ""),
		LocalEmbeddingAPIKey:     os.Getenv("LOCAL_EMBEDDING_API_KEY"),
		LocalEmbeddingModel:      getEnv("LOCAL_EMBEDDING_MODEL", ""),

		LocalEmbeddingDimOverride: getEnvInt("LOCAL_EMBEDDING_DIM", 0),
		DisableEmbeddingProbe:     getEnvBool("DISABLE_EMBEDDING_PROBE", false),
	}
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "TRUE", "yes", "YES":
		return true
	case "0", "false", "FALSE", "no", "NO":
		return false
	default:
		return fallback
	}
}