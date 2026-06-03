package providers

import (
	"os"
	"strconv"
	"time"
)

type ProviderConfig struct {
	EmbeddingProvider    string
	EnableHNSWIndex      bool
	OpenAIAPIKey         string
	OpenAIBaseURL        string
	OpenAIEmbeddingModel string

	LocalEmbeddingURL    string
	LocalEmbeddingAPIKey string
	LocalEmbeddingModel  string

	LocalEmbeddingDimOverride int
	DisableEmbeddingProbe     bool

	HTTPTimeoutSeconds     int
	HTTPMaxRetries         int
	HTTPRetryBaseDelayMs   int
	HTTPCircuitThreshold   int
	HTTPCircuitCooldownSec int

	LLMFallbackEnabled   bool
	LLMFallbackBaseURL   string
	LLMFallbackAPIKey    string
	LLMFallbackChatModel string
}

func LoadProviderConfig() ProviderConfig {
	return ProviderConfig{
		EmbeddingProvider:    getEnv("EMBEDDING_PROVIDER", "openai"),
		EnableHNSWIndex:      getEnvBool("ENABLE_HNSW_INDEX", true),
		OpenAIAPIKey:         os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL:        getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIEmbeddingModel: getEnv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small"),

		LocalEmbeddingURL:    getEnv("LOCAL_EMBEDDING_URL", ""),
		LocalEmbeddingAPIKey: os.Getenv("LOCAL_EMBEDDING_API_KEY"),
		LocalEmbeddingModel:  getEnv("LOCAL_EMBEDDING_MODEL", ""),

		LocalEmbeddingDimOverride: getEnvInt("LOCAL_EMBEDDING_DIM", 0),
		DisableEmbeddingProbe:     getEnvBool("DISABLE_EMBEDDING_PROBE", false),

		HTTPTimeoutSeconds:     getEnvInt("HTTP_TIMEOUT_SECONDS", 20),
		HTTPMaxRetries:         getEnvInt("HTTP_MAX_RETRIES", 2),
		HTTPRetryBaseDelayMs:   getEnvInt("HTTP_RETRY_BASE_DELAY_MS", 500),
		HTTPCircuitThreshold:   getEnvInt("HTTP_CIRCUIT_THRESHOLD", 3),
		HTTPCircuitCooldownSec: getEnvInt("HTTP_CIRCUIT_COOLDOWN_SECONDS", 30),

		LLMFallbackEnabled:   getEnvBool("LLM_FALLBACK_ENABLED", false),
		LLMFallbackBaseURL:   getEnv("LLM_FALLBACK_BASE_URL", ""),
		LLMFallbackAPIKey:    getEnv("LLM_FALLBACK_API_KEY", ""),
		LLMFallbackChatModel: getEnv("LLM_FALLBACK_CHAT_MODEL", ""),
	}
}

func (c ProviderConfig) HTTPSettings() HTTPSettings {
	return HTTPSettings{
		Timeout:          time.Duration(maxInt(c.HTTPTimeoutSeconds, 20)) * time.Second,
		MaxRetries:       maxInt(c.HTTPMaxRetries, 2),
		RetryBaseDelay:   time.Duration(maxInt(c.HTTPRetryBaseDelayMs, 500)) * time.Millisecond,
		CircuitThreshold: maxInt(c.HTTPCircuitThreshold, 3),
		CircuitCooldown:  time.Duration(maxInt(c.HTTPCircuitCooldownSec, 30)) * time.Second,
	}
}

func maxInt(v int, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
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
