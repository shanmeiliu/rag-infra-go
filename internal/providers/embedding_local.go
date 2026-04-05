package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type LocalEmbeddingClient struct {
	baseURL string
	model   string
	dim     int
}

func NewLocalEmbeddingClient(cfg ProviderConfig) *LocalEmbeddingClient {
	return &LocalEmbeddingClient{
		baseURL: cfg.LocalEmbeddingBaseURL,
		model:   cfg.LocalEmbeddingModel,
		dim:     768,
	}
}

func (c *LocalEmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]any{
		"model":  c.model,
		"prompt": text,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embeddings", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("local embeddings failed with status %s", resp.Status)
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return result.Embedding, nil
}

func (c *LocalEmbeddingClient) Dimension() int {
	return c.dim
}