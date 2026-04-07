package rerank

import (
	"os"
	"strconv"
)

type Config struct {
	Provider string
	URL      string
	APIKey   string
	Model    string
	TopK     int
}

func LoadConfig() Config {
	return Config{
		Provider: getEnv("RERANKER_PROVIDER", "none"),
		URL:      getEnv("RERANKER_URL", ""),
		APIKey:   os.Getenv("RERANKER_API_KEY"),
		Model:    getEnv("RERANKER_MODEL", ""),
		TopK:     getEnvInt("RERANKER_TOP_K", 5),
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
