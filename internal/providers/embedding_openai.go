package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type OpenAIEmbeddingClient struct {
	apiKey  string
	baseURL string
	model   string
	dim     int
	client  *http.Client
	cb      *CircuitBreaker
	httpCfg HTTPSettings
}

func NewOpenAIEmbeddingClient(cfg ProviderConfig) *OpenAIEmbeddingClient {
	httpCfg := cfg.HTTPSettings()

	return &OpenAIEmbeddingClient{
		apiKey:  cfg.OpenAIAPIKey,
		baseURL: cfg.OpenAIBaseURL,
		model:   cfg.OpenAIEmbeddingModel,
		dim:     1536,
		client:  &http.Client{Timeout: httpCfg.Timeout},
		cb:      NewCircuitBreaker(httpCfg.CircuitThreshold, httpCfg.CircuitCooldown),
		httpCfg: httpCfg,
	}
}

func (c *OpenAIEmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	if err := c.cb.Allow(); err != nil {
		return nil, err
	}

	reqBody := map[string]any{
		"model": c.model,
		"input": text,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	var lastErr error

	for attempt := 0; attempt <= c.httpCfg.MaxRetries; attempt++ {
		emb, err := c.doEmbedRequest(ctx, data)
		if err == nil {
			c.cb.RecordSuccess()
			return emb, nil
		}

		lastErr = err

		if attempt == c.httpCfg.MaxRetries || !shouldRetryError(err) {
			break
		}

		delay := c.httpCfg.RetryBaseDelay * time.Duration(attempt+1)
		if sleepErr := sleepWithContext(ctx, delay); sleepErr != nil {
			c.cb.RecordFailure()
			return nil, sleepErr
		}
	}

	c.cb.RecordFailure()
	return nil, lastErr
}

func (c *OpenAIEmbeddingClient) doEmbedRequest(ctx context.Context, data []byte) ([]float32, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if shouldRetryStatus(resp.StatusCode) {
		return nil, fmt.Errorf("retryable embedding request failure: %s", resp.Status)
	}
	if resp.StatusCode >= 300 {
		var raw bytes.Buffer
		_, _ = raw.ReadFrom(resp.Body)
		return nil, fmt.Errorf("openai embeddings failed: %s - %s", resp.Status, raw.String())
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return result.Data[0].Embedding, nil
}

func (c *OpenAIEmbeddingClient) Dimension() int       { return c.dim }
func (c *OpenAIEmbeddingClient) SetDimension(dim int) { c.dim = dim }
func (c *OpenAIEmbeddingClient) ProviderName() string { return "openai" }
func (c *OpenAIEmbeddingClient) ModelName() string    { return c.model }
