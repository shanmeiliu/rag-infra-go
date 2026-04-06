package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type LocalEmbeddingClient struct {
	url     string
	apiKey  string
	model   string
	dim     int
	client  *http.Client
	cb      *CircuitBreaker
	httpCfg HTTPSettings
}

func NewLocalEmbeddingClient(cfg ProviderConfig) *LocalEmbeddingClient {
	httpCfg := cfg.HTTPSettings()

	return &LocalEmbeddingClient{
		url:     cfg.LocalEmbeddingURL,
		apiKey:  cfg.LocalEmbeddingAPIKey,
		model:   cfg.LocalEmbeddingModel,
		dim:     cfg.LocalEmbeddingDimOverride,
		client:  &http.Client{Timeout: httpCfg.Timeout},
		cb:      NewCircuitBreaker(httpCfg.CircuitThreshold, httpCfg.CircuitCooldown),
		httpCfg: httpCfg,
	}
}

func (c *LocalEmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
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
			if c.dim == 0 {
				c.dim = len(emb)
			}
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

func (c *LocalEmbeddingClient) doEmbedRequest(ctx context.Context, data []byte) ([]float32, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

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
		return nil, fmt.Errorf("local embeddings failed: %s - %s", resp.Status, raw.String())
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Embedding []float32 `json:"embedding"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Data) > 0 && len(result.Data[0].Embedding) > 0 {
		return result.Data[0].Embedding, nil
	}
	if len(result.Embedding) > 0 {
		return result.Embedding, nil
	}

	return nil, fmt.Errorf("no embedding returned")
}

func (c *LocalEmbeddingClient) Dimension() int       { return c.dim }
func (c *LocalEmbeddingClient) SetDimension(dim int) { c.dim = dim }
func (c *LocalEmbeddingClient) ProviderName() string { return "local" }
func (c *LocalEmbeddingClient) ModelName() string    { return c.model }