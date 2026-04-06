package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type LocalEmbeddingClient struct {
	url   string
	apiKey string
	model string
	dim   int
}

func NewLocalEmbeddingClient(cfg ProviderConfig) *LocalEmbeddingClient {
	return &LocalEmbeddingClient{
		url:   cfg.LocalEmbeddingURL,
		apiKey: cfg.LocalEmbeddingAPIKey,
		model: cfg.LocalEmbeddingModel,
		dim:   cfg.LocalEmbeddingDimOverride,
	}
}

func (c *LocalEmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]any{
		"model": c.model,
		"input": text,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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
		emb := result.Data[0].Embedding
		if c.dim == 0 {
			c.dim = len(emb)
		}
		return emb, nil
	}

	if len(result.Embedding) > 0 {
		emb := result.Embedding
		if c.dim == 0 {
			c.dim = len(emb)
		}
		return emb, nil
	}

	return nil, fmt.Errorf("no embedding returned")
}

func (c *LocalEmbeddingClient) Dimension() int       { return c.dim }
func (c *LocalEmbeddingClient) SetDimension(dim int) { c.dim = dim }
func (c *LocalEmbeddingClient) ProviderName() string { return "local" }
func (c *LocalEmbeddingClient) ModelName() string    { return c.model }